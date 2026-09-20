// Package mcpserver exposes the core to Claude Code as MCP tools. One server process
// lives for one session and holds the plans the model has built but not yet applied.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/atlas-obsidian/internal/capture"
	"github.com/nathanaday/atlas-obsidian/internal/describe"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/ledger"
	"github.com/nathanaday/atlas-obsidian/internal/links"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/place"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
	"github.com/nathanaday/atlas-obsidian/internal/txn"
)

// Name is the MCP server name; Claude Code exposes tools as mcp__plugin_atlas-obsidian_atlas__<tool>.
const Name = "atlas"

// Options configure a server.
type Options struct {
	// Version of the binary.
	Version string
	// PluginRoot is ${CLAUDE_PLUGIN_ROOT}; used to read the plugin's version.
	PluginRoot string
	// ProjectDir is where the session started; the place is found by walking up from it.
	ProjectDir string
	// Env resolves environment variables. nil means os.Getenv.
	Env func(string) string
	// Now returns the current time. nil means time.Now.
	Now func() time.Time
}

// Server holds session state.
type Server struct {
	opts  Options
	mu    sync.Mutex
	plans map[string]*txn.Plan
}

// New builds a server.
func New(opts Options) *Server {
	if opts.Env == nil {
		opts.Env = os.Getenv
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Server{opts: opts, plans: map[string]*txn.Plan{}}
}

// home is the atlas home the session reads.
func (s *Server) home() home.Home { return home.Resolve(s.opts.Env(home.EnvHome)) }

// where finds the session's project once per call, anywhere inside its work. Nothing is
// registered here; the session-start hook heals the config.
func (s *Server) where() (*place.Place, error) {
	return place.Resolve(s.home(), "", s.opts.Env(place.EnvPlace), s.opts.ProjectDir, false)
}

// place is the project a call acts on, with what the atlas knows about it.
func (s *Server) place() (*place.Place, *project.Project, error) {
	pl, err := s.where()
	if err != nil {
		return nil, nil, err
	}
	return pl, pl.Project, nil
}

// projectOf resolves the project a thread tool acts on: the session's own when arg is
// empty, else one the atlas knows by name, id, or path.
func (s *Server) projectOf(pl *place.Place, arg string) (*project.Project, *registry.Entry, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return pl.Project, pl.Entry, nil
	}
	if pl.Index == nil {
		return nil, nil, errors.New("no atlas config on this machine; run atlas-obsidian setup")
	}
	e, err := pl.Index.Find(arg)
	if err != nil {
		return nil, nil, err
	}
	p, err := project.Open(e.Path)
	if err != nil {
		return nil, nil, err
	}
	return p, e, nil
}

func (s *Server) pluginVersion() string {
	if s.opts.PluginRoot == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(s.opts.PluginRoot, ".claude-plugin", "plugin.json"))
	if err != nil {
		return ""
	}
	var m struct {
		Version string `json:"version"`
	}
	json.Unmarshal(data, &m)
	return m.Version
}

// Empty is the argument of a tool that takes none.
type Empty struct{}

// GitInfo is what git says about a project's work folder.
type GitInfo struct {
	Branch string `json:"branch,omitempty"`
	Dirty  int    `json:"dirty"`
}

// DescribedInfo is the page that describes the work, and how current it is.
type DescribedInfo struct {
	registry.Description
	Summary string `json:"summary"`
}

// ProjectThreads is a project's thread counts and its phases in order.
type ProjectThreads struct {
	Counts threads.Counts `json:"counts"`
	Phases []string       `json:"phases"`
}

// Versions are the binary's and the plugin's.
type Versions struct {
	Binary string `json:"binary"`
	Plugin string `json:"plugin,omitempty"`
}

// Status is the status tool's output: the project, its wiki, and its threads.
type Status struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Atlas       string `json:"atlas"`
	Description string `json:"description,omitempty"`
	Mode        string `json:"mode"`
	// The work.
	Git       *GitInfo       `json:"git,omitempty"`
	Described *DescribedInfo `json:"described,omitempty"`
	// The threads.
	Threads *ProjectThreads `json:"threads,omitempty"`
	// The wiki.
	Pages         int            `json:"pages"`
	Stubs         int            `json:"stubs"`
	WantedPages   int            `json:"wanted_pages"`
	WikiGit       *txn.Status    `json:"wiki_git,omitempty"`
	LastOperation *txn.Operation `json:"last_operation,omitempty"`
	// The inbox, split by what each file looks like.
	InboxSources int `json:"inbox_sources"`
	InboxNotes   int `json:"inbox_notes"`

	PendingRecovery bool     `json:"pending_recovery"`
	Versions        Versions `json:"versions"`
	Warnings        []string `json:"warnings"`
}

