package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

func TestAtlasReadsWithoutWritingAndRefreshWrites(t *testing.T) {
	a := newAtlas(t)
	c := a.session(t)
	var out AtlasOut
	if msg := c.call("atlas", nil, &out); msg != "" {
		t.Fatal(msg)
	}
	if len(out.Projects) != 1 || out.Projects[0].Name != "webapp" || out.Projects[0].State == nil {
		t.Fatalf("atlas %+v", out)
	}
	if len(out.Problems) != 0 || out.Settings.NewDays != home.DefaultNewDays {
		t.Fatalf("problems %+v settings %+v", out.Problems, out.Settings)
	}
	if _, err := os.Stat(registry.File(a.h.StateDir())); !os.IsNotExist(err) {
		t.Fatalf("a read writes no registry: %v", err)
	}
	if msg := c.call("atlas", map[string]any{"refresh": true}, &out); msg != "" || len(out.Projects) != 1 {
		t.Fatalf("refresh %q %+v", msg, out)
	}
	if _, err := os.Stat(registry.File(a.h.StateDir())); err != nil {
		t.Fatalf("refresh writes the registry: %v", err)
	}
	// A folder the atlas cannot read is a problem, not an entry.
	broken := filepath.Join(t.TempDir(), "broken")
	os.MkdirAll(filepath.Join(broken, project.Dir, "broken"), 0o755)
	os.WriteFile(filepath.Join(broken, project.Dir, "broken", project.Marker), []byte("{nope"), 0o644)
	a.cfg.AddProject(broken)
	a.h.Save(a.cfg)
	if msg := c.call("atlas", nil, &out); msg != "" || len(out.Projects) != 1 || len(out.Problems) != 1 {
		t.Fatalf("a problem %q %+v", msg, out)
	}
}

func TestProjectInitFromAPlainFolder(t *testing.T) {
	a := newAtlas(t)
	work := filepath.Join(t.TempDir(), "thesis")
	os.MkdirAll(work, 0o755)
	// The session sits in the folder, with no place yet; init makes it a project.
	c := connectIn(t, a.h, work)
	if msg := c.call("status", nil, nil); !strings.Contains(msg, "not in a atlas-obsidian") {
		t.Fatalf("no place before init: %q", msg)
	}
	var out ProjectToolOut
	if msg := c.call("project", map[string]any{"action": "init", "description": "My thesis.", "mode": "lyt"}, &out); msg != "" {
		t.Fatal(msg)
	}
	if out.Project == nil || out.Project.Name != "thesis" || out.Project.Path != work || out.Project.Description != "My thesis." || out.Project.Mode != project.LYT {
		t.Fatalf("init: %+v", out)
	}
	if len(out.Written) == 0 || out.Git != string(project.GitCreated) || out.Commit == "" {
		t.Fatalf("init wrote and committed: %+v", out)
	}
	if !project.IsProject(work) {
		t.Fatal("atlas/<name>/project.json is there")
	}
	cfg, _ := a.h.Load()
	if !cfg.HasProject(work) {
		t.Fatalf("the config lists the project: %+v", cfg.Projects)
	}
	var st Status
	if msg := c.call("status", nil, &st); msg != "" || st.Name != "thesis" || st.Pages != 4 {
		t.Fatalf("the same session is now a project session: %q %+v", msg, st)
	}
	if msg := c.call("project", map[string]any{"action": "init"}, nil); !strings.Contains(msg, "already") {
		t.Fatalf("init twice: %q", msg)
	}
	os.MkdirAll(filepath.Join(work, "chapter"), 0o755)
	if msg := c.call("project", map[string]any{"action": "init", "work": filepath.Join(work, "chapter")}, nil); !strings.Contains(msg, "inside the project") {
		t.Fatalf("a project inside a project: %q", msg)
	}
	if msg := c.call("project", map[string]any{"action": "init", "work": t.TempDir(), "mode": "para"}, nil); !strings.Contains(msg, "mode must be") {
		t.Fatalf("an unknown mode: %q", msg)
	}
}

func TestProjectEditAndForget(t *testing.T) {
	a := newAtlas(t)
	c := a.session(t)
	var out ProjectToolOut
	desc := "The web app."
	if msg := c.call("project", map[string]any{"action": "edit", "name": "Web App", "description": desc}, &out); msg != "" || out.Project.Name != "Web App" || out.Project.Description != desc {
		t.Fatalf("edit: %q %+v", msg, out.Project)
	}
	// The folder follows the name; the work folder, which the config holds, does not move.
	p, err := project.Open(a.work)
	if err != nil || p.Folder != "Web App" {
		t.Fatalf("the folder follows: %+v %v", p, err)
	}
	if msg := c.call("project", map[string]any{"action": "edit"}, nil); !strings.Contains(msg, "needs name, description, mode") {
		t.Fatalf("empty edit: %q", msg)
	}
	if msg := c.call("project", map[string]any{"action": "grow"}, nil); !strings.Contains(msg, "action must be") {
		t.Fatalf("unknown action: %q", msg)
	}
	// From another folder, the project is named with work.
	elsewhere := connectIn(t, a.h, t.TempDir())
	if msg := elsewhere.call("project", map[string]any{"action": "edit", "name": "x"}, nil); !strings.Contains(msg, "name the project") {
		t.Fatalf("no project in this session: %q", msg)
	}
	out = ProjectToolOut{}
	if msg := elsewhere.call("project", map[string]any{"action": "forget", "work": "Web App"}, &out); msg != "" || out.Forgotten != a.work {
		t.Fatalf("forget by name: %q %+v", msg, out)
	}
	if !project.IsProject(a.work) {
		t.Fatal("the folder and its atlas/<name>/ stay")
	}
	cfg, _ := a.h.Load()
	if cfg.HasProject(a.work) {
		t.Fatal("the config no longer lists the project")
	}
}

