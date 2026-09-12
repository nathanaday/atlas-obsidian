package product

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/claude-atlas/internal/home"
)

func fakeProduct(t *testing.T, version string) string {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "scripts"), 0o755)
	os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755)
	os.WriteFile(filepath.Join(root, "scripts", "claude-obsidian.py"), []byte("print('{}')\n"), 0o644)
	os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(`{"version":"`+version+`"}`), 0o644)
	return root
}

func TestOperationID(t *testing.T) {
	if got := OperationID("init", "2026-09-11T20:14:03Z"); got != "init-20260911T201403Z" {
		t.Fatalf("got %s", got)
	}
}

func TestLocateFromConfigPath(t *testing.T) {
	root := fakeProduct(t, "9.9.9")
	p, err := Locate(home.ProductConfig{Path: root})
	if err != nil || p.Root != root || p.Version != "9.9.9" || p.Source != "config" || p.Tested() {
		t.Fatalf("got %+v %v", p, err)
	}
	if _, err := Locate(home.ProductConfig{Path: t.TempDir()}); err == nil || !strings.Contains(err.Error(), "does not contain") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestLocateFromInstalledPlugin(t *testing.T) {
	root := fakeProduct(t, TestedVersion)
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	os.MkdirAll(filepath.Join(dir, "plugins"), 0o755)
	os.WriteFile(filepath.Join(dir, "plugins", "installed_plugins.json"),
		[]byte(`{"plugins":{"claude-obsidian@m":[{"installPath":"`+root+`","version":"`+TestedVersion+`"}]}}`), 0o644)
	p, err := Locate(home.ProductConfig{Plugin: "claude-obsidian@m"})
	if err != nil || p.Root != root || p.Source != "plugin" || !p.Tested() {
		t.Fatalf("got %+v %v", p, err)
	}
	if _, err := Locate(home.ProductConfig{Plugin: "missing@m"}); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("expected not installed, got %v", err)
	}
}