func (s *Server) status(ctx context.Context, req *mcp.CallToolRequest, a Empty) (*mcp.CallToolResult, Status, error) {
	pl, p, err := s.place()
	if err != nil {
		return nil, Status{}, err
	}
	now := s.opts.Now()
	out := Status{Warnings: []string{}, Versions: Versions{Binary: s.opts.Version, Plugin: s.pluginVersion()}}
	out.ID, out.Name, out.Path, out.Atlas = p.Config.ID, p.Name(), p.Root, p.Atlas()
	out.Description, out.Mode = p.Config.Description, string(p.Config.Mode)
	if fact := links.Inspect(links.Repo, p.Root); fact.OK {
		git := &GitInfo{Branch: fact.Branch}
		if fact.Dirty != nil {
			git.Dirty = *fact.Dirty
		}
		out.Git = git
	}
	if pl.Entry != nil {
		if d := describe.Page(*pl.Entry); d != nil {
			out.Described = &DescribedInfo{Description: *d, Summary: d.Summary()}
			if d.Behind > describe.BehindThreshold {
				out.Warnings = append(out.Warnings, fmt.Sprintf("the page describing this work is %d commits behind; the describe skill brings it up to date", d.Behind))
			}
		} else {
			out.Warnings = append(out.Warnings, registry.NotDescribed+"; the describe skill writes the page")
		}
	}
	if st, err := txn.Inspect(p); err == nil {
		out.WikiGit = st
		if !st.HasHistory {
			out.Warnings = append(out.Warnings, "the project has no git history, so no operation can run; make "+home.Display(p.Root)+" a git repository")
		}
	}
	if report, err := lint.Run(p.Atlas(), lint.Options{AsOf: now}); err == nil {
		out.Pages = report.Summary.PagesScanned
		out.Stubs = len(report.Stubs)
		out.WantedPages = len(report.WantedPages)
	}
	if files, err := capture.ListInbox(p, now); err == nil {
		for _, f := range files {
			switch {
			case f.Captured:
			case capture.LooksLikeNote(f):
				out.InboxNotes++
			default:
				out.InboxSources++
			}
		}
	}
	if ops, err := txn.History(p, 1, false); err == nil && len(ops) > 0 {
		out.LastOperation = &ops[0]
	}
	if threads.Legacy(p) {
		out.Warnings = append(out.Warnings, "this project holds task pages from before threads; `atlas-obsidian upgrade` turns each one into a thread")
	}
	if board, err := threads.Load(p); err == nil {
		pt := &ProjectThreads{Counts: board.Counts(now), Phases: []string{}}
		pt.Counts.Notes = out.InboxNotes
		for _, ph := range board.Phases {
			pt.Phases = append(pt.Phases, ph.Title)
		}
		out.Threads = pt
		var stale []string
		for _, t := range board.Open() {
			if threads.Stale(t, now) {
				stale = append(stale, t.Title)
			}
		}
		if len(stale) > 0 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%d thread%s with a plan untouched for %d days: %s", len(stale), plural(len(stale)), threads.StaleDays, strings.Join(stale, "; ")))
		}
		for _, pr := range board.Problems {
			out.Warnings = append(out.Warnings, pr.Path+": "+pr.Reason)
		}
	}
	if out.InboxNotes > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("%d note%s wait in %s/%s/; the thread-stub skill opens a thread from each", out.InboxNotes, plural(out.InboxNotes), p.Rel(), project.InboxDir))
	}
	if pending, _ := txn.Pending(p); pending != nil {
		out.PendingRecovery = true
		out.Warnings = append(out.Warnings, "an operation was interrupted; run `atlas-obsidian recover "+p.Root+"` before changing the wiki")
	}
	if out.Versions.Plugin != "" && out.Versions.Binary != "dev" && out.Versions.Plugin != out.Versions.Binary {
		out.Warnings = append(out.Warnings, fmt.Sprintf("plugin %s and binary %s differ; update one of them", out.Versions.Plugin, out.Versions.Binary))
	}
	return nil, out, nil
}

// InboxOut is what waits in the project's one inbox. Each file carries a hint, source or
// note, which says which skill takes it.
type InboxOut struct {
	Project string              `json:"project"`
	Files   []capture.InboxFile `json:"files"`
	Next    string              `json:"next"`
}

