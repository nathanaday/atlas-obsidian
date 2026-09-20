package actions

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/manage"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

// atlas builds an empty atlas home.
func atlas(t *testing.T) (home.Home, *home.Config) {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	h := home.Home{Root: filepath.Join(t.TempDir(), "home")}
	cfg := h.Default()
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return h, cfg
}

func entry(t *testing.T, a Atlas, path string) registry.Entry {
	t.Helper()
	ix, err := a.Scan()
	if err != nil {
		t.Fatal(err)
	}
	e := ix.ByPath(path)
	if e == nil {
		t.Fatalf("no entry at %s", path)
	}
	return *e
}

func TestBindSetsEveryField(t *testing.T) {
	a := Bind(home.Home{Root: t.TempDir()}, &home.Config{}, nil)
	v := reflect.ValueOf(a)
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Func && f.IsNil() {
			t.Errorf("Bind leaves %s nil", v.Type().Field(i).Name)
		}
	}
}

func TestProjectsInitEditStageAndForget(t *testing.T) {
	h, cfg := atlas(t)
	a := Bind(h, cfg, nil)
	work := filepath.Join(t.TempDir(), "webapp")
	os.MkdirAll(work, 0o755)
	made, err := a.InitProject(InitProject{Work: work, Description: "The web app.", Mode: "lyt"})
	if err != nil {
		t.Fatal(err)
	}
	if made.Project.Config.Mode != project.LYT || made.Git != project.GitCreated || made.Commit == "" {
		t.Fatalf("init: %+v", made)
	}
	if _, err := a.InitProject(InitProject{Work: t.TempDir(), Mode: "para"}); err == nil {
		t.Fatal("a bad mode is refused")
	}
	e := entry(t, a, work)
	if e.Description != "The web app." || e.Mode != project.LYT || e.State == nil || e.State.Threads == nil || e.State.Pages == nil {
		t.Fatalf("project: %+v", e)
	}
	desc := "Described."
	if err := a.EditProject(e, manage.Edit{Name: "Web App", Description: &desc, Mode: project.Generic}); err != nil {
		t.Fatal(err)
	}
	e = entry(t, a, work)
	if e.Name != "Web App" || e.Description != desc || e.Mode != project.Generic {
		t.Fatalf("edited: %+v", e)
	}
	// A snapshot of the work goes into the project's own inbox.
	staged, err := a.StageProject(e)
	if err != nil || !staged.New || !strings.HasPrefix(staged.To, "inbox/Web App-") {
		t.Fatalf("stage: %+v %v", staged, err)
	}
	// Staging files from outside: the plan, then the copy.
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "paper.pdf"), []byte("%PDF"), 0o644)
	plan, err := a.StagePlan(e, []string{outside})
	if err != nil || len(plan.New) != 1 {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	res, remembered, err := a.Stage(e, plan)
	if err != nil || len(res.Staged) != 1 || len(remembered) != 1 {
		t.Fatalf("stage: %+v %v %v", res, remembered, err)
	}
	if got := a.Sources(e); len(got) != 1 {
		t.Fatalf("the project remembers the folder: %v", got)
	}
	if err := a.ForgetProject(e); err != nil {
		t.Fatal(err)
	}
	if cfg2, _ := h.Load(); cfg2.HasProject(work) {
		t.Fatal("forgotten")
	}
	if !project.IsProject(work) {
		t.Fatal("forget keeps the folder")
	}
}

func TestThreadsAndPhasesThroughTheActions(t *testing.T) {
	h, cfg := atlas(t)
	a := Bind(h, cfg, nil)
	work := filepath.Join(t.TempDir(), "webapp")
	os.MkdirAll(work, 0o755)
	if _, err := a.InitProject(InitProject{Work: work}); err != nil {
		t.Fatal(err)
	}
	e := entry(t, a, work)
	ph, err := a.AddPhase(e, "Alpha", "First.", nil)
	if err != nil || ph.Order != 1 {
		t.Fatalf("phase: %+v %v", ph, err)
	}
	th, err := a.StartThread(e, threads.New{Title: "Fix it", Phase: "Alpha"})
	if err != nil || th.Phase != "Alpha" || th.Stage != threads.Stub {
		t.Fatalf("start: %+v %v", th, err)
	}
	board, notes, err := a.Threads(e)
	if err != nil || len(board.Threads) != 1 || len(board.Phases) != 1 || len(notes) != 0 {
		t.Fatalf("threads: %+v %v %v", board, notes, err)
	}
	high := "high"
	set, err := a.SetThread(e, th.ID, threads.Changes{Priority: &high})
	if err != nil || set.Priority != "high" {
		t.Fatalf("set: %+v %v", set, err)
	}
	if filed, err := a.FileThread(e, th.ID, threads.Filing{Stage: threads.Receipt, Outcome: threads.Completed, Text: "Fixed."}); err != nil || !filed.Closed() {
		t.Fatalf("file: %+v %v", filed, err)
	}
	if opened, err := a.Reopen(e, th.ID); err != nil || opened.Closed() {
		t.Fatalf("reopen: %+v %v", opened, err)
	}
	if err := a.RemovePhase(e, "Alpha"); err == nil {
		t.Fatal("a phase with a thread is not removed")
	}
	none := ""
	if _, err := a.SetThread(e, th.ID, threads.Changes{Phase: &none}); err != nil {
		t.Fatal(err)
	}
	if err := a.RemovePhase(e, "Alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddPhase(e, "Beta", "", nil); err != nil {
		t.Fatal(err)
	}
	if ph, err := a.OrderPhase(e, "Beta", 5); err != nil || ph.Order != 5 {
		t.Fatalf("order: %+v %v", ph, err)
	}
	if ph, err := a.RenamePhase(e, "Beta", "Gamma"); err != nil || ph.Title != "Gamma" {
		t.Fatalf("rename: %+v %v", ph, err)
	}
}

func TestLoadRefreshAndScan(t *testing.T) {
	h, cfg := atlas(t)
	a := Bind(h, cfg, nil)
	work := filepath.Join(t.TempDir(), "webapp")
	os.MkdirAll(work, 0o755)
	if _, err := a.InitProject(InitProject{Work: work}); err != nil {
		t.Fatal(err)
	}
	if entries, err := a.Load(); err != nil || len(entries) != 1 {
		t.Fatalf("load: %v %v", entries, err)
	}
	if ix, err := a.Refresh(); err != nil || len(ix.Projects()) != 1 {
		t.Fatalf("refresh: %v %v", ix, err)
	}
	if ix, err := a.Scan(); err != nil || len(ix.Projects()) != 1 || ix.Projects()[0].State == nil {
		t.Fatalf("scan derives state: %v %v", ix, err)
	}
}
