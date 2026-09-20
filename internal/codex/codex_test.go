package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledPlugin(t *testing.T) {
	for _, tc := range []struct {
		name, output        string
		found, enabled, bad bool
	}{
		{"enabled", `{"installed":[{"name":"atlas-obsidian","marketplaceName":"atlas","version":"5.2.0","enabled":true}]}`, true, true, false},
		{"disabled", `{"installed":[{"pluginId":"atlas-obsidian@atlas","enabled":false}]}`, true, false, false},
		{"other marketplace", `{"installed":[{"name":"atlas-obsidian","marketplaceName":"other","enabled":true}]}`, false, false, false},
		{"absent", `{"installed":[],"available":[]}`, false, false, false},
		{"bad json", `not json`, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := "#!/bin/sh\n[ \"$*\" = 'plugin list --json --marketplace atlas' ] || exit 1\nprintf '%s' '" + tc.output + "'\n"
			if err := os.WriteFile(filepath.Join(dir, "codex"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir)
			inst, err := InstalledPlugin("atlas-obsidian@atlas")
			if (err != nil) != tc.bad || (inst != nil) != tc.found {
				t.Fatalf("install=%+v err=%v", inst, err)
			}
			if inst != nil && inst.Enabled != tc.enabled {
				t.Fatalf("enabled=%v", inst.Enabled)
			}
		})
	}
}

func TestMissingCodex(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := InstalledPlugin("atlas-obsidian@atlas"); err == nil || !strings.Contains(err.Error(), "codex") {
		t.Fatalf("missing CLI error: %v", err)
	}
}
