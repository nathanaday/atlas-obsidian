package manage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/console"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/project"
)

var now = time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

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

func work(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestInitRegistersTheProject(t *testing.T) {
	h, cfg := atlas(t)
	dir := work(t, "webapp")
	res, err := Init(h, cfg, dir, project.Options{Description: "The app."}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Project.Name() != "webapp" || res.Commit == "" {
		t.Fatalf("init %+v", res)
	}
	saved, err := h.Load()
	if err != nil || !saved.HasProject(dir) {
		t.Fatalf("the config lists the project: %v %v", saved.Projects, err)
	}
	if _, err := Init(h, cfg, dir, project.Options{}, nil, false); err == nil {
		t.Fatal("twice")
	}
}

func TestInitPreviewsAndCanBeDeclined(t *testing.T) {
	h, cfg := atlas(t)
	dir := work(t, "webapp")
	var out strings.Builder
	c := console.NewWith(false, strings.NewReader("n\n"), &out, true)
	if _, err := Init(h, cfg, dir, project.Options{}, c, true); err != ErrCancelled {
		t.Fatalf("declined: %v", err)
	}
	text := out.String()
	for _, want := range []string{"atlas/webapp/", "wiki/index.md", "project.json", "git repository"} {
		if !strings.Contains(text, want) {
			t.Errorf("the preview lacks %q:\n%s", want, text)
		}
	}
	if project.IsProject(dir) {
		t.Fatal("a declined init writes nothing")
	}
}

func TestEditProjectRenamesTheFolderAndKeepsTheConfigEntry(t *testing.T) {
	h, cfg := atlas(t)
	dir := work(t, "webapp")
	if _, err := Init(h, cfg, dir, project.Options{}, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := EditProject(cfg, dir, Edit{Name: "Web App", Mode: project.LYT}, now); err != nil {
		t.Fatal(err)
	}
	p, err := project.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "Web App" || p.Folder != "Web App" || p.Config.Mode != project.LYT {
		t.Fatalf("project %+v", p.Config)
	}
	// The work folder, which the config holds, never moves.
	saved, _ := h.Load()
	if !saved.HasProject(dir) {
		t.Fatalf("the config still points at the work: %v", saved.Projects)
	}
	desc := "Notes."
	if err := EditProject(cfg, dir, Edit{Description: &desc}, now); err != nil {
		t.Fatal(err)
	}
	if p, _ := project.Open(dir); p.Config.Description != desc {
		t.Fatalf("description %q", p.Config.Description)
	}
	if err := EditProject(cfg, dir, Edit{}, now); err != nil {
		t.Fatalf("nothing to change: %v", err)
	}
}

func TestForgetDropsAProject(t *testing.T) {
	h, cfg := atlas(t)
	dir := work(t, "webapp")
	if _, err := Init(h, cfg, dir, project.Options{}, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := Forget(h, cfg, dir); err != nil {
		t.Fatal(err)
	}
	saved, _ := h.Load()
	if len(saved.Projects) != 0 {
		t.Fatalf("config %+v", saved)
	}
	if !project.IsProject(dir) {
		t.Fatal("the folder stays")
	}
	if err := Forget(h, cfg, dir); err == nil {
		t.Fatal("forgetting what the atlas does not hold")
	}
}

func TestRegisterProjectHeals(t *testing.T) {
	h, cfg := atlas(t)
	dir := work(t, "webapp")
	if _, err := Init(h, cfg, dir, project.Options{}, nil, false); err != nil {
		t.Fatal(err)
	}
	p, _ := project.Open(dir)
	if heal, err := RegisterProject(h, cfg, p); err != nil || heal != HealNone {
		t.Fatalf("already listed: %v %v", heal, err)
	}
	// A clone the config does not know is added.
	clone := work(t, "clone")
	if _, err := Init(h, cfg, clone, project.Options{}, nil, false); err != nil {
		t.Fatal(err)
	}
	cfg.RemoveProject(clone)
	h.Save(cfg)
	q, _ := project.Open(clone)
	if heal, err := RegisterProject(h, cfg, q); err != nil || heal != HealAdded {
		t.Fatalf("added: %v %v", heal, err)
	}
	// The same id at another path moves the entry.
	moved := work(t, "moved")
	os.MkdirAll(filepath.Join(moved, project.Dir, "webapp"), 0o755)
	data, _ := os.ReadFile(p.Path(project.Marker))
	os.WriteFile(filepath.Join(moved, project.Dir, "webapp", project.Marker), data, 0o644)
	r, err := project.Open(moved)
	if err != nil {
		t.Fatal(err)
	}
	if heal, err := RegisterProject(h, cfg, r); err != nil || heal != HealMoved {
		t.Fatalf("moved: %v %v", heal, err)
	}
	saved, _ := h.Load()
	if saved.HasProject(dir) || !saved.HasProject(moved) {
		t.Fatalf("the old entry is gone: %v", saved.Projects)
	}
}

func TestResolvePath(t *testing.T) {
	if _, err := ResolvePath("  "); err == nil {
		t.Fatal("an empty path")
	}
	if got, err := ResolvePath("~"); err != nil || !filepath.IsAbs(got) {
		t.Fatalf("~: %q %v", got, err)
	}
}
