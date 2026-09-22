package terminal

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestOpenStartsATerminalAtTheFolder(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "args")
	t.Setenv("TERMINAL_TEST_LOG", log)
	t.Setenv("PATH", bin)
	root := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\" > \"$TERMINAL_TEST_LOG\"\n"
	switch runtime.GOOS {
	case "darwin":
		t.Setenv("TERM_PROGRAM", "iTerm.app")
		if err := os.WriteFile(filepath.Join(bin, "open"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	case "linux":
		t.Setenv("TERMINAL", "fake-term")
		if err := os.WriteFile(filepath.Join(bin, "fake-term"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	default:
		t.Skip("no terminal launcher on " + runtime.GOOS)
	}
	if err := Open(filepath.Join(root, "missing")); err == nil {
		t.Fatal("a missing folder is refused")
	}
	if err := Open(root); err != nil {
		t.Fatal(err)
	}
	var data []byte
	for i := 0; i < 50; i++ {
		if data, _ = os.ReadFile(log); len(data) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	got := string(data)
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(got, "-a\niTerm\n"+root) {
			t.Fatalf("args:\n%s", got)
		}
	case "linux":
		if !strings.HasPrefix(got, root) {
			t.Fatalf("cwd:\n%s", got)
		}
	}
}
