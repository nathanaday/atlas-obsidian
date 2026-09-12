// Package refresh derives state.json for every node and renders Atlas.md.
package refresh

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/links"
	"github.com/nathanaday/claude-atlas/internal/product"
	"github.com/nathanaday/claude-atlas/internal/tree"
)

const (
	NewDays  = 7
	HotDays  = 7
	WarmDays = 30
)

var (
	logHeading  = regexp.MustCompile(`(?m)^##\s+(\d{4}-\d{2}-\d{2})\b`)
	statusLine  = regexp.MustCompile(`(?m)^status:\s*(.+?)\s*$`)
	createdLine = regexp.MustCompile(`(?m)^created:\s*(\d{4}-\d{2}-\d{2})`)
	wikiLink    = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
)

func ptr[T any](v T) *T { return &v }

// Heat maps a vault's age and idleness to new, hot, warm, or cold; unknown idleness gives "".
// A vault created within NewDays is "new" whatever its activity, so a fresh, possibly
// empty vault is not mistaken for one with a long active history.
func Heat(daysIdle, daysOld *int) string {
	switch {
	case daysIdle == nil:
		return ""
	case daysOld != nil && *daysOld < NewDays:
		return "new"
	case *daysIdle < HotDays:
		return "hot"
	case *daysIdle < WarmDays:
		return "warm"
	default:
		return "cold"
	}
}

func parseDate(s string) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	return t, err == nil
}

func dateOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// NewestLogDate is the latest `## YYYY-MM-DD` heading in wiki/log.md.
func NewestLogDate(vault string) (time.Time, bool) {
	data, err := os.ReadFile(filepath.Join(vault, "wiki", "log.md"))
	if err != nil {
		return time.Time{}, false
	}
	var newest time.Time
	found := false
	for _, m := range logHeading.FindAllStringSubmatch(string(data), -1) {
		if t, ok := parseDate(m[1]); ok && (!found || t.After(newest)) {
			newest, found = t, true
		}
	}
	return newest, found
}

