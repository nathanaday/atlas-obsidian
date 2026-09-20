package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/home"
)

var now = time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

func needGit(t *testing.T) {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
}

// newWork is an empty folder to make a project in.
func newWork(t *testing.T, name string) string {
	t.Helper()
	needGit(t)
	work := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	return work
}

func newProject(t *testing.T) *Project {
	t.Helper()
	res, err := Init(newWork(t, "webapp"), Options{}, now)
	if err != nil {
		t.Fatal(err)
	}
	return res.Project
}

func TestInitWritesBothHalvesAndCommitsThem(t *testing.T) {
	work := newWork(t, "webapp")
	res, err := Init(work, Options{Description: "The app.", Mode: Generic}, now)
	if err != nil {
		t.Fatal(err)
	}
	p := res.Project
	if p.Root != work || p.Folder != "webapp" || p.Rel() != "atlas/webapp" {
		t.Fatalf("project %+v", p)
	}
	if res.Git != GitCreated || res.Commit == "" || !Same(res.Host, work) {
		t.Fatalf("git %+v", res)
	}
	// The wiki, the threads, the inbox, the ideas, and the Obsidian settings, in one folder.
	for _, rel := range []string{
		Marker, LedgerPath, LogPage, HotPage, IndexPage, OverviewPage, Snippet, AppFile, AppearanceFile, ".gitignore",
		"inbox/.gitkeep", "ideas/.gitkeep",
	} {
		if _, err := os.Stat(p.Path(rel)); err != nil {
			t.Errorf("a new project lacks %s", rel)
		}
	}
	for _, dir := range Folders {
		if info, err := os.Stat(p.Path(dir)); err != nil || !info.IsDir() {
			t.Errorf("a new project lacks the folder %s", dir)
		}
	}
	// Nothing of the two-entity layout is left.
	for _, rel := range []string{KnowledgeMarker, "stubs", "specs", "plans", "receipts", "phases", "wiki/questions", "kb", "repos"} {
		if _, err := os.Stat(p.Path(rel)); err == nil {
			t.Errorf("a new project has %s", rel)
		}
	}
	cfg, ok := ReadMarker(p.Atlas())
	if !ok || cfg.Schema != Schema || cfg.ID == "" || cfg.Name != "webapp" || cfg.Mode != Generic || cfg.Description != "The app." || cfg.Created != "2026-09-19" {
		t.Fatalf("identity %+v", cfg)
	}
	// The setup commit holds the whole folder, and the work is clean afterwards.
	repo := p.Work()
	if dirty, _ := repo.Dirty(); dirty {
		st, _ := repo.Status()
		t.Fatalf("the work is clean after init: %+v", st)
	}
	ops, err := repo.Log(1)
	if err != nil || len(ops) != 1 || !strings.HasPrefix(ops[0].Subject, "setup: initialize project webapp") {
		t.Fatalf("log %+v %v", ops, err)
	}
}

