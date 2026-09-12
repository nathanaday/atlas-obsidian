// Package obsidian opens vaults in the Obsidian desktop app.
package obsidian

import (
	"net/url"
	"os/exec"
	"runtime"
)

// OpenURI is the obsidian:// link that opens (and registers) a vault by path.
func OpenURI(vault string) string {
	return "obsidian://open?path=" + url.QueryEscape(vault)
}

// Open asks the desktop to open the vault. It returns false when nothing could launch it.
func Open(vault string) bool {
	uri := OpenURI(vault)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", uri)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", uri)
	default:
		opener, err := exec.LookPath("xdg-open")
		if err != nil {
			return false
		}
		cmd = exec.Command(opener, uri)
	}
	return cmd.Run() == nil
}
