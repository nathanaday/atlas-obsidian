package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchCommandSetsVaultAndConsent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", dir)
	cmd, err := LaunchCommand(LaunchConfig{Command: "claude", Prompt: "/atlas-obsidian:wiki", Args: []string{"--model", "opus"}, SessionContext: true}, "/v/one", "")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Dir != "/v/one" || strings.Join(cmd.Args[1:], " ") != "--model opus /atlas-obsidian:wiki" {
		t.Fatalf("dir=%s args=%v", cmd.Dir, cmd.Args)
	}
	env := strings.Join(cmd.Env, "\n")
	for _, want := range []string{"ATLAS_OBSIDIAN_VAULT=/v/one", "ATLAS_OBSIDIAN_SESSION_CONTEXT=1"} {
		if !strings.Contains(env, want) {
			t.Errorf("missing %s", want)
		}
	}
	quiet, _ := LaunchCommand(LaunchConfig{SessionContext: false}, "/v/one", "")
	if !strings.Contains(strings.Join(quiet.Env, "\n"), "ATLAS_OBSIDIAN_SESSION_CONTEXT=0") {
		t.Fatal("session context should be off when not enabled")
	}
}

func TestLaunchCommandPromptOverride(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"), 0o755)
	t.Setenv("PATH", dir)
	cmd, _ := LaunchCommand(LaunchConfig{Prompt: "/atlas-obsidian:wiki"}, "/v", IngestPrompt)
	if strings.Join(cmd.Args[1:], " ") != IngestPrompt {
		t.Fatalf("args %v", cmd.Args)
	}
}

func TestLaunchCommandWithoutClaude(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := LaunchCommand(LaunchConfig{}, "/v", ""); err != ErrNoClaude {
		t.Fatalf("got %v", err)
	}
}

func TestLaunchCodex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	cmd, err := LaunchCommand(LaunchConfig{Command: "codex", SessionContext: true}, "/work", "$thread-run thr-123")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Dir != "/work" || len(cmd.Args) != 2 || cmd.Args[1] != "$thread-run thr-123" {
		t.Fatalf("command=%+v", cmd)
	}
	if !strings.Contains(strings.Join(cmd.Env, "\n"), "ATLAS_OBSIDIAN_PROJECT=/work") {
		t.Fatal("missing explicit project selection")
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := LaunchCommand(LaunchConfig{Command: "codex"}, "/work", ""); err == nil || !strings.Contains(err.Error(), "codex") {
		t.Fatalf("missing Codex: %v", err)
	}
}
