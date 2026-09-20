package home

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	t.Setenv(EnvHome, "/env/home")
	if h := Resolve("/flag"); h.Root != "/flag" {
		t.Fatalf("flag should win, got %s", h.Root)
	}
	if h := Resolve(""); h.Root != "/env/home" {
		t.Fatalf("env should win, got %s", h.Root)
	}
	t.Setenv(EnvHome, "")
	if h := Resolve(""); filepath.Base(h.Root) != ".atlas-obsidian" {
		t.Fatalf("default wrong: %s", h.Root)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	h := Home{Root: filepath.Join(t.TempDir(), "home")}
	if h.Exists() {
		t.Fatal("should not exist yet")
	}
	if _, err := h.Load(); err == nil {
		t.Fatal("load before setup should fail")
	}
	cfg := h.Default()
	cfg.AddKnowledge("~/Docs/notes")
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	back, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	userHome, _ := os.UserHomeDir()
	if len(back.Knowledge) != 1 || back.Knowledge[0] != filepath.Join(userHome, "Docs", "notes") || !back.HasKnowledge("~/Docs/notes") {
		t.Fatalf("got %+v", back)
	}
	if back.Plugin.ID == "" {
		t.Fatalf("got %+v", back)
	}
}

func TestConfigFieldsAndOlderConfigsUpgrade(t *testing.T) {
	h := Home{Root: t.TempDir()}
	cfg := h.Default()
	cfg.Schema = ConfigSchemaV1
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := h.Load()
	if err != nil || loaded.Schema != ConfigSchema {
		t.Fatalf("a v1 config loads as the current one: %+v %v", loaded, err)
	}
	if !loaded.AddKnowledge("~/Elsewhere/side") || loaded.AddKnowledge("~/Elsewhere/side") {
		t.Fatal("AddKnowledge dedupes")
	}
	if !loaded.AddProject("~/Code/webapp") || loaded.AddProject("~/Code/webapp") {
		t.Fatal("AddProject dedupes")
	}
	if err := h.Save(loaded); err != nil {
		t.Fatal(err)
	}
	again, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Knowledge) != 1 || again.Knowledge[0] != Expand("~/Elsewhere/side") || len(again.Projects) != 1 || again.Projects[0] != Expand("~/Code/webapp") {
		t.Fatalf("the config's lists %+v", again)
	}
	if !again.HasProject("~/Code/webapp") || again.HasProject("~/Code/other") {
		t.Fatal("HasProject")
	}
	if !again.RemoveKnowledge(again.Knowledge[0]) || again.RemoveKnowledge("~/nope") || len(again.Knowledge) != 0 {
		t.Fatalf("knowledge removal %+v", again)
	}
	if !again.RemoveProject("~/Code/webapp") || again.RemoveProject("~/Code/webapp") || len(again.Projects) != 0 {
		t.Fatalf("project removal %+v", again)
	}
	data, _ := os.ReadFile(h.ConfigPath())
	if !strings.Contains(string(data), `"schema": "`+ConfigSchema+`"`) {
		t.Fatalf("saved schema:\n%s", data)
	}
}

func TestV2ConfigVaultsBecomeKnowledge(t *testing.T) {
	h := Home{Root: t.TempDir()}
	if err := os.MkdirAll(h.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	v2 := `{
  "schema": "claude-atlas.config.v2",
  "vaults_dir": "~/Vaults",
  "vaults": ["~/Elsewhere/kb"],
  "repos": {"id/paper": "~/Code/paper"},
  "default_repo_changes": "pr",
  "plugin": {"id": "atlas-obsidian@x", "source": "x"},
  "claude_code": {"command": "claude", "session_context": true}
}
`
	if err := os.WriteFile(h.ConfigPath(), []byte(v2), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Schema != ConfigSchema || len(cfg.Knowledge) != 1 || cfg.Knowledge[0] != Expand("~/Elsewhere/kb") || len(cfg.Projects) != 0 {
		t.Fatalf("v2 vaults become knowledge bases: %+v", cfg)
	}
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(h.ConfigPath())
	for _, gone := range []string{"repos", "default_repo_changes", `"vaults"`, "vaults_dir"} {
		if strings.Contains(string(data), gone) {
			t.Fatalf("%s should not survive a save:\n%s", gone, data)
		}
	}
}

func TestDisplayAndExpand(t *testing.T) {
	userHome, _ := os.UserHomeDir()
	if Display(filepath.Join(userHome, "x")) != "~/x" || Display("/opt/x") != "/opt/x" {
		t.Fatal("display wrong")
	}
	if Expand("~/x") != filepath.Join(userHome, "x") || Expand("/abs") != "/abs" {
		t.Fatal("expand wrong")
	}
}

// A config written before the rename loads, and Save raises its schema.
func TestLoadAcceptsTheSchemaWrittenBeforeTheRename(t *testing.T) {
	h := Home{Root: t.TempDir()}
	raw := `{"schema":"claude-atlas.config.v4","projects":["/work/webapp"]}`
	if err := os.WriteFile(h.ConfigPath(), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := h.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Schema != ConfigSchema {
		t.Fatalf("schema %q, want %q", cfg.Schema, ConfigSchema)
	}
	if !cfg.HasProject("/work/webapp") {
		t.Fatalf("the projects were dropped: %+v", cfg.Projects)
	}
}

// adopt moves an atlas left at the old name; it never overwrites one at the new name.
func TestAdoptMovesTheAtlasLeftAtTheOldName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	old := filepath.Join(dir, ".claude-atlas")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "config.json"), []byte(`{"schema":"claude-atlas.config.v4"}`), 0o644)

	root := filepath.Join(dir, ".atlas-obsidian")
	if got := adopt(root); got != root {
		t.Fatalf("adopt returned %q, want %q", got, root)
	}
	if _, err := os.Stat(filepath.Join(root, "config.json")); err != nil {
		t.Fatalf("the config did not move: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("the old home is still there: %v", err)
	}
}

func TestAdoptKeepsAnAtlasAlreadyAtTheNewName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	old := filepath.Join(dir, ".claude-atlas")
	os.MkdirAll(old, 0o755)
	os.WriteFile(filepath.Join(old, "config.json"), []byte(`{"schema":"claude-atlas.config.v4"}`), 0o644)
	root := filepath.Join(dir, ".atlas-obsidian")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"schema":"atlas-obsidian.config.v4"}`), 0o644)

	if got := adopt(root); got != root {
		t.Fatalf("adopt returned %q, want %q", got, root)
	}
	data, _ := os.ReadFile(filepath.Join(root, "config.json"))
	if !strings.Contains(string(data), "atlas-obsidian.config.v4") {
		t.Fatalf("the new home was overwritten: %s", data)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("the old home was moved anyway: %v", err)
	}
}
