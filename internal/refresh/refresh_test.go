package refresh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/project"
	"github.com/nathanaday/claude-atlas/internal/registry"
	"github.com/nathanaday/claude-atlas/internal/threads"
)

// fakeProject writes a project's folder by hand, with the wiki pages a test needs. It
// returns the work folder and the project's own folder, which the wiki functions read.
func fakeProject(t *testing.T, log, hot string, pages map[string]string) (string, string) {
	t.Helper()
	work := filepath.Join(t.TempDir(), "work")
	atlas := filepath.Join(work, project.Dir, "v")
	os.MkdirAll(filepath.Join(atlas, "wiki"), 0o755)
	os.WriteFile(filepath.Join(atlas, project.Marker), []byte(`{"schema":"`+project.Schema+`","id":"00000000-0000-4000-8000-000000000001","name":"v","mode":"generic"}`), 0o644)
	os.WriteFile(filepath.Join(atlas, "wiki", "log.md"), []byte(log), 0o644)
	os.WriteFile(filepath.Join(atlas, "wiki", "hot.md"), []byte(hot), 0o644)
	for name, text := range pages {
		os.WriteFile(filepath.Join(atlas, "wiki", name), []byte(text), 0o644)
	}
	return work, atlas
}

func TestHeat(t *testing.T) {
	for days, want := range map[int]string{0: "hot", 6: "hot", 7: "warm", 29: "warm", 30: "cold"} {
		d := days
		if got := Heat(&d, nil, 7); got != want {
			t.Errorf("%d days: got %s want %s", days, got, want)
		}
	}
	if Heat(nil, p(0), 7) != "" {
		t.Error("nil idleness should be unknown even when new")
	}
	if Heat(p(0), p(3), 7) != "new" || Heat(p(40), p(6), 7) != "new" {
		t.Error("a vault under 7 days old is new whatever its idleness")
	}
	if Heat(p(0), p(7), 7) != "hot" {
		t.Error("7 days old is no longer new")
	}
	if Heat(p(0), p(3), 1) != "hot" || Heat(p(0), p(0), 1) != "new" || Heat(p(0), p(0), 0) != "hot" {
		t.Error("the threshold is configurable; 0 turns new off")
	}
}

func p(v int) *int { return &v }

func TestCreatedDateComesFromTheIndexPage(t *testing.T) {
	_, atlas := fakeProject(t, "", "", map[string]string{
		"index.md":    "---\ntitle: Wiki Index\ncreated: 2026-09-01\nupdated: 2026-09-10\n---\n",
		"overview.md": "---\ncreated: 2020-01-01\n---\n",
	})
	got, ok := CreatedDate(atlas)
	if !ok || got.Format("2006-01-02") != "2026-09-01" {
		t.Fatalf("got %v %v", got, ok)
	}
	if _, bare := fakeProject(t, "", "", nil); true {
		if _, ok := CreatedDate(bare); ok {
			t.Fatal("no created field should report none")
		}
	}
}

func TestNewestLogDate(t *testing.T) {
	_, atlas := fakeProject(t, "# Log\n\n## 2026-09-04 — a\n\n## 2026-09-10 — b\n\n## not-a-date\n", "", nil)
	got, ok := NewestLogDate(atlas)
	if !ok || got.Format("2006-01-02") != "2026-09-10" {
		t.Fatalf("got %v %v", got, ok)
	}
	if _, ok := NewestLogDate(filepath.Join(t.TempDir(), "missing")); ok {
		t.Fatal("missing vault should report no date")
	}
}

func TestHotTopicsJoinsContinuationLines(t *testing.T) {
	_, atlas := fakeProject(t, "", "# R\n\n## Recent Changes\n\n- ignored\n\n## Active Threads\n\n- First thread\n  continues here.\n- Second\n\n## Later\n\n- no\n", nil)
	got := HotTopics(atlas)
	if strings.Join(got, "|") != "First thread continues here.|Second" {
		t.Fatalf("got %v", got)
	}
}

func TestPlainTextStripsWikilinks(t *testing.T) {
	if got := PlainText("See [[Spec]] and [[Long Name|alias]]."); got != "See Spec and alias." {
		t.Fatalf("got %q", got)
	}
}

