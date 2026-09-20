// Package registry knows every project the atlas config lists: it reads each one's
// identity file. Nothing here is stored beyond what Write derives, and Scan rebuilds the
// whole picture from the folders themselves every time.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

// The reasons an entry the atlas knows cannot be read. An Entry with an Error carries
// one, so a command decides on the code and not on the sentence.
const (
	ReasonUnreadable = "unreadable"  // the identity file is not JSON
	ReasonSchema     = "schema"      // an identity file from a later version
	ReasonMissing    = "missing"     // a registered path whose folder is gone
	ReasonNotProject = "not-project" // a registered work folder with no atlas/<name>/project.json
	ReasonFlat       = "flat"        // a project directly in atlas/; upgrade moves it into atlas/<name>/
	ReasonV3Split    = "v3-split"    // a 3.x project, or the knowledge base it used; upgrade merges them
)

// Entry is one project the atlas knows. Refresh adds the derived State.
type Entry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Path is the project's work folder, the parent of atlas/.
	Path        string       `json:"path"`
	Created     string       `json:"created,omitempty"`
	Mode        project.Mode `json:"mode,omitempty"`
	Description string       `json:"description,omitempty"`
	// Error is set for an entry the atlas knows but could not read. Such an entry has
	// Path, Error, and Reason, and nothing else.
	Error string `json:"error,omitempty"`
	// Reason is the code behind Error.
	Reason string `json:"reason,omitempty"`
	State  *State `json:"state,omitempty"`
}

// State is what refresh derived for one entry; the refresh package fills it.
type State struct {
	GeneratedAt string `json:"generated_at"`
	// OK is set when the entry could be read in full.
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// The wiki's state.
	PendingRecovery bool       `json:"pending_recovery,omitempty"`
	LastOperation   string     `json:"last_operation,omitempty"`
	Pages           *int       `json:"pages,omitempty"`
	Inbox           *int       `json:"inbox,omitempty"`
	HotTopics       []string   `json:"hot_topics,omitempty"`
	Unfinished      Unfinished `json:"unfinished"`
	LastTouched     string     `json:"last_touched,omitempty"`
	DaysIdle        *int       `json:"days_idle"`
	Heat            string     `json:"heat"`
	// The threads' state.
	Threads *ThreadSummary `json:"threads,omitempty"`
	// Described is the page in the project's wiki that describes its work, when one
	// does.
	Described *Description `json:"described,omitempty"`
	// Git is what git says about the work folder, when it is a repository.
	Git *links.Link `json:"git,omitempty"`
}

// Description is the page that describes a project's work: an entity page in its own wiki
// whose `project` property names the project's id or name, and the commit it was written
// from when the work is a repository. The atlas derives it and never writes it.
type Description struct {
	Page   string `json:"page"` // vault-relative path of the page
	Commit string `json:"commit,omitempty"`
	// Behind counts the commits on the work's current branch since Commit; -1 when
	// Commit is empty, the work is not a repository, or Commit is not in its history.
	Behind int `json:"behind"`
}

// Summary says where the page is and how current it is: "described in
// wiki/entities/webapp.md at fc70d93, 12 commits behind".
func (d Description) Summary() string {
	commit := d.Commit
	if len(commit) > 7 {
		commit = commit[:7]
	}
	where := "described in " + d.Page
	switch {
	case commit == "":
		return where
	case d.Behind < 0:
		return fmt.Sprintf("%s at %s, not in the repository's history", where, commit)
	case d.Behind == 0:
		return fmt.Sprintf("%s at %s, current", where, commit)
	case d.Behind == 1:
		return fmt.Sprintf("%s at %s, 1 commit behind", where, commit)
	}
	return fmt.Sprintf("%s at %s, %d commits behind", where, commit, d.Behind)
}

// NotDescribed is what every surface says for a project no page describes.
const NotDescribed = "not described in the wiki"