// CreatedDate is the day claude-obsidian initialized the vault: the `created:` field it
// writes into wiki/index.md (or wiki/overview.md) from its template.
func CreatedDate(vault string) (time.Time, bool) {
	for _, name := range []string{"index.md", "overview.md", "log.md"} {
		data, err := os.ReadFile(filepath.Join(vault, "wiki", name))
		if err != nil {
			continue
		}
		front, _, ok := tree.SplitFrontmatter(string(data))
		if !ok {
			continue
		}
		if m := createdLine.FindStringSubmatch(front); m != nil {
			if t, ok := parseDate(m[1]); ok {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// NewestWikiMtime is the latest modification date of any file under wiki/.
func NewestWikiMtime(vault string) (time.Time, bool) {
	var newest time.Time
	found := false
	filepath.WalkDir(filepath.Join(vault, "wiki"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if !found || info.ModTime().After(newest) {
			newest, found = info.ModTime(), true
		}
		return nil
	})
	if !found {
		return time.Time{}, false
	}
	return dateOf(newest), true
}

// ActiveThreads lists the bullets under `## Active Threads` in wiki/hot.md. Prose, so best effort.
func ActiveThreads(vault string) []string {
	data, err := os.ReadFile(filepath.Join(vault, "wiki", "hot.md"))
	if err != nil {
		return nil
	}
	var threads []string
	inside := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			inside = strings.EqualFold(strings.TrimSpace(line[3:]), "active threads")
			continue
		}
		if !inside {
			continue
		}
		s := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* "):
			threads = append(threads, strings.TrimSpace(s[2:]))
		case s != "" && len(threads) > 0 && !strings.HasPrefix(s, "#"):
			threads[len(threads)-1] += " " + s
		}
	}
	return threads
}

// SeedPages counts wiki pages whose frontmatter says `status: seed`.
func SeedPages(vault string) int {
	count := 0
	filepath.WalkDir(filepath.Join(vault, "wiki"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		front, _, ok := tree.SplitFrontmatter(string(data))
		if !ok {
			return nil
		}
		if m := statusLine.FindStringSubmatch(front); m != nil && strings.Trim(m[1], `"'`) == "seed" {
			count++
		}
		return nil
	})
	return count
}

// PlainText strips wikilinks; a link copied from another vault resolves to nothing in the atlas.
func PlainText(s string) string {
	return wikiLink.ReplaceAllStringFunc(s, func(m string) string {
		sub := wikiLink.FindStringSubmatch(m)
		if sub[2] != "" {
			return sub[2]
		}
		return sub[1]
	})
}

func baseState(generatedAt string) *tree.State {
	return &tree.State{Schema: tree.StateSchema, GeneratedAt: generatedAt, OpenThreads: []string{}}
}

// Derive observes one project's vault. A nil product records the vault as unverified.
func Derive(p *product.Product, project *tree.Project, today time.Time, generatedAt string) *tree.State {
	state := baseState(generatedAt)
	state.Project = project.Rel
	vault := project.VaultPath()
	state.Vault = vault
	if _, err := os.Stat(filepath.Join(vault, ".claude-obsidian.json")); err != nil {
		if _, err := os.Stat(vault); err != nil {
			state.VaultError = "not found"
		} else {
			state.VaultError = "not a claude-obsidian vault"
		}
		return state
	}
	var touched time.Time
	touchedFound := false
	if op, ok := NewestLogDate(vault); ok {
		state.LastOperation = op.Format("2006-01-02")
		touched, touchedFound = op, true
	}
	if mt, ok := NewestWikiMtime(vault); ok && (!touchedFound || mt.After(touched)) {
		touched, touchedFound = mt, true
	}
	// Work in a linked repo or on linked material counts as work on the project.
	state.Links = inspectLinks(project)
	for _, link := range state.Links {
		if t, ok := link.Touched(); ok && (!touchedFound || t.After(touched)) {
			touched, touchedFound = t, true
		}
	}
	var daysOld *int
	if created, ok := CreatedDate(vault); ok {
		state.Created = created.Format("2006-01-02")
		daysOld = ptr(int(dateOf(today).Sub(dateOf(created)).Hours() / 24))
	}
	if touchedFound {
		state.LastTouched = touched.Format("2006-01-02")
		days := int(dateOf(today).Sub(dateOf(touched)).Hours() / 24)
		state.DaysIdle = ptr(days)
		state.Heat = Heat(state.DaysIdle, daysOld)
	}
	state.OpenThreads = ActiveThreads(vault)
	if state.OpenThreads == nil {
		state.OpenThreads = []string{}
	}
	state.Unfinished.SeedPages = ptr(SeedPages(vault))

	if p == nil {
		state.VaultError = "claude-obsidian is not installed"
		return state
	}
	doctor, err := p.Doctor(vault)
	if err != nil {
		state.VaultError = err.Error()
		return state
	}
	if !doctor.OK {
		var failed []string
		for name, ok := range doctor.Checks {
			if !ok {
				failed = append(failed, name)
			}
		}
		sort.Strings(failed)
		state.VaultError = "doctor: " + strings.Join(failed, ", ")
		return state
	}
	summary, err := p.Lint(vault)
	if err != nil {
		state.VaultError = err.Error()
		return state
	}
	state.VaultOK = true
	state.Pages = ptr(summary.PagesScanned)
	state.Unfinished.EmptySections = ptr(summary.CategoryCounts["empty_sections"])
	state.Unfinished.DeadLinks = ptr(summary.CategoryCounts["dead_links"])
	return state
}

func inspectLinks(project *tree.Project) []links.Link {
	out := []links.Link{}
	for _, path := range project.Repos {
		out = append(out, links.Inspect(links.Repo, path))
	}
	for _, path := range project.Materials {
		out = append(out, links.Inspect(links.Materials, path))
	}
	return out
}

// LinkSummary renders one link's derived facts on a line.
func LinkSummary(l links.Link) string {
	if !l.OK {
		return l.Error
	}
	var bits []string
	if l.Kind == links.Repo {
		if l.Branch != "" {
			bits = append(bits, l.Branch)
		}
		if l.Dirty != nil {
			if *l.Dirty == 0 {
				bits = append(bits, "clean")
			} else {
				bits = append(bits, fmt.Sprintf("%d uncommitted", *l.Dirty))
			}
		}
		if l.LastCommit != "" {
			bits = append(bits, "last commit "+l.LastCommit)
		}
	} else {
		if l.Files != nil {
			bits = append(bits, fmt.Sprintf("%d file%s", *l.Files, plural(*l.Files)))
		}
		if l.Bytes != nil {
			bits = append(bits, links.HumanBytes(*l.Bytes))
		}
		if l.Newest != "" {
			bits = append(bits, "newest "+l.Newest)
		}
	}
	if len(bits) == 0 {
		return "ok"
	}
	return strings.Join(bits, " · ")
}

func sumPtr(values []*int) *int {
	total, any := 0, false
	for _, v := range values {
		if v != nil {
			total, any = total+*v, true
		}
	}
	if !any {
		return nil
	}
	return ptr(total)
}

// Row pairs a project with its derived state.
type Row struct {
	Project *tree.Project
	State   *tree.State
}

// Result is one refresh: rows in tree order plus files that could not be read as projects.
type Result struct {
	Rows     []Row
	Problems []tree.Problem
}

// Tree derives state for every project and rewrites the state directory from scratch.
func Tree(cfg *home.Config, stateDir string, p *product.Product, today time.Time, generatedAt string) (*Result, error) {
	projects, problems, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return nil, err
	}
	if err := os.RemoveAll(stateDir); err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(projects))
	for _, project := range projects {
		state := Derive(p, project, today, generatedAt)
		if err := tree.WriteState(stateDir, project.Rel, state); err != nil {
			return nil, err
		}
		rows = append(rows, Row{project, state})
	}
	return &Result{Rows: rows, Problems: problems}, nil
}

