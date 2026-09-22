package lint

import (
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/project"
)

// Vault is one project's files indexed for link resolution, as Run sees them: every file
// under the folder, with the wiki's pages parsed for their aliases. The mirror uses it to
// rewrite a member's links, so a link means the same thing to the mirror and to lint.
type Vault struct {
	files    []string
	pages    map[string]*page
	resolver *resolver
}

// Link is one link a page holds: the target as written, without its alias, and the
// one file it resolves to, or "" when it names none or more than one.
type Link struct {
	Target   string `json:"target"`
	Resolved string `json:"resolved,omitempty"`
}

// PageInfo is what lint knows about one wiki page, for a caller that reads pages as
// lint reads them: the overlap report compares pages by these fields and never parses
// markdown itself.
type PageInfo struct {
	Path    string   `json:"path"`
	Title   string   `json:"title"`
	Type    string   `json:"type,omitempty"`
	Aliases []string `json:"aliases,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	// Headings are the page's headings, normalized, in path order.
	Headings []string `json:"headings,omitempty"`
	// Text is the body with the frontmatter, code, and comments blanked.
	Text   string         `json:"-"`
	Fields map[string]any `json:"-"`
	Links  []Link         `json:"links,omitempty"`
}

// LoadVault indexes the project folder at root.
func LoadVault(root string) (*Vault, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	files, err := walk(root)
	if err != nil {
		return nil, err
	}
	var targets []target
	pages := map[string]*page{}
	for _, rel := range files {
		t := target{path: rel}
		if strings.HasPrefix(rel, "wiki/") && strings.EqualFold(path.Ext(rel), ".md") {
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
				t.page = parsePage(rel, string(data))
				pages[rel] = t.page
			}
		}
		targets = append(targets, t)
	}
	return &Vault{files: files, pages: pages, resolver: newResolver(targets)}, nil
}

// Files lists the vault's files relative to its root, with slashes.
func (v *Vault) Files() []string { return append([]string(nil), v.files...) }

// Pages lists every wiki page in path order, with each link resolved as lint resolves it.
func (v *Vault) Pages() []PageInfo {
	var out []PageInfo
	for _, rel := range v.files {
		pg, ok := v.pages[rel]
		if !ok {
			continue
		}
		info := PageInfo{Path: rel, Title: project.StringField(pg.fields, "title"), Type: project.StringField(pg.fields, "type"), Aliases: pg.aliases, Tags: project.StringList(pg.fields, "tags"), Text: pg.masked, Fields: pg.fields}
		if info.Title == "" {
			info.Title = strings.TrimSuffix(path.Base(rel), path.Ext(rel))
		}
		for h := range pg.headings {
			info.Headings = append(info.Headings, h)
		}
		sort.Strings(info.Headings)
		for _, l := range pg.links {
			link := Link{Target: l.filePart}
			if found := v.resolver.resolve(l); len(found) == 1 {
				link.Resolved = found[0].path
			}
			info.Links = append(info.Links, link)
		}
		out = append(out, info)
	}
	return out
}

// NameKey is a name reduced to its lowercase letters and digits, so that two names that
// differ only in case, spacing, or punctuation compare equal.
func NameKey(name string) string { return string(nameKey(name)) }

// Near reports whether two names are the same name with a small typing difference, by
// the rule the wanted-page suggestions use: one edit for 5 to 8 letters and digits, two
// for more, and the same digits.
func Near(a, b string) bool {
	ka, kb := nameKey(a), nameKey(b)
	if len(ka) == 0 || len(kb) == 0 || digits(ka) != digits(kb) {
		return false
	}
	limit := 0
	switch {
	case len(ka) > 8:
		limit = 2
	case len(ka) > 4:
		limit = 1
	}
	if diff := len(ka) - len(kb); diff > limit || -diff > limit {
		return false
	}
	return editDistance(ka, kb) <= limit
}

// Resolve returns the one file a link names from the page at source, or "" when the link
// names none or more than one. target is the link as written, without alias.
func (v *Vault) Resolve(source, target string, markdown bool) string {
	file, _, _ := splitFragment(target)
	found := v.resolver.resolve(link{source: source, filePart: file, mdRelative: markdown})
	if len(found) != 1 {
		return ""
	}
	return found[0].path
}

// Rewrite returns the text of the page at source with every link that resolves to one
// file rewritten. replace is given the resolved path and returns the path to write, with
// its extension, or "" to leave the link as written. A wikilink drops the .md extension
// and keeps its fragment; one without an alias gets the original link text as its alias,
// so Obsidian still shows the page's name. A markdown link keeps its text and gets the
// new path percent-encoded. Links inside code, comments, and the frontmatter are not
// touched.
func (v *Vault) Rewrite(source, text string, replace func(resolved string) string) string {
	masked := maskCode(text)
	type span struct {
		start, end int
		out        string
	}
	var spans []span
	var occupied [][2]int
	for _, m := range wikiLink.FindAllStringSubmatchIndex(masked, -1) {
		occupied = append(occupied, [2]int{m[0], m[1]})
		body := text[m[4]:m[5]]
		targetPart, alias, hasAlias := strings.Cut(body, "|")
		raw := wikiTarget(body)
		if raw == "" {
			continue
		}
		resolved := v.Resolve(source, raw, false)
		if resolved == "" {
			continue
		}
		next := replace(resolved)
		if next == "" {
			continue
		}
		if strings.EqualFold(path.Ext(next), ".md") {
			next = strings.TrimSuffix(next, path.Ext(next))
		}
		_, fragment := splitRawFragment(targetPart)
		out := next + fragment
		if hasAlias {
			out += "|" + alias
		} else {
			out += "|" + strings.TrimSpace(targetPart)
		}
		prefix := "[["
		if m[2] >= 0 {
			prefix = "![["
		}
		spans = append(spans, span{start: m[0], end: m[1], out: prefix + out + "]]"})
	}
	for _, m := range mdLink.FindAllStringSubmatchIndex(masked, -1) {
		inside := false
		for _, o := range occupied {
			if m[0] >= o[0] && m[0] < o[1] {
				inside = true
				break
			}
		}
		if inside {
			continue
		}
		dest := mdDestination(text[m[6]:m[7]])
		if dest == "" || strings.HasPrefix(dest, "//") || uriScheme.MatchString(dest) || strings.HasPrefix(dest, "#") {
			continue
		}
		decoded, err := url.PathUnescape(dest)
		if err != nil {
			decoded = dest
		}
		resolved := v.Resolve(source, decoded, true)
		if resolved == "" {
			continue
		}
		next := replace(resolved)
		if next == "" {
			continue
		}
		_, fragment := splitRawFragment(decoded)
		prefix := "["
		if m[2] >= 0 {
			prefix = "!["
		}
		spans = append(spans, span{start: m[0], end: m[1], out: prefix + text[m[4]:m[5]] + "](" + escapePath(next) + fragment + ")"})
	}
	if len(spans) == 0 {
		return text
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var b strings.Builder
	last := 0
	for _, s := range spans {
		b.WriteString(text[last:s.start])
		b.WriteString(s.out)
		last = s.end
	}
	b.WriteString(text[last:])
	return b.String()
}

// splitRawFragment splits a link target as written into its file part and its fragment
// with the # kept, honouring a backslash before #.
func splitRawFragment(target string) (string, string) {
	escaped := false
	for i, ch := range target {
		if ch == '\\' && !escaped {
			escaped = true
			continue
		}
		if ch == '#' && !escaped {
			return target[:i], target[i:]
		}
		escaped = false
	}
	return target, ""
}

// escapePath percent-encodes the characters a markdown link destination cannot hold bare.
func escapePath(p string) string {
	return strings.NewReplacer("%", "%25", " ", "%20", "(", "%28", ")", "%29").Replace(p)
}
