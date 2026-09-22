// Package mirror copies the wikis of a project's members into the project's own wiki,
// under wiki/projects/, as one sync operation. The copy is derived: sync computes every
// file the closure of members produces, makes the folder match, and commits once through
// the engine. Nothing is written into a member. See docs/members-design.md.
package mirror

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/txn"
)

// Member is one project in a hub's closure: a member, or a member of a member.
type Member struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// Folder is the mirror's folder under wiki/projects/, the member's own folder name.
	Folder string `json:"folder,omitempty"`
	Path   string `json:"path,omitempty"`
	// Commit is the newest commit that touched the member's wiki, when it has one.
	Commit string `json:"commit,omitempty"`
	// Error says why the member could not be mirrored; its old mirror stays.
	Error string `json:"error,omitempty"`
	Pages int    `json:"pages"`
}

// ErrCycle means the member graph reaches a project again.
var ErrCycle = errors.New("cycle")

// Closure lists the projects root mirrors: the transitive closure of its members, each
// once, root excluded, ordered by folder name with the ones it could not read last. A
// member the index cannot read is listed with an Error and contributes no members of
// its own. It refuses a cycle, more than
// MaxMembers projects, and two projects with one folder name.
func Closure(ix *registry.Index, root registry.Entry) ([]Member, error) {
	seen := map[string]bool{root.ID: true}
	open := map[string]bool{root.ID: true}
	label := func(id string) string {
		if id == root.ID {
			return root.Name
		}
		if en := ix.ByID(id); en != nil && en.Name != "" {
			return en.Name
		}
		return id
	}
	var out []Member
	var visit func(from registry.Entry, ids []string, trail []string) error
	visit = func(from registry.Entry, ids []string, trail []string) error {
		for _, id := range ids {
			if open[id] {
				var names []string
				for _, t := range append(trail, id) {
					names = append(names, label(t))
				}
				return fmt.Errorf("%w: %s lists %s, which leads back to it (%s)", ErrCycle, from.Name, label(id), strings.Join(names, " > "))
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			en := ix.ByID(id)
			switch {
			case en == nil:
				out = append(out, Member{ID: id, Error: "not in the atlas config; work in it once to register it"})
				continue
			case en.Error != "":
				out = append(out, Member{ID: id, Path: en.Path, Error: en.Error})
				continue
			}
			// The folder is the name cleaned, as Save keeps it; Open confirms when the
			// member is read.
			out = append(out, Member{ID: id, Name: en.Name, Folder: project.FolderName(en.Name), Path: en.Path})
			if len(out) > project.MaxMembers {
				return fmt.Errorf("the members reach more than %d projects", project.MaxMembers)
			}
			open[id] = true
			if err := visit(*en, en.Members, append(trail, id)); err != nil {
				return err
			}
			delete(open, id)
		}
		return nil
	}
	if err := visit(root, root.Members, []string{root.ID}); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Error == "") != (out[j].Error == "") {
			return out[i].Error == ""
		}
		if out[i].Folder != out[j].Folder {
			return out[i].Folder < out[j].Folder
		}
		return out[i].ID < out[j].ID
	})
	byFolder := map[string]string{}
	for _, m := range out {
		if m.Folder == "" {
			continue
		}
		if other, taken := byFolder[m.Folder]; taken {
			return nil, fmt.Errorf("two projects would mirror to %s/%s: %s and %s; rename one", project.MirrorDir, m.Folder, other, m.Name)
		}
		byFolder[m.Folder] = m.Name
	}
	return out, nil
}