// inboxNext says how the two kinds of inbox file are handled.
const inboxNext = "A file hinted source is ingested into the wiki (the wiki-ingest skill); one hinted note opens a thread (the thread-stub skill, with from set to the file). The hint is a guess from the file's kind and size: say what you will do with each file before you do it."

func (s *Server) inbox(ctx context.Context, req *mcp.CallToolRequest, a Empty) (*mcp.CallToolResult, InboxOut, error) {
	_, p, err := s.place()
	if err != nil {
		return nil, InboxOut{}, err
	}
	files, err := capture.ListInbox(p, s.opts.Now())
	if err != nil {
		return nil, InboxOut{}, err
	}
	return nil, InboxOut{Project: p.Root, Files: files, Next: inboxNext}, nil
}

type CaptureArgs struct {
	Paths []string `json:"paths" jsonschema:"files in the project's inbox/ to capture, as inbox-relative or project-relative paths"`
}

func (s *Server) capture(ctx context.Context, req *mcp.CallToolRequest, a CaptureArgs) (*mcp.CallToolResult, capture.Result, error) {
	_, v, err := s.place()
	if err != nil {
		return nil, capture.Result{}, err
	}
	res, err := capture.Capture(v, a.Paths, nil, s.opts.Now())
	if err != nil {
		return nil, capture.Result{}, err
	}
	return nil, *res, nil
}

type RouteArgs struct {
	Type  string `json:"type" jsonschema:"page type: source, entity, concept; in lyt mode also note or moc"`
	Title string `json:"title" jsonschema:"the page title; it becomes the file name"`
}

// routeNext tells the model what a match means: a reason to link, not to duplicate.
const routeNext = "A match means link to it instead of creating a page."

// RouteOut is where a page belongs and whether one by that title or alias already exists.
type RouteOut struct {
	project.Route
	Project string         `json:"project"`
	Match   *project.Match `json:"match,omitempty"`
	Next    string         `json:"next"`
}

func (s *Server) route(ctx context.Context, req *mcp.CallToolRequest, a RouteArgs) (*mcp.CallToolResult, RouteOut, error) {
	_, v, err := s.place()
	if err != nil {
		return nil, RouteOut{}, err
	}
	r, err := v.RouteFor(a.Type, a.Title, s.opts.Now())
	if err != nil {
		return nil, RouteOut{}, err
	}
	match, err := project.FindPage(v.Atlas(), a.Title)
	if err != nil {
		return nil, RouteOut{}, err
	}
	return nil, RouteOut{Route: *r, Project: v.Root, Match: match, Next: routeNext}, nil
}

type StubArgs struct {
	Titles []txn.StubTitle `json:"titles,omitempty" jsonschema:"the pages to stub; omit to stub every wanted page and every empty page a link points to"`
	Type   string          `json:"type,omitempty" jsonschema:"the type for titles that name none: concept or entity; in lyt mode note or moc as well; the default is concept, or note in lyt mode"`
}

func (s *Server) stub(ctx context.Context, req *mcp.CallToolRequest, a StubArgs) (*mcp.CallToolResult, txn.StubResult, error) {
	_, v, err := s.place()
	if err != nil {
		return nil, txn.StubResult{}, err
	}
	res, err := txn.StubPages(v, a.Titles, a.Type, s.opts.Now())
	if err != nil {
		return nil, txn.StubResult{}, err
	}
	if res.Stubs == nil {
		res.Stubs = []txn.Stubbed{}
	}
	return nil, res, nil
}

// ProjectArg names the project a thread tool acts on.
type ProjectArg struct {
	Project string `json:"project,omitempty" jsonschema:"another project the atlas lists, by name, id, or path; omit for this session's project"`
}

// ThreadInfo is a thread with the absolute path of its card and of each document.
type ThreadInfo struct {
	threads.Thread
	Stale bool `json:"stale,omitempty"`
	// Card is the card's absolute path; code owns the card.
	Card string `json:"card"`
	// Files maps each stage that has a document to its absolute path.
	Files map[string]string `json:"files"`
}

func threadInfo(p *project.Project, t threads.Thread, now time.Time) ThreadInfo {
	info := ThreadInfo{Thread: t, Stale: threads.Stale(t, now), Card: p.Path(t.Path), Files: map[string]string{}}
	for _, d := range t.Docs {
		info.Files[d.Stage] = p.Path(d.Path)
	}
	return info
}