var (
	heatOrder     = map[string]int{"hot": 0, "new": 1, "warm": 2, "cold": 3, "": 4}
	priorityOrder = map[string]int{"high": 0, "normal": 1, "low": 2, "someday": 3}
)

func idle(state *tree.State) string {
	switch {
	case state.DaysIdle == nil:
		return "—"
	case *state.DaysIdle == 0:
		return "today"
	default:
		return fmt.Sprintf("%dd", *state.DaysIdle)
	}
}

func intOr(v *int, def string) string {
	if v == nil {
		return def
	}
	return fmt.Sprint(*v)
}

func unfinishedTotal(u tree.Unfinished) string {
	return intOr(sumPtr([]*int{u.EmptySections, u.SeedPages, u.DeadLinks}), "—")
}

// Signals crosses authored intent with derived state; these lines are the point of the page.
func Signals(node *tree.Project, state *tree.State, today time.Time) []string {
	var notes []string
	if !state.VaultOK {
		notes = append(notes, "vault unreachable: "+state.VaultError)
	}
	if state.Heat == "cold" && node.State == "active" && (node.Priority == "high" || node.Priority == "normal") {
		notes = append(notes, fmt.Sprintf("declared priority %s, active, but cold for %d days", node.Priority, *state.DaysIdle))
	}
	for _, link := range state.Links {
		if !link.OK {
			notes = append(notes, fmt.Sprintf("%s %s: %s", link.Kind, home.Display(home.Expand(link.Path)), link.Error))
		}
	}
	if node.State == "blocked" {
		on := node.BlockedOn
		if on == "" {
			on = "(nothing recorded)"
		}
		notes = append(notes, "blocked on: "+on)
	}
	if node.ReviewAfter != "" {
		if t, ok := parseDate(node.ReviewAfter); !ok {
			notes = append(notes, fmt.Sprintf("review_after %q is not a date", node.ReviewAfter))
		} else if t.Before(dateOf(today)) {
			notes = append(notes, fmt.Sprintf("review date %s has passed; intent may be stale", node.ReviewAfter))
		}
	}
	return notes
}