func TestDeriveTakesLaterOfLogAndMtime(t *testing.T) {
	work, atlas := fakeProject(t, "## 2026-08-01 — old\n", "", nil)
	os.MkdirAll(filepath.Join(atlas, "inbox"), 0o755)
	os.WriteFile(filepath.Join(atlas, "inbox", "paper.md"), []byte("x"), 0o644)
	e := registry.Entry{Path: work}
	state := Derive(e, time.Now(), "t", 7)
	if state.LastOperation != "2026-08-01" || state.LastTouched != time.Now().Format("2006-01-02") {
		t.Fatalf("got %+v", state)
	}
	if state.DaysIdle == nil || *state.DaysIdle != 0 || state.Heat != "hot" {
		t.Fatalf("idle %v heat %s", state.DaysIdle, state.Heat)
	}
	if !state.OK || state.Pages == nil || *state.Pages != 2 || state.Inbox == nil || *state.Inbox != 1 {
		t.Fatalf("got %+v", state)
	}
	if state.Threads == nil || state.Threads.Counts.Open != 0 {
		t.Fatalf("a project with no thread still reports its board: %+v", state.Threads)
	}
	if got := Derive(registry.Entry{Path: work, Error: "a 3.x project"}, time.Now(), "t", 7); got.OK || got.Error != "a 3.x project" {
		t.Fatalf("an error entry: %+v", got)
	}
}

// projectFixture makes one project in a work folder that is a git repository, and returns
// the config, the project's scanned entry, and the project.
func projectFixture(t *testing.T, now time.Time) (*home.Config, registry.Entry, *project.Project) {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	cfg := &home.Config{Schema: home.ConfigSchema, Heat: &home.HeatConfig{NewDays: 7}}
	code := filepath.Join(t.TempDir(), "code")
	os.MkdirAll(code, 0o755)
	res, err := project.Init(code, project.Options{Name: "code"}, now)
	if err != nil {
		t.Fatal(err)
	}
	p := res.Project
	os.WriteFile(filepath.Join(code, "a.txt"), []byte("a"), 0o644)
	r := p.Work()
	r.AddAll()
	if _, err := r.Commit("feat: one"); err != nil {
		t.Fatal(err)
	}
	cfg.AddProject(code)
	ix, err := registry.Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := ix.ByPath(code)
	if e == nil {
		t.Fatal("the project was not scanned")
	}
	return cfg, *e, p
}

func TestDeriveAProject(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.Local)
	cfg, e, p := projectFixture(t, now)
	state := Derive(e, now, "t", 7)
	if !state.OK || state.Git == nil || !state.Git.OK || state.Threads == nil || state.Threads.Counts.Open != 0 || state.Described != nil {
		t.Fatalf("fresh project: %+v", state)
	}
	if state.Pages == nil || *state.Pages < 4 {
		t.Fatalf("a fresh project has the template's pages: %+v", state.Pages)
	}
	if state.Heat != "new" || state.LastTouched == "" {
		t.Fatalf("the last commit touches the project: %+v", state)
	}
	if _, err := threads.CreatePhase(p, "Alpha", "", nil, now); err != nil {
		t.Fatal(err)
	}
	if _, err := threads.Start(p, threads.New{Title: "Blocked one", Phase: "Alpha"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := threads.Start(p, threads.New{Title: "Stale one"}, now.AddDate(0, 0, -20)); err != nil {
		t.Fatal(err)
	}
	if _, err := threads.File(p, "Stale one", threads.Filing{Stage: threads.Plan, Text: "1. Go."}, now.AddDate(0, 0, -20)); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(p.Path("inbox/note.md"), []byte("later"), 0o644)
	head, _ := (gitx.Repo{Dir: e.Path}).Head()
	os.MkdirAll(filepath.Join(e.Wiki(), "entities"), 0o755)
	os.WriteFile(filepath.Join(e.Wiki(), "entities", "code.md"), []byte("---\ntitle: code\ntype: entity\nentity_type: project\nproject: "+e.ID+"\ncommit: "+head+"\n---\n"), 0o644)
	os.WriteFile(filepath.Join(e.Path, "a.txt"), []byte("b"), 0o644)
	r := gitx.Repo{Dir: e.Path}
	r.AddAll()
	r.Commit("two")
	ix, _ := registry.Scan(cfg)
	e = *ix.ByPath(e.Path)
	state = Derive(e, now, "t", 7)
	if state.Threads == nil || state.Threads.Counts.Open != 2 || state.Threads.Counts.Stale != 1 || state.Threads.Counts.Notes != 1 || len(state.Threads.Open) != 2 {
		t.Fatalf("threads %+v", state.Threads)
	}
	if strings.Join(state.Threads.Phases, ",") != "Alpha" || state.Threads.Open[0].Title != "Stale one" || !state.Threads.Open[0].Stale || state.Threads.Open[1].Phase != "Alpha" || !filepath.IsAbs(state.Threads.Open[1].Path) {
		t.Fatalf("open %+v phases %v", state.Threads.Open, state.Threads.Phases)
	}
	if state.Described == nil || state.Described.Page != "wiki/entities/code.md" || state.Described.Behind != 1 {
		t.Fatalf("described %+v", state.Described)
	}
	e.State = state
	got := strings.Join(Signals(e, now), "\n")
	for _, want := range []string{"1 stale thread"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, registry.NotDescribed) {
		t.Errorf("a described project has no describe signal:\n%s", got)
	}
}

func TestAFreshProjectIsNewAndGoesColdWithoutWork(t *testing.T) {
	today := time.Now()
	work := filepath.Join(t.TempDir(), "notes")
	os.MkdirAll(work, 0o755)
	// No repository, so the wiki's own files are the only sign of work.
	if _, err := project.Init(work, project.Options{NoGit: true}, today); err != nil {
		t.Fatal(err)
	}
	cfg := &home.Config{}
	cfg.AddProject(work)
	ix, err := registry.Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := *ix.ByPath(work)
	stamp := today.Format("2006-01-02")
	if state := Derive(e, today, "t", 7); state.Heat != "new" || state.LastTouched != stamp || state.DaysIdle == nil || *state.DaysIdle != 0 {
		t.Fatalf("a project made today is new: %+v", state)
	}
	if state := Derive(e, today.AddDate(0, 3, 0), "t", 7); state.Heat != "cold" || state.LastTouched != stamp {
		t.Fatalf("and cold once nothing touched it for months: %+v", state)
	}
}

func TestRegistryDerivesEveryEntry(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.Local)
	cfg, _, _ := projectFixture(t, now)
	// A 3.x knowledge base the config still lists is an entry the scan cannot use.
	old := filepath.Join(t.TempDir(), "old")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, project.KnowledgeMarker), []byte(`{"schema":"claude-atlas.vault.v3","id":"k1","kind":"knowledge","name":"old"}`), 0o644)
	cfg.AddKnowledge(old)
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	entries, ix, err := Registry(home.Home{Root: filepath.Join(root, "home")}, cfg, stateDir, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || len(ix.Problems) != 1 {
		t.Fatalf("entries %d problems %+v", len(entries), ix.Problems)
	}
	for _, e := range entries {
		if e.Error != "" {
			if e.State == nil || e.State.OK || !strings.Contains(e.State.Error, "3.x") {
				t.Errorf("the leftover knowledge base %+v", e)
			}
			continue
		}
		if e.State == nil || !e.State.OK || e.State.Threads == nil || e.State.Git == nil || e.State.Pages == nil || *e.State.Pages < 4 {
			t.Errorf("project state %+v", e.State)
		}
	}
	read, _, err := registry.Read(stateDir)
	if err != nil || len(read) != 2 {
		t.Fatalf("registry file %v %d", err, len(read))
	}
}