// Validate says whether root may list members: none is root itself, every one is a
// project the atlas lists, there are at most MaxMembers, and the closure they reach
// stays within the limits and holds no cycle.
func Validate(ix *registry.Index, root registry.Entry, members []string) error {
	if len(members) > project.MaxMembers {
		return fmt.Errorf("a project lists at most %d members", project.MaxMembers)
	}
	seen := map[string]bool{}
	for _, id := range members {
		if id == root.ID {
			return fmt.Errorf("%s cannot be its own member", root.Name)
		}
		if seen[id] {
			return fmt.Errorf("%s is listed twice", id)
		}
		seen[id] = true
		en := ix.ByID(id)
		if en == nil {
			return fmt.Errorf("%s is not a project the atlas lists", id)
		}
		if en.Error != "" {
			return fmt.Errorf("%s: %s", en.Name, en.Error)
		}
	}
	trial := &registry.Index{Entries: append([]registry.Entry(nil), ix.Entries...)}
	root.Members = members
	replaced := false
	for i := range trial.Entries {
		if trial.Entries[i].ID == root.ID {
			trial.Entries[i] = root
			replaced = true
		}
	}
	if !replaced {
		trial.Entries = append(trial.Entries, root)
	}
	_, err := Closure(trial, root)
	return err
}

// Plan is what one sync would do.
type Plan struct {
	Members []Member `json:"members"`
	Creates int      `json:"creates"`
	Updates int      `json:"updates"`
	Removes int      `json:"removes"`
	request txn.Request
}

// Result reports a sync. OperationID and Commit are empty when nothing changed.
type Result struct {
	Members     []Member `json:"members"`
	Creates     int      `json:"creates"`
	Updates     int      `json:"updates"`
	Removes     int      `json:"removes"`
	OperationID string   `json:"operation_id,omitempty"`
	Commit      string   `json:"commit,omitempty"`
}

// The frontmatter every mirrored page carries.
const (
	ProjectKey  = "project"
	MirrorOfKey = "mirror_of"
	CommitKey   = "commit"
)

// Build computes the sync for p over the projects ix lists. The plan's writes are the
// difference between the folder and what the closure produces.
func Build(p *project.Project, ix *registry.Index) (*Plan, error) {
	root := registry.Entry{ID: p.Config.ID, Name: p.Name(), Path: p.Root, Members: p.Config.Members}
	members, err := Closure(ix, root)
	if err != nil {
		return nil, err
	}
	desired := map[string][]byte{}
	keep := map[string]bool{} // mirror folders of members that could not be read
	for i := range members {
		m := &members[i]
		if m.Error != "" {
			continue
		}
		own := map[string][]byte{}
		if err := mirrorMember(m, own); err != nil {
			m.Error = err.Error()
			m.Folder = ""
			m.Pages = 0
			continue
		}
		for rel, data := range own {
			desired[rel] = data
		}
	}
	existing, err := existingMirrors(p)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		if m.Error != "" {
			if folder := existing.folderOf(m.ID); folder != "" {
				keep[folder] = true
			}
		}
	}
	desired[project.MirrorIndex] = indexPage(members, existing)
	plan := &Plan{Members: members}
	var names []string
	for _, m := range members {
		if m.Error == "" {
			names = append(names, m.Folder)
		}
	}
	var writes []txn.Write
	var paths []string
	for rel := range desired {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		content := desired[rel]
		current, ok := existing.files[rel]
		switch {
		case !ok:
			writes = append(writes, txn.Write{Path: rel, Mode: txn.Create, Content: content})
			plan.Creates++
		case sha(current) != sha(content):
			writes = append(writes, txn.Write{Path: rel, Mode: txn.Replace, Content: content, BaseSHA256: sha(current)})
			plan.Updates++
		}
	}
	var stale []string
	for rel := range existing.files {
		if _, wanted := desired[rel]; wanted {
			continue
		}
		if keep[mirrorFolder(rel)] {
			continue
		}
		stale = append(stale, rel)
	}
	sort.Strings(stale)
	for _, rel := range stale {
		writes = append(writes, txn.Write{Path: rel, Mode: txn.Delete, BaseSHA256: sha(existing.files[rel])})
		plan.Removes++
	}
	summary := fmt.Sprintf("sync %d project%s", len(names), plural(len(names)))
	if len(names) > 0 {
		summary += ": " + strings.Join(names, ", ")
	}
	plan.request = txn.Request{Kind: txn.Sync, Summary: summary, Writes: writes}
	return plan, nil
}

