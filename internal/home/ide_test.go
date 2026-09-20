package home

import "testing"

func TestPreferredIDE(t *testing.T) {
	h := Home{Root: t.TempDir()}
	cfg := h.Default()
	if cfg.IDE() != "vscode" {
		t.Fatal("default IDE")
	}
	if err := cfg.SetPreferredIDE("vscode"); err != nil {
		t.Fatal(err)
	}
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	saved, err := h.Load()
	if err != nil || saved.PreferredIDE != "vscode" {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	if err := cfg.SetPreferredIDE("cursor"); err == nil || cfg.IDE() != "vscode" {
		t.Fatal("unsupported IDE accepted")
	}
	cfg.PreferredIDE = "cursor"
	if err := h.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Load(); err == nil {
		t.Fatal("invalid on-disk preference accepted")
	}
}