func TestSignalsOverAnEntry(t *testing.T) {
	e := registry.Entry{Name: "p", Path: "/code/p",
		State: &registry.State{OK: true, PendingRecovery: true, Threads: &registry.ThreadSummary{Open: []registry.ThreadLine{{Title: "A", Stage: "stub", Blocked: "the vendor"}, {Title: "B", Stage: "plan", Stale: true}}}},
	}
	got := strings.Join(Signals(e, time.Now()), "\n")
	for _, want := range []string{"interrupted", "1 blocked thread: A", "1 stale thread", registry.NotDescribed} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	described := registry.Entry{Name: "p", State: &registry.State{OK: true, Described: &registry.Description{Page: "wiki/entities/p.md", Commit: "abc", Behind: 0}}}
	if got := strings.Join(Signals(described, time.Now()), "\n"); strings.Contains(got, registry.NotDescribed) {
		t.Errorf("a described project has no describe signal:\n%s", got)
	}
	behind := registry.Entry{Name: "p", State: &registry.State{OK: true, Described: &registry.Description{Page: "wiki/entities/p.md", Commit: "abc", Behind: 40}}}
	if got := strings.Join(Signals(behind, time.Now()), "\n"); !strings.Contains(got, "40 commits behind") {
		t.Fatalf("a page far behind:\n%s", got)
	}
	if got := Signals(registry.Entry{Name: "k", Error: "a 3.x project"}, time.Now()); len(got) != 1 || !strings.Contains(got[0], "3.x") {
		t.Fatalf("error entry %v", got)
	}
	if got := Signals(registry.Entry{Name: "k"}, time.Now()); len(got) != 1 || got[0] != "not refreshed" {
		t.Fatalf("no state %v", got)
	}
	if got := Signals(registry.Entry{Name: "k", State: &registry.State{Error: "boom"}}, time.Now()); len(got) < 1 || !strings.Contains(got[0], "unreachable: boom") {
		t.Fatalf("a state that failed %v", got)
	}
}