// Unfinished counts work a wiki still owes. nil means unknown.
type Unfinished struct {
	EmptySections *int `json:"empty_sections"`
	Stubs         *int `json:"stubs"`
	WantedPages   *int `json:"wanted_pages"`
	DeadLinks     *int `json:"dead_links"`
}

func (u Unfinished) counts() []struct {
	label string
	n     *int
} {
	return []struct {
		label string
		n     *int
	}{{"empty sections", u.EmptySections}, {"stubs", u.Stubs}, {"wanted pages", u.WantedPages}, {"dead links", u.DeadLinks}}
}

// Text lists the known counts with their labels, such as "1 empty sections · 2 stubs".
func (u Unfinished) Text() string {
	var bits []string
	for _, c := range u.counts() {
		if c.n != nil {
			bits = append(bits, fmt.Sprintf("%d %s", *c.n, c.label))
		}
	}
	return strings.Join(bits, " · ")
}

// Total adds the known counts; nil when none is known.
func (u Unfinished) Total() *int {
	var total *int
	for _, c := range u.counts() {
		if c.n != nil {
			if total == nil {
				total = new(int)
			}
			*total += *c.n
		}
	}
	return total
}

// ThreadLine is one open thread as the atlas shows it.
type ThreadLine struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Stage    string `json:"stage"`
	Priority string `json:"priority"`
	Phase    string `json:"phase,omitempty"`
	Blocked  string `json:"blocked,omitempty"`
	Updated  string `json:"updated"`
	Path     string `json:"path"` // absolute path of the document of the thread's stage
	Stale    bool   `json:"stale,omitempty"`
}

// ThreadSummary is what refresh read from a project's threads.
type ThreadSummary struct {
	Counts threads.Counts `json:"counts"`
	Open   []ThreadLine   `json:"open"`
	// Phases lists the phases in order, finished ones last.
	Phases []string `json:"phases,omitempty"`
}

// Problem is a path the scan could not use.
type Problem struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Index is one scan.
type Index struct {
	Entries  []Entry
	Problems []Problem
}

var ErrAmbiguous = errors.New("ambiguous")
var ErrNotFound = errors.New("no such vault or project")

// ErrStale says the registry state file is one this version cannot read: another
// schema, or not JSON. The file is derived, so a refresh replaces it.
var ErrStale = errors.New("stale registry")

// Scan reads atlas/<name>/project.json under every work folder cfg.Projects lists. It
// searches no folder: a project the config does not list is not in the atlas. A knowledge
// base left in cfg.Knowledge by 3.x is reported as a problem, because upgrade absorbs it
// into a project.
func Scan(cfg *home.Config) (*Index, error) {
	ix := &Index{}
	found := map[string]bool{}

	for _, work := range cfg.Projects {
		abs, err := filepath.Abs(work)
		if err != nil {
			abs = work
		}
		key := realPath(abs)
		if found[key] {
			continue
		}
		found[key] = true
		scanProject(ix, abs)
	}

	for _, v := range cfg.Knowledge {
		abs, err := filepath.Abs(v)
		if err != nil {
			abs = v
		}
		key := realPath(abs)
		if found[key] {
			continue
		}
		found[key] = true
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			fail(ix, abs, "a knowledge base of 3.x whose folder is gone; run atlas-obsidian forget "+abs, ReasonMissing)
			continue
		}
		fail(ix, abs, "a knowledge base of 3.x; run atlas-obsidian upgrade "+abs+" to make it a project, or atlas-obsidian forget to drop it", ReasonV3Split)
	}

	sortEntries(ix.Entries)
	return ix, nil
}

