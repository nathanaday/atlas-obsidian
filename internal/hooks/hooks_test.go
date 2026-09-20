package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/manage"
	"github.com/nathanaday/claude-atlas/internal/project"
	"github.com/nathanaday/claude-atlas/internal/threads"
)

// env answers CLAUDE_ATLAS_HOME with a temp path that does not exist, so a test that names
// no home reads no atlas at all instead of the developer's ~/.claude-atlas.
func env(t *testing.T, values map[string]string) Env {
	t.Helper()
	noAtlas := filepath.Join(t.TempDir(), "no-atlas")
	return func(k string) string {
		if k == home.EnvHome && values[k] == "" {
			return noAtlas
		}
		return values[k]
	}
}

// atlas makes a home with one project in a git repository, and returns the home and the
// work folder.
func atlas(t *testing.T, now time.Time) (home.Home, string) {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	h := home.Home{Root: filepath.Join(root, "home")}
	cfg := h.Default()
	work := filepath.Join(root, "code")
	os.MkdirAll(filepath.Join(work, "src"), 0o755)
	os.WriteFile(filepath.Join(work, "src", "main.go"), []byte("package main\n"), 0o644)
	if _, err := manage.Init(h, cfg, work, project.Options{Name: "code", Description: "The web app."}, nil, false); err != nil {
		t.Fatal(err)
	}
	return h, work
}