// Sync makes wiki/projects/ match the closure of p's members and commits the change as one
// sync operation. Nothing changed means no commit.
func Sync(p *project.Project, ix *registry.Index, now time.Time) (*Result, error) {
	plan, err := Build(p, ix)
	if err != nil {
		return nil, err
	}
	res := &Result{Members: plan.Members, Creates: plan.Creates, Updates: plan.Updates, Removes: plan.Removes}
	if len(plan.request.Writes) == 0 {
		return res, nil
	}
	prepared, err := txn.Prepare(p, plan.request, now)
	if err != nil {
		return nil, err
	}
	applied, err := txn.Apply(p, prepared, now)
	if err != nil {
		return nil, err
	}
	res.OperationID, res.Commit = applied.OperationID, applied.Commit
	pruneEmpty(p.Path(project.MirrorDir))
	return res, nil
}

// pruneEmpty removes the folders a sync emptied. Git never tracked them, so the commit
// is the same with or without them; Obsidian shows them until they go.
func pruneEmpty(root string) {
	var dirs []string
	filepath.WalkDir(root, func(fp string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && fp != root {
			dirs = append(dirs, fp)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, dir := range dirs {
		os.Remove(dir) // fails, and is meant to, when the folder holds anything
	}
	os.Remove(root)
}

// excluded says which of a member's wiki files are not mirrored: the log and the hot
// cache, which are the member's own record; the ledger and everything under meta/; and
// the member's own mirrors, which the closure flattens instead.
func excluded(rel string) bool {
	switch {
	case rel == project.LogPage, rel == project.HotPage:
		return true
	case strings.HasPrefix(rel, project.WikiDir+"/meta/"), lint.Mirrored(rel):
		return true
	}
	return false
}

// destination is where a member's wiki file lands in the hub, or "" when it is not
// mirrored. The member's index page takes the folder's name.
func destination(folder, rel string) string {
	if !strings.HasPrefix(rel, project.WikiDir+"/") || excluded(rel) {
		return ""
	}
	if rel == project.IndexPage {
		return project.MirrorDir + "/" + folder + "/" + folder + ".md"
	}
	return project.MirrorDir + "/" + folder + "/" + strings.TrimPrefix(rel, project.WikiDir+"/")
}

// mirrorMember reads one member's wiki and adds its transformed files to desired.
func mirrorMember(m *Member, desired map[string][]byte) error {
	mp, err := project.Open(m.Path)
	if err != nil {
		return err
	}
	if mp.Folder != m.Folder {
		return fmt.Errorf("its folder is %s, not %s; run atlas-obsidian edit to make the name and the folder agree", mp.Folder, m.Folder)
	}
	if commits, err := mp.Engine().Log(1); err == nil && len(commits) == 1 {
		m.Commit = commits[0].SHA
	}
	vault, err := lint.LoadVault(mp.Atlas())
	if err != nil {
		return err
	}
	replace := func(resolved string) string { return destination(m.Folder, resolved) }
	for _, rel := range vault.Files() {
		dest := destination(m.Folder, rel)
		if dest == "" {
			continue
		}
		data, err := os.ReadFile(mp.Path(rel))
		if err != nil {
			return err
		}
		switch strings.ToLower(path.Ext(rel)) {
		case ".md":
			text := vault.Rewrite(rel, string(data), replace)
			data = []byte(stamp(text, [][2]string{{ProjectKey, m.ID}, {MirrorOfKey, rel}, {CommitKey, m.Commit}}))
			m.Pages++
		case ".canvas":
			data = rewriteCanvas(data, func(file string) string {
				resolved := vault.Resolve(rel, file, false)
				if resolved == "" {
					return ""
				}
				return destination(m.Folder, resolved)
			})
		}
		desired[dest] = data
	}
	return nil
}

var frontKey = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*):`)

// stamp sets the mirror's own frontmatter keys and keeps every other line as written. A
// page without frontmatter gets a block holding only them. An empty value writes no line.
func stamp(text string, fields [][2]string) string {
	own := map[string]bool{}
	for _, f := range fields {
		own[f[0]] = true
	}
	front, body, ok, err := project.SplitFrontmatter(text)
	if err != nil {
		ok = false
		body = text
	}
	var lines []string
	if ok {
		for _, line := range strings.Split(strings.TrimRight(front, "\n"), "\n") {
			if m := frontKey.FindStringSubmatch(line); m != nil && own[m[1]] {
				continue
			}
			lines = append(lines, line)
		}
		if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
			lines = nil
		}
	}
	for _, f := range fields {
		if f[1] != "" {
			lines = append(lines, f[0]+": "+strconv.Quote(f[1]))
		}
	}
	return "---\n" + strings.Join(lines, "\n") + "\n---\n" + body
}

// rewriteCanvas maps the file of every file node through replace. A canvas that is not
// JSON is copied as it is.
func rewriteCanvas(data []byte, replace func(string) string) []byte {
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return data
	}
	nodes, _ := doc["nodes"].([]any)
	changed := false
	for _, n := range nodes {
		node, ok := n.(map[string]any)
		if !ok || node["type"] != "file" {
			continue
		}
		file, _ := node["file"].(string)
		if file == "" {
			continue
		}
		if next := replace(file); next != "" && next != file {
			node["file"] = next
			changed = true
		}
	}
	if !changed {
		return data
	}
	out, err := json.MarshalIndent(doc, "", "\t")
	if err != nil {
		return data
	}
	return append(out, '\n')
}

// mirrors is what the hub's wiki/projects/ holds now.
type mirrors struct {
	files map[string][]byte
	roots map[string]string // folder -> the project id its root page names
}

func (m mirrors) folderOf(id string) string {
	for folder, named := range m.roots {
		if named == id {
			return folder
		}
	}
	return ""
}

func mirrorFolder(rel string) string {
	rest := strings.TrimPrefix(rel, project.MirrorDir+"/")
	folder, _, _ := strings.Cut(rest, "/")
	return folder
}

// existingMirrors reads every file under wiki/projects/ and the id each mirror's root
// page names.
func existingMirrors(p *project.Project) (mirrors, error) {
	out := mirrors{files: map[string][]byte{}, roots: map[string]string{}}
	root := p.Path(project.MirrorDir)
	err := filepath.WalkDir(root, func(fp string, d fs.DirEntry, err error) error {
		if err != nil {
			if fp == root && errors.Is(err, fs.ErrNotExist) {
				return fs.SkipAll
			}
			return err
		}
		if d.IsDir() {
			if fp != root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(p.Atlas(), fp)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(fp)
		if err != nil {
			return err
		}
		out.files[rel] = data
		folder := mirrorFolder(rel)
		if rel == project.MirrorDir+"/"+folder+"/"+folder+".md" {
			if fields, _, err := project.Frontmatter(string(data)); err == nil && fields != nil {
				out.roots[folder] = project.StringField(fields, ProjectKey)
			}
		}
		return nil
	})
	return out, err
}

// indexPage renders wiki/projects/projects.md: one row per project in the closure. A
// member that could not be read keeps its row, marked, when its old mirror is there.
func indexPage(members []Member, existing mirrors) []byte {
	var b strings.Builder
	b.WriteString("---\ntitle: Projects\ntype: meta\nstatus: evergreen\ntags:\n  - meta\n  - projects\n---\n\n# Projects\n\n")
	b.WriteString("The wikis this project mirrors, one folder each under `" + project.MirrorDir + "/`. Sync derives them from the project's members; a page here changes in its own project.\n\n")
	if len(members) == 0 {
		b.WriteString("No members.\n")
		return []byte(b.String())
	}
	b.WriteString("| Project | Id | Commit | State |\n|---|---|---|---|\n")
	for _, m := range members {
		name, state := m.Name, "mirrored"
		folder := m.Folder
		if m.Error != "" {
			folder = existing.folderOf(m.ID)
			state = "not read: " + m.Error
			if folder != "" {
				state += "; the mirror is from an earlier sync"
			}
		}
		if name == "" {
			name = m.ID
		}
		cell := name
		if folder != "" {
			cell = fmt.Sprintf("[[%s/%s/%s\\|%s]]", project.MirrorDir, folder, folder, name)
		}
		commit := ""
		if m.Commit != "" {
			commit = "`" + m.Commit[:min(12, len(m.Commit))] + "`"
		}
		fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", cell, m.ID, commit, state)
	}
	return []byte(b.String())
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