// realPath is the dedupe key of a root: two spellings of one folder, one of them through
// a symlink, resolve to the same key and yield one entry. A path that cannot be resolved,
// such as a registered folder that is gone, keeps its own spelling.
func realPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// scanProject reads atlas/<name>/project.json under a registered work folder.
func scanProject(ix *Index, work string) {
	info, err := os.Stat(work)
	if err != nil || !info.IsDir() {
		fail(ix, work, "not found; work in it again to heal the path, or run atlas-obsidian forget", ReasonMissing)
		return
	}
	if _, err := project.Locate(work); err != nil {
		switch {
		case errors.Is(err, project.ErrFlat):
			fail(ix, work, "the project sits directly in "+project.Dir+"/; run atlas-obsidian upgrade "+work, ReasonFlat)
		case errors.Is(err, project.ErrNotProject):
			fail(ix, work, "no "+project.Dir+"/<name>/"+project.Marker+"; run atlas-obsidian init there, or atlas-obsidian forget", ReasonNotProject)
		default:
			fail(ix, work, err.Error(), ReasonUnreadable)
		}
		return
	}
	cfg, ok := project.ReadConfig(work)
	if !ok {
		fail(ix, work, "identity file is not JSON", ReasonUnreadable)
		return
	}
	switch {
	case project.Current(cfg.Schema):
	case cfg.Schema == project.SchemaV3:
		fail(ix, work, "a 3.x project, whose knowledge base sits outside it; run atlas-obsidian upgrade "+work, ReasonV3Split)
		return
	default:
		fail(ix, work, fmt.Sprintf("unsupported schema %q", cfg.Schema), ReasonSchema)
		return
	}
	if cfg.ID == "" {
		fail(ix, work, "identity file has no id", ReasonUnreadable)
		return
	}
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		name = filepath.Base(work)
	}
	mode := cfg.Mode
	if mode == "" {
		mode = project.Generic
	}
	ix.Entries = append(ix.Entries, Entry{ID: cfg.ID, Name: name, Path: work, Created: cfg.Created, Mode: mode, Description: cfg.Description})
}

// fail records an entry the atlas knows but could not read: the same reason in both a
// Problem and an Entry{Path, Error, Reason}.
func fail(ix *Index, root, reason, code string) {
	ix.Problems = append(ix.Problems, Problem{Path: root, Reason: reason})
	ix.Entries = append(ix.Entries, Entry{Path: root, Error: reason, Reason: code})
}

// sortEntries orders valid entries before entries with an Error, then by lowercased name,
// then by path.
func sortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if (a.Error == "") != (b.Error == "") {
			return a.Error == ""
		}
		if a.Error != "" {
			return a.Path < b.Path
		}
		an, bn := strings.ToLower(a.Name), strings.ToLower(b.Name)
		if an != bn {
			return an < bn
		}
		return a.Path < b.Path
	})
}

// ByID finds an entry by its id.
func (ix *Index) ByID(id string) *Entry {
	for i := range ix.Entries {
		if ix.Entries[i].ID == id {
			return &ix.Entries[i]
		}
	}
	return nil
}

// ByPath finds an entry by its root path. A path that reaches the folder through a
// symlinked parent, which is what a session hands the hooks and the server on macOS,
// where /tmp links to /private/tmp, matches the entry it resolves to.
func (ix *Index) ByPath(path string) *Entry {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	for i := range ix.Entries {
		if ix.Entries[i].Path == abs {
			return &ix.Entries[i]
		}
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil
	}
	for i := range ix.Entries {
		if entry, err := filepath.EvalSymlinks(ix.Entries[i].Path); err == nil && entry == resolved {
			return &ix.Entries[i]
		}
	}
	return nil
}

