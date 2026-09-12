// Package home locates and manages the atlas home directory and its config.
package home

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ConfigSchema  = "claude-atlas.config.v1"
	EnvHome       = "CLAUDE_ATLAS_HOME"
	defaultHome   = "~/.claude-atlas"
	DefaultVaults = "~/Documents/Vaults"
	DefaultAtlas  = "~/Documents/Atlas"
)

// ProductConfig says where claude-obsidian comes from.
type ProductConfig struct {
	// Plugin is the Claude Code plugin id, e.g. claude-obsidian@agricidaniel-claude-obsidian.
	Plugin string `json:"plugin"`
	// Marketplace is the source passed to `claude plugin marketplace add`.
	Marketplace string `json:"marketplace"`
	// Path, when set, points at a product checkout and bypasses the plugin lookup.
	Path string `json:"path,omitempty"`
}

// Config is the contents of config.json. Paths are absolute.
type Config struct {
	Schema         string        `json:"schema"`
	VaultsDir      string        `json:"vaults_dir"`
	AtlasVault     string        `json:"atlas_vault"`
	ClaudeObsidian ProductConfig `json:"claude_obsidian"`
}

// TreeRoot is the directory of nodes inside the atlas vault.
func (c *Config) TreeRoot() string { return filepath.Join(c.AtlasVault, "tree") }

// Home is the atlas home directory.
type Home struct {
	Root string
}

// Resolve picks the home: an explicit flag, then $CLAUDE_ATLAS_HOME, then ~/.claude-atlas.
func Resolve(explicit string) Home {
	value := explicit
	if value == "" {
		value = os.Getenv(EnvHome)
	}
	if value == "" {
		value = defaultHome
	}
	abs, err := filepath.Abs(Expand(value))
	if err != nil {
		abs = Expand(value)
	}
	return Home{Root: abs}
}

func (h Home) ConfigPath() string { return filepath.Join(h.Root, "config.json") }

// StateDir holds derived state for every project, mirroring the tree. Safe to delete.
func (h Home) StateDir() string { return filepath.Join(h.Root, "state") }

func (h Home) Exists() bool {
	info, err := os.Stat(h.ConfigPath())
	return err == nil && info.Mode().IsRegular()
}

// Default is the config a fresh setup starts from. Empty arguments take the defaults.
func (h Home) Default(vaultsDir, atlasVault string) *Config {
	if vaultsDir == "" {
		vaultsDir = DefaultVaults
	}
	if atlasVault == "" {
		atlasVault = DefaultAtlas
	}
	return &Config{
		Schema:     ConfigSchema,
		VaultsDir:  Expand(vaultsDir),
		AtlasVault: Expand(atlasVault),
		ClaudeObsidian: ProductConfig{
			Plugin:      "claude-obsidian@agricidaniel-claude-obsidian",
			Marketplace: "AgriciDaniel/claude-obsidian",
		},
	}
}

var ErrNoAtlas = errors.New("no atlas here; run `claude-atlas setup` first")

func (h Home) Load() (*Config, error) {
	data, err := os.ReadFile(h.ConfigPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w (looked in %s)", ErrNoAtlas, Display(h.Root))
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", h.ConfigPath(), err)
	}
	if cfg.Schema != ConfigSchema {
		return nil, fmt.Errorf("%s: unsupported schema %q", h.ConfigPath(), cfg.Schema)
	}
	cfg.VaultsDir = Expand(cfg.VaultsDir)
	cfg.AtlasVault = Expand(cfg.AtlasVault)
	cfg.ClaudeObsidian.Path = Expand(cfg.ClaudeObsidian.Path)
	return &cfg, nil
}

func (h Home) Save(cfg *Config) error {
	if err := os.MkdirAll(h.Root, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.ConfigPath(), append(data, '\n'), 0o644)
}

// Expand replaces a leading ~ with the user's home directory.
func Expand(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if dir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(dir, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

// Display shortens a path under the home directory to ~/... for output.
func Display(path string) string {
	dir, err := os.UserHomeDir()
	if err != nil || dir == "" {
		return path
	}
	if path == dir {
		return "~"
	}
	if strings.HasPrefix(path, dir+string(filepath.Separator)) {
		return "~" + path[len(dir):]
	}
	return path
}