func TestSettings(t *testing.T) {
	a := newAtlas(t)
	c := a.session(t)
	var s Settings
	if msg := c.call("settings", nil, &s); msg != "" || s.NewDays != home.DefaultNewDays {
		t.Fatalf("read: %q %+v", msg, s)
	}
	if msg := c.call("settings", map[string]any{"new_days": 3}, &s); msg != "" || s.NewDays != 3 {
		t.Fatalf("set: %q %+v", msg, s)
	}
	cfg, _ := a.h.Load()
	if cfg.NewDays() != 3 {
		t.Fatalf("saved: %d", cfg.NewDays())
	}
	if msg := c.call("settings", map[string]any{"new_days": -1}, nil); !strings.Contains(msg, "0 or more") {
		t.Fatalf("negative: %q", msg)
	}
	if _, err := os.Stat(registry.File(a.h.StateDir())); err != nil {
		t.Fatal("a write rewrites the registry")
	}
}

func TestProjectMembersAndSync(t *testing.T) {
	a := newAtlas(t)
	c := a.session(t)
	svc := filepath.Join(filepath.Dir(a.work), "svc")
	os.MkdirAll(svc, 0o755)
	var made ProjectToolOut
	if msg := c.call("project", map[string]any{"action": "init", "work": svc, "name": "svc"}, &made); msg != "" {
		t.Fatal(msg)
	}
	var out ProjectToolOut
	if msg := c.call("project", map[string]any{"action": "edit", "add_members": []string{"webapp"}}, nil); !strings.Contains(msg, "own member") {
		t.Fatalf("self: %q", msg)
	}
	if msg := c.call("project", map[string]any{"action": "edit", "add_members": []string{"nope"}}, nil); !strings.Contains(msg, "no project named") {
		t.Fatalf("unknown: %q", msg)
	}
	if msg := c.call("project", map[string]any{"action": "edit", "add_members": []string{"svc"}}, &out); msg != "" || len(out.Project.Members) != 1 || out.Project.Members[0] != made.Project.ID {
		t.Fatalf("add: %q %+v", msg, out.Project)
	}
	// svc listing webapp would close a cycle.
	if msg := c.call("project", map[string]any{"action": "edit", "work": "svc", "add_members": []string{"webapp"}}, nil); !strings.Contains(msg, "cycle") {
		t.Fatalf("cycle: %q", msg)
	}
	var st Status
	if msg := c.call("status", nil, &st); msg != "" || len(st.Members) != 1 || st.Members[0].Mirrored {
		t.Fatalf("status before sync: %q %+v", msg, st.Members)
	}
	if !strings.Contains(strings.Join(st.Warnings, "|"), "not mirrored yet") {
		t.Fatalf("warnings %v", st.Warnings)
	}
	if msg := c.call("project", map[string]any{"action": "sync"}, &out); msg != "" || out.Sync == nil || out.Sync.Commit == "" || len(out.Sync.Members) != 1 || out.Sync.Members[0].Pages == 0 {
		t.Fatalf("sync: %q %+v", msg, out.Sync)
	}
	if _, err := os.Stat(a.p.Path("wiki/projects/svc/svc.md")); err != nil {
		t.Fatal("the mirror is not there")
	}
	if msg := c.call("status", nil, &st); msg != "" || !st.Members[0].Mirrored {
		t.Fatalf("status after sync: %q %+v", msg, st.Members)
	}
	var again ProjectToolOut
	if msg := c.call("project", map[string]any{"action": "sync"}, &again); msg != "" || again.Sync.Commit != "" {
		t.Fatalf("second sync: %q %+v", msg, again.Sync)
	}
	// The mirror is the sync's; a plan may not write it.
	if msg := c.call("plan", map[string]any{"kind": "save", "summary": "x", "writes": []map[string]any{{"path": "wiki/projects/svc/svc.md", "mode": "replace", "content": "x"}}}, nil); !strings.Contains(msg, "rewritten by sync") {
		t.Fatalf("plan into the mirror: %q", msg)
	}
	var dropped ProjectToolOut
	if msg := c.call("project", map[string]any{"action": "edit", "remove_members": []string{"svc"}}, &dropped); msg != "" || len(dropped.Project.Members) != 0 {
		t.Fatalf("remove: %q %+v", msg, dropped.Project)
	}
	var removed ProjectToolOut
	if msg := c.call("project", map[string]any{"action": "sync"}, &removed); msg != "" || removed.Sync.Removes == 0 {
		t.Fatalf("sync after remove: %q %+v", msg, removed.Sync)
	}
	if _, err := os.Stat(a.p.Path("wiki/projects/svc")); err == nil {
		t.Fatal("the mirror stayed")
	}
}
