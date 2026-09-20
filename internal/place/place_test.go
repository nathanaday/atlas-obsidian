package place

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/manage"
	"github.com/nathanaday/atlas-obsidian/internal/project"
)

var now = time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

// atlas makes a home whose config lists one project, and returns the home, the config, and
// the project's work folder.
func atlas(t *testing.T) (home.Home, *home.Config, string) {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	h := home.Home{Root: filepath.Join(root, "home")}
	cfg := h.Default()
	work := filepath.Join(root, "Code", "webapp")
	if err := os.MkdirAll(filepath.Join(work, "src", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := manage.Init(h, cfg, work, project.Options{Name: "webapp"}, nil, false); err != nil {
		t.Fatal(err)
	}
	return h, cfg, work
}

func TestResolveFindsTheProjectFromAnywhereInsideTheWork(t *testing.T) {
	h, _, work := atlas(t)
	for _, from := range []string{work, filepath.Join(work, "src", "deep")} {
		pl, err := Resolve(h, "", "", from, false)
		if err != nil {
			t.Fatal(err)
		}
		if pl.Project == nil || pl.Project.Root != work || pl.Entry == nil || pl.Entry.Name != "webapp" || pl.Index == nil || pl.Heal != manage.HealNone {
			t.Fatalf("from %s: %+v", from, pl)
		}
	}
	// Inside the project's own folder too.
	if pl, err := Resolve(h, "", "", filepath.Join(work, project.Dir, "webapp", "wiki"), false); err != nil || pl.Project == nil {
		t.Fatalf("inside the project's folder: %+v %v", pl, err)
	}
	// An explicit path wins over the environment, and both over the walk up.
	other := filepath.Join(t.TempDir(), "other")
	os.MkdirAll(other, 0o755)
	if _, err := project.Init(other, project.Options{Name: "other"}, now); err != nil {
		t.Fatal(err)
	}
	if pl, err := Resolve(h, other, work, work, false); err != nil || pl.Project.Root != other {
		t.Fatalf("the explicit path wins: %+v %v", pl, err)
	}
	if pl, err := Resolve(h, "", other, work, false); err != nil || pl.Project.Root != other {
		t.Fatalf("the environment next: %+v %v", pl, err)
	}
	if _, err := Resolve(h, "", "", t.TempDir(), false); !errors.Is(err, ErrNoPlace) {
		t.Fatalf("nothing above: %v", err)
	}
	if _, err := Resolve(h, "", "", "", false); !errors.Is(err, ErrNoPlace) {
		t.Fatalf("no start: %v", err)
	}
	if Cwd() == "" {
		t.Fatal("Cwd")
	}
}

func TestResolveNamesAKnowledgeBaseOfAnEarlierVersion(t *testing.T) {
	h, _, _ := atlas(t)
	kb := filepath.Join(t.TempDir(), "notes")
	os.MkdirAll(filepath.Join(kb, "wiki"), 0o755)
	os.WriteFile(filepath.Join(kb, project.KnowledgeMarker), []byte(`{"schema":"claude-atlas.vault.v3","id":"k1","kind":"knowledge","name":"notes"}`), 0o644)
	_, err := Resolve(h, "", "", filepath.Join(kb, "wiki"), false)
	if !errors.Is(err, ErrNoPlace) || !strings.Contains(err.Error(), "upgrade") {
		t.Fatalf("a 3.x knowledge base: %v", err)
	}
}

func TestResolveWithoutAnAtlasConfig(t *testing.T) {
	_, _, work := atlas(t)
	nowhere := home.Home{Root: filepath.Join(t.TempDir(), "no-atlas")}
	pl, err := Resolve(nowhere, "", "", work, true)
	if err != nil {
		t.Fatal(err)
	}
	if pl.Project == nil || pl.Entry != nil || pl.Index != nil || !strings.Contains(pl.ConfigError, "no atlas config") {
		t.Fatalf("no atlas: %+v", pl)
	}
}

func TestResolveHealsTheConfig(t *testing.T) {
	h, cfg, work := atlas(t)
	// Not registered: a clone.
	clone := filepath.Join(t.TempDir(), "clone")
	os.MkdirAll(clone, 0o755)
	if _, err := project.Init(clone, project.Options{Name: "clone"}, now); err != nil {
		t.Fatal(err)
	}
	if pl, err := Resolve(h, "", "", clone, false); err != nil || pl.Heal != manage.HealNone || pl.Entry != nil {
		t.Fatalf("without register nothing changes: %+v %v", pl, err)
	}
	pl, err := Resolve(h, "", "", clone, true)
	if err != nil || pl.Heal != manage.HealAdded || pl.Entry == nil || pl.Entry.Path != clone {
		t.Fatalf("added: %+v %v", pl, err)
	}
	saved, _ := h.Load()
	if !saved.HasProject(clone) {
		t.Fatalf("the heal is saved: %v", saved.Projects)
	}
	// Moved: the same id at another readable path.
	copied := filepath.Join(t.TempDir(), "copied")
	os.MkdirAll(filepath.Join(copied, project.Dir, "webapp"), 0o755)
	orig, _ := project.Open(work)
	data, _ := os.ReadFile(orig.Path(project.Marker))
	os.WriteFile(filepath.Join(copied, project.Dir, "webapp", project.Marker), data, 0o644)
	pl, err = Resolve(h, "", "", copied, true)
	if err != nil || pl.Heal != manage.HealMoved || pl.Entry == nil || pl.Entry.Path != copied {
		t.Fatalf("moved by id: %+v %v", pl, err)
	}
	saved, _ = h.Load()
	if !saved.HasProject(copied) || saved.HasProject(work) {
		t.Fatalf("the old entry is gone: %v", saved.Projects)
	}
	// Moved: the only listed path that is gone.
	os.RemoveAll(copied)
	moved := filepath.Join(t.TempDir(), "moved")
	os.MkdirAll(filepath.Join(moved, project.Dir, "webapp"), 0o755)
	os.WriteFile(filepath.Join(moved, project.Dir, "webapp", project.Marker), data, 0o644)
	pl, err = Resolve(h, "", "", moved, true)
	if err != nil || pl.Heal != manage.HealMoved || pl.Entry == nil || pl.Entry.Path != moved {
		t.Fatalf("moved by absence: %+v %v", pl, err)
	}
	saved, _ = h.Load()
	if !saved.HasProject(moved) || saved.HasProject(copied) || len(saved.Projects) != 2 {
		t.Fatalf("projects %v (cfg %v)", saved.Projects, cfg.Projects)
	}
	if pl, err := Resolve(h, "", "", moved, true); err != nil || pl.Heal != manage.HealNone {
		t.Fatalf("a second session heals nothing: %+v %v", pl, err)
	}
}

func TestResolveRefusesAProjectOfAnEarlierVersion(t *testing.T) {
	h, _, _ := atlas(t)
	old := filepath.Join(t.TempDir(), "old")
	os.MkdirAll(filepath.Join(old, project.Dir, "old"), 0o755)
	os.WriteFile(filepath.Join(old, project.Dir, "old", project.Marker),
		[]byte(`{"schema":"claude-atlas.project.v3","id":"p1","name":"old","knowledge":{"id":"k1","name":"notes"}}`), 0o644)
	if _, err := Resolve(h, "", "", old, true); !errors.Is(err, project.ErrSplit) {
		t.Fatalf("a 3.x project: %v", err)
	}
}