// Find matches a name without regard to case, an id or an id prefix of at least 8
// characters, or a path. Two entries with one name make it return ErrAmbiguous with both.
func (ix *Index) Find(arg string) (*Entry, error) {
	if strings.ContainsAny(arg, "/\\") || strings.HasPrefix(arg, "~") {
		abs, err := filepath.Abs(home.Expand(arg))
		if err == nil {
			if e := ix.ByPath(abs); e != nil && e.Error == "" {
				return e, nil
			}
		}
		return nil, fmt.Errorf("%w: %s", ErrNotFound, arg)
	}
	var byName []*Entry
	for i := range ix.Entries {
		e := &ix.Entries[i]
		if e.Error != "" {
			continue
		}
		if strings.EqualFold(e.Name, arg) {
			byName = append(byName, e)
		}
	}
	if len(byName) == 1 {
		return byName[0], nil
	}
	if len(byName) > 1 {
		var paths []string
		for _, e := range byName {
			paths = append(paths, e.Path)
		}
		return nil, fmt.Errorf("%w: %s is the name of %d entries (%s); use the path or the id", ErrAmbiguous, arg, len(byName), strings.Join(paths, ", "))
	}
	var byID []*Entry
	for i := range ix.Entries {
		e := &ix.Entries[i]
		if e.Error != "" {
			continue
		}
		if e.ID == arg || (len(arg) >= 8 && strings.HasPrefix(e.ID, arg)) {
			byID = append(byID, e)
		}
	}
	if len(byID) == 1 {
		return byID[0], nil
	}
	if len(byID) > 1 {
		var paths []string
		for _, e := range byID {
			paths = append(paths, e.Path)
		}
		return nil, fmt.Errorf("%w: %s is the id of %d entries (%s); use the full id", ErrAmbiguous, arg, len(byID), strings.Join(paths, ", "))
	}
	return nil, fmt.Errorf("%w: no project named %s", ErrNotFound, arg)
}

// Projects lists the valid entries.
func (ix *Index) Projects() []Entry {
	var out []Entry
	for _, e := range ix.Entries {
		if e.Error == "" {
			out = append(out, e)
		}
	}
	return out
}

// Rel is the entry's place in the view: projects/<name>, or problems/<folder> for an entry
// the atlas could not read.
func (e Entry) Rel() string {
	if e.Error != "" {
		return "problems/" + filepath.Base(e.Path)
	}
	return "projects/" + e.Name
}

// Atlas is the project's own folder, atlas/<name>/, or "" when the scan could not read it.
func (e Entry) Atlas() string {
	if e.Error != "" {
		return ""
	}
	folder, err := project.Locate(e.Path)
	if err != nil {
		return ""
	}
	return filepath.Join(e.Path, project.Dir, folder)
}

// Wiki is the project's wiki folder.
func (e Entry) Wiki() string { return filepath.Join(e.Atlas(), project.WikiDir) }

// StateSchema is the schema the registry state file declares.
const StateSchema = "atlas-obsidian.registry.v4"

// registryFile is the on-disk shape of the state file.
type registryFile struct {
	Schema      string  `json:"schema"`
	GeneratedAt string  `json:"generated_at"`
	Entries     []Entry `json:"entries"`
}

// File is the registry state file under stateDir.
func File(stateDir string) string { return filepath.Join(stateDir, "registry.json") }

// Write records entries atomically: a temp file in stateDir, then a rename.
func Write(stateDir string, entries []Entry, generatedAt string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(registryFile{Schema: StateSchema, GeneratedAt: generatedAt, Entries: entries}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(stateDir, "registry-*.json.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, File(stateDir)); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// Read loads the registry state file. It reports os.ErrNotExist when refresh has never
// run, and ErrStale when the file is not one this version reads.
func Read(stateDir string) ([]Entry, string, error) {
	data, err := os.ReadFile(File(stateDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", fmt.Errorf("%s: %w", File(stateDir), os.ErrNotExist)
	}
	if err != nil {
		return nil, "", err
	}
	var doc registryFile
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, "", fmt.Errorf("%w: %s: %v", ErrStale, File(stateDir), err)
	}
	if doc.Schema != StateSchema {
		return nil, "", fmt.Errorf("%w: %s: unsupported schema %q; run atlas-obsidian refresh", ErrStale, File(stateDir), doc.Schema)
	}
	return doc.Entries, doc.GeneratedAt, nil
}
