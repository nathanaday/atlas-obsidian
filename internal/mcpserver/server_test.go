package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

var now = time.Date(2026, 9, 17, 15, 0, 0, 0, time.UTC)

type client struct {
	t    *testing.T
	sess *mcp.ClientSession
}

// connectIn starts a server whose atlas home is h and whose session started in dir.
func connectIn(t *testing.T, h home.Home, dir string) *client {
	t.Helper()
	s := New(Options{Version: "test", ProjectDir: dir, Env: func(k string) string {
		if k == home.EnvHome {
			return h.Root
		}
		return ""
	}, Now: func() time.Time { return now }})
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := s.MCP().Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	sess, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sess.Close() })
	return &client{t: t, sess: sess}
}

// call invokes a tool and decodes its structured result into out. It returns the error
// text for tool errors.
func (c *client) call(name string, args map[string]any, out any) string {
	c.t.Helper()
	res, err := c.sess.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		c.t.Fatalf("%s: protocol error %v", name, err)
	}
	if res.IsError {
		var parts []string
		for _, content := range res.Content {
			if tc, ok := content.(*mcp.TextContent); ok {
				parts = append(parts, tc.Text)
			}
		}
		return strings.Join(parts, " ")
	}
	if out != nil {
		data, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("%s: decode %v: %s", name, err, data)
		}
	}
	return ""
}

// atlas is one machine: a home with a config and one project, webapp, in a work folder that
// git init made a repository.
type atlas struct {
	h    home.Home
	cfg  *home.Config
	work string
	p    *project.Project
}

