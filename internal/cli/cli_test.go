package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/claude-atlas/internal/console"
	"github.com/nathanaday/claude-atlas/internal/testutil"
)

type harness struct {
	t    *testing.T
	home string
	out  bytes.Buffer
	err  bytes.Buffer
}

func (h *harness) run(args ...string) int {
	h.out.Reset()
	h.err.Reset()
	c := console.NewWith(true, strings.NewReader(""), &h.out, false)
	return run(append([]string{"--home", h.home}, args...), strings.NewReader(""), &h.out, &h.err, c)
}

func setup(t *testing.T) (*harness, string) {
	prod := testutil.Product(t)
	root := t.TempDir()
	h := &harness{t: t, home: filepath.Join(root, "home")}
	vaults := filepath.Join(root, "Vaults")
	code := h.run("setup", "--no-plugin", "--vaults-dir", vaults, "--atlas-vault", filepath.Join(root, "Atlas"), "--first-vault", "welcome", "--claude-obsidian", prod.Root)
	if code != 0 {
		t.Fatalf("setup exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	return h, vaults
}

func TestSetupCreatesHomeAtlasAndFirstVault(t *testing.T) {
	h, vaults := setup(t)
	if !strings.Contains(h.out.String(), "Setup complete.") {
		t.Fatalf("output:\n%s", h.out.String())
	}
	for _, path := range []string{
		filepath.Join(vaults, "welcome", ".claude-obsidian.json"),
		filepath.Join(filepath.Dir(h.home), "Atlas", ".obsidian", "app.json"),
		filepath.Join(filepath.Dir(h.home), "Atlas", "tree", "welcome.md"),
		filepath.Join(h.home, "state", "welcome.json"),
		filepath.Join(h.home, "config.json"),
		filepath.Join(filepath.Dir(h.home), "Atlas", "About.md"),
		filepath.Join(filepath.Dir(h.home), "Atlas", "Reference.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s", path)
		}
	}
	page, _ := os.ReadFile(filepath.Join(filepath.Dir(h.home), "Atlas", "Overview.md"))
	if !strings.Contains(string(page), "[[tree/welcome\\|welcome]]") {
		t.Fatalf("atlas page:\n%s", page)
	}
	if code := h.run("setup", "--no-plugin"); code != 0 || !strings.Contains(h.out.String(), "keep       1 registered") {
		t.Fatalf("rerun exit %d:\n%s", code, h.out.String())
	}
}

func TestVaultCommands(t *testing.T) {
	h, vaults := setup(t)
	if code := h.run("new-vault", "triage", "--category", "work", "--purpose", "Sort sensors."); code != 0 {
		t.Fatalf("new-vault exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if !strings.Contains(h.out.String(), "tree/work/triage.md") {
		t.Fatalf("output:\n%s", h.out.String())
	}
	if _, err := os.Stat(filepath.Join(vaults, "triage", "wiki", "hot.md")); err != nil {
		t.Fatal("vault not created")
	}
	if code := h.run("new-vault", "--from", filepath.Join(vaults, "triage")); code != 1 || !strings.Contains(h.err.String(), "already registered as work/triage") {
		t.Fatalf("exit %d err %s", code, h.err.String())
	}
	if code := h.run("new-vault", "--from", t.TempDir(), "--name", "Plain"); code != 1 || !strings.Contains(h.err.String(), "not a claude-obsidian vault") {
		t.Fatalf("exit %d err %s", code, h.err.String())
	}
	if code := h.run("list"); code != 0 {
		t.Fatal("list failed")
	}
	for _, want := range []string{"work/triage", "welcome"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("list missing %q:\n%s", want, h.out.String())
		}
	}
	if code := h.run("doctor"); code != 0 || !strings.Contains(h.out.String(), "2 registered") {
		t.Fatalf("doctor exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("refresh"); code != 0 || !strings.Contains(h.out.String(), "work/triage") {
		t.Fatalf("refresh exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("info"); code != 0 {
		t.Fatalf("info exit %d:\n%s", code, h.out.String())
	}
	for _, want := range []string{"overview", "Overview.md", "claude-obsidian", "2 registered", "work/triage"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("info missing %q:\n%s", want, h.out.String())
		}
	}
}

func TestCommandsNeedSetupFirst(t *testing.T) {
	h := &harness{t: t, home: filepath.Join(t.TempDir(), "none")}
	if code := h.run("refresh"); code != 1 || !strings.Contains(h.err.String(), "claude-atlas setup") {
		t.Fatalf("exit %d err %s", code, h.err.String())
	}
	if code := h.run("info"); code != 0 || !strings.Contains(h.out.String(), "not set up") {
		t.Fatalf("info before setup: %d %s", code, h.out.String())
	}
	if code := h.run("new-vault", "a", "--from", "/b"); code != 2 {
		t.Fatalf("name and --from together should be a usage error, got %d", code)
	}
	if code := h.run("bogus"); code != 2 {
		t.Fatalf("unknown command exit %d", code)
	}
	if code := h.run("version"); code != 0 || strings.TrimSpace(h.out.String()) != Version {
		t.Fatalf("version: %q", h.out.String())
	}
}
