package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/vault"
)

func newVault(t *testing.T) *vault.Vault {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := filepath.Join(t.TempDir(), "v")
	if _, err := vault.Init(root, vault.Generic, time.Now()); err != nil {
		t.Fatal(err)
	}
	v, _ := vault.Open(root)
	return v
}

func env(values map[string]string) Env {
	return func(k string) string { return values[k] }
}

func TestSessionStart(t *testing.T) {
	v := newVault(t)
	var out bytes.Buffer
	if err := SessionStart(strings.NewReader(`{"cwd":"`+v.Path("wiki")+`"}`), &out, env(nil), true); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{"claude-atlas vault: v (generic mode)", "<vault-context>", "Active Threads", "/claude-atlas:wiki"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "type: meta") {
		t.Fatal("frontmatter should be stripped")
	}
	out.Reset()
	SessionStart(strings.NewReader(`{"cwd":"`+t.TempDir()+`"}`), &out, env(nil), true)
	if out.Len() != 0 {
		t.Fatal("silent outside a vault")
	}
	out.Reset()
	SessionStart(strings.NewReader(`{"cwd":"/nowhere"}`), &out, env(map[string]string{vault.EnvVault: v.Root, "CLAUDE_ATLAS_SESSION_CONTEXT": "0"}), true)
	if !strings.Contains(out.String(), "claude-atlas vault") || strings.Contains(out.String(), "<vault-context>") {
		t.Fatalf("env vault with context off:\n%s", out.String())
	}
	os.MkdirAll(v.Path(".vault-meta"), 0o755)
	os.WriteFile(v.Path(".vault-meta/inflight.json"), []byte(`{"operation_id":"save-x","paths":[]}`), 0o644)
	out.Reset()
	SessionStart(strings.NewReader(`{"cwd":"`+v.Root+`"}`), &out, env(nil), false)
	if !strings.Contains(out.String(), "WARNING: operation save-x was interrupted") {
		t.Fatalf("recovery warning:\n%s", out.String())
	}
	out.Reset()
	Stop(strings.NewReader(`{"cwd":"`+v.Root+`"}`), &out, env(nil))
	if !strings.Contains(out.String(), `"systemMessage"`) || !strings.Contains(out.String(), "save-x") {
		t.Fatalf("stop:\n%s", out.String())
	}
}

func TestGuard(t *testing.T) {
	v := newVault(t)
	cases := map[string]bool{
		v.Path("wiki/concepts/A.md"):            true,
		v.Path(".raw/captured/x.pdf"):           true,
		v.Path(".claude-atlas.json"):            true,
		v.Path("inbox/paper.md"):                false,
		v.Path("notes.md"):                      false,
		filepath.Join(t.TempDir(), "wiki/x.md"): false,
	}
	for path, deny := range cases {
		var out bytes.Buffer
		if err := Guard(strings.NewReader(`{"tool_name":"Write","tool_input":{"file_path":"`+path+`"}}`), &out); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(out.String(), `"deny"`); got != deny {
			t.Errorf("%s: deny=%v, got %q", path, deny, out.String())
		}
	}
	var out bytes.Buffer
	Guard(strings.NewReader(`{"tool_name":"Edit","cwd":"`+v.Root+`","tool_input":{"file_path":"wiki/hot.md"}}`), &out)
	if !strings.Contains(out.String(), "deny") {
		t.Fatal("relative paths resolve against cwd")
	}
}
