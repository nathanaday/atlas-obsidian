// Package terminal opens a new terminal window at a folder.
package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// macApps maps what TERM_PROGRAM says to the application `open -a` starts. A terminal
// not listed here falls back to Terminal.
var macApps = map[string]string{
	"Apple_Terminal": "Terminal",
	"iTerm.app":      "iTerm",
	"WezTerm":        "WezTerm",
	"ghostty":        "Ghostty",
	"kitty":          "kitty",
	"Alacritty":      "Alacritty",
	"WarpTerminal":   "Warp",
	"Hyper":          "Hyper",
	"tabby":          "Tabby",
}

// linuxTerminals are tried in order when TERMINAL is not set. Every one starts in the
// current directory, so the folder is the command's working directory.
var linuxTerminals = []string{"x-terminal-emulator", "gnome-terminal", "konsole", "xfce4-terminal", "alacritty", "kitty", "wezterm", "foot", "xterm"}

// Open starts a terminal window at root and returns once it has been launched. On
// macOS it is the terminal the user is running in, or Terminal; on Linux it is
// $TERMINAL, or the first known emulator on PATH.
func Open(root string) error {
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a folder", root)
	}
	switch runtime.GOOS {
	case "darwin":
		app := macApps[os.Getenv("TERM_PROGRAM")]
		if app == "" {
			app = "Terminal"
		}
		out, err := exec.Command("open", "-a", app, root).CombinedOutput()
		if err != nil {
			return fmt.Errorf("open %s: %w: %s", app, err, strings.TrimSpace(string(out)))
		}
		return nil
	case "linux":
		name := os.Getenv("TERMINAL")
		if name == "" {
			for _, candidate := range linuxTerminals {
				if _, err := exec.LookPath(candidate); err == nil {
					name = candidate
					break
				}
			}
		}
		if name == "" {
			return fmt.Errorf("no terminal found; set TERMINAL to the emulator to start")
		}
		cmd := exec.Command(name)
		cmd.Dir = root
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start %s: %w", name, err)
		}
		go cmd.Wait()
		return nil
	}
	return fmt.Errorf("opening a terminal is not supported on %s", runtime.GOOS)
}
