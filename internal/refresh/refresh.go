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
	"github.com/nathanaday/claude-atlas/internal/product"
	"github.com/nathanaday/claude-atlas/internal/tree"
)

const (
	HotDays  = 7
	WarmDays = 30
)

var (
	logHeading = regexp.MustCompile(`(?m)^##\s+(\d{4}-\d{2}-\d{2})\b`)
	statusLine = regexp.MustCompile(`(?m)^status:\s*(.+?)\s*$`)
	wikiLink   = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)
)

func ptr[T any](v T) *T { return &v }

// Heat maps idle days to hot, warm, or cold; unknown idleness gives "".
func Heat(daysIdle *int) string {
	switch {
	case daysIdle == nil:
		return ""
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

// DeriveLeaf observes one vault. A nil product records the vault as unverified.
func DeriveLeaf(p *product.Product, node *tree.Node, today time.Time, generatedAt string) *tree.State {
	state := baseState(generatedAt)
	vault := node.VaultPath()
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
	if touchedFound {
		state.LastTouched = touched.Format("2006-01-02")
		days := int(dateOf(today).Sub(dateOf(touched)).Hours() / 24)
		state.DaysIdle = ptr(days)
		state.Heat = Heat(state.DaysIdle)
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

// DeriveCluster rolls children up: hottest heat, newest dates, summed counts.
func DeriveCluster(children []*tree.State, generatedAt string) *tree.State {
	state := baseState(generatedAt)
	state.VaultOK = len(children) > 0
	var pages, empty, seed, dead []*int
	leaves := 0
	for _, c := range children {
		state.VaultOK = state.VaultOK && c.VaultOK
		if c.LastOperation > state.LastOperation {
			state.LastOperation = c.LastOperation
		}
		if c.LastTouched > state.LastTouched {
			state.LastTouched = c.LastTouched
		}
		if c.DaysIdle != nil && (state.DaysIdle == nil || *c.DaysIdle < *state.DaysIdle) {
			state.DaysIdle = ptr(*c.DaysIdle)
		}
		state.OpenThreads = append(state.OpenThreads, c.OpenThreads...)
		pages = append(pages, c.Pages)
		empty = append(empty, c.Unfinished.EmptySections)
		seed = append(seed, c.Unfinished.SeedPages)
		dead = append(dead, c.Unfinished.DeadLinks)
		if c.Leaves != nil {
			leaves += *c.Leaves
		} else {
			leaves++
		}
	}
	if !state.VaultOK {
		state.VaultError = "one or more vaults unreachable"
	}
	state.Heat = Heat(state.DaysIdle)
	state.Pages = sumPtr(pages)
	state.Unfinished = tree.Unfinished{EmptySections: sumPtr(empty), SeedPages: sumPtr(seed), DeadLinks: sumPtr(dead)}
	state.Leaves = ptr(leaves)
	return state
}

// Row pairs a node with its derived state.
type Row struct {
	Node  *tree.Node
	State *tree.State
}

// Tree derives and writes state for every node, leaves first, then clusters.
func Tree(cfg *home.Config, p *product.Product, today time.Time, generatedAt string) ([]Row, error) {
	nodes, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return nil, err
	}
	states := map[string]*tree.State{}
	order := append([]*tree.Node(nil), nodes...)
	sort.SliceStable(order, func(i, j int) bool { return order[i].Depth() > order[j].Depth() })
	for _, node := range order {
		var state *tree.State
		if node.IsLeaf() {
			state = DeriveLeaf(p, node, today, generatedAt)
		} else {
			var children []*tree.State
			for _, other := range nodes {
				if strings.HasPrefix(other.Rel, node.Rel+"/") && other.Depth() == node.Depth()+1 {
					children = append(children, states[other.Rel])
				}
			}
			state = DeriveCluster(children, generatedAt)
		}
		if err := tree.WriteState(node.Dir, state); err != nil {
			return nil, err
		}
		states[node.Rel] = state
	}
	rows := make([]Row, len(nodes))
	for i, node := range nodes {
		rows[i] = Row{node, states[node.Rel]}
	}
	return rows, nil
}

var (
	heatOrder     = map[string]int{"hot": 0, "warm": 1, "cold": 2, "": 3}
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
func Signals(node *tree.Node, state *tree.State, today time.Time) []string {
	var notes []string
	if !state.VaultOK && node.IsLeaf() {
		notes = append(notes, "vault unreachable: "+state.VaultError)
	}
	if state.Heat == "cold" && node.State == "active" && (node.Priority == "high" || node.Priority == "normal") {
		notes = append(notes, fmt.Sprintf("declared priority %s, active, but cold for %d days", node.Priority, *state.DaysIdle))
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
func Render(rows []Row, generatedAt string, today time.Time) string {
	stamp, _ := time.Parse("2006-01-02T15:04:05Z", generatedAt)
	var leafCount int
	heats := map[string]int{}
	for _, r := range rows {
		if r.Node.IsLeaf() {
			leafCount++
			heats[r.State.Heat]++
		}
	}
	parts := []string{fmt.Sprintf("%d vault%s", leafCount, plural(leafCount))}
	for _, h := range []string{"hot", "warm", "cold"} {
		if heats[h] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", heats[h], h))
		}
	}
	if heats[""] > 0 {
		parts = append(parts, fmt.Sprintf("%d unreachable", heats[""]))
	}

	var b strings.Builder
	fmt.Fprintf(&b, "---\ntitle: Overview\ngenerated_at: %s\n---\n\n# Overview\n\n", generatedAt)
	fmt.Fprintf(&b, "> [!info] Generated page\n> `claude-atlas refresh` rewrites this page from every registered vault. Edit intent in each project's `node.md` and refresh again; edits made here are lost.\n> **%s** · refreshed %s\n\n",
		strings.Join(parts, " · "), stamp.Local().Format("2006-01-02 15:04"))

	b.WriteString("## All vaults\n\n")
	b.WriteString("| Heat | Project | Priority | State | Idle | Pages | Threads | Unfinished |\n")
	b.WriteString("|:--|:--|:--|:--|--:|--:|--:|--:|\n")
	sorted := append([]Row(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, c := sorted[i], sorted[j]
		if heatOrder[a.State.Heat] != heatOrder[c.State.Heat] {
			return heatOrder[a.State.Heat] < heatOrder[c.State.Heat]
		}
		if priorityOrder[a.Node.Priority] != priorityOrder[c.Node.Priority] {
			return priorityOrder[a.Node.Priority] < priorityOrder[c.Node.Priority]
		}
		return a.Node.Rel < c.Node.Rel
	})
	for _, r := range sorted {
		label := r.Node.Rel
		if !r.Node.IsLeaf() {
			label += "/"
		}
		fmt.Fprintf(&b, "| %s | [[tree/%s/node\\|%s]] | %s | %s | %s | %s | %d | %s |\n",
			heatLabel(r.State.Heat), r.Node.Rel, label, r.Node.Priority, r.Node.State, idle(r.State),
			intOr(r.State.Pages, "—"), len(r.State.OpenThreads), unfinishedTotal(r.State.Unfinished))
	}

	b.WriteString("\n## Signals\n\n")
	flagged := 0
	for _, r := range rows {
		for _, note := range Signals(r.Node, r.State, today) {
			flagged++
			fmt.Fprintf(&b, "> [!%s] %s\n> %s\n\n", calloutFor(note), r.Node.Rel, capitalize(note))
		}
	}
	if flagged == 0 {
		b.WriteString("> [!success] Nothing needs attention\n> No vault is cold against its declared priority, blocked, unreachable, or past its review date.\n\n")
	}

	b.WriteString("## Projects\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "\n### %s\n\n", r.Node.Rel)
		if r.Node.Purpose != "" {
			title := r.Node.Name
			if title == "" {
				title = r.Node.ID()
			}
			fmt.Fprintf(&b, "> [!abstract] %s\n> %s\n\n", title, strings.ReplaceAll(strings.TrimSpace(r.Node.Purpose), "\n", "\n> "))
		}
		b.WriteString("| | |\n|:--|:--|\n")
		if v := r.Node.VaultPath(); v != "" {
			fmt.Fprintf(&b, "| Vault | `%s` |\n", home.Display(v))
		} else {
			fmt.Fprintf(&b, "| Cluster | %s vaults |\n", intOr(r.State.Leaves, "0"))
		}
		fmt.Fprintf(&b, "| Node | [[tree/%s/node\\|node.md]] |\n", r.Node.Rel)
		fmt.Fprintf(&b, "| Priority | %s |\n| State | %s |\n", r.Node.Priority, r.Node.State)
		if r.Node.BlockedOn != "" {
			fmt.Fprintf(&b, "| Blocked on | %s |\n", r.Node.BlockedOn)
		}
		switch {
		case !r.State.VaultOK && r.Node.IsLeaf():
			fmt.Fprintf(&b, "| Heat | %s · %s |\n", heatLabel(""), r.State.VaultError)
		case r.State.LastTouched != "":
			fmt.Fprintf(&b, "| Heat | %s · last touched %s (%s) |\n", heatLabel(r.State.Heat), r.State.LastTouched, idle(r.State))
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
		if r.Node.ReviewAfter != "" {
			fmt.Fprintf(&b, "| Review after | %s |\n", r.Node.ReviewAfter)
		}
		if len(r.Node.Repos) > 0 {
			fmt.Fprintf(&b, "| Repos | %s |\n", strings.Join(r.Node.Repos, " · "))
		}
		if r.Node.DefinitionOfDone != "" {
			fmt.Fprintf(&b, "\n> [!success] Done when\n> %s\n", strings.TrimSpace(r.Node.DefinitionOfDone))
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

func heatLabel(heat string) string {
	switch heat {
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
	case strings.HasPrefix(note, "vault unreachable"):
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

// Run refreshes every node and writes Overview.md; it returns the page path and rows.
func Run(cfg *home.Config, p *product.Product, today time.Time) (string, []Row, error) {
	generatedAt := product.NowUTC()
	rows, err := Tree(cfg, p, today, generatedAt)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(cfg.AtlasVault, 0o755); err != nil {
		return "", nil, err
	}
	page := filepath.Join(cfg.AtlasVault, "Overview.md")
	if err := os.WriteFile(page, []byte(Render(rows, generatedAt, today)), 0o644); err != nil {
		return "", nil, err
	}
	return page, rows, nil
}