func threadInfos(p *project.Project, list []threads.Thread, now time.Time) []ThreadInfo {
	out := make([]ThreadInfo, 0, len(list))
	for _, t := range list {
		out = append(out, threadInfo(p, t, now))
	}
	return out
}

// PhaseInfo is one phase as the threads tool lists it.
type PhaseInfo struct {
	Title    string `json:"title"`
	Order    int    `json:"order"`
	Finished bool   `json:"finished"`
	Open     int    `json:"open"`
	Path     string `json:"path"`
}

// ProjectBoard is one project's threads.
type ProjectBoard struct {
	Name     string            `json:"name"`
	Path     string            `json:"path"`
	Atlas    string            `json:"atlas"`
	Board    string            `json:"board"`
	Counts   threads.Counts    `json:"counts"`
	Phases   []PhaseInfo       `json:"phases"`
	Open     []ThreadInfo      `json:"open"`
	Closed   []ThreadInfo      `json:"closed"`
	Problems []threads.Problem `json:"problems,omitempty"`
	Notes    []string          `json:"notes"`
}

// ThreadsOut is one board per project.
type ThreadsOut struct {
	Projects []ProjectBoard `json:"projects"`
}

type ThreadsArgs struct {
	ProjectArg
	ID string `json:"id,omitempty" jsonschema:"only this thread, by its id, its title, or the start of its title"`
}

func (s *Server) threads(ctx context.Context, req *mcp.CallToolRequest, a ThreadsArgs) (*mcp.CallToolResult, ThreadsOut, error) {
	pl, err := s.where()
	if err != nil {
		return nil, ThreadsOut{}, err
	}
	out := ThreadsOut{Projects: []ProjectBoard{}}
	p, _, err := s.projectOf(pl, a.Project)
	if err != nil {
		return nil, ThreadsOut{}, err
	}
	projects := []*project.Project{p}
	now := s.opts.Now()
	for _, p := range projects {
		board, err := threads.Load(p)
		if err != nil {
			return nil, ThreadsOut{}, err
		}
		pb := ProjectBoard{Name: p.Name(), Path: p.Root, Atlas: p.Atlas(), Board: p.Path(project.ThreadsIndex), Counts: board.Counts(now), Phases: []PhaseInfo{}, Problems: board.Problems, Notes: threads.Notes(p)}
		pb.Counts.Notes = len(pb.Notes)
		if pb.Notes == nil {
			pb.Notes = []string{}
		}
		for _, ph := range board.Phases {
			pb.Phases = append(pb.Phases, PhaseInfo{Title: ph.Title, Order: ph.Order, Finished: board.Finished(ph.Title), Open: len(board.In(ph.Title)), Path: ph.Path})
		}
		if a.ID != "" {
			t, err := board.Resolve(a.ID)
			if err != nil {
				if len(projects) > 1 {
					continue
				}
				return nil, ThreadsOut{}, err
			}
			one := threadInfos(p, []threads.Thread{*t}, now)
			pb.Open, pb.Closed = one, []ThreadInfo{}
			if t.Closed() {
				pb.Open, pb.Closed = []ThreadInfo{}, one
			}
		} else {
			pb.Open, pb.Closed = threadInfos(p, board.Open(), now), threadInfos(p, board.Closed(), now)
		}
		out.Projects = append(out.Projects, pb)
	}
	return nil, out, nil
}

type ThreadArgs struct {
	ProjectArg
	ID       string  `json:"id,omitempty" jsonschema:"the thread, by its id, its title, or the start of its title; omit to open a new thread"`
	Title    *string `json:"title,omitempty" jsonschema:"a new thread's title, taken from the text when omitted; on an existing thread, a new title, which renames its card and documents"`
	Stage    string  `json:"stage,omitempty" jsonschema:"on an existing thread, the document to file: spec, plan, or receipt (stub, when the thread lost its own). Filing it moves the thread to that stage. A stage may be skipped"`
	Text     string  `json:"text,omitempty" jsonschema:"the document's text in markdown, without frontmatter: on a new thread the stub, in the user's words; with stage, that document. Revise a document that exists with Edit"`
	Outcome  string  `json:"outcome,omitempty" jsonschema:"with stage receipt: completed or killed"`
	Priority *string `json:"priority,omitempty" jsonschema:"high, normal, low, or someday; default normal"`
	Phase    *string `json:"phase,omitempty" jsonschema:"the phase the thread belongs to, by title; it must exist; an empty string clears it"`
	Blocked  *string `json:"blocked,omitempty" jsonschema:"what the thread waits on, in one line; an empty string unblocks it"`
	From     string  `json:"from,omitempty" jsonschema:"a new thread only: the note under atlas/<name>/inbox/ it comes from, relative to the project folder; its content is the stub when text is omitted, and it is removed once the stub exists"`
	Reopen   bool    `json:"reopen,omitempty" jsonschema:"delete a closed thread's receipt, which opens the thread again at the stage before it"`
}

