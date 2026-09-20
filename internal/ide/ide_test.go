package ide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpen(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "args")
	t.Setenv("IDE_TEST_LOG", log)
	t.Setenv("PATH", bin)
	if err := Open("vscode", "/work"); err == nil || !strings.Contains(err.Error(), "not on PATH") {
		t.Fatalf("missing code: %v", err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$IDE_TEST_LOG\"\n"
	code := filepath.Join(bin, "code")
	if err := os.WriteFile(code, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	root := "/work/project with spaces; literal"
	if err := Open("vscode", root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil || string(data) != "--new-window\n"+root+"\n" {
		t.Fatalf("args=%q err=%v", data, err)
	}
	if err := Open("cursor", root); err == nil {
		t.Fatal("unsupported IDE accepted")
	}
	if err := os.WriteFile(code, []byte("#!/bin/sh\necho launch failed >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Open("vscode", root); err == nil || !strings.Contains(err.Error(), "launch failed") {
		t.Fatalf("launch error: %v", err)
	}
}
