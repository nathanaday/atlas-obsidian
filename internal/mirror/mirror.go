// Package mirror copies the wikis and the threads of a project's members into the
// project's own folder: wikis under wiki/projects/ as one sync operation, threads under
// threads/projects/ as plain files, because the threads have no engine. The copy is
// derived: sync computes every file the closure of members produces and makes each
// folder match. Nothing is written into a member. See docs/members-design.md.
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
	"github.com/nathanaday/atlas-obsidian/internal/threads"
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
	// Threads counts the thread pages mirrored: zero when the hub or the member has
	// threads off.
	Threads int `json:"threads"`
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

// Plan is what one sync would do: the wiki writes, which go through the engine, and
// the thread writes, which are plain files. The counts cover both.
type Plan struct {
	Members []Member `json:"members"`
	Creates int      `json:"creates"`
	Updates int      `json:"updates"`
	Removes int      `json:"removes"`
	request txn.Request
	threads []txn.Write
}

// Result reports a sync. OperationID and Commit are empty when no wiki file changed.
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

// The frontmatter a merge leaves on a member page whose content moved into a hub:
// the page's path in the hub and the hub's id. The hub named does not mirror the page
// and sends every link to it to the hub's page; any other hub mirrors it as written.
const (
	MovedToKey        = "moved_to"
	MovedToProjectKey = "moved_to_project"
)

// Build computes the sync for p over the projects ix lists. The plan's writes are the
// difference between each mirror folder and what the closure produces.
func Build(p *project.Project, ix *registry.Index) (*Plan, error) {
	root := registry.Entry{ID: p.Config.ID, Name: p.Name(), Path: p.Root, Members: p.Config.Members}
	members, err := Closure(ix, root)
	if err != nil {
		return nil, err
	}
	wiki, thr := map[string][]byte{}, map[string][]byte{}
	for i := range members {
		m := &members[i]
		if m.Error != "" {
			continue
		}
		own, ownThreads := map[string][]byte{}, map[string][]byte{}
		if err := mirrorMember(m, p, own, ownThreads); err != nil {
			m.Error = err.Error()
			m.Folder = ""
			m.Pages, m.Threads = 0, 0
			continue
		}
		for rel, data := range own {
			wiki[rel] = data
		}
		for rel, data := range ownThreads {
			thr[rel] = data
		}
	}
	existing, err := existingMirrors(p)
	if err != nil {
		return nil, err
	}
	keep := map[string]bool{} // mirror folders of members that could not be read
	for _, m := range members {
		if m.Error != "" {
			if folder := existing.folderOf(m.ID); folder != "" {
				keep[folder] = true
			}
		}
	}
	wiki[project.MirrorIndex] = indexPage(members, existing)
	plan := &Plan{Members: members}
	var names []string
	for _, m := range members {
		if m.Error == "" {
			names = append(names, m.Folder)
		}
	}
	writes := reconcile(existing.wiki, wiki, keep, project.MirrorDir, plan)
	plan.threads = reconcile(existing.threads, thr, keep, project.ThreadMirrorDir, plan)
	summary := fmt.Sprintf("sync %d project%s", len(names), plural(len(names)))
	if len(names) > 0 {
		summary += ": " + strings.Join(names, ", ")
	}
	plan.request = txn.Request{Kind: txn.Sync, Summary: summary, Writes: writes}
	return plan, nil
}

