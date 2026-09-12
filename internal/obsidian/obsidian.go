// Package obsidian reads and extends the desktop app's vault registry and opens vaults.
//
// Obsidian only opens vaults it already knows, and it reads its registry once at
// launch. So opening a folder means: add it to the registry, restart the app if it
// is running, then open it through the obsidian:// URI.
package obsidian

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// RegistryPath is where the desktop app keeps its vault list on this platform.
func RegistryPath() (string, error) {
	if override := os.Getenv("OBSIDIAN_CONFIG_DIR"); override != "" {
		return filepath.Join(override, "obsidian.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "obsidian", "obsidian.json"), nil
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "obsidian", "obsidian.json"), nil
	default:
		flatpak := filepath.Join(home, ".var", "app", "md.obsidian.Obsidian", "config", "obsidian", "obsidian.json")
		if _, err := os.Stat(flatpak); err == nil {
			return flatpak, nil
		}
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			base = filepath.Join(home, ".config")
		}
		return filepath.Join(base, "obsidian", "obsidian.json"), nil
	}
}

// Vault is one registry entry.
type Vault struct {
	Path string `json:"path"`
	TS   int64  `json:"ts"`
	Open bool   `json:"open,omitempty"`
}

// Registry is the app's vault list plus whatever else the file holds, kept verbatim.
type Registry struct {
	path   string
	raw    map[string]json.RawMessage
	Vaults map[string]Vault
}

var ErrNoRegistry = errors.New("Obsidian's vault registry was not found; open Obsidian once so it creates it")

// LoadRegistry reads the registry file.
func LoadRegistry() (*Registry, error) {
	path, err := RegistryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w (looked at %s)", ErrNoRegistry, path)
	}
	if err != nil {
		return nil, err
	}
	reg := &Registry{path: path, raw: map[string]json.RawMessage{}, Vaults: map[string]Vault{}}
	if err := json.Unmarshal(data, &reg.raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if rawVaults, ok := reg.raw["vaults"]; ok {
		if err := json.Unmarshal(rawVaults, &reg.Vaults); err != nil {
			return nil, fmt.Errorf("%s: vaults: %w", path, err)
		}
	}
	return reg, nil
}

// Find returns the entry for a vault directory, if any.
func (r *Registry) Find(vault string) (id string, entry Vault, ok bool) {
	target := filepath.Clean(vault)
	for id, v := range r.Vaults {
		if filepath.Clean(v.Path) == target {
			return id, v, true
		}
	}
	return "", Vault{}, false
}

// Register adds a vault the way the app does: a random 16-hex id and a timestamp.
func (r *Registry) Register(vault string) (string, error) {
	if id, _, ok := r.Find(vault); ok {
		return id, nil
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b[:])
	r.Vaults[id] = Vault{Path: filepath.Clean(vault), TS: time.Now().UnixMilli()}
	return id, r.save()
}

func (r *Registry) save() error {
	vaults, err := json.Marshal(r.Vaults)
	if err != nil {
		return err
	}
	r.raw["vaults"] = vaults
	data, err := json.Marshal(r.raw)
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}

// OpenURI is the link that opens a registered vault by path.
func OpenURI(vault string) string {
	return "obsidian://open?path=" + url.QueryEscape(vault)
}

// Open asks the desktop to open a registered vault. False means nothing could launch it.
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

// Running reports whether the desktop app has a process.
func Running() bool {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("pgrep", "-x", "Obsidian").Run() == nil
	case "linux":
		return exec.Command("pgrep", "-x", "obsidian").Run() == nil
	case "windows":
		out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq Obsidian.exe").Output()
		return err == nil && strings.Contains(string(out), "Obsidian.exe")
	}
	return false
}

// Restart quits and relaunches the desktop app so it rereads its registry.
func Restart() error {
	if runtime.GOOS != "darwin" {
		return errors.New("restarting Obsidian is only automated on macOS; restart it by hand, then run the command again")
	}
	if err := exec.Command("osascript", "-e", `quit app "Obsidian"`).Run(); err != nil {
		return fmt.Errorf("quit Obsidian: %w", err)
	}
	for i := 0; i < 100 && Running(); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if err := exec.Command("open", "-a", "Obsidian").Run(); err != nil {
		return fmt.Errorf("relaunch Obsidian: %w", err)
	}
	// Give the app time to load its registry before a URI arrives.
	time.Sleep(4 * time.Second)
	return nil
}
