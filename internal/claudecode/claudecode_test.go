package claudecode

import (
	"os"
	"path/filepath"
	"testing"
)

func fakeConfig(t *testing.T, installed, marketplaces string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	os.MkdirAll(filepath.Join(dir, "plugins"), 0o755)
	if installed != "" {
		os.WriteFile(filepath.Join(dir, "plugins", "installed_plugins.json"), []byte(installed), 0o644)
	}
	if marketplaces != "" {
		os.WriteFile(filepath.Join(dir, "plugins", "known_marketplaces.json"), []byte(marketplaces), 0o644)
	}
}

func TestInstalledPluginReadsListAndObjectForms(t *testing.T) {
	fakeConfig(t, `{"plugins":{"a@m":[{"scope":"user","installPath":"/p/a","version":"1.0.0"}],"b@m":{"installPath":"/p/b","version":"2"}}}`, "")
	a, err := InstalledPlugin("a@m")
	if err != nil || a == nil || a.InstallPath != "/p/a" || a.Version != "1.0.0" {
		t.Fatalf("got %+v %v", a, err)
	}
	b, err := InstalledPlugin("b@m")
	if err != nil || b == nil || b.InstallPath != "/p/b" {
		t.Fatalf("got %+v %v", b, err)
	}
	if c, err := InstalledPlugin("c@m"); err != nil || c != nil {
		t.Fatalf("expected nil for unknown, got %+v %v", c, err)
	}
}

func TestInstalledPluginWithoutRegistry(t *testing.T) {
	fakeConfig(t, "", "")
	if inst, err := InstalledPlugin("a@m"); err != nil || inst != nil {
		t.Fatalf("got %+v %v", inst, err)
	}
}

func TestMarketplaceKnownAndName(t *testing.T) {
	fakeConfig(t, "", `{"agricidaniel-claude-obsidian":{"source":{}}}`)
	if !MarketplaceKnown("agricidaniel-claude-obsidian") || MarketplaceKnown("other") {
		t.Fatal("marketplace lookup wrong")
	}
	if MarketplaceName("claude-obsidian@agricidaniel-claude-obsidian") != "agricidaniel-claude-obsidian" || MarketplaceName("bare") != "" {
		t.Fatal("marketplace name wrong")
	}
}
