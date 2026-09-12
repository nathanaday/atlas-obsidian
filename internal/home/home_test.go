package home

import (
	"os"
	"path/filepath"
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
	if h := Resolve(""); filepath.Base(h.Root) != ".claude-atlas" {
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
	cfg := h.Default("~/Docs/Vaults")
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	back, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	userHome, _ := os.UserHomeDir()
	if back.VaultsDir != filepath.Join(userHome, "Docs", "Vaults") || back.AtlasVault != filepath.Join(h.Root, "atlas") {
		t.Fatalf("got %+v", back)
	}
	if back.ClaudeObsidian.Plugin == "" || back.TreeRoot() != filepath.Join(h.Root, "atlas", "tree") {
		t.Fatalf("got %+v", back)
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