func TestInitInsideARepositoryCommitsThereAndNeverInitsAgain(t *testing.T) {
	needGit(t)
	host := gitx.Repo{Dir: t.TempDir()}
	if err := host.Init(); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(host.Dir, "main.go"), []byte("package main\n"), 0o644)
	host.AddAll()
	host.Commit("feat: code")
	work := filepath.Join(host.Dir, "sub")
	os.MkdirAll(work, 0o755)
	res, err := Init(work, Options{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Git != GitEnclosed || !Same(res.Host, host.Dir) {
		t.Fatalf("git %+v", res)
	}
	if _, err := os.Stat(filepath.Join(work, ".git")); err == nil {
		t.Fatal("no repository inside another")
	}
	if commits, _ := host.Log(0); len(commits) != 2 {
		t.Fatalf("the setup commit goes into the host: %+v", commits)
	}
}

func TestInitRefusals(t *testing.T) {
	needGit(t)
	work := newWork(t, "webapp")
	if _, err := Init(work, Options{}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(work, Options{}, now); err == nil || !strings.Contains(err.Error(), "a project already") {
		t.Fatalf("twice: %v", err)
	}
	inside := filepath.Join(work, "deep")
	os.MkdirAll(inside, 0o755)
	if _, err := Init(inside, Options{}, now); err == nil || !strings.Contains(err.Error(), "inside the project") {
		t.Fatalf("inside another project: %v", err)
	}
	if _, err := Init(filepath.Join(t.TempDir(), "gone"), Options{}, now); err == nil {
		t.Fatal("a folder that is not there")
	}
	// A 3.x knowledge base is not a folder to init; upgrade absorbs it.
	kb := newWork(t, "notes")
	os.WriteFile(filepath.Join(kb, KnowledgeMarker), []byte(`{"schema":"claude-atlas.vault.v3","id":"k1","kind":"knowledge","name":"notes"}`), 0o644)
	if _, err := Init(kb, Options{}, now); err == nil || !strings.Contains(err.Error(), "upgrade") {
		t.Fatalf("a 3.x knowledge base: %v", err)
	}
	// A folder the repository ignores could not be committed.
	host := newWork(t, "host")
	repo := gitx.Repo{Dir: host}
	repo.Init()
	os.WriteFile(filepath.Join(host, ".gitignore"), []byte("atlas/\n"), 0o644)
	if _, err := Init(host, Options{}, now); err == nil || !strings.Contains(err.Error(), "ignored") {
		t.Fatalf("an ignored folder: %v", err)
	}
	// A name that leaves no folder.
	if _, err := Init(newWork(t, "n2"), Options{Name: "///"}, now); err == nil || !strings.Contains(err.Error(), "usable folder name") {
		t.Fatalf("an unusable name: %v", err)
	}
	// A taken atlas/<name>/.
	taken := newWork(t, "taken")
	os.MkdirAll(filepath.Join(taken, Dir, "taken"), 0o755)
	os.WriteFile(filepath.Join(taken, Dir, "taken", "notes.md"), []byte("x"), 0o644)
	if _, err := Init(taken, Options{}, now); err == nil || !strings.Contains(err.Error(), "holds something") {
		t.Fatalf("a taken folder: %v", err)
	}
}

func TestInitWithoutGitLeavesNoHistory(t *testing.T) {
	work := newWork(t, "docs")
	res, err := Init(work, Options{NoGit: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Git != GitSkipped || res.Commit != "" || res.Host != "" {
		t.Fatalf("git %+v", res)
	}
	if _, err := os.Stat(filepath.Join(work, ".git")); err == nil {
		t.Fatal("--no-git made a repository")
	}
}

func TestOpenRefusesTheLayoutsOfEarlierVersions(t *testing.T) {
	needGit(t)
	// A 3.x project, which named a knowledge base of its own.
	work := newWork(t, "webapp")
	os.MkdirAll(filepath.Join(work, Dir, "webapp"), 0o755)
	os.WriteFile(filepath.Join(work, Dir, "webapp", Marker),
		[]byte(`{"schema":"claude-atlas.project.v3","id":"p1","name":"webapp","knowledge":{"id":"k1","name":"notes"}}`), 0o644)
	if !IsProject(work) {
		t.Fatal("a 3.x project is still a project on disk")
	}
	_, err := Open(work)
	if !errors.Is(err, ErrSplit) || !strings.Contains(err.Error(), "upgrade") {
		t.Fatalf("v3: %v", err)
	}
	if v3, ok := ReadV3(work); !ok || v3.Knowledge == nil || v3.Knowledge.ID != "k1" {
		t.Fatalf("ReadV3 %+v %v", v3, ok)
	}
	// The flat layout of 2.2.0 and earlier.
	flat := newWork(t, "flat")
	os.MkdirAll(filepath.Join(flat, Dir), 0o755)
	os.WriteFile(filepath.Join(flat, Dir, Marker), []byte(`{"schema":"claude-atlas.project.v3","id":"p2","name":"flat"}`), 0o644)
	if _, err := Open(flat); !errors.Is(err, ErrFlat) {
		t.Fatalf("flat: %v", err)
	}
	// An unknown schema, and a folder that is no project at all.
	later := newWork(t, "later")
	os.MkdirAll(filepath.Join(later, Dir, "later"), 0o755)
	os.WriteFile(filepath.Join(later, Dir, "later", Marker), []byte(`{"schema":"claude-atlas.project.v9","id":"p3"}`), 0o644)
	if _, err := Open(later); err == nil || !strings.Contains(err.Error(), "unsupported schema") {
		t.Fatalf("v9: %v", err)
	}
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotProject) {
		t.Fatalf("no project: %v", err)
	}
}

func TestLocateRefusesTwoProjectsAndReadsPastAFileNamedAtlas(t *testing.T) {
	needGit(t)
	work := newWork(t, "two")
	for _, name := range []string{"a", "b"} {
		os.MkdirAll(filepath.Join(work, Dir, name), 0o755)
		os.WriteFile(filepath.Join(work, Dir, name, Marker), []byte(`{"schema":"`+Schema+`","id":"`+name+`"}`), 0o644)
	}
	if _, err := Locate(work); err == nil || !strings.Contains(err.Error(), "holds 2 projects") {
		t.Fatalf("two projects: %v", err)
	}
	// A file named atlas, such as a binary, is not a project.
	bin := newWork(t, "bin")
	os.WriteFile(filepath.Join(bin, Dir), []byte("ELF"), 0o644)
	if _, err := Locate(bin); !errors.Is(err, ErrNotProject) {
		t.Fatalf("a file named atlas: %v", err)
	}
	if IsProject(bin) {
		t.Fatal("a file named atlas is not a project")
	}
}

func TestFindAboveAndKnowledgeAbove(t *testing.T) {
	p := newProject(t)
	deep := filepath.Join(p.Root, "src", "deep")
	os.MkdirAll(deep, 0o755)
	if FindAbove(deep) != p.Root {
		t.Fatal("a session anywhere inside the work belongs to the project")
	}
	if FindAbove(p.Atlas()) != p.Root {
		t.Fatal("inside the project's own folder too")
	}
	if FindAbove(t.TempDir()) != "" {
		t.Fatal("nothing above")
	}
	kb := newWork(t, "notes")
	os.WriteFile(filepath.Join(kb, KnowledgeMarker), []byte("{}"), 0o644)
	if KnowledgeAbove(filepath.Join(kb, "wiki")) != kb || KnowledgeAbove(p.Root) != "" {
		t.Fatal("KnowledgeAbove")
	}
}

func TestSaveRenamesTheFolderAndRefusesATakenOne(t *testing.T) {
	p := newProject(t)
	p.Config.Name = "Web App"
	p.Config.Description = "  spaced  "
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	if p.Folder != "Web App" || p.Rel() != "atlas/Web App" {
		t.Fatalf("folder %q", p.Folder)
	}
	if _, err := os.Stat(p.Path(Marker)); err != nil {
		t.Fatal("the identity file moved with the folder")
	}
	if _, err := os.Stat(filepath.Join(p.Root, Dir, "webapp")); err == nil {
		t.Fatal("the old folder is gone")
	}
	again, err := Open(p.Root)
	if err != nil || again.Config.Name != "Web App" || again.Config.Description != "spaced" {
		t.Fatalf("reopened %+v %v", again, err)
	}
	// A folder that is taken refuses the whole save.
	os.MkdirAll(filepath.Join(p.Root, Dir, "Other"), 0o755)
	p.Config.Name = "Other"
	if err := p.Save(); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("taken: %v", err)
	}
	if p.Folder != "Web App" {
		t.Fatalf("the folder stays after a refused save: %q", p.Folder)
	}
	p.Config.Name = " "
	if err := p.Save(); err == nil {
		t.Fatal("a blank name")
	}
}

func TestUpdateConfigCommitsOnceAndValidates(t *testing.T) {
	p := newProject(t)
	before, _ := p.Engine().Log(0)
	if err := UpdateConfig(p.Root, "edit mode", now, func(c *Config) error {
		c.Mode = LYT
		c.Description = "Notes about the app."
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	after, _ := Open(p.Root)
	if after.Config.Mode != LYT || after.Config.Description != "Notes about the app." {
		t.Fatalf("identity %+v", after.Config)
	}
	log, _ := p.Engine().Log(0)
	if len(log) != len(before)+1 || log[0].Subject != "setup: edit mode" {
		t.Fatalf("one commit: %+v", log)
	}
	// An unchanged file makes no commit.
	if err := UpdateConfig(p.Root, "edit again", now, func(c *Config) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if again, _ := p.Engine().Log(0); len(again) != len(log) {
		t.Fatalf("no commit for no change: %+v", again)
	}
	for _, bad := range []func(*Config){
		func(c *Config) { c.ID = "other" },
		func(c *Config) { c.Name = "" },
		func(c *Config) { c.Mode = "para" },
	} {
		if err := UpdateConfig(p.Root, "bad", now, func(c *Config) error { bad(c); return nil }); err == nil {
			t.Fatal("an invalid change was accepted")
		}
	}
	if err := UpdateConfig(p.Root, "x", now, func(c *Config) error { return errors.New("no") }); err == nil {
		t.Fatal("the change function's error stands")
	}
}

func TestTheEngineScopeIsTheWikiAndNothingElse(t *testing.T) {
	p := newProject(t)
	engine := p.Engine()
	if engine.Prefix != "atlas/webapp/" || len(engine.Scope) != 4 {
		t.Fatalf("scope %+v", engine)
	}
	// A change to the code and to a thread document is outside the engine's sight.
	os.WriteFile(filepath.Join(p.Root, "main.go"), []byte("package main\n"), 0o644)
	os.WriteFile(p.Path(PlansDir+"/Fix it.md"), []byte("progress\n"), 0o644)
	os.MkdirAll(p.Path("wiki/concepts"), 0o755)
	if err := os.WriteFile(p.Path("wiki/concepts/A.md"), []byte("---\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := engine.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 1 || st[0].Path != "wiki/concepts/A.md" {
		t.Fatalf("the engine sees the wiki only: %+v", st)
	}
	if dirty, _ := p.Work().Dirty(); !dirty {
		t.Fatal("the work has its own changes")
	}
}

func TestEnsureFoldersRebuildsWhatACloneLeftOut(t *testing.T) {
	p := newProject(t)
	for _, dir := range Folders {
		os.RemoveAll(p.Path(dir))
	}
	if err := p.EnsureFolders(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range Folders {
		if info, err := os.Stat(p.Path(dir)); err != nil || !info.IsDir() {
			t.Errorf("%s was not rebuilt", dir)
		}
	}
}

func TestFolderNameAndHostFor(t *testing.T) {
	needGit(t)
	if FolderName(" Web App ") != "Web App" || FolderName("a/b") == "a/b" || FolderName("///") != "" {
		t.Fatalf("FolderName: %q %q %q", FolderName(" Web App "), FolderName("a/b"), FolderName("///"))
	}
	host := newWork(t, "host")
	repo := gitx.Repo{Dir: host}
	repo.Init()
	got, err := HostFor(filepath.Join(host, Dir, "x"))
	if err != nil || got != host {
		t.Fatalf("HostFor inside a repository: %q %v", got, err)
	}
	if got, err := HostFor(filepath.Join(t.TempDir(), "nowhere")); err != nil || got != "" {
		t.Fatalf("HostFor outside one: %q %v", got, err)
	}
}

func TestResolveOrder(t *testing.T) {
	p := newProject(t)
	other := newProject(t)
	if got, err := Resolve(other.Root, p.Root, p.Root); err != nil || got.Root != other.Root {
		t.Fatalf("explicit wins: %v %v", got, err)
	}
	if got, err := Resolve("", p.Root, ""); err != nil || got.Root != p.Root {
		t.Fatalf("the environment next: %v %v", got, err)
	}
	if got, err := Resolve("", "", filepath.Join(p.Root, "src")); err != nil || got.Root != p.Root {
		t.Fatalf("then the walk up: %v %v", got, err)
	}
	if _, err := Resolve("", "", t.TempDir()); !errors.Is(err, ErrNotProject) {
		t.Fatalf("nothing: %v", err)
	}
}

func TestDisplayPathsInErrors(t *testing.T) {
	// home.Display shortens a path under the user's home, so a message never carries the
	// full path when it need not.
	if home.Display(filepath.Join(os.Getenv("HOME"), "code")) != "~/code" {
		t.Skip("no HOME to shorten")
	}
}
