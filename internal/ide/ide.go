// Package ide opens a project's work folder in a desktop IDE.
package ide

import (
	"fmt"
	"os/exec"
	"strings"
)

// Open launches the selected IDE without waiting for its window to close.
// Future launchers: Cursor, Windsurf, Zed, JetBrains IDEs, and Sublime Text.
func Open(preferred, root string) error {
	if preferred != "vscode" {
		return fmt.Errorf("unsupported IDE %q; only vscode is supported", preferred)
	}
	code, err := exec.LookPath("code")
	if err != nil {
		return fmt.Errorf("VS Code's `code` command is not on PATH; install its command-line launcher first")
	}
	out, err := exec.Command(code, "--new-window", root).CombinedOutput()
	if err != nil {
		return fmt.Errorf("open VS Code: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
