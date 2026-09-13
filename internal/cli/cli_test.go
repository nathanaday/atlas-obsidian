package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/claude-atlas/internal/console"
	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/tree"
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
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	h := &harness{t: t, home: filepath.Join(root, "home")}
	vaults := filepath.Join(root, "Vaults")
	code := h.run("setup", "--no-plugin", "--vaults-dir", vaults, "--atlas-vault", filepath.Join(root, "Atlas"), "--first-vault", "welcome")
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
		filepath.Join(vaults, "welcome", ".claude-atlas.json"),
		filepath.Join(vaults, "welcome", ".git", "HEAD"),
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
	if code := h.run("new-vault", "--from", filepath.Join(vaults, "triage")); code != 0 || !strings.Contains(h.out.String(), "already tree/work/triage.md") {
		t.Fatalf("exit %d out %s err %s", code, h.out.String(), h.err.String())
	}
	if code := h.run("new-vault", "--from", t.TempDir(), "--name", "Plain"); code != 1 || !strings.Contains(h.err.String(), "is not a vault") {
		t.Fatalf("exit %d err %s", code, h.err.String())
	}
	legacy := filepath.Join(t.TempDir(), "old")
	os.MkdirAll(filepath.Join(legacy, "wiki"), 0o755)
	os.WriteFile(filepath.Join(legacy, ".claude-obsidian.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(legacy, "wiki", "index.md"), []byte("---\ntitle: I\n---\n"), 0o644)
	if code := h.run("adopt", legacy, "--category", "archive"); code != 0 || !strings.Contains(h.out.String(), "adopted") || !strings.Contains(h.out.String(), "tree/archive/old.md") {
		t.Fatalf("adopt exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if _, err := os.Stat(filepath.Join(legacy, ".claude-atlas.json")); err != nil {
		t.Fatal("adopt should add the identity file")
	}
	if code := h.run("adopt", legacy); code != 0 || !strings.Contains(h.out.String(), "already") {
		t.Fatalf("second adopt exit %d\n%s", code, h.out.String())
	}
	if code := h.run("list"); code != 0 {
		t.Fatal("list failed")
	}
	for _, want := range []string{"work/triage", "welcome"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("list missing %q:\n%s", want, h.out.String())
		}
	}
	if code := h.run("doctor"); !strings.Contains(h.out.String(), "3 registered") {
		t.Fatalf("doctor exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("lint", "work/triage"); code != 0 || !strings.Contains(h.out.String(), "# Wiki lint") {
		t.Fatalf("lint exit %d:\n%s%s", code, h.out.String(), h.err.String())
	}
	if code := h.run("history", filepath.Join(vaults, "triage")); code != 0 || !strings.Contains(h.out.String(), "setup") {
		t.Fatalf("history exit %d:\n%s%s", code, h.out.String(), h.err.String())
	}
	if code := h.run("mode", "work/triage"); code != 0 || strings.TrimSpace(h.out.String()) != "generic" {
		t.Fatalf("mode exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("mode", "work/triage", "lyt"); code != 0 || !strings.Contains(h.out.String(), "generic → lyt") {
		t.Fatalf("mode set exit %d:\n%s%s", code, h.out.String(), h.err.String())
	}
	if code := h.run("recover", "work/triage"); code != 0 || !strings.Contains(h.out.String(), "nothing was interrupted") {
		t.Fatalf("recover exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("refresh"); code != 0 || !strings.Contains(h.out.String(), "work/triage") {
		t.Fatalf("refresh exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("info"); code != 0 {
		t.Fatalf("info exit %d:\n%s", code, h.out.String())
	}
	for _, want := range []string{"overview", "Overview.md", "plugin", "3 registered", "work/triage"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("info missing %q:\n%s", want, h.out.String())
		}
	}
}

func TestLinkCommands(t *testing.T) {
	h, vaults := setup(t)
	docs := filepath.Join(vaults, "docs")
	os.MkdirAll(docs, 0o755)
	os.WriteFile(filepath.Join(docs, "a.pdf"), []byte("x"), 0o644)
	if code := h.run("link", "welcome", docs); code != 0 || !strings.Contains(h.out.String(), "(materials)") {
		t.Fatalf("link exit %d\n%s%s", code, h.out.String(), h.err.String())
	}
	if code := h.run("link", "welcome", docs); code != 1 || !strings.Contains(h.err.String(), "already linked") {
		t.Fatalf("duplicate: %d %s", code, h.err.String())
	}
	if code := h.run("links", "welcome"); code != 0 || !strings.Contains(h.out.String(), "materials") || !strings.Contains(h.out.String(), "1 file") {
		t.Fatalf("links exit %d:\n%s", code, h.out.String())
	}
	if code := h.run("unlink", "welcome", docs); code != 0 {
		t.Fatalf("unlink exit %d %s", code, h.err.String())
	}
	if code := h.run("links", "welcome"); code != 0 || !strings.Contains(h.out.String(), "no links") {
		t.Fatalf("after unlink:\n%s", h.out.String())
	}
	if code := h.run("link", "welcome", docs, "--kind", "bogus"); code != 2 {
		t.Fatalf("bad kind exit %d", code)
	}
}

func TestResolveVault(t *testing.T) {
	h, vaults := setup(t)
	cfg, _ := home.Home{Root: h.home}.Load()
	projects, _, _ := tree.Walk(cfg.TreeRoot())
	if got, label, err := resolveVault(cfg, projects, ""); err != nil || got != cfg.AtlasVault || label != "the atlas" {
		t.Fatalf("atlas: %s %s %v", got, label, err)
	}
	if got, _, err := resolveVault(cfg, projects, "welcome"); err != nil || got != filepath.Join(vaults, "welcome") {
		t.Fatalf("project: %s %v", got, err)
	}
	if got, _, err := resolveVault(cfg, projects, vaults); err != nil || got != vaults {
		t.Fatalf("path: %s %v", got, err)
	}
	if _, _, err := resolveVault(cfg, projects, "nope"); err == nil {
		t.Fatal("unknown name should fail")
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