// reconcile is the writes that make one mirror folder hold desired and nothing else,
// except the folders in keep. It adds what it counts to the plan.
func reconcile(existing, desired map[string][]byte, keep map[string]bool, dir string, plan *Plan) []txn.Write {
	var writes []txn.Write
	var paths []string
	for rel := range desired {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		content := desired[rel]
		current, ok := existing[rel]
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
	for rel := range existing {
		if _, wanted := desired[rel]; wanted {
			continue
		}
		if keep[mirrorFolder(dir, rel)] {
			continue
		}
		stale = append(stale, rel)
	}
	sort.Strings(stale)
	for _, rel := range stale {
		writes = append(writes, txn.Write{Path: rel, Mode: txn.Delete, BaseSHA256: sha(existing[rel])})
		plan.Removes++
	}
	return writes
}

// Sync makes wiki/projects/ and threads/projects/ match the closure of p's members. The
// wiki half commits as one sync operation; the thread half is written as it is, and the
// board is regenerated to show it. Nothing changed means no commit.
func Sync(p *project.Project, ix *registry.Index, now time.Time) (*Result, error) {
	plan, err := Build(p, ix)
	if err != nil {
		return nil, err
	}
	res := &Result{Members: plan.Members, Creates: plan.Creates, Updates: plan.Updates, Removes: plan.Removes}
	if len(plan.request.Writes) > 0 {
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
	}
	return res, applyThreads(p, plan, now)
}

// SyncThreads makes threads/projects/ alone match the closure, for the thread tools to
// call after a write in a member, when the wiki can wait for the next full sync.
func SyncThreads(p *project.Project, ix *registry.Index, now time.Time) (*Result, error) {
	plan, err := Build(p, ix)
	if err != nil {
		return nil, err
	}
	res := &Result{Members: plan.Members}
	for _, w := range plan.threads {
		switch w.Mode {
		case txn.Create:
			res.Creates++
		case txn.Replace:
			res.Updates++
		case txn.Delete:
			res.Removes++
		}
	}
	return res, applyThreads(p, plan, now)
}

// applyThreads writes the thread half of a plan and regenerates the board, which embeds
// each mirrored board. A hub with threads off writes nothing and has no board.
func applyThreads(p *project.Project, plan *Plan, now time.Time) error {
	for _, w := range plan.threads {
		file := p.Path(w.Path)
		if w.Mode == txn.Delete {
			if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(file, w.Content, 0o644); err != nil {
			return err
		}
	}
	pruneEmpty(p.Path(project.ThreadMirrorDir))
	if !p.Config.Threads {
		return nil
	}
	_, err := threads.Sync(p, now)
	return err
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

// destination is where a member's file lands in the hub, or "" when it is not mirrored.
// A wiki file lands under wiki/projects/<folder>/, and the member's index page takes the
// folder's name. A thread page lands under threads/projects/<folder>/ at the path it has
// in the member, when withThreads, so the relative links inside it still hold.
func destination(folder string, withThreads bool, rel string) string {
	switch {
	case strings.HasPrefix(rel, project.WikiDir+"/"):
		if excluded(rel) {
			return ""
		}
		if rel == project.IndexPage {
			return project.MirrorDir + "/" + folder + "/" + folder + ".md"
		}
		return project.MirrorDir + "/" + folder + "/" + strings.TrimPrefix(rel, project.WikiDir+"/")
	case withThreads && strings.HasPrefix(rel, project.ThreadsDir+"/") && !threads.Mirrored(rel):
		return project.ThreadMirrorDir + "/" + folder + "/" + strings.TrimPrefix(rel, project.ThreadsDir+"/")
	}
	return ""
}

// mirrorMember reads one member's folder and adds its transformed wiki files to wiki and
// its thread pages to thr. Threads are mirrored only when the hub and the member both
// track them. One link resolver covers both halves, so a wiki page that cites a thread
// document, or a thread document that cites a wiki page, resolves in the hub. A page
// whose content moved into this hub is left out, and a link to it lands on the hub's
// page.
func mirrorMember(m *Member, hub *project.Project, wiki, thr map[string][]byte) error {
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
	moved := movedInto(hub, vault)
	withThreads := hub.Config.Threads && mp.Config.Threads
	replace := func(resolved string) string {
		if to, ok := moved[resolved]; ok {
			return to
		}
		return destination(m.Folder, withThreads, resolved)
	}
	for _, rel := range vault.Files() {
		dest := destination(m.Folder, withThreads, rel)
		if dest == "" || moved[rel] != "" {
			continue
		}
		data, err := os.ReadFile(mp.Path(rel))
		if err != nil {
			return err
		}
		if threads.Mirrored(dest) {
			if strings.EqualFold(path.Ext(rel), ".md") {
				text := vault.Rewrite(rel, string(data), replace)
				data = []byte(stamp(text, [][2]string{{ProjectKey, m.ID}, {MirrorOfKey, rel}}))
				m.Threads++
			}
			thr[dest] = data
			continue
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
				return replace(resolved)
			})
		}
		wiki[dest] = data
	}
	return nil
}

// movedInto maps each member page whose content moved into hub to its page there: the
// pages whose moved_to_project is the hub's id and whose moved_to names a page the hub
// holds under wiki/. A pointer to a page the hub does not hold is mirrored as written.
func movedInto(hub *project.Project, vault *lint.Vault) map[string]string {
	moved := map[string]string{}
	for _, pg := range vault.Pages() {
		if project.StringField(pg.Fields, MovedToProjectKey) != hub.Config.ID {
			continue
		}
		to := project.StringField(pg.Fields, MovedToKey)
		if to == "" || !strings.HasPrefix(to, project.WikiDir+"/") || lint.Mirrored(to) {
			continue
		}
		if _, err := os.Stat(hub.Path(to)); err != nil {
			continue
		}
		moved[pg.Path] = to
	}
	return moved
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

// mirrors is what the hub's mirror folders hold now.
type mirrors struct {
	wiki    map[string][]byte
	threads map[string][]byte
	roots   map[string]string // folder -> the project id its wiki root page names
}

func (m mirrors) folderOf(id string) string {
	for folder, named := range m.roots {
		if named == id {
			return folder
		}
	}
	return ""
}

// mirrorFolder is the member folder a path under dir lies in.
func mirrorFolder(dir, rel string) string {
	rest := strings.TrimPrefix(rel, dir+"/")
	folder, _, _ := strings.Cut(rest, "/")
	return folder
}

// existingMirrors reads every file under wiki/projects/ and threads/projects/, and the id
// each wiki mirror's root page names.
func existingMirrors(p *project.Project) (mirrors, error) {
	out := mirrors{wiki: map[string][]byte{}, threads: map[string][]byte{}, roots: map[string]string{}}
	if err := readTree(p, project.MirrorDir, out.wiki); err != nil {
		return out, err
	}
	if err := readTree(p, project.ThreadMirrorDir, out.threads); err != nil {
		return out, err
	}
	for rel, data := range out.wiki {
		folder := mirrorFolder(project.MirrorDir, rel)
		if rel == project.MirrorDir+"/"+folder+"/"+folder+".md" {
			if fields, _, err := project.Frontmatter(string(data)); err == nil && fields != nil {
				out.roots[folder] = project.StringField(fields, ProjectKey)
			}
		}
	}
	return out, nil
}

// readTree reads every regular file under dir, relative to the project folder, into files.
func readTree(p *project.Project, dir string, files map[string][]byte) error {
	root := p.Path(dir)
	return filepath.WalkDir(root, func(fp string, d fs.DirEntry, err error) error {
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
		data, err := os.ReadFile(fp)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
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
