package vaults

import (
	"path/filepath"
	"testing"
)

func TestResolveNewPath(t *testing.T) {
	got, _ := ResolveNewPath("triage", "/vaults")
	if got != filepath.Join("/vaults", "triage") {
		t.Fatalf("bare name: %s", got)
	}
	got, _ = ResolveNewPath("/elsewhere/v", "/vaults")
	if got != "/elsewhere/v" {
		t.Fatalf("absolute: %s", got)
	}
	got, _ = ResolveNewPath("./rel", "/vaults")
	if filepath.Base(got) != "rel" || got == filepath.Join("/vaults", "rel") {
		t.Fatalf("relative: %s", got)
	}
}