// Render writes the overview page: callouts and tables, nothing else.
func Render(res *Result, generatedAt string, today time.Time) string {
	rows := res.Rows
	stamp, _ := time.Parse("2006-01-02T15:04:05Z", generatedAt)
	heats := map[string]int{}
	for _, r := range rows {
		heats[r.State.Heat]++
	}
	parts := []string{fmt.Sprintf("%d project%s", len(rows), plural(len(rows)))}
	for _, h := range []string{"hot", "new", "warm", "cold"} {
		if heats[h] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", heats[h], h))
		}
	}
	if heats[""] > 0 {
		parts = append(parts, fmt.Sprintf("%d unreachable", heats[""]))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: Overview\ngenerated_at: %s\n---\n\n", generatedAt)
	fmt.Fprintf(&b, "> [!info] Generated page\n> `claude-atlas refresh` rewrites this page from every project under `tree/`. Edit a project's own page and refresh again; edits made here are lost.\n> **%s** · refreshed %s\n\n",
		strings.Join(parts, " · "), stamp.Local().Format("2006-01-02 15:04"))

	b.WriteString("## All projects\n\n")
	b.WriteString("| Heat | Project | Category | Priority | State | Idle | Pages | Threads | Unfinished |\n")
	b.WriteString("|:--|:--|:--|:--|:--|--:|--:|--:|--:|\n")
	sorted := append([]Row(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, c := sorted[i], sorted[j]
		if heatOrder[a.State.Heat] != heatOrder[c.State.Heat] {
			return heatOrder[a.State.Heat] < heatOrder[c.State.Heat]
		}
		if priorityOrder[a.Project.Priority] != priorityOrder[c.Project.Priority] {
			return priorityOrder[a.Project.Priority] < priorityOrder[c.Project.Priority]
		}
		return a.Project.Rel < c.Project.Rel
	})
	for _, r := range sorted {
		fmt.Fprintf(&b, "| %s | [[tree/%s\\|%s]] | %s | %s | %s | %s | %s | %d | %s |\n",
			heatLabel(r.State.Heat), r.Project.Rel, r.Project.Name, categoryLabel(r.Project.Category()),
			r.Project.Priority, r.Project.State, idle(r.State),
			intOr(r.State.Pages, "—"), len(r.State.OpenThreads), unfinishedTotal(r.State.Unfinished))
	}

	b.WriteString("\n## Signals\n\n")
	flagged := 0
	for _, problem := range res.Problems {
		flagged++
		fmt.Fprintf(&b, "> [!failure] tree/%s.md\n> Not a project: %s.\n\n", problem.Rel, problem.Reason)
	}
	for _, r := range rows {
		for _, note := range Signals(r.Project, r.State, today) {
			flagged++
			fmt.Fprintf(&b, "> [!%s] %s\n> %s\n\n", calloutFor(note), r.Project.Rel, capitalize(note))
		}
	}
	if flagged == 0 {
		b.WriteString("> [!success] Nothing needs attention\n> No project is cold against its declared priority, blocked, unreachable, or past its review date.\n\n")
	}

	category := "\x00"
	for _, r := range rows {
		if cat := r.Project.Category(); cat != category {
			category = cat
			if cat == "" {
				b.WriteString("\n## Projects\n")
			} else {
				fmt.Fprintf(&b, "\n## %s\n", cat)
			}
		}
		fmt.Fprintf(&b, "\n### %s\n\n", r.Project.Name)
		if r.Project.Purpose != "" {
			fmt.Fprintf(&b, "> [!abstract] Purpose\n> %s\n\n", strings.ReplaceAll(strings.TrimSpace(r.Project.Purpose), "\n", "\n> "))
		}
		b.WriteString("| | |\n|:--|:--|\n")
		fmt.Fprintf(&b, "| Page | [[tree/%s\\|%s]] |\n", r.Project.Rel, r.Project.Rel)
		fmt.Fprintf(&b, "| Vault | `%s` |\n", home.Display(r.Project.VaultPath()))
		fmt.Fprintf(&b, "| Priority | %s |\n| State | %s |\n", r.Project.Priority, r.Project.State)
		if r.Project.BlockedOn != "" {
			fmt.Fprintf(&b, "| Blocked on | %s |\n", r.Project.BlockedOn)
		}
		switch {
		case !r.State.VaultOK:
			fmt.Fprintf(&b, "| Heat | %s · %s |\n", heatLabel(""), r.State.VaultError)
		case r.State.LastTouched != "":
			fmt.Fprintf(&b, "| Heat | %s · last touched %s (%s) |\n", heatLabel(r.State.Heat), r.State.LastTouched, idle(r.State))
		}
		if r.State.Created != "" {
			fmt.Fprintf(&b, "| Created | %s |\n", r.State.Created)
		}
		if r.State.LastOperation != "" {
			fmt.Fprintf(&b, "| Last operation | %s |\n", r.State.LastOperation)
		}
		if r.State.Pages != nil {
			fmt.Fprintf(&b, "| Pages | %d |\n", *r.State.Pages)
		}
		u := r.State.Unfinished
		if u.EmptySections != nil || u.SeedPages != nil || u.DeadLinks != nil {
			var bits []string
			for _, kv := range []struct {
				k string
				v *int
			}{{"empty sections", u.EmptySections}, {"seed pages", u.SeedPages}, {"dead links", u.DeadLinks}} {
				if kv.v != nil {
					bits = append(bits, fmt.Sprintf("%d %s", *kv.v, kv.k))
				}
			}
			fmt.Fprintf(&b, "| Unfinished | %s |\n", strings.Join(bits, " · "))
		}
		if r.Project.ReviewAfter != "" {
			fmt.Fprintf(&b, "| Review after | %s |\n", r.Project.ReviewAfter)
		}
		for _, link := range r.State.Links {
			label := "Repo"
			if link.Kind == links.Materials {
				label = "Materials"
			}
			fmt.Fprintf(&b, "| %s | `%s` · %s |\n", label, home.Display(home.Expand(link.Path)), LinkSummary(link))
		}
		if r.Project.DefinitionOfDone != "" {
			fmt.Fprintf(&b, "\n> [!success] Done when\n> %s\n", strings.TrimSpace(r.Project.DefinitionOfDone))
		}
		if len(r.State.OpenThreads) > 0 {
			b.WriteString("\n> [!todo] Open threads\n")
			for _, t := range r.State.OpenThreads {
				b.WriteString("> - " + PlainText(t) + "\n")
			}
		}
	}
	return b.String()
}

