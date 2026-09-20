package home

import "testing"

func TestPreferredHarness(t *testing.T) {
	h := Home{Root: t.TempDir()}
	cfg := h.Default()
	if cfg.Harness() != "claude" {
		t.Fatal("old configs default to Claude")
	}
	cfg.ClaudeCode.Command = "custom-claude"
	if cfg.HarnessLaunch("claude").Command != "custom-claude" || cfg.HarnessLaunch("codex").Command != "codex" {
		t.Fatal("incorrect launch config")
	}
	if err := cfg.SetPreferredHarness("codex"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetPreferredHarness("other"); err == nil || cfg.Harness() != "codex" {
		t.Fatal("invalid value accepted")
	}
	cfg.PreferredHarness = "other"
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Load(); err == nil {
		t.Fatal("invalid on-disk preference accepted")
	}
}