// ThreadOut is the thread after the change.
type ThreadOut struct {
	ThreadInfo
	Project string `json:"project"`
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *Server) thread(ctx context.Context, req *mcp.CallToolRequest, a ThreadArgs) (*mcp.CallToolResult, ThreadOut, error) {
	pl, err := s.where()
	if err != nil {
		return nil, ThreadOut{}, err
	}
	p, _, err := s.projectOf(pl, a.Project)
	if err != nil {
		return nil, ThreadOut{}, err
	}
	now := s.opts.Now()
	id := strings.TrimSpace(a.ID)
	var t *threads.Thread
	switch {
	case id == "":
		if a.Stage != "" && a.Stage != threads.Stub || a.Outcome != "" || a.Reopen {
			return nil, ThreadOut{}, errors.New("a new thread starts with its stub; pass id to file a later document on a thread")
		}
		t, err = threads.Start(p, threads.New{Title: deref(a.Title), Text: a.Text, Priority: deref(a.Priority), Phase: deref(a.Phase), From: a.From}, now)
		if err == nil && a.Blocked != nil {
			t, err = threads.Set(p, t.ID, threads.Changes{Blocked: a.Blocked}, now)
		}
	case a.From != "":
		return nil, ThreadOut{}, errors.New("from opens a new thread; omit id")
	default:
		ch := threads.Changes{Title: a.Title, Priority: a.Priority, Phase: a.Phase, Blocked: a.Blocked}
		if a.Stage == "" && a.Text != "" {
			return nil, ThreadOut{}, errors.New("text needs stage: name the document to file; revise one that exists with Edit")
		}
		if a.Reopen {
			if t, err = threads.Reopen(p, id, now); err != nil {
				return nil, ThreadOut{}, err
			}
			id = t.ID
		}
		if a.Stage != "" {
			if t, err = threads.File(p, id, threads.Filing{Stage: a.Stage, Text: a.Text, Outcome: a.Outcome}, now); err != nil {
				return nil, ThreadOut{}, err
			}
			id = t.ID
		} else if a.Outcome != "" {
			return nil, ThreadOut{}, errors.New("outcome needs stage: receipt")
		}
		// With nothing else to do, Set marks the thread as touched today.
		if !ch.Empty() || t == nil {
			t, err = threads.Set(p, id, ch, now)
		}
	}
	if err != nil {
		return nil, ThreadOut{}, err
	}
	return nil, ThreadOut{ThreadInfo: threadInfo(p, *t, now), Project: p.Name()}, nil
}

type PhaseArgs struct {
	ProjectArg
	Action   string `json:"action" jsonschema:"create, rename, reorder, or remove"`
	Title    string `json:"title" jsonschema:"the phase's title; on rename, the current one"`
	Goal     string `json:"goal,omitempty" jsonschema:"create: what the phase delivers, in the user's words"`
	Order    *int   `json:"order,omitempty" jsonschema:"create, reorder: its place in the timeline; create takes the next one when omitted"`
	NewTitle string `json:"new_title,omitempty" jsonschema:"rename: the new title; every thread that names the phase follows"`
}

// PhaseOut is the phase after the change, or what remove dropped.
type PhaseOut struct {
	Project string         `json:"project"`
	Phase   *threads.Phase `json:"phase,omitempty"`
	File    string         `json:"file,omitempty"`
	Removed string         `json:"removed,omitempty"`
}

