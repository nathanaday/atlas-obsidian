package wizard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/console"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
)

func TestCodexSetup(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	for _, tc := range []struct {
		name          string
		install, fail bool
	}{
		{"install", true, false},
		{"no plugin", false, false},
		{"failed install", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			log := filepath.Join(root, "commands")
			t.Setenv("ATLAS_TEST_COMMAND_LOG", log)
			script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$ATLAS_TEST_COMMAND_LOG\"\n"
			if tc.fail {
				script += "echo install-failed >&2\nexit 1\n"
			}
			if err := os.WriteFile(filepath.Join(root, "codex"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
			var out bytes.Buffer
			c := console.NewWith(true, strings.NewReader(""), &out, false)
			h := home.Home{Root: filepath.Join(root, "atlas")}
			code, err := Run(h, c, Options{Agent: "codex", PluginSource: "/checkout with spaces", WithPlugin: tc.install})
			if tc.fail {
				if code != 1 || err == nil || !strings.Contains(err.Error(), "install-failed") {
					t.Fatalf("code=%d err=%v", code, err)
				}
				if strings.Contains(out.String(), "Setup complete") {
					t.Fatal("failed installation reported success")
				}
				return
			}
			if code != 0 || err != nil {
				t.Fatalf("code=%d err=%v", code, err)
			}
			data, readErr := os.ReadFile(log)
			if !tc.install {
				if !os.IsNotExist(readErr) || strings.Contains(out.String(), "plugin install") {
					t.Fatalf("no-plugin invoked or suggested an installer: %s %s", data, out.String())
				}
				return
			}
			want := "plugin marketplace add /checkout with spaces\nplugin add " + home.DefaultPluginID + "\n"
			if readErr != nil || string(data) != want {
				t.Fatalf("commands=%q err=%v", data, readErr)
			}
			if !strings.Contains(out.String(), "/hooks") {
				t.Fatal("missing hook trust instructions")
			}
		})
	}
}