func newAtlas(t *testing.T) *atlas {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	h := home.Home{Root: filepath.Join(root, "home")}
	cfg := h.Default()
	work := filepath.Join(root, "code", "webapp")
	if err := os.MkdirAll(filepath.Join(work, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# webapp\n\nThe app.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := project.Init(work, project.Options{Name: "webapp", Description: "The app."}, now)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AddProject(work)
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return &atlas{h: h, cfg: cfg, work: work, p: res.Project}
}

func (a *atlas) session(t *testing.T) *client { return connectIn(t, a.h, filepath.Join(a.work, "src")) }

const page = "---\ntitle: %s\ntype: %s\nstatus: seed\ncreated: 2026-09-12\nupdated: 2026-09-12\ntags:\n  - x\n---\n# %s\n\n%s\n"

func TestToolsListAndStatus(t *testing.T) {
	a := newAtlas(t)
	c := a.session(t)
	tools, err := c.sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != strings.Join(ToolNames(), ",") {
		t.Fatalf("ToolNames %v, registered %v", ToolNames(), names)
	}
	if len(names) != 18 {
		t.Fatalf("%d tools: %v", len(names), names)
	}
	for _, gone := range []string{"vault", "mode"} {
		if strings.Contains(strings.Join(names, ","), gone) {
			t.Errorf("the %s tool is gone", gone)
		}
	}
	var st Status
	if msg := c.call("status", nil, &st); msg != "" {
		t.Fatal(msg)
	}
	if st.Name != "webapp" || st.Path != a.work || st.Atlas != a.p.Atlas() || st.Description != "The app." || st.Mode != "generic" {
		t.Fatalf("status %+v", st)
	}
	if st.Git == nil || st.Threads == nil || st.Described != nil || st.Pages != 4 || st.WikiGit == nil || !st.WikiGit.HasHistory || st.LastOperation == nil {
		t.Fatalf("both halves in one status: %+v", st)
	}
	if !strings.Contains(strings.Join(st.Warnings, " "), registry.NotDescribed) {
		t.Fatalf("an undescribed project warns: %v", st.Warnings)
	}
	none := connectIn(t, a.h, t.TempDir())
	if msg := none.call("status", nil, nil); !strings.Contains(msg, "not in a atlas-obsidian") {
		t.Fatalf("no place: %q", msg)
	}
}

func TestIngestWorkflow(t *testing.T) {
	a := newAtlas(t)
	c := a.session(t)
	os.WriteFile(a.p.Path("inbox/paper.md"), []byte("# A paper\n\nThe claim.\n"+strings.Repeat("x", 5000)), 0o644)
	os.WriteFile(a.p.Path("inbox/idea.md"), []byte("Try the thing.\n"), 0o644)

	var inbox InboxOut
	if msg := c.call("inbox", nil, &inbox); msg != "" {
		t.Fatal(msg)
	}
	if inbox.Project != a.work || len(inbox.Files) != 2 {
		t.Fatalf("inbox %+v", inbox)
	}
	hints := map[string]string{}
	for _, f := range inbox.Files {
		hints[f.Path] = f.Hint
	}
	if hints["inbox/paper.md"] != "source" || hints["inbox/idea.md"] != "note" {
		t.Fatalf("one inbox, two hints: %+v", hints)
	}
	var cap struct {
		Sources []struct {
			SourceID   string `json:"source_id"`
			StoredPath string `json:"stored_path"`
		} `json:"sources"`
		Commit string `json:"commit"`
	}
	if msg := c.call("capture", map[string]any{"paths": []string{"paper.md"}}, &cap); msg != "" {
		t.Fatal(msg)
	}
	if len(cap.Sources) != 1 || cap.Commit == "" {
		t.Fatalf("capture %+v", cap)
	}
	var route RouteOut
	c.call("route", map[string]any{"type": "source", "title": "A paper"}, &route)
	if route.Path != "wiki/sources/A paper.md" || route.Exists || route.Project != a.work {
		t.Fatalf("route %+v", route)
	}
	index, _ := os.ReadFile(a.p.Path(project.IndexPage))
	newIndex := strings.Replace(string(index), "- No sources yet.", "- [[A paper]]", 1)
	var plan PlanOut
	msg := c.call("plan", map[string]any{
		"kind": "ingest", "summary": "Ingest A paper",
		"writes": []map[string]any{
			{"path": route.Path, "mode": "create", "content": strings.NewReplacer("%s", "A paper").Replace(page)},
			{"path": project.IndexPage, "mode": "replace", "content": newIndex},
			{"path": "inbox/paper.md", "mode": "delete"},
		},
		"sources": []map[string]any{{"id": cap.Sources[0].SourceID, "ingested": true, "pages": []string{route.Path}, "authority": "primary"}},
	}, &plan)
	if msg != "" {
		t.Fatal(msg)
	}
	if len(plan.Preview.Creates) != 1 || len(plan.Preview.Replaces) != 1 || len(plan.Preview.Deletes) != 1 || len(plan.Warnings) != 0 || plan.Summary != "Ingest A paper" {
		t.Fatalf("plan %+v", plan)
	}
	if plan.Project != a.p.Atlas() {
		t.Fatalf("the plan names the project's folder: %q", plan.Project)
	}
	var res struct {
		Commit string `json:"commit"`
	}
	if msg := c.call("apply", map[string]any{"plan_id": plan.PlanID}, &res); msg != "" || res.Commit == "" {
		t.Fatalf("apply %q %+v", msg, res)
	}
	if _, err := os.Stat(a.p.Path("inbox/paper.md")); !os.IsNotExist(err) {
		t.Fatal("the inbox file is gone after the ingest")
	}
	if msg := c.call("apply", map[string]any{"plan_id": plan.PlanID}, nil); !strings.Contains(msg, "single-use") {
		t.Fatalf("a plan applies once: %q", msg)
	}
	var hist HistoryOut
	c.call("history", map[string]any{"limit": 5}, &hist)
	if hist.Project != a.work || len(hist.Operations) < 2 || hist.Operations[0].Kind != "ingest" {
		t.Fatalf("history %+v", hist)
	}
	var lintOut struct {
		Summary struct {
			PagesScanned int `json:"pages_scanned"`
		} `json:"summary"`
	}
	if msg := c.call("lint", nil, &lintOut); msg != "" || lintOut.Summary.PagesScanned != 5 {
		t.Fatalf("lint %q %+v", msg, lintOut)
	}
	if msg := c.call("undo", map[string]any{"operation_id": hist.Operations[0].ID}, nil); msg != "" {
		t.Fatal(msg)
	}
	if _, err := os.Stat(a.p.Path(route.Path)); !os.IsNotExist(err) {
		t.Fatal("undo removed the page")
	}
}

func TestPlanRefusals(t *testing.T) {
	a := newAtlas(t)
	c := a.session(t)
	if msg := c.call("plan", map[string]any{"kind": "config", "summary": "x", "writes": []map[string]any{{"path": project.Marker, "mode": "replace", "content": "{}"}}}, nil); !strings.Contains(msg, "kind must be one of") {
		t.Fatalf("the identity file is not a plan's to write: %q", msg)
	}
	if msg := c.call("plan", map[string]any{"kind": "save", "summary": "x", "writes": []map[string]any{{"path": "ideas/x.md", "mode": "create", "content": "x"}}}, nil); !strings.Contains(msg, "scratch") {
		t.Fatalf("ideas are the user's: %q", msg)
	}
	if msg := c.call("plan", map[string]any{"kind": "save", "summary": "x", "writes": []map[string]any{{"path": "threads/stubs/x.md", "mode": "create", "content": "x"}}}, nil); !strings.Contains(msg, "only under wiki/") {
		t.Fatalf("the threads are not a plan's to write: %q", msg)
	}
	// The mode changes through the project tool now.
	var out ProjectToolOut
	if msg := c.call("project", map[string]any{"action": "edit", "mode": "lyt"}, &out); msg != "" || out.Project == nil || out.Project.Mode != project.LYT {
		t.Fatalf("edit the mode %q %+v", msg, out)
	}
	var route RouteOut
	if msg := c.call("route", map[string]any{"type": "note", "title": "Atomic"}, &route); msg != "" || route.Path != "wiki/notes/Atomic.md" {
		t.Fatalf("the new mode routes %q %+v", msg, route)
	}
}

func TestThreadTools(t *testing.T) {
	a := newAtlas(t)
	c := a.session(t)
	os.WriteFile(a.p.Path("inbox/note.md"), []byte("# Fix the login\n\nIt loops.\n"), 0o644)

	var ph PhaseOut
	if msg := c.call("phase", map[string]any{"action": "create", "title": "Alarm quality", "goal": "Fewer false alarms."}, &ph); msg != "" {
		t.Fatal(msg)
	}
	if ph.Project != "webapp" || ph.Phase == nil || ph.Phase.Order != 1 || ph.File != a.p.Path(project.PhasesDir+"/Alarm quality.md") {
		t.Fatalf("phase %+v", ph)
	}
	if msg := c.call("thread", map[string]any{"title": "Filter vehicles", "text": "Cars trip the alarm.", "phase": "Nope"}, nil); !strings.Contains(msg, "no phase named") {
		t.Fatalf("an unknown phase is refused: %q", msg)
	}
	var opened ThreadOut
	if msg := c.call("thread", map[string]any{"title": "Filter vehicles", "text": "Cars trip the alarm.", "phase": "alarm quality", "priority": "high"}, &opened); msg != "" {
		t.Fatal(msg)
	}
	if opened.Stage != threads.Stub || opened.Phase != "Alarm quality" || opened.Priority != "high" || opened.Files["stub"] != a.p.Path(project.StubsDir+"/Filter vehicles.md") || opened.Card != a.p.Path("threads/Filter vehicles.md") {
		t.Fatalf("opened %+v", opened)
	}
	var fromNote ThreadOut
	if msg := c.call("thread", map[string]any{"from": "inbox/note.md"}, &fromNote); msg != "" {
		t.Fatal(msg)
	}
	if fromNote.Title != "Fix the login" {
		t.Fatalf("title from the note: %+v", fromNote)
	}
	if _, err := os.Stat(a.p.Path("inbox/note.md")); !os.IsNotExist(err) {
		t.Fatal("the note is removed once the stub exists")
	}
	if msg := c.call("thread", map[string]any{"title": "x", "stage": "plan", "text": "x"}, nil); !strings.Contains(msg, "starts with its stub") {
		t.Fatalf("a new thread with a later stage: %q", msg)
	}
	if msg := c.call("thread", map[string]any{"id": opened.ID, "text": "x"}, nil); !strings.Contains(msg, "text needs stage") {
		t.Fatalf("text with no stage: %q", msg)
	}

	// File documents, with a card change in the same call.
	var filed ThreadOut
	if msg := c.call("thread", map[string]any{"id": opened.ID, "stage": "spec", "text": "Vehicles never alarm.", "priority": "normal"}, &filed); msg != "" {
		t.Fatal(msg)
	}
	if filed.Stage != threads.Spec || filed.Priority != "normal" || filed.Files["spec"] != a.p.Path(project.SpecsDir+"/Filter vehicles.md") {
		t.Fatalf("filed %+v", filed)
	}
	if msg := c.call("thread", map[string]any{"id": opened.ID, "stage": "spec", "text": "again"}, nil); !strings.Contains(msg, "revise it with Edit") {
		t.Fatalf("a second spec: %q", msg)
	}
	var board ThreadsOut
	if msg := c.call("threads", nil, &board); msg != "" {
		t.Fatal(msg)
	}
	if len(board.Projects) != 1 {
		t.Fatalf("one project: %+v", board)
	}
	pb := board.Projects[0]
	if pb.Name != "webapp" || len(pb.Open) != 2 || pb.Open[0].ID != opened.ID || len(pb.Phases) != 1 || pb.Phases[0].Open != 1 || pb.Counts.Open != 2 || pb.Counts.Spec != 1 || pb.Board != a.p.Path(project.ThreadsIndex) {
		t.Fatalf("threads %+v", pb)
	}
	if msg := c.call("threads", map[string]any{"id": "fix the"}, &board); msg != "" || len(board.Projects[0].Open) != 1 || board.Projects[0].Open[0].ID != fromNote.ID {
		t.Fatalf("one thread %q %+v", msg, board)
	}

	var set ThreadOut
	if msg := c.call("thread", map[string]any{"id": "Fix the login", "priority": "low", "blocked": "the vendor"}, &set); msg != "" || set.Priority != "low" || set.Blocked != "the vendor" {
		t.Fatalf("set by title %q %+v", msg, set)
	}
	if msg := c.call("thread", map[string]any{"id": fromNote.ID}, &set); msg != "" || set.ID != fromNote.ID {
		t.Fatalf("a touch %q %+v", msg, set)
	}
	if msg := c.call("thread", map[string]any{"id": opened.ID, "stage": "receipt", "text": "Shipped."}, nil); !strings.Contains(msg, "needs an outcome") {
		t.Fatalf("a receipt with no outcome: %q", msg)
	}
	if msg := c.call("thread", map[string]any{"id": opened.ID, "stage": "receipt", "outcome": "completed", "text": "Shipped."}, &set); msg != "" {
		t.Fatal(msg)
	}
	if !set.Closed() || set.Outcome != threads.Completed || set.Path != "threads/archive/Filter vehicles.md" {
		t.Fatalf("a receipt closes the thread: %+v", set)
	}
	index, _ := os.ReadFile(a.p.Path(project.ThreadsIndex))
	if !strings.Contains(string(index), "finished") || !strings.Contains(string(index), "Fix the login") {
		t.Fatalf("the board follows: %s", index)
	}
	if msg := c.call("phase", map[string]any{"action": "remove", "title": "Alarm quality"}, nil); !strings.Contains(msg, "still names") {
		t.Fatalf("remove refuses while a thread names the phase: %q", msg)
	}
	if msg := c.call("phase", map[string]any{"action": "rename", "title": "Alarm quality", "new_title": "Alarms"}, &ph); msg != "" || ph.Phase.Title != "Alarms" {
		t.Fatalf("rename %q %+v", msg, ph)
	}
	moved, _ := os.ReadFile(a.p.Path("threads/archive/Filter vehicles.md"))
	if !strings.Contains(string(moved), `phase: "Alarms"`) {
		t.Fatalf("rename follows the thread: %s", moved)
	}
	if msg := c.call("thread", map[string]any{"id": opened.ID, "reopen": true}, &set); msg != "" || set.Closed() || set.Stage != threads.Spec {
		t.Fatalf("reopen %q %+v", msg, set)
	}
	order := 5
	if msg := c.call("phase", map[string]any{"action": "reorder", "title": "Alarms", "order": order}, &ph); msg != "" || ph.Phase.Order != 5 {
		t.Fatalf("reorder %q %+v", msg, ph)
	}
	if msg := c.call("phase", map[string]any{"action": "grow", "title": "x"}, nil); !strings.Contains(msg, "action must be") {
		t.Fatalf("unknown action: %q", msg)
	}
}

// A tool may name another project the atlas lists.
func TestThreadToolsNameAnotherProject(t *testing.T) {
	a := newAtlas(t)
	other := filepath.Join(filepath.Dir(a.work), "other")
	os.MkdirAll(other, 0o755)
	if _, err := project.Init(other, project.Options{Name: "other"}, now); err != nil {
		t.Fatal(err)
	}
	a.cfg.AddProject(other)
	a.h.Save(a.cfg)
	c := a.session(t)
	var opened ThreadOut
	if msg := c.call("thread", map[string]any{"project": "other", "title": "Do it", "text": "Now."}, &opened); msg != "" || opened.Project != "other" {
		t.Fatalf("a thread in another project %q %+v", msg, opened)
	}
	var board ThreadsOut
	if msg := c.call("threads", map[string]any{"project": "other"}, &board); msg != "" || len(board.Projects) != 1 || len(board.Projects[0].Open) != 1 {
		t.Fatalf("its board %q %+v", msg, board)
	}
	if msg := c.call("threads", map[string]any{"project": "nope"}, nil); !strings.Contains(msg, "no project named") {
		t.Fatalf("a project the atlas does not list: %q", msg)
	}
	// This session's own project still has none.
	if msg := c.call("threads", nil, &board); msg != "" || len(board.Projects[0].Open) != 0 {
		t.Fatalf("this project %q %+v", msg, board)
	}
}

func TestStubAndStage(t *testing.T) {
	a := newAtlas(t)
	front := "---\ntitle: Backprop\ntype: concept\nstatus: developing\ncreated: 2026-09-12\nupdated: 2026-09-12\ntags:\n  - concept\n---\n\n# Backprop\n\nSee [[Gradient]].\n"
	os.MkdirAll(a.p.Path("wiki/concepts"), 0o755)
	os.WriteFile(a.p.Path("wiki/concepts/Backprop.md"), []byte(front), 0o644)
	c := a.session(t)
	var stub struct {
		Stubs []struct {
			Title string `json:"title"`
			Path  string `json:"path"`
		} `json:"stubs"`
		OperationID string `json:"operation_id"`
	}
	if msg := c.call("stub", nil, &stub); msg != "" || len(stub.Stubs) != 1 || stub.Stubs[0].Path != "wiki/concepts/Gradient.md" {
		t.Fatalf("stub %q %+v", msg, stub)
	}
	var hist HistoryOut
	c.call("history", map[string]any{"limit": 1}, &hist)
	if len(hist.Operations) != 1 || hist.Operations[0].Summary != "stub Gradient" {
		t.Fatalf("the stub's operation: %+v", hist.Operations)
	}

	// A snapshot of the work goes into the project's own inbox.
	var staged StageOut
	if msg := c.call("stage", map[string]any{"snapshot": true}, &staged); msg != "" {
		t.Fatal(msg)
	}
	if staged.Snapshot == nil || !staged.Snapshot.New || staged.Snapshot.Commit == "" || !strings.HasPrefix(staged.Snapshot.To, "inbox/webapp-") {
		t.Fatalf("snapshot %+v", staged)
	}
	if msg := c.call("stage", map[string]any{"snapshot": true}, &staged); msg != "" || staged.Snapshot.New {
		t.Fatalf("the same snapshot is not written twice: %q %+v", msg, staged)
	}
	// Files from a folder outside the project.
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "notes.md"), []byte("notes\n"), 0o644)
	if msg := c.call("stage", map[string]any{"paths": []string{src}, "dry_run": true}, &staged); msg != "" || staged.Plan == nil || len(staged.Plan.New) != 1 || staged.Result != nil {
		t.Fatalf("dry run %q %+v", msg, staged)
	}
	if msg := c.call("stage", map[string]any{"paths": []string{src}}, &staged); msg != "" || staged.Result == nil || len(staged.Result.Staged) != 1 {
		t.Fatalf("stage %q %+v", msg, staged)
	}
	if _, err := os.Stat(a.p.Path("inbox/" + filepath.Base(src) + "/notes.md")); err != nil {
		t.Fatal("the file is in the inbox")
	}
	if msg := c.call("stage", map[string]any{"paths": []string{src}, "snapshot": true}, nil); !strings.Contains(msg, "not both") {
		t.Fatalf("snapshot or paths: %q", msg)
	}
	if msg := c.call("stage", map[string]any{}, &staged); msg != "" || staged.Plan == nil || len(staged.Plan.New) != 0 || len(staged.Plan.Unchanged) != 1 {
		t.Fatalf("no paths stages what is new in the remembered folder: %q %+v", msg, staged.Plan)
	}
}