func (s *Server) phase(ctx context.Context, req *mcp.CallToolRequest, a PhaseArgs) (*mcp.CallToolResult, PhaseOut, error) {
	pl, err := s.where()
	if err != nil {
		return nil, PhaseOut{}, err
	}
	p, _, err := s.projectOf(pl, a.Project)
	if err != nil {
		return nil, PhaseOut{}, err
	}
	now := s.opts.Now()
	var ph *threads.Phase
	switch a.Action {
	case "create":
		ph, err = threads.CreatePhase(p, a.Title, a.Goal, a.Order, now)
	case "rename":
		ph, err = threads.RenamePhase(p, a.Title, a.NewTitle, now)
	case "reorder":
		if a.Order == nil {
			return nil, PhaseOut{}, errors.New("reorder needs order")
		}
		ph, err = threads.ReorderPhase(p, a.Title, *a.Order, now)
	case "remove":
		if err := threads.RemovePhase(p, a.Title, now); err != nil {
			return nil, PhaseOut{}, err
		}
		return nil, PhaseOut{Project: p.Name(), Removed: a.Title}, nil
	default:
		return nil, PhaseOut{}, fmt.Errorf("action must be create, rename, reorder, or remove, not %q", a.Action)
	}
	if err != nil {
		return nil, PhaseOut{}, err
	}
	return nil, PhaseOut{Project: p.Name(), Phase: ph, File: p.Path(ph.Path)}, nil
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

type PlanWrite struct {
	Path       string `json:"path" jsonschema:"vault-relative path, e.g. wiki/concepts/Contextual Retrieval.md"`
	Mode       string `json:"mode" jsonschema:"create, replace, or delete"`
	Content    string `json:"content,omitempty" jsonschema:"the complete new file content for create and replace"`
	BaseSHA256 string `json:"base_sha256,omitempty" jsonschema:"sha256 of the content you read, for replace and delete; omit to accept the current file"`
}

type PlanSource struct {
	ID        string   `json:"id" jsonschema:"source id from the capture or inbox tool"`
	Ingested  bool     `json:"ingested,omitempty" jsonschema:"true once the source's knowledge is in the wiki"`
	Pages     []string `json:"pages,omitempty" jsonschema:"wiki pages derived from this source"`
	Authority string   `json:"authority,omitempty" jsonschema:"official, primary, secondary, community, synthetic, or unknown"`
	Title     string   `json:"title,omitempty"`
	Notes     string   `json:"notes,omitempty"`
}

type PlanArgs struct {
	Kind    string       `json:"kind" jsonschema:"ingest, save, markdown, repair, fold, canvas, or base"`
	Summary string       `json:"summary" jsonschema:"one line saying what the operation does; it becomes the log entry and commit subject"`
	Writes  []PlanWrite  `json:"writes,omitempty"`
	Sources []PlanSource `json:"sources,omitempty" jsonschema:"ledger updates for sources this operation ingests"`
}

type PlanOut struct {
	PlanID      string      `json:"plan_id"`
	OperationID string      `json:"operation_id"`
	Project     string      `json:"project"`
	Kind        string      `json:"kind"`
	Summary     string      `json:"summary"`
	Preview     txn.Preview `json:"preview"`
	Warnings    []string    `json:"warnings"`
	Next        string      `json:"next"`
}

func (s *Server) plan(ctx context.Context, req *mcp.CallToolRequest, a PlanArgs) (*mcp.CallToolResult, PlanOut, error) {
	_, v, err := s.place()
	if err != nil {
		return nil, PlanOut{}, err
	}
	kind := txn.Kind(a.Kind)
	allowedKind := false
	for _, k := range txn.ModelKinds {
		if k == kind {
			allowedKind = true
		}
	}
	if !allowedKind {
		return nil, PlanOut{}, fmt.Errorf("kind must be one of ingest, save, markdown, repair, fold, canvas, base")
	}
	r := txn.Request{Kind: kind, Summary: a.Summary}
	for _, w := range a.Writes {
		r.Writes = append(r.Writes, txn.Write{Path: w.Path, Mode: txn.WriteMode(w.Mode), Content: []byte(w.Content), BaseSHA256: w.BaseSHA256})
	}
	for _, src := range a.Sources {
		r.Sources = append(r.Sources, ledger.Update{ID: src.ID, Ingested: src.Ingested, Pages: src.Pages, Authority: src.Authority, Title: src.Title, Notes: src.Notes})
	}
	plan, err := txn.Prepare(v, r, s.opts.Now())
	if err != nil {
		return nil, PlanOut{}, err
	}
	s.hold(plan)
	return nil, s.planOut(plan), nil
}

func (s *Server) hold(plan *txn.Plan) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, p := range s.plans {
		if p.Folder == plan.Folder {
			delete(s.plans, id)
		}
	}
	s.plans[plan.ID] = plan
}

