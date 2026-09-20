package manage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/project"
	"github.com/nathanaday/claude-atlas/internal/threads"
)

// v3Knowledge writes a 3.x knowledge base at dir: its identity file, a wiki page, an inbox
// file, and an idea.
func v3Knowledge(t *testing.T, dir, id, name, scope string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, "wiki", "concepts"), 0o755)
	os.MkdirAll(filepath.Join(dir, "inbox"), 0o755)
	os.MkdirAll(filepath.Join(dir, "ideas"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".raw", "captured"), 0o755)
	os.WriteFile(filepath.Join(dir, project.KnowledgeMarker),
		[]byte(`{"schema":"claude-atlas.vault.v3","id":"`+id+`","kind":"knowledge","name":"`+name+`","mode":"lyt","created":"2026-09-14","scope":"`+scope+`"}`), 0o644)
	os.WriteFile(filepath.Join(dir, "wiki", "index.md"), []byte("---\ntitle: Index\ntype: meta\nstatus: evergreen\ncreated: 2026-09-14\nupdated: 2026-09-14\ntags: []\n---\n\n# Index\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "wiki", "concepts", "Alarm.md"), []byte("---\ntitle: Alarm\ntype: concept\nstatus: seed\ncreated: 2026-09-14\nupdated: 2026-09-14\ntags: []\n---\n\n# Alarm\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "inbox", "paper.pdf"), []byte("%PDF"), 0o644)
	os.WriteFile(filepath.Join(dir, "ideas", "note.md"), []byte("an idea\n"), 0o644)
	os.WriteFile(filepath.Join(dir, ".raw", "captured", "abc.pdf"), []byte("%PDF"), 0o644)
}

// v3Project writes a 3.x project at work, with the stage folders beside threads/ and a
// knowledge base named by id.
func v3Project(t *testing.T, work, id, name, knowledgeID, knowledgeName string) {
	t.Helper()
	folder := filepath.Join(work, project.Dir, name)
	for _, dir := range []string{"threads", "stubs", "specs", "plans", "receipts", "phases", "inbox"} {
		os.MkdirAll(filepath.Join(folder, dir), 0o755)
	}
	knowledge := ""
	if knowledgeID != "" {
		knowledge = `,"knowledge":{"id":"` + knowledgeID + `","name":"` + knowledgeName + `"}`
	}
	os.WriteFile(filepath.Join(folder, project.Marker),
		[]byte(`{"schema":"claude-atlas.project.v3","id":"`+id+`","name":"`+name+`","description":"The work."`+knowledge+`,"created":"2026-09-17"}`), 0o644)
	os.WriteFile(filepath.Join(folder, "threads", "Fix it.md"),
		[]byte("---\ntype: thread\nthread_id: thr-20260917-1111\ntitle: \"Fix it\"\nstage: stub\noutcome: \"\"\npriority: normal\nphase: \"\"\nblocked: \"\"\ncreated: 2026-09-17\nupdated: 2026-09-17\n---\n\n> [!thread] Fix it\n"), 0o644)
	os.WriteFile(filepath.Join(folder, "stubs", "Fix it.md"),
		[]byte("---\ntype: stub\nthread: thr-20260917-1111\ntitle: \"Fix it\"\ncreated: 2026-09-17\n---\n\n> [!stub] Fix it\n\nThe cookie is dropped.\n"), 0o644)
	os.WriteFile(filepath.Join(folder, "phases", "Alpha.md"),
		[]byte("---\ntype: phase\ntitle: \"Alpha\"\norder: 1\ncreated: 2026-09-17\nupdated: 2026-09-17\n---\n\n> [!phase] Alpha\n\n## Goal\n\nShip.\n"), 0o644)
}

func commitAll(t *testing.T, dir string) gitx.Repo {
	t.Helper()
	repo := gitx.Repo{Dir: dir}
	if !repo.IsRepo() {
		if err := repo.Init(); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.AddAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Commit("chore: before the upgrade"); err != nil {
		t.Fatal(err)
	}
	return repo
}

// The knowledge base is inside the work: git mv moves it, so the history follows.
func TestUpgradeAbsorbsAKnowledgeBaseInsideTheWork(t *testing.T) {
	h, cfg := atlas(t)
	dir := work(t, "webapp")
	kb := filepath.Join(dir, "atlas_kb")
	v3Knowledge(t, kb, "k1", "atlas_kb", "The fire-detection product line.")
	v3Project(t, dir, "p1", "webapp", "k1", "atlas_kb")
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644)
	repo := commitAll(t, dir)
	cfg.AddProject(dir)
	cfg.AddKnowledge(kb)
	h.Save(cfg)

	up, err := PlanUpgrade(cfg, dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if up.Absorb != kb || up.Refuse != "" || len(up.Did) == 0 {
		t.Fatalf("plan %+v", up)
	}
	if err := RunUpgrade(h, cfg, up, "", now); err != nil {
		t.Fatal(err)
	}
	p, err := project.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// The wiki, the raw store, the inbox, and the ideas are the project's now.
	for _, rel := range []string{"wiki/index.md", "wiki/concepts/Alarm.md", "inbox/paper.pdf", "ideas/note.md", ".raw/captured/abc.pdf"} {
		if _, err := os.Stat(p.Path(rel)); err != nil {
			t.Errorf("the project lacks %s", rel)
		}
	}
	// The identity file carries the mode and the joined description, and the old marker is gone.
	if p.Config.Mode != project.LYT || !strings.Contains(p.Config.Description, "The work.") || !strings.Contains(p.Config.Description, "fire-detection") {
		t.Fatalf("identity %+v", p.Config)
	}
	// The absorbed folder held nothing but the atlas's own files, so it is gone.
	if _, err := os.Stat(kb); err == nil {
		t.Error("the emptied knowledge base folder is removed after a move")
	}
	if !strings.Contains(strings.Join(up.Did, " "), "removed the empty folder") {
		t.Errorf("the report says so: %v", up.Did)
	}
	// The stage folders moved under threads/, and the thread still reads.
	for _, old := range []string{"stubs", "specs", "plans", "receipts", "phases"} {
		if _, err := os.Stat(p.Path(old)); err == nil {
			t.Errorf("%s/ is still beside threads/", old)
		}
	}
	board, err := threads.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(board.Threads) != 1 || board.Threads[0].Stage != threads.Stub || len(board.Problems) != 0 {
		t.Fatalf("board %+v", board)
	}
	if len(board.Phases) != 1 || board.Phases[0].Title != "Alpha" {
		t.Fatalf("phases %+v", board.Phases)
	}
	// git mv kept the history of the moved page.
	log, err := repo.LogFollow(p.Rel() + "/wiki/concepts/Alarm.md")
	if err != nil || len(log) < 2 {
		t.Fatalf("the moved page keeps its history: %+v %v", log, err)
	}
	// The knowledge base is out of the config, the project is in it.
	saved, _ := h.Load()
	if len(saved.Knowledge) != 0 || !saved.HasProject(dir) {
		t.Fatalf("config %+v", saved)
	}
	// Running it again changes nothing.
	again, err := PlanUpgrade(cfg, dir, now)
	if err != nil || len(again.Did) != 0 {
		t.Fatalf("a second upgrade: %+v %v", again, err)
	}
}

// The knowledge base is elsewhere and serves this project alone: it is copied, and the old
// folder is left on disk.
func TestUpgradeCopiesAKnowledgeBaseFromOutsideTheWork(t *testing.T) {
	h, cfg := atlas(t)
	dir := work(t, "webapp")
	kb := work(t, "notes")
	v3Knowledge(t, kb, "k2", "notes", "Notes.")
	v3Project(t, dir, "p2", "webapp", "k2", "notes")
	commitAll(t, dir)
	cfg.AddProject(dir)
	cfg.AddKnowledge(kb)
	h.Save(cfg)

	up, err := PlanUpgrade(cfg, dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if up.Absorb != kb {
		t.Fatalf("plan %+v", up)
	}
	if err := RunUpgrade(h, cfg, up, "", now); err != nil {
		t.Fatal(err)
	}
	p, _ := project.Open(dir)
	if _, err := os.Stat(p.Path("wiki/concepts/Alarm.md")); err != nil {
		t.Fatal("the page was copied in")
	}
	if _, err := os.Stat(filepath.Join(kb, "wiki", "concepts", "Alarm.md")); err != nil {
		t.Fatal("the old folder stays on disk")
	}
	if _, err := os.Stat(filepath.Join(kb, project.KnowledgeMarker)); err != nil {
		t.Fatal("a copied knowledge base keeps its identity file")
	}
	if !strings.Contains(strings.Join(up.Did, " "), "copied") {
		t.Fatalf("the report says it copied: %v", up.Did)
	}
}

// One knowledge base, several projects: refused, with the projects named.
func TestUpgradeRefusesASharedKnowledgeBase(t *testing.T) {
	h, cfg := atlas(t)
	kb := work(t, "shared")
	v3Knowledge(t, kb, "k3", "shared", "Shared.")
	one, two := work(t, "one"), work(t, "two")
	v3Project(t, one, "p3", "one", "k3", "shared")
	v3Project(t, two, "p4", "two", "k3", "shared")
	commitAll(t, one)
	commitAll(t, two)
	cfg.AddProject(one)
	cfg.AddProject(two)
	cfg.AddKnowledge(kb)
	h.Save(cfg)

	// The knowledge base itself refuses, and names the projects.
	up, err := PlanUpgrade(cfg, kb, now)
	if err != nil {
		t.Fatal(err)
	}
	if up.Refuse == "" || !strings.Contains(up.Refuse, "2 projects") {
		t.Fatalf("refuse %+v", up)
	}
	if err := RunUpgrade(h, cfg, up, "", now); err == nil {
		t.Fatal("a refused upgrade does nothing")
	}
	// Each project absorbs it in turn: the first by plan, the second by --absorb.
	first, _ := PlanUpgrade(cfg, one, now)
	if err := RunUpgrade(h, cfg, first, "", now); err != nil {
		t.Fatal(err)
	}
	if p, _ := project.Open(one); p == nil {
		t.Fatal("the first project upgraded")
	}
	second, _ := PlanUpgrade(cfg, two, now)
	if err := RunUpgrade(h, cfg, second, kb, now); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{one, two} {
		p, err := project.Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(p.Path("wiki/concepts/Alarm.md")); err != nil {
			t.Errorf("%s has the page", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(kb, project.KnowledgeMarker)); err != nil {
		t.Fatal("a copied knowledge base keeps its folder and its identity file")
	}
}

// A knowledge base no project uses becomes a project of its own.
func TestUpgradeMakesALoneKnowledgeBaseAProject(t *testing.T) {
	h, cfg := atlas(t)
	kb := work(t, "papers")
	v3Knowledge(t, kb, "k4", "papers", "Everything I read.")
	commitAll(t, kb)
	cfg.AddKnowledge(kb)
	h.Save(cfg)

	up, err := PlanUpgrade(cfg, kb, now)
	if err != nil {
		t.Fatal(err)
	}
	if up.Refuse != "" || len(up.Did) == 0 {
		t.Fatalf("plan %+v", up)
	}
	if err := RunUpgrade(h, cfg, up, "", now); err != nil {
		t.Fatal(err)
	}
	p, err := project.Open(kb)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "papers" || p.Config.ID != "k4" || p.Config.Mode != project.LYT || p.Config.Description != "Everything I read." {
		t.Fatalf("identity %+v", p.Config)
	}
	for _, rel := range []string{"wiki/index.md", "inbox/paper.pdf", "ideas/note.md", project.LedgerPath, project.Snippet} {
		if _, err := os.Stat(p.Path(rel)); err != nil {
			t.Errorf("the new project lacks %s", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(kb, "wiki")); err == nil {
		t.Error("the wiki moved out of the folder's root")
	}
	saved, _ := h.Load()
	if !saved.HasProject(kb) || len(saved.Knowledge) != 0 {
		t.Fatalf("config %+v", saved)
	}
}

// The flat layout of 2.2.0, with task pages, upgrades in one run.
func TestUpgradeMovesAFlatProjectAndMigratesTasks(t *testing.T) {
	h, cfg := atlas(t)
	dir := work(t, "old")
	flat := filepath.Join(dir, project.Dir)
	os.MkdirAll(filepath.Join(flat, "tasks"), 0o755)
	os.WriteFile(filepath.Join(flat, project.Marker),
		[]byte(`{"schema":"claude-atlas.project.v3","id":"p5","name":"old","created":"2026-09-16"}`), 0o644)
	os.WriteFile(filepath.Join(flat, "tasks", "Do it.md"),
		[]byte("---\ntype: task\ntitle: \"Do it\"\nstatus: planted\npriority: high\nphase: \"\"\ndue: \"\"\ncreated: 2026-09-16\nupdated: 2026-09-16\ntask_id: task-20260916-aaaa\n---\n\n## Idea\n\nThe idea.\n"), 0o644)
	commitAll(t, dir)
	cfg.AddProject(dir)
	h.Save(cfg)

	up, err := PlanUpgrade(cfg, dir, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := RunUpgrade(h, cfg, up, "", now); err != nil {
		t.Fatal(err)
	}
	p, err := project.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Rel() != project.Dir+"/old" {
		t.Fatalf("folder %s", p.Rel())
	}
	board, err := threads.Load(p)
	if err != nil || len(board.Threads) != 1 || board.Threads[0].ID != "thr-20260916-aaaa" {
		t.Fatalf("board %+v %v", board, err)
	}
	if _, err := os.Stat(p.Path(project.StubsDir + "/Do it.md")); err != nil {
		t.Fatal("the task's idea became a stub under threads/")
	}
}

func TestUpgradeTargetsAndStale(t *testing.T) {
	h, cfg := atlas(t)
	dir := work(t, "webapp")
	kb := work(t, "notes")
	cfg.AddProject(dir)
	cfg.AddKnowledge(kb)
	h.Save(cfg)
	targets := UpgradeTargets(cfg)
	if len(targets) != 2 || targets[0] != dir || targets[1] != kb {
		t.Fatalf("projects come first: %v", targets)
	}
	if _, err := PlanUpgrade(cfg, t.TempDir(), now); err == nil {
		t.Fatal("a folder that is neither")
	}
}