func categoryLabel(category string) string {
	if category == "" {
		return "—"
	}
	return category
}

func heatLabel(heat string) string {
	switch heat {
	case "new":
		return "✨ new"
	case "hot":
		return "🔥 hot"
	case "warm":
		return "🌤️ warm"
	case "cold":
		return "❄️ cold"
	default:
		return "⛔ unreachable"
	}
}

func calloutFor(note string) string {
	switch {
	case strings.HasPrefix(note, "vault unreachable"), strings.HasPrefix(note, "repo "), strings.HasPrefix(note, "materials "):
		return "failure"
	case strings.HasPrefix(note, "blocked on"):
		return "danger"
	case strings.HasPrefix(note, "review"):
		return "question"
	default:
		return "warning"
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Run refreshes every project and writes Overview.md; it returns the page path and the result.
func Run(cfg *home.Config, stateDir string, p *product.Product, today time.Time) (string, *Result, error) {
	generatedAt := product.NowUTC()
	res, err := Tree(cfg, stateDir, p, today, generatedAt)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(cfg.AtlasVault, 0o755); err != nil {
		return "", nil, err
	}
	page := filepath.Join(cfg.AtlasVault, "Overview.md")
	if err := os.WriteFile(page, []byte(Render(res, generatedAt, today)), 0o644); err != nil {
		return "", nil, err
	}
	return page, res, nil
}