func (s *Server) planOut(plan *txn.Plan) PlanOut {
	next := "Show the user this preview. If they approve, call apply with plan_id " + plan.ID + "."
	if len(plan.Warnings) > 0 {
		next = "Resolve or explain the warnings, show the user the preview, then call apply with plan_id " + plan.ID + " if they approve."
	}
	return PlanOut{PlanID: plan.ID, OperationID: plan.OperationID, Project: plan.Folder, Kind: string(plan.Kind), Summary: plan.Summary, Preview: plan.Preview, Warnings: plan.Warnings, Next: next}
}

type ApplyArgs struct {
	PlanID string `json:"plan_id" jsonschema:"the plan_id returned by plan"`
}

func (s *Server) apply(ctx context.Context, req *mcp.CallToolRequest, a ApplyArgs) (*mcp.CallToolResult, txn.Result, error) {
	s.mu.Lock()
	plan, ok := s.plans[a.PlanID]
	if ok {
		delete(s.plans, a.PlanID)
	}
	s.mu.Unlock()
	if !ok {
		return nil, txn.Result{}, fmt.Errorf("no plan %q is pending; plans are single-use and the newest plan replaces older ones, so call plan again", a.PlanID)
	}
	v, err := project.Open(project.FindAbove(plan.Folder))
	if err != nil {
		return nil, txn.Result{}, err
	}
	res, err := txn.Apply(v, plan, s.opts.Now())
	if err != nil {
		return nil, txn.Result{}, err
	}
	return nil, *res, nil
}

type UndoArgs struct {
	OperationID string `json:"operation_id" jsonschema:"the operation to revert, from history"`
}

func (s *Server) undo(ctx context.Context, req *mcp.CallToolRequest, a UndoArgs) (*mcp.CallToolResult, txn.Result, error) {
	_, v, err := s.place()
	if err != nil {
		return nil, txn.Result{}, err
	}
	res, err := txn.UndoOperation(v, a.OperationID, s.opts.Now())
	if err != nil {
		return nil, txn.Result{}, err
	}
	return nil, *res, nil
}

type HistoryArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"how many operations, newest first (default 10)"`
}

type HistoryOut struct {
	Project    string          `json:"project"`
	Operations []txn.Operation `json:"operations"`
}

func (s *Server) history(ctx context.Context, req *mcp.CallToolRequest, a HistoryArgs) (*mcp.CallToolResult, HistoryOut, error) {
	_, v, err := s.place()
	if err != nil {
		return nil, HistoryOut{}, err
	}
	limit := a.Limit
	if limit <= 0 {
		limit = 10
	}
	ops, err := txn.History(v, limit, true)
	if err != nil {
		return nil, HistoryOut{}, err
	}
	if ops == nil {
		ops = []txn.Operation{}
	}
	return nil, HistoryOut{Project: v.Root, Operations: ops}, nil
}

type LintArgs struct {
	Exclude []string `json:"exclude,omitempty" jsonschema:"path globs to leave out, e.g. wiki/scratch/*"`
}

func (s *Server) lint(ctx context.Context, req *mcp.CallToolRequest, a LintArgs) (*mcp.CallToolResult, lint.Report, error) {
	_, v, err := s.place()
	if err != nil {
		return nil, lint.Report{}, err
	}
	report, err := lint.Run(v.Atlas(), lint.Options{Exclude: a.Exclude, AsOf: s.opts.Now()})
	if err != nil {
		return nil, lint.Report{}, err
	}
	return nil, *report, nil
}

func ro() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true}
}

// MCP builds the protocol server with every tool registered.
func (s *Server) MCP() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: Name, Title: "atlas-obsidian", Version: s.opts.Version}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "status", Annotations: ro(),
		Description: "Describe the session's project: its folder, its description and mode, what git says about the work, the page that describes the work, its thread counts by stage and its phases, its wiki's page count and git state, what waits in its inbox, and warnings. Call this first."}, s.status)
	mcp.AddTool(server, &mcp.Tool{Name: "inbox", Annotations: ro(),
		Description: "List what waits in the project's inbox/ with size, kind, hash, whether each already has a captured copy, and a hint: source, to ingest into the wiki, or note, to open as a thread."}, s.inbox)
	mcp.AddTool(server, &mcp.Tool{Name: "capture",
		Description: "Copy files from the project's inbox into the immutable raw store and record them in the source ledger, as one commit. Returns each file's source id and stored path; read the stored file afterwards with Read."}, s.capture)
	mcp.AddTool(server, &mcp.Tool{Name: "route", Annotations: ro(),
		Description: "Say where a new wiki page of a type belongs under the project's mode, whether a page with that title or alias already exists, and give a skeleton with the frontmatter conventions."}, s.route)
	mcp.AddTool(server, &mcp.Tool{Name: "plan", Annotations: ro(),
		Description: "Validate a set of changes to the wiki and hold them as a plan. Returns a plan_id, a preview of creates, replaces, and deletes, and warnings such as links that do not resolve. Nothing is written. Show the preview to the user before apply."}, s.plan)
	mcp.AddTool(server, &mcp.Tool{Name: "apply",
		Description: "Apply a held plan as one git commit and write its log entry. Edits made by hand in the wiki are committed first, so the operation can always be undone exactly; the work outside the wiki is never touched. The plan is consumed."}, s.apply)
	mcp.AddTool(server, &mcp.Tool{Name: "undo",
		Description: "Take back one applied operation as a new commit. It touches only the pages that operation wrote, and refuses when one of them changed since."}, s.undo)
	mcp.AddTool(server, &mcp.Tool{Name: "history", Annotations: ro(),
		Description: "List the wiki's recent operations, newest first, with their kind, summary, date, commit, and changed paths."}, s.history)
	mcp.AddTool(server, &mcp.Tool{Name: "lint", Annotations: ro(),
		Description: "Run the deterministic health check on the project's wiki: dead and ambiguous links, duplicate basenames, orphans, pages missing from every index, missing frontmatter, empty sections, stale index entries, and ledger problems. Read-only."}, s.lint)
	mcp.AddTool(server, &mcp.Tool{Name: "stub",
		Description: "Create seed pages in the wiki for the pages it links to but nobody has written (lint's wanted pages). One commit, no plan preview; undo takes it back. Omit titles to stub every one of them with the mode's default type. Pass a title with a type when the name is a person, product, project, or organization (entity)."}, s.stub)
	mcp.AddTool(server, &mcp.Tool{Name: "threads", Annotations: ro(),
		Description: "List the project's threads: counts by stage, the phases in order, the open threads with the furthest stage first, the closed ones, the notes waiting in the inbox, and the pages it could not read. Each thread carries the absolute path of each of its documents. Pass id for one thread."}, s.threads)
	mcp.AddTool(server, &mcp.Tool{Name: "thread",
		Description: "Open a thread or move one along. A thread is one line of work with a document per stage: stub, spec, plan, receipt. Without id: open a thread from text, which becomes its stub. With id and stage: file that stage's document from text, which moves the thread to that stage; a receipt needs outcome (completed or killed) and closes the thread. With id and priority, phase, blocked, or title: change its card. With id alone: mark it touched today. The stage is never set directly; it is the furthest document that exists."}, s.thread)
	mcp.AddTool(server, &mcp.Tool{Name: "phase",
		Description: "Create, rename, reorder, or remove a phase of the project: a named slice of the timeline that threads belong to. Rename follows every thread that names it; remove refuses while one does."}, s.phase)
	mcp.AddTool(server, &mcp.Tool{Name: "atlas",
		Description: "Read the whole atlas: every project on this machine with its path, description, wiki, and open threads; the folders the atlas cannot read; and the settings. Pass refresh to also rewrite the registry."}, s.atlasTool)
	mcp.AddTool(server, &mcp.Tool{Name: "project",
		Description: "Make a folder a project, with its wiki and its threads (init); change its name, description, or filing mode (edit); or drop it from the atlas, leaving the folder (forget). State the change and get a yes before calling."}, s.projectTool)
	mcp.AddTool(server, &mcp.Tool{Name: "settings",
		Description: "Set an atlas setting and return them all: new_days, how long a project counts as new. With no arguments it only reads."}, s.settingsTool)
	mcp.AddTool(server, &mcp.Tool{Name: "stage",
		Description: "Copy files or folders from outside the project into its inbox, skipping what it already captured or holds; omit paths to stage what is new in the folders it staged from before. With snapshot, write a snapshot of the work into the inbox for the describe skill. dry_run plans and copies nothing."}, s.stageTool)
	return server
}

// Run serves over stdio until the client disconnects.
func Run(ctx context.Context, opts Options) error {
	return New(opts).MCP().Run(ctx, &mcp.StdioTransport{})
}

// ToolNames lists every tool MCP registers, sorted, for docs and tests.
func ToolNames() []string {
	names := []string{"apply", "atlas", "capture", "history", "inbox", "lint", "phase", "plan", "project", "route", "settings", "stage", "status", "stub", "thread", "threads", "undo"}
	sort.Strings(names)
	return names
}
