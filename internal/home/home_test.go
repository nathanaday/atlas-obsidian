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
	cfg.AddProject("~/Code/webapp")
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	back, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	userHome, _ := os.UserHomeDir()
	if len(back.Projects) != 1 || back.Projects[0] != filepath.Join(userHome, "Code", "webapp") || !back.HasProject("~/Code/webapp") {
		t.Fatalf("got %+v", back)
	}
	if back.Plugin.ID == "" {
		t.Fatalf("got %+v", back)
	}
}

func TestConfigFields(t *testing.T) {
	h := Home{Root: t.TempDir()}
	if err := h.Save(h.Default()); err != nil {
		t.Fatal(err)
	}
	loaded, err := h.Load()
	if err != nil || loaded.Schema != ConfigSchema {
		t.Fatalf("the config loads: %+v %v", loaded, err)
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
	if len(again.Projects) != 1 || again.Projects[0] != Expand("~/Code/webapp") {
		t.Fatalf("the config's lists %+v", again)
	}
	if !again.HasProject("~/Code/webapp") || again.HasProject("~/Code/other") {
		t.Fatal("HasProject")
	}
	if !again.RemoveProject("~/Code/webapp") || again.RemoveProject("~/Code/webapp") || len(again.Projects) != 0 {
		t.Fatalf("project removal %+v", again)
	}
	data, _ := os.ReadFile(h.ConfigPath())
	if !strings.Contains(string(data), `"schema": "`+ConfigSchema+`"`) {
		t.Fatalf("saved schema:\n%s", data)
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
