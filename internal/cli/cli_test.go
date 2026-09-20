package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/console"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

type harness struct {
	t    *testing.T
	home string
	out  bytes.Buffer
	err  bytes.Buffer
}

func (h *harness) run(args ...string) int {
	h.out.Reset()
	h.err.Reset()
	c := console.NewWith(true, strings.NewReader(""), &h.out, false)
	return run(append([]string{"--home", h.home}, args...), strings.NewReader(""), &h.out, &h.err, c)
}

// setup makes an atlas home and runs from a folder that is inside nothing.
func setup(t *testing.T) *harness {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	t.Chdir(root)
	fakeClaudeCode(t, root)
	h := &harness{t: t, home: filepath.Join(root, "home")}
	if code := h.run("setup", "--no-plugin"); code != 0 {
		t.Fatalf("setup exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	return h
}

// fakeClaudeCode points Claude Code's config directory at a temporary one that reports the
// plugin as installed, so doctor reads a fixture and never the developer's own ~/.claude.
func fakeClaudeCode(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "claude-code")
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	installed := fmt.Sprintf(`{"plugins":{%q:[{"scope":"user","installPath":%q,"version":""}]}}`,
		home.DefaultPluginID, filepath.Join(dir, "plugins", "atlas-obsidian"))
	if err := os.WriteFile(filepath.Join(dir, "plugins", "installed_plugins.json"), []byte(installed), 0o644); err != nil {
		t.Fatal(err)
	}
	marketplaces := fmt.Sprintf(`{%q:{"source":{}}}`, "nathanaday-atlas-obsidian")
	if err := os.WriteFile(filepath.Join(dir, "plugins", "known_marketplaces.json"), []byte(marketplaces), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (h *harness) config(t *testing.T) *home.Config {
	t.Helper()
	cfg, err := home.Home{Root: h.home}.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

// work makes a folder to become a project, as a git repository when asked.
func work(t *testing.T, name string, git bool) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if git {
		repo := gitx.Repo{Dir: dir}
		if err := repo.Init(); err != nil {
			t.Fatal(err)
		}
		if err := repo.AddAll(); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Commit("chore: initial"); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSetupMakesTheHomeAndNothingElse(t *testing.T) {
	h := setup(t)
	if !strings.Contains(h.out.String(), "Setup complete.") || !strings.Contains(h.out.String(), "atlas-obsidian init") {
		t.Fatalf("output:\n%s", h.out.String())
	}
	for _, path := range []string{filepath.Join(h.home, "config.json"), filepath.Join(h.home, "state", "registry.json")} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s", path)
		}
	}
	cfg, _ := os.ReadFile(filepath.Join(h.home, "config.json"))
	if !strings.Contains(string(cfg), `"schema": "atlas-obsidian.config.v1"`) {
		t.Fatalf("config.json:\n%s", cfg)
	}
	if code := h.run("setup", "--no-plugin"); code != 0 || !strings.Contains(h.out.String(), "keep       0 listed") {
		t.Fatalf("rerun exit %d:\n%s", code, h.out.String())
	}
}

func TestInitWritesBothHalvesAndRegistersTheProject(t *testing.T) {
	h := setup(t)
	dir := work(t, "webapp", true)
	code := h.run("init", dir, "--description", "The web app.", "--mode", "lyt")
	if code != 0 {
		t.Fatalf("init exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	out := h.out.String()
	for _, want := range []string{"project", "webapp at", "wrote", "atlas/webapp/", "wiki/index.md", "committed", "registered", "Next:", "atlas-obsidian describe", "open-vault"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	p, err := project.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Config.Description != "The web app." || p.Config.Mode != project.LYT {
		t.Fatalf("identity %+v", p.Config)
	}
	for _, rel := range []string{project.Marker, project.IndexPage, project.LedgerPath, project.Snippet, "threads", "inbox", "ideas"} {
		if _, err := os.Stat(p.Path(rel)); err != nil {
			t.Errorf("missing %s", rel)
		}
	}
	if !h.config(t).HasProject(dir) {
		t.Fatal("the config lists the work folder")
	}
	// list and show
	if code := h.run("list"); code != 0 || !strings.Contains(h.out.String(), "webapp") {
		t.Fatalf("list exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("show", "webapp"); code != 0 {
		t.Fatalf("show exit %d %s", code, h.err.String())
	}
	for _, want := range []string{"Name", "webapp", "Wiki", "Mode", "lyt", "Description", "The web app.", "Threads", "Page"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("show misses %q:\n%s", want, h.out.String())
		}
	}
}

func TestInitRefusals(t *testing.T) {
	h := setup(t)
	dir := work(t, "webapp", false)
	if code := h.run("init", dir); code != 0 {
		t.Fatalf("init exit %d %s", code, h.err.String())
	}
	if code := h.run("init", dir); code != 1 || !strings.Contains(h.err.String(), "a project already") {
		t.Fatalf("twice exit %d %s", code, h.err.String())
	}
	inside := filepath.Join(dir, "sub")
	os.MkdirAll(inside, 0o755)
	if code := h.run("init", inside); code != 1 || !strings.Contains(h.err.String(), "inside the project") {
		t.Fatalf("inside exit %d %s", code, h.err.String())
	}
	if code := h.run("init", filepath.Join(t.TempDir(), "gone")); code != 1 {
		t.Fatalf("a folder that is not there exit %d", code)
	}
	if code := h.run("init", work(t, "bad", false), "--mode", "para"); code != 1 || !strings.Contains(h.err.String(), "mode must be") {
		t.Fatalf("a bad mode exit %d %s", code, h.err.String())
	}
}

func TestInitMakesTheWorkAGitRepository(t *testing.T) {
	h := setup(t)
	plain := work(t, "docs", false)
	if code := h.run("init", plain); code != 0 || !strings.Contains(h.out.String(), "initialized a repository") {
		t.Fatalf("init exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if !(gitx.Repo{Dir: plain}).IsRepo() {
		t.Fatal("the work is a repository")
	}
	// The wiki's first commit is in it.
	if commits, _ := (gitx.Repo{Dir: plain}).Log(0); len(commits) != 1 || !strings.HasPrefix(commits[0].Subject, "setup:") {
		t.Fatalf("log %+v", commits)
	}
	none := work(t, "nogit", false)
	if code := h.run("init", none, "--no-git"); code != 0 || !strings.Contains(h.out.String(), "--no-git") {
		t.Fatalf("--no-git exit %d\n%s", code, h.out.String())
	}
	if (gitx.Repo{Dir: none}).IsRepo() {
		t.Fatal("--no-git made a repository")
	}
}

func TestEditRenamesTheFolderAndForgetKeepsIt(t *testing.T) {
	h := setup(t)
	dir := work(t, "webapp", true)
	h.run("init", dir)
	if code := h.run("edit", "webapp", "--name", "Web App", "--description", "The app.", "--mode", "lyt"); code != 0 {
		t.Fatalf("edit exit %d %s", code, h.err.String())
	}
	if !strings.Contains(h.out.String(), "atlas/Web App/") {
		t.Fatalf("the folder follows the name:\n%s", h.out.String())
	}
	p, err := project.Open(dir)
	if err != nil || p.Folder != "Web App" || p.Config.Mode != project.LYT || p.Config.Description != "The app." {
		t.Fatalf("project %+v %v", p, err)
	}
	if code := h.run("edit", "Web App"); code != 2 {
		t.Fatalf("edit with nothing to change exit %d", code)
	}
	if code := h.run("forget", "Web App"); code != 0 || !strings.Contains(h.out.String(), "forgot") {
		t.Fatalf("forget exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if h.config(t).HasProject(dir) {
		t.Fatal("the config no longer lists it")
	}
	if !project.IsProject(dir) {
		t.Fatal("the folder stays")
	}
}

func TestThreadAndPhaseCommands(t *testing.T) {
	h := setup(t)
	if code := h.run("threads"); code != 0 || !strings.Contains(h.out.String(), "no open threads in any project") {
		t.Fatalf("threads exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	dir := work(t, "webapp", false)
	if code := h.run("init", dir); code != 0 {
		t.Fatalf("init exit %d %s", code, h.err.String())
	}
	atlas := filepath.Join(dir, project.Dir, "webapp")
	if code := h.run("thread", "webapp", "new", "Fix", "the", "dialog", "--priority", "high"); code != 0 || !strings.Contains(h.out.String(), "threads/stubs/Fix the dialog.md") {
		t.Fatalf("new exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if data, _ := os.ReadFile(filepath.Join(atlas, "threads", "threads.md")); !strings.Contains(string(data), "Fix the dialog") {
		t.Fatalf("threads.md should list the thread:\n%s", data)
	}
	if code := h.run("threads", "webapp"); code != 0 || !strings.Contains(h.out.String(), "  stub\n    high     Fix the dialog") {
		t.Fatalf("threads webapp exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("threads"); code != 0 || !strings.Contains(h.out.String(), "webapp\n") {
		t.Fatalf("all threads:\n%s", h.out.String())
	}
	if code := h.run("thread", "webapp"); code != 2 {
		t.Fatalf("thread with no action exit %d", code)
	}
	if code := h.run("thread", "webapp", "new"); code != 1 || !strings.Contains(h.err.String(), "needs a title or some text") {
		t.Fatalf("new without text exit %d %s", code, h.err.String())
	}
	if code := h.run("thread", "webapp", "new", "x", "--phase", "Nope"); code != 1 || !strings.Contains(h.err.String(), "no phase named") {
		t.Fatalf("an unknown phase exit %d %s", code, h.err.String())
	}
	// Phases.
	if code := h.run("phase", "webapp", "create", "Alarm quality", "--goal", "Fewer false alarms."); code != 0 {
		t.Fatalf("phase create exit %d %s", code, h.err.String())
	}
	if code := h.run("thread", "webapp", "new", "Filter vehicles", "--phase", "alarm quality"); code != 0 {
		t.Fatalf("new in a phase exit %d %s", code, h.err.String())
	}
	if code := h.run("phase", "webapp", "rename", "Alarm quality", "--to", "Alarms"); code != 0 {
		t.Fatalf("phase rename exit %d %s", code, h.err.String())
	}
	if data, _ := os.ReadFile(filepath.Join(atlas, "threads", "Filter vehicles.md")); !strings.Contains(string(data), `phase: "Alarms"`) {
		t.Fatalf("rename should follow into the thread:\n%s", data)
	}
	if code := h.run("phase", "webapp", "remove", "Alarms"); code != 1 || !strings.Contains(h.err.String(), "still names") {
		t.Fatalf("remove a phase in use exit %d %s", code, h.err.String())
	}
	// Documents.
	if code := h.run("thread", "webapp", "file", "fix the", "spec", "--text", "Enter confirms."); code != 0 || !strings.Contains(h.out.String(), "spec · high") {
		t.Fatalf("file spec exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	planFile := filepath.Join(dir, "plan.md")
	os.WriteFile(planFile, []byte("1. Bind the key.\n"), 0o644)
	if code := h.run("thread", "webapp", "file", "Fix the dialog", "plan", "--file", planFile); code != 0 {
		t.Fatalf("file plan exit %d %s", code, h.err.String())
	}
	if data, _ := os.ReadFile(filepath.Join(atlas, "threads", "plans", "Fix the dialog.md")); !strings.Contains(string(data), "1. Bind the key.") {
		t.Fatalf("the plan holds the file's text:\n%s", data)
	}
	if code := h.run("thread", "webapp", "show", "Fix the dialog", "--json"); code != 0 || !strings.Contains(h.out.String(), `"stage": "plan"`) {
		t.Fatalf("show --json exit %d\n%s", code, h.out.String())
	}
	if code := h.run("thread", "webapp", "set", "Filter vehicles", "--phase", "", "--blocked", "the vendor"); code != 0 || !strings.Contains(h.out.String(), "blocked: the vendor") {
		t.Fatalf("set exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if code := h.run("phase", "webapp", "remove", "Alarms"); code != 0 {
		t.Fatalf("remove a free phase exit %d %s", code, h.err.String())
	}
	if code := h.run("thread", "webapp", "close", "Fix the dialog", "Enter", "confirms", "now."); code != 0 || !strings.Contains(h.out.String(), "completed") {
		t.Fatalf("close exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if _, err := os.Stat(filepath.Join(atlas, "threads", "archive", "Fix the dialog.md")); err != nil {
		t.Fatal("a closed thread's card moves to the archive")
	}
	if code := h.run("threads", "webapp", "--all"); code != 0 || !strings.Contains(h.out.String(), "receipt") {
		t.Fatalf("threads --all:\n%s", h.out.String())
	}
	if code := h.run("thread", "webapp", "reopen", "Fix the dialog"); code != 0 || !strings.Contains(h.out.String(), "reopened") {
		t.Fatalf("reopen exit %d %s", code, h.err.String())
	}
	// From inside the work, "." and nothing both mean this project.
	t.Chdir(dir)
	if code := h.run("thread", ".", "new", "From inside"); code != 0 {
		t.Fatalf("new with . exit %d %s", code, h.err.String())
	}
	if code := h.run("threads"); code != 0 || !strings.Contains(h.out.String(), "From inside") || strings.Contains(h.out.String(), "webapp\n") {
		t.Fatalf("threads inside the work lists this project alone:\n%s", h.out.String())
	}
}

func TestTheWikiCommandsRunAgainstTheProject(t *testing.T) {
	h := setup(t)
	dir := work(t, "webapp", true)
	h.run("init", dir)
	p, _ := project.Open(dir)
	// ingest stages a folder's files into the inbox and remembers the folder.
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "paper.md"), []byte("# A paper\n"), 0o644)
	if code := h.run("ingest", "webapp", src, "--no-claude"); code != 0 || !strings.Contains(h.out.String(), "staged") {
		t.Fatalf("ingest exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if _, err := os.Stat(p.Path("inbox/" + filepath.Base(src) + "/paper.md")); err != nil {
		t.Fatalf("the file is in the inbox: %v", err)
	}
	if code := h.run("ingest", "webapp", "--dry-run"); code != 0 || !strings.Contains(h.out.String(), "unchanged") {
		t.Fatalf("a second ingest of the remembered folder:\n%s", h.out.String())
	}
	// A page that links a page nobody wrote, then stub, lint, history, undo.
	os.MkdirAll(p.Path("wiki/concepts"), 0o755)
	os.WriteFile(p.Path("wiki/concepts/Seed.md"), []byte("---\ntitle: Seed\ntype: concept\nstatus: seed\ncreated: 2026-09-19\nupdated: 2026-09-19\ntags:\n  - concept\n---\n\n# Seed\n\nSee [[Gradient]].\n"), 0o644)
	if code := h.run("lint", "webapp"); code != 0 || !strings.Contains(h.out.String(), "Gradient") {
		t.Fatalf("lint exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("lint", "webapp", "--strict"); code != 1 {
		t.Fatalf("--strict exit %d", code)
	}
	if code := h.run("stub", "webapp"); code != 0 || !strings.Contains(h.out.String(), "wiki/concepts/Gradient.md") {
		t.Fatalf("stub exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if code := h.run("history", "webapp"); code != 0 || !strings.Contains(h.out.String(), "stub") {
		t.Fatalf("history exit %d:\n%s", code, h.out.String())
	}
	var op string
	for _, line := range strings.Split(h.out.String(), "\n") {
		if strings.Contains(line, "stub-") {
			for _, field := range strings.Fields(line) {
				if strings.HasPrefix(field, "stub-") {
					op = field
				}
			}
		}
	}
	if op == "" {
		t.Fatalf("no operation id in:\n%s", h.out.String())
	}
	if code := h.run("undo", "webapp", op); code != 0 || !strings.Contains(h.out.String(), "undone") {
		t.Fatalf("undo exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if _, err := os.Stat(p.Path("wiki/concepts/Gradient.md")); !os.IsNotExist(err) {
		t.Fatal("undo took the stub back")
	}
	if code := h.run("recover", "webapp"); code != 0 || !strings.Contains(h.out.String(), "nothing was interrupted") {
		t.Fatalf("recover exit %d:\n%s", code, h.out.String())
	}
	// From inside the work, the commands need no name.
	t.Chdir(dir)
	if code := h.run("lint"); code != 0 {
		t.Fatalf("lint inside the work exit %d %s", code, h.err.String())
	}
}

func TestDescribeStagesASnapshot(t *testing.T) {
	h := setup(t)
	dir := work(t, "webapp", true)
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# guide\n"), 0o644)
	if code := h.run("init", dir); code != 0 {
		t.Fatalf("init exit %d %s", code, h.err.String())
	}
	if code := h.run("describe", "webapp", "--no-claude"); code != 0 || !strings.Contains(h.out.String(), "staged") || !strings.Contains(h.out.String(), "/atlas-obsidian:describe") {
		t.Fatalf("describe exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	p, _ := project.Open(dir)
	files, err := os.ReadDir(p.Path("inbox"))
	if err != nil || len(files) == 0 {
		t.Fatalf("the snapshot is in the project's inbox: %v %v", files, err)
	}
	if code := h.run("describe", "webapp", "--no-claude"); code != 0 || !strings.Contains(h.out.String(), "unchanged") {
		t.Fatalf("the same snapshot twice:\n%s", h.out.String())
	}
}

func TestRefreshAndDoctorReportProblems(t *testing.T) {
	h := setup(t)
	dir := work(t, "webapp", true)
	h.run("init", dir)
	if code := h.run("refresh"); code != 0 || !strings.Contains(h.out.String(), "webapp") || !strings.Contains(h.out.String(), "wrote") {
		t.Fatalf("refresh exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("doctor"); code != 0 {
		t.Fatalf("doctor on a clean atlas exit %d:\n%s", code, h.out.String())
	}
	// A registered folder that is no project.
	stray := work(t, "stray", false)
	cfg := h.config(t)
	cfg.AddProject(stray)
	home.Home{Root: h.home}.Save(cfg)
	if code := h.run("doctor"); code != 1 || !strings.Contains(h.out.String(), "atlas-obsidian init") {
		t.Fatalf("doctor names the stray folder exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("forget", stray); code != 0 {
		t.Fatalf("forget exit %d %s", code, h.err.String())
	}
	// A folder that is gone.
	gone := work(t, "gone", true)
	h.run("init", gone)
	os.RemoveAll(gone)
	if code := h.run("doctor"); code != 1 || !strings.Contains(h.out.String(), "missing") {
		t.Fatalf("doctor names the missing folder:\n%s", h.out.String())
	}
	if code := h.run("forget", gone); code != 0 {
		t.Fatalf("forget a gone project exit %d %s", code, h.err.String())
	}
	if code := h.run("info"); code != 0 || !strings.Contains(h.out.String(), "atlas-obsidian") || !strings.Contains(h.out.String(), "entries") {
		t.Fatalf("info exit %d:\n%s", code, h.out.String())
	}
}

func TestCommandsNeedSetupFirst(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	t.Chdir(root)
	h := &harness{t: t, home: filepath.Join(root, "home")}
	for _, args := range [][]string{{"list"}, {"refresh"}, {"init"}} {
		if code := h.run(args...); code == 0 {
			t.Errorf("%v without an atlas exit 0:\n%s%s", args, h.out.String(), h.err.String())
		}
	}
}

func TestBareCommandExplainsWithNoAtlas(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	t.Chdir(root)
	h := &harness{t: t, home: filepath.Join(root, "home")}
	var out, errOut bytes.Buffer
	c := console.NewWith(true, strings.NewReader(""), &out, true)
	if code := run([]string{"--home", h.home}, strings.NewReader(""), &out, &errOut, c); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), "run `atlas-obsidian setup`") || !strings.Contains(out.String(), "atlas-obsidian init") {
		t.Fatalf("the bare command explains:\n%s", out.String())
	}
}

func TestConfigNewDays(t *testing.T) {
	h := setup(t)
	if code := h.run("config"); code != 0 || !strings.Contains(h.out.String(), "new-days") {
		t.Fatalf("config exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("config", "new-days", "3"); code != 0 {
		t.Fatalf("set exit %d %s", code, h.err.String())
	}
	if h.config(t).NewDays() != 3 {
		t.Fatal("saved")
	}
	if code := h.run("config", "new-days", "-1"); code == 0 {
		t.Fatal("a negative value is refused")
	}
	if code := h.run("config", "nope", "1"); code != 2 {
		t.Fatalf("an unknown key exit %d", code)
	}
}

func TestListRebuildsAStaleRegistry(t *testing.T) {
	h := setup(t)
	dir := work(t, "webapp", true)
	h.run("init", dir)
	file := filepath.Join(h.home, "state", "registry.json")
	os.WriteFile(file, []byte(`{"schema":"atlas-obsidian.registry.v0","entries":[]}`), 0o644)
	if code := h.run("list"); code != 0 || !strings.Contains(h.out.String(), "webapp") {
		t.Fatalf("list exit %d:\n%s%s", code, h.out.String(), h.err.String())
	}
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), registry.StateSchema) {
		t.Fatalf("the registry was rewritten:\n%s", data)
	}
}

func TestVaultArgResolvesNamesPathsAndTheCurrentFolder(t *testing.T) {
	h := setup(t)
	dir := work(t, "webapp", true)
	h.run("init", dir)
	e := &env{home: home.Home{Root: h.home}, console: console.NewWith(true, strings.NewReader(""), &h.out, false)}
	for _, arg := range []string{"webapp", dir, filepath.Join(dir, "README.md")} {
		p, err := e.vaultArg(arg)
		if err != nil || p.Root != dir {
			t.Errorf("vaultArg(%q): %+v %v", arg, p, err)
		}
	}
	t.Chdir(dir)
	if p, err := e.vaultArg(""); err != nil || p.Root != dir {
		t.Errorf("vaultArg(\"\") inside the work: %+v %v", p, err)
	}
	if _, err := e.vaultArg("nope"); err == nil {
		t.Error("an unknown name")
	}
	t.Chdir(t.TempDir())
	if _, err := e.vaultArg(""); err == nil {
		t.Error("outside every project")
	}
}

func TestShellArg(t *testing.T) {
	cases := map[string]string{
		"webapp":     "webapp",
		"Web App":    "'Web App'",
		"~/code":     "'~/code'",
		"it's":       `'it'\''s'`,
		"":           "''",
		"a/b_c-d.e":  "a/b_c-d.e",
		"semi;colon": "'semi;colon'",
	}
	for in, want := range cases {
		if got := shellArg(in); got != want {
			t.Errorf("shellArg(%q) = %q, want %q", in, got, want)
		}
	}
}
