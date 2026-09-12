package tree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const authored = `---
schema: atlas.project.v1
name: Capstone
# my own comment
vault: /v/capstone
tags:
  - school
priority: normal
state: active
---

# Capstone

Body stays.
`

func TestUpdateFrontmatterTouchesOnlyGivenKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capstone.md")
	os.WriteFile(path, []byte(authored), 0o644)
	if err := UpdateFrontmatter(path, map[string]any{"name": "Capstone II", "priority": "high", "purpose": "", "repos": []string{"/r/one"}, "materials": []string{}}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	text := string(got)
	for _, want := range []string{"name: Capstone II\n", "# my own comment\n", "tags:\n  - school\n", "priority: high\n", "purpose: \"\"\n", "repos:\n  - /r/one\n", "materials: []\n", "\n# Capstone\n\nBody stays.\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Index(text, "name:") > strings.Index(text, "vault:") {
		t.Fatal("key order changed")
	}
	p, err := Load(path, filepath.Dir(path))
	if err != nil || p.Name != "Capstone II" || p.Priority != "high" || len(p.Repos) != 1 || p.Repos[0] != "/r/one" {
		t.Fatalf("reload: %+v %v", p, err)
	}
}

func TestUpdateFrontmatterOnEmptyBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "n.md")
	os.WriteFile(path, []byte("---\n---\nbody\n"), 0o644)
	if err := UpdateFrontmatter(path, map[string]any{"vault": "/v"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "---\nvault: /v\n---\n\nbody\n" {
		t.Fatalf("got %q", got)
	}
	if err := UpdateFrontmatter(filepath.Join(t.TempDir(), "missing.md"), nil); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMoveAndUnlink(t *testing.T) {
	root := t.TempDir()
	path, _ := Create(root, ProjectOptions{ID: "a", Name: "a", Vault: "/v"})
	p, _ := Load(path, root)
	moved, err := Move(root, p, "work/deep")
	if err != nil || moved != filepath.Join(root, "work", "deep", "a.md") {
		t.Fatalf("move: %s %v", moved, err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("old page still exists")
	}
	p, _ = Load(moved, root)
	if same, err := Move(root, p, "work/deep"); err != nil || same != moved {
		t.Fatalf("no-op move: %s %v", same, err)
	}
	Create(root, ProjectOptions{ID: "a", Name: "a", Vault: "/other"})
	if _, err := Move(root, p, ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected collision, got %v", err)
	}
	if _, err := Move(root, p, "../escape"); err == nil {
		t.Fatal("expected escape error")
	}
	if err := Unlink(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(moved); err == nil {
		t.Fatal("page still exists after unlink")
	}
}