func run(t *testing.T, cwd string, e Env, context bool, now time.Time) string {
	t.Helper()
	var out bytes.Buffer
	if err := SessionStart(strings.NewReader(`{"cwd":"`+cwd+`"}`), &out, e, context, now); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestSessionStartInAProject(t *testing.T) {
	now := time.Now()
	h, work := atlas(t, now)
	e := env(t, map[string]string{home.EnvHome: h.Root})
	p, _ := project.Open(work)
	text := run(t, filepath.Join(work, "src"), e, true, now)
	for _, want := range []string{
		"claude-atlas: project code at " + home.Display(work) + " (git, ",
		"Description: The web app.",
		"Wiki: atlas/code/wiki · ",
		" pages · generic mode",
		"The wiki has no page describing this work; the describe skill writes it.",
		SearchSentence + " " + WriteSentence,
		"Skills: " + Skills,
		"Open threads: none. Open one with the thread-stub skill.",
		"<vault-context>",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	for _, absent := range []string{"type: meta", "Projects:", "knowledge base"} {
		if strings.Contains(text, absent) {
			t.Errorf("%q should not be there:\n%s", absent, text)
		}
	}
	// Context off, and the place named by the environment rather than the folder.
	off := env(t, map[string]string{home.EnvHome: h.Root, project.EnvProject: work, "CLAUDE_ATLAS_SESSION_CONTEXT": "0"})
	if text := run(t, "/nowhere", off, true, now); !strings.Contains(text, "claude-atlas: project code") || strings.Contains(text, "<vault-context>") {
		t.Fatalf("the environment names the project, with context off:\n%s", text)
	}
	// Outside every project the hook is silent.
	if text := run(t, t.TempDir(), env(t, nil), true, now); text != "" {
		t.Fatalf("silent outside a project:\n%s", text)
	}
	// Threads, phases, a note, an unreadable page, task pages, and the page that describes
	// the work.
	if _, err := threads.CreatePhase(p, "Alpha", "", nil, now); err != nil {
		t.Fatal(err)
	}
	if _, err := threads.Start(p, threads.New{Title: "Fix the dialog", Text: "It quits on Enter.", Phase: "Alpha", Priority: "high"}, now); err != nil {
		t.Fatal(err)
	}
	old := now.AddDate(0, 0, -20)
	if _, err := threads.Start(p, threads.New{Title: "Stale one"}, old); err != nil {
		t.Fatal(err)
	}
	if _, err := threads.File(p, "Stale one", threads.Filing{Stage: threads.Plan, Text: "1. Go."}, old); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p.Path("inbox/idea.md"), []byte("An idea."), 0o644)
	os.WriteFile(p.Path("inbox/paper.pdf"), []byte("%PDF"), 0o644)
	os.WriteFile(p.Path(project.SpecsDir+"/broken.md"), []byte("x"), 0o644)
	os.MkdirAll(p.Path(project.LegacyTasksDir), 0o755)
	os.WriteFile(p.Path(project.LegacyTasksDir+"/Old.md"), []byte("---\ntype: task\n---\n"), 0o644)
	head, _ := p.Work().Head()
	os.MkdirAll(p.Path("wiki/entities"), 0o755)
	os.WriteFile(p.Path("wiki/entities/code.md"), []byte("---\ntitle: code\ntype: entity\nentity_type: project\nproject: "+p.Config.ID+"\ncommit: "+head+"\nstatus: developing\ncreated: 2026-09-17\nupdated: 2026-09-17\ntags:\n  - entity\n---\n\n# code\n"), 0o644)
	text = run(t, work, e, false, now)
	for _, want := range []string{
		"The work is described in wiki/entities/code.md at " + head[:7] + ", current.",
		"Open threads: 2 (plan 1, spec 0, stub 1; 1 stale) in 1 phase. A thread moves stub, spec, plan, receipt",
		"- [plan] Stale one (thr-",
		" · stale\n",
		"- [stub] Fix the dialog (thr-",
		" · Alpha · high · updated " + now.Format("2006-01-02") + "\n",
		"Inbox: 1 source for the wiki-ingest skill, 1 note for the thread-stub skill.",
		"Not readable: threads/specs/broken.md (",
		"task pages from before threads; `claude-atlas upgrade",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "no page describing") {
		t.Errorf("a described project:\n%s", text)
	}
}

func TestSessionStartReportsTheWikisCountsAndAnInterruptedOperation(t *testing.T) {
	now := time.Now()
	h, work := atlas(t, now)
	e := env(t, map[string]string{home.EnvHome: h.Root})
	p, _ := project.Open(work)
	os.MkdirAll(p.Path("wiki/concepts"), 0o755)
	training := project.Skeleton("concept", "Training", now)
	training = strings.Replace(training, "## Related\n\n", "## Related\n\n[[Optimizer]], [[Backpropagation]]\n\n", 1)
	os.WriteFile(p.Path("wiki/concepts/Training.md"), []byte(training), 0o644)
	os.WriteFile(p.Path("wiki/concepts/Backpropagation.md"), []byte(project.Skeleton("concept", "Backpropagation", now)), 0o644)
	text := run(t, work, e, false, now)
	want := "Stubs: 1 page to fill (Backpropagation). Wanted: 1 linked page does not exist yet (Optimizer). Fill or stub them with the wiki-lint skill."
	if !strings.Contains(text, want) {
		t.Errorf("missing the counts in:\n%s", text)
	}
	four := project.Skeleton("concept", "Four Wants", now)
	four = strings.Replace(four, "## Related\n\n", "## Related\n\n[[Alpha]], [[Bravo]], [[Charlie]], [[Delta]]\n\n", 1)
	os.WriteFile(p.Path("wiki/concepts/Four Wants.md"), []byte(four), 0o644)
	if text = run(t, work, e, false, now); !strings.Contains(text, "Wanted: 5 linked pages do not exist yet (Alpha, Bravo, Charlie, …).") {
		t.Errorf("missing the capped wanted list in:\n%s", text)
	}
	// An interrupted operation warns at start and at stop.
	os.MkdirAll(p.Path(project.MetaDir), 0o755)
	os.WriteFile(p.Path(project.MetaDir+"/inflight.json"), []byte(`{"operation_id":"save-x","paths":[]}`), 0o644)
	if text = run(t, work, e, false, now); !strings.Contains(text, "WARNING: operation save-x was interrupted") {
		t.Fatalf("recovery warning:\n%s", text)
	}
	var out bytes.Buffer
	Stop(strings.NewReader(`{"cwd":"`+work+`"}`), &out, e)
	if !strings.Contains(out.String(), `"systemMessage"`) || !strings.Contains(out.String(), "save-x") {
		t.Fatalf("stop:\n%s", out.String())
	}
	out.Reset()
	Stop(strings.NewReader(`{"cwd":"`+t.TempDir()+`"}`), &out, env(t, nil))
	if out.Len() != 0 {
		t.Fatal("stop is silent outside a project")
	}
}

func TestSessionStartHealsTheConfig(t *testing.T) {
	now := time.Now()
	h, work := atlas(t, now)
	e := env(t, map[string]string{home.EnvHome: h.Root})
	clone := filepath.Join(t.TempDir(), "clone")
	os.MkdirAll(clone, 0o755)
	if _, err := project.Init(clone, project.Options{Name: "clone"}, now); err != nil {
		t.Fatal(err)
	}
	if text := run(t, clone, e, false, now); !strings.Contains(text, "The atlas config did not list this project; it does now.") {
		t.Errorf("missing the added line:\n%s", text)
	}
	if cfg, _ := h.Load(); !cfg.HasProject(clone) {
		t.Fatal("the config lists the clone now")
	}
	copied := filepath.Join(t.TempDir(), "copied")
	os.MkdirAll(filepath.Join(copied, project.Dir, "code"), 0o755)
	orig, _ := project.Open(work)
	data, _ := os.ReadFile(orig.Path(project.Marker))
	os.WriteFile(filepath.Join(copied, project.Dir, "code", project.Marker), data, 0o644)
	if text := run(t, copied, e, false, now); !strings.Contains(text, "The atlas config listed this project at another path; it now points here.") {
		t.Errorf("missing the moved line:\n%s", text)
	}
}

func TestSessionStartNamesTheLayoutsOfEarlierVersions(t *testing.T) {
	// A 3.x project, whose knowledge base sits outside it.
	split := t.TempDir()
	os.MkdirAll(filepath.Join(split, project.Dir, "old"), 0o755)
	os.WriteFile(filepath.Join(split, project.Dir, "old", project.Marker),
		[]byte(`{"schema":"claude-atlas.project.v3","id":"p1","name":"old","knowledge":{"id":"k1","name":"notes"}}`), 0o644)
	text := run(t, split, env(t, nil), true, time.Now())
	if !strings.Contains(text, "3.x project") || !strings.Contains(text, "claude-atlas upgrade") {
		t.Errorf("a 3.x project:\n%s", text)
	}
	// The flat layout of 2.2.0.
	flat := t.TempDir()
	os.MkdirAll(filepath.Join(flat, project.Dir), 0o755)
	os.WriteFile(filepath.Join(flat, project.Dir, project.Marker), []byte(`{"schema":"`+project.Schema+`","id":"f","name":"flat"}`), 0o644)
	if text := run(t, flat, env(t, nil), true, time.Now()); !strings.Contains(text, "sits directly in atlas/; run claude-atlas upgrade") {
		t.Errorf("a flat project:\n%s", text)
	}
	// A 3.x knowledge base.
	kb := t.TempDir()
	os.MkdirAll(filepath.Join(kb, "wiki"), 0o755)
	os.WriteFile(filepath.Join(kb, project.KnowledgeMarker), []byte(`{"schema":"claude-atlas.vault.v3","id":"k1","kind":"knowledge","name":"notes"}`), 0o644)
	if text := run(t, filepath.Join(kb, "wiki"), env(t, nil), true, time.Now()); text != "" {
		t.Errorf("a knowledge base is not a place, and the hook stays silent:\n%s", text)
	}
}

func TestGuard(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	work := filepath.Join(t.TempDir(), "work")
	os.MkdirAll(work, 0o755)
	res, err := project.Init(work, project.Options{Name: "work"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	p := res.Project
	if _, err := threads.Start(p, threads.New{Title: "Fix it"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		p.Path("wiki/concepts/A.md"):                 true,
		p.Path("wiki/hot.md"):                        true,
		p.Path(".raw/captured/x.pdf"):                true,
		p.Path(project.Marker):                       true,
		p.Path(project.MetaDir + "/lock"):            true,
		p.Path(project.ThreadsIndex):                 true,
		p.Path("threads/Fix it.md"):                  true,
		p.Path("threads/archive/Fix it.md"):          true,
		p.Path(project.SpecsDir + "/New.md"):         true,
		p.Path(project.StubsDir + "/Fix it.md"):      false,
		p.Path(project.PhasesDir + "/Alpha.md"):      false,
		p.Path("inbox/paper.md"):                     false,
		p.Path("ideas/note.md"):                      false,
		p.Path(project.SpecsDir + "/notes.txt"):      false,
		filepath.Join(work, "src", "main.go"):        false,
		filepath.Join(work, "threads", "threads.md"): false,
		filepath.Join(t.TempDir(), "wiki", "x.md"):   false,
	}
	for path, deny := range cases {
		var out bytes.Buffer
		if err := Guard(strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"`+path+`"}}`), &out); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(out.String(), `"deny"`); got != deny {
			t.Errorf("%s: deny=%v, got %q", path, deny, out.String())
		}
	}
	var out bytes.Buffer
	Guard(strings.NewReader(`{"tool_name":"Edit","cwd":"`+p.Atlas()+`","tool_input":{"file_path":"wiki/hot.md"}}`), &out)
	if !strings.Contains(out.String(), "deny") {
		t.Fatal("relative paths resolve against cwd")
	}
	out.Reset()
	Guard(strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"`+p.Path(project.ThreadsIndex)+`"}}`), &out)
	if !strings.Contains(out.String(), "are generated") {
		t.Errorf("the board's reason: %q", out.String())
	}
	out.Reset()
	Guard(strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"`+p.Path(project.PlansDir+"/New.md")+`"}}`), &out)
	if !strings.Contains(out.String(), "a new plan comes from the thread tool (id, stage: plan, text)") {
		t.Errorf("a new document's reason: %q", out.String())
	}
	out.Reset()
	Guard(strings.NewReader(`{"tool_name":"NotebookEdit","tool_input":{"notebook_path":"`+p.Path("wiki/x.ipynb")+`"}}`), &out)
	if !strings.Contains(out.String(), "deny") {
		t.Fatal("a notebook path is guarded too")
	}
	out.Reset()
	Guard(strings.NewReader(`{"tool_name":"Write","tool_input":{}}`), &out)
	if out.Len() != 0 {
		t.Fatal("no path, no decision")
	}
}

func TestTouchedMarksTheThread(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	work := filepath.Join(t.TempDir(), "work")
	os.MkdirAll(work, 0o755)
	day := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	res, err := project.Init(work, project.Options{Name: "work"}, day)
	if err != nil {
		t.Fatal(err)
	}
	p := res.Project
	th, err := threads.Start(p, threads.New{Title: "Fix it"}, day)
	if err != nil {
		t.Fatal(err)
	}
	in := `{"tool_name":"Edit","cwd":"` + work + `","tool_input":{"file_path":"` + p.Path(th.Docs[0].Path) + `"}}`
	if err := Touched(strings.NewReader(in), day.AddDate(0, 0, 3)); err != nil {
		t.Fatal(err)
	}
	board, _ := threads.Load(p)
	if got := board.Find(th.ID).Updated; got != "2026-09-20" {
		t.Fatalf("updated %s", got)
	}
	// Any other file is none of its business.
	if err := Touched(strings.NewReader(`{"tool_input":{"file_path":"`+filepath.Join(work, "main.go")+`"}}`), day); err != nil {
		t.Fatal(err)
	}
}
