package vaults

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/tree"
)

func fakeVault(t *testing.T, dir string) string {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, ".claude-obsidian.json"), []byte("{}"), 0o644)
	return dir
}

func setup(t *testing.T) (*home.Config, *tree.Project) {
	root := t.TempDir()
	cfg := &home.Config{Schema: home.ConfigSchema, VaultsDir: filepath.Join(root, "Vaults"), AtlasVault: filepath.Join(root, "Atlas")}
	os.MkdirAll(cfg.TreeRoot(), 0o755)
	vault := fakeVault(t, filepath.Join(cfg.VaultsDir, "a"))
	p, err := Register(cfg, vault, RegisterOptions{Name: "A", Purpose: "why", Category: "work"})
	if err != nil {
		t.Fatal(err)
	}
	return cfg, p
}

func TestUpdateFieldsAndCategory(t *testing.T) {
	cfg, p := setup(t)
	top := ""
	if err := Update(cfg, p, Edit{Name: "A2", ClearPurpose: true, Priority: "high", State: "paused", Category: &top}); err != nil {
		t.Fatal(err)
	}
	projects, problems, _ := tree.Walk(cfg.TreeRoot())
	if len(problems) != 0 || len(projects) != 1 {
		t.Fatalf("walk: %v %v", projects, problems)
	}
	got := projects[0]
	if got.Rel != "a" || got.Name != "A2" || got.Purpose != "" || got.Priority != "high" || got.State != "paused" {
		t.Fatalf("got %+v", got.Frontmatter)
	}
}

func TestUpdateRepointsToAnExistingVault(t *testing.T) {
	cfg, p := setup(t)
	other := fakeVault(t, filepath.Join(cfg.VaultsDir, "elsewhere"))
	if err := Update(cfg, p, Edit{Vault: other}); err != nil {
		t.Fatal(err)
	}
	projects, _, _ := tree.Walk(cfg.TreeRoot())
	if projects[0].VaultPath() != other {
		t.Fatalf("vault %s", projects[0].VaultPath())
	}
	if err := Update(cfg, projects[0], Edit{Vault: t.TempDir()}); err == nil {
		t.Fatal("repointing at a non-vault should fail")
	}
}

func TestUpdateMovesTheVaultDirectoryWhenAsked(t *testing.T) {
	cfg, p := setup(t)
	target := filepath.Join(cfg.VaultsDir, "moved", "a")
	if err := Update(cfg, p, Edit{Vault: target}); err == nil {
		t.Fatal("a missing target without MoveVault should fail")
	}
	if err := Update(cfg, p, Edit{Vault: target, MoveVault: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, ".claude-obsidian.json")); err != nil {
		t.Fatal("vault not moved")
	}
	if _, err := os.Stat(p.VaultPath()); err == nil {
		t.Fatal("old vault still present")
	}
	projects, _, _ := tree.Walk(cfg.TreeRoot())
	if projects[0].VaultPath() != target {
		t.Fatalf("page not repointed: %s", projects[0].VaultPath())
	}
}

func TestUnlinkLeavesTheVault(t *testing.T) {
	cfg, p := setup(t)
	if err := Unlink(p); err != nil {
		t.Fatal(err)
	}
	projects, _, _ := tree.Walk(cfg.TreeRoot())
	if len(projects) != 0 {
		t.Fatal("project still registered")
	}
	if _, err := os.Stat(filepath.Join(p.VaultPath(), ".claude-obsidian.json")); err != nil {
		t.Fatal("vault was touched")
	}
}