func TestAProjectWithoutAHistory(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	h := home.Home{Root: filepath.Join(root, "home")}
	cfg := h.Default()
	work := filepath.Join(root, "docs")
	os.MkdirAll(work, 0o755)
	if _, err := project.Init(work, project.Options{Name: "docs", NoGit: true}, now); err != nil {
		t.Fatal(err)
	}
	cfg.AddProject(work)
	h.Save(cfg)
	c := connectIn(t, h, work)
	var st Status
	if msg := c.call("status", nil, &st); msg != "" {
		t.Fatal(msg)
	}
	if st.Git != nil || !strings.Contains(strings.Join(st.Warnings, " "), "no git history") {
		t.Fatalf("status %+v", st)
	}
	// The threads work without a repository; the wiki does not.
	var opened ThreadOut
	if msg := c.call("thread", map[string]any{"title": "Still works"}, &opened); msg != "" || opened.ID == "" {
		t.Fatalf("threads work with no git: %q", msg)
	}
	if msg := c.call("plan", map[string]any{"kind": "save", "summary": "x", "writes": []map[string]any{{"path": "wiki/concepts/A.md", "mode": "create", "content": strings.NewReplacer("%s", "A").Replace(page)}}}, nil); msg != "" {
		t.Fatalf("a plan is only a plan: %q", msg)
	}
}

func TestThreadToolRoutesToTheMemberThatOwnsTheThread(t *testing.T) {
	a := newAtlas(t)
	member := filepath.Join(filepath.Dir(a.work), "svc")
	os.MkdirAll(member, 0o755)
	res, err := project.Init(member, project.Options{Name: "svc"}, now)
	if err != nil {
		t.Fatal(err)
	}
	owned, err := threads.Start(res.Project, threads.New{Title: "Fix it", Text: "Do it."}, now)
	if err != nil {
		t.Fatal(err)
	}
	a.cfg.AddProject(member)
	a.h.Save(a.cfg)
	c := a.session(t)
	var out ProjectToolOut
	if msg := c.call("project", map[string]any{"action": "edit", "add_members": []string{"svc"}}, &out); msg != "" {
		t.Fatal(msg)
	}
	if msg := c.call("project", map[string]any{"action": "sync"}, &out); msg != "" || out.Sync == nil || out.Sync.Members[0].Threads == 0 {
		t.Fatalf("sync %q %+v", msg, out.Sync)
	}
	// The hub's board lists its own, then the member's.
	var board ThreadsOut
	if msg := c.call("threads", nil, &board); msg != "" || len(board.Projects) != 2 || board.Projects[1].Name != "svc" || len(board.Projects[1].Open) != 1 {
		t.Fatalf("boards %q %+v", msg, board)
	}
	if msg := c.call("threads", map[string]any{"id": "fix"}, &board); msg != "" || len(board.Projects) != 1 || board.Projects[0].Name != "svc" {
		t.Fatalf("one thread %q %+v", msg, board)
	}
	if msg := c.call("threads", map[string]any{"id": "nope"}, nil); !strings.Contains(msg, "it mirrors") {
		t.Fatalf("no thread: %q", msg)
	}
	// Filing a spec by the member's id lands in the member, and the mirror follows.
	var filed ThreadOut
	if msg := c.call("thread", map[string]any{"id": owned.ID, "stage": "spec", "text": "Done when it works."}, &filed); msg != "" || filed.Project != "svc" || filed.Stage != threads.Spec {
		t.Fatalf("filed %q %+v", msg, filed)
	}
	if _, err := os.Stat(filepath.Join(res.Project.Path(project.SpecsDir), "Fix it.md")); err != nil {
		t.Fatal("the spec is not in the member")
	}
	if _, err := os.Stat(a.p.Path("threads/projects/svc/specs/Fix it.md")); err != nil {
		t.Fatal("the hub's mirror is behind")
	}
	if _, err := os.Stat(a.p.Path(project.SpecsDir + "/Fix it.md")); err == nil {
		t.Fatal("the spec landed in the hub")
	}
	// A title that matches nothing anywhere.
	if msg := c.call("thread", map[string]any{"id": "nothing", "priority": "high"}, nil); !strings.Contains(msg, "no thread") {
		t.Fatalf("unknown: %q", msg)
	}
}
