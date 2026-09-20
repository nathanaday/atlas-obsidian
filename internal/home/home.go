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
	ConfigSchema = "atlas-obsidian.config.v1"
	EnvHome      = "ATLAS_OBSIDIAN_HOME"
	defaultHome  = "~/.atlas-obsidian"

	// DefaultPluginID is the atlas-obsidian plugin as Claude Code names it.
	DefaultPluginID = "atlas-obsidian@nathanaday-atlas-obsidian"
	// DefaultPluginSource is what `claude plugin marketplace add` takes: this repository.
	DefaultPluginSource = "nathanaday/atlas-obsidian"

	// DefaultNewDays is how many days after its creation a vault counts as new.
	DefaultNewDays = 7
)

// HeatConfig tunes how the overview reads a vault's activity. NewDays is the age, in
// days, under which a vault is "new" whatever its activity; 0 turns that off.
type HeatConfig struct {
	NewDays int `json:"new_days"`
}

// PluginConfig says where the atlas-obsidian plugin comes from.
type PluginConfig struct {
	// ID is the plugin id, name@marketplace.
	ID string `json:"id"`
	// Source is passed to `claude plugin marketplace add`: a GitHub slug or a local path.
	Source string `json:"source"`
}

// LaunchConfig says how to start Claude Code inside a vault. It mirrors
// claudecode.LaunchConfig; home cannot import that package.
type LaunchConfig struct {
	Command        string   `json:"command"`
	Args           []string `json:"args,omitempty"`
	Prompt         string   `json:"prompt,omitempty"`
	SessionContext bool     `json:"session_context"`
}

// Config is the contents of config.json. Paths are absolute. The atlas knows a knowledge
// base or a project only when the config lists its folder; it never searches for one.
type Config struct {
	PreferredHarness string       `json:"preferred_harness,omitempty"`
	Schema           string       `json:"schema"`
	Plugin           PluginConfig `json:"plugin"`
	ClaudeCode       LaunchConfig `json:"claude_code"`
	// Heat is nil in a config written before the section existed; NewDays reads it.
	Heat *HeatConfig `json:"heat,omitempty"`
	// Projects holds every project's work folder, the parent of its atlas/<name>/ folder.
	Projects []string `json:"projects,omitempty"`
}

// Harness returns the preferred interactive agent, defaulting to Claude for older configs.
func (c *Config) Harness() string {
	if c.PreferredHarness == "" {
		return "claude"
	}
	return c.PreferredHarness
}

func (c *Config) SetPreferredHarness(harness string) error {
	if harness != "claude" && harness != "codex" {
		return fmt.Errorf("preferred-harness must be claude or codex, got %q", harness)
	}
	c.PreferredHarness = harness
	return nil
}

// HarnessLaunch preserves the configured Claude command and uses the Codex launcher defaults.
func (c *Config) HarnessLaunch(harness string) LaunchConfig {
	if harness == "codex" {
		return LaunchConfig{Command: "codex", SessionContext: true}
	}
	return c.ClaudeCode
}

// AddProject records a project's work folder; it reports whether it was added.
func (c *Config) AddProject(work string) bool {
	work = Expand(work)
	if contains(c.Projects, work) {
		return false
	}
	c.Projects = append(c.Projects, work)
	return true
}

// RemoveProject drops a project's work folder; it reports whether it was present.
func (c *Config) RemoveProject(work string) bool {
	var ok bool
	c.Projects, ok = remove(c.Projects, Expand(work))
	return ok
}

// HasProject reports whether the config lists work.
func (c *Config) HasProject(work string) bool { return contains(c.Projects, Expand(work)) }

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func remove(list []string, s string) ([]string, bool) {
	for i, item := range list {
		if item == s {
			return append(list[:i], list[i+1:]...), true
		}
	}
	return list, false
}

// NewDays is the configured age under which a vault is new, or the default.
func (c *Config) NewDays() int {
	if c.Heat == nil {
		return DefaultNewDays
	}
	return c.Heat.NewDays
}

// SetNewDays records the threshold; it must not be negative.
func (c *Config) SetNewDays(days int) error {
	if days < 0 {
		return fmt.Errorf("new_days must be 0 or more, got %d", days)
	}
	c.Heat = &HeatConfig{NewDays: days}
	return nil
}

// Home is the atlas home directory.
type Home struct {
	Root string
}

// Resolve picks the home: an explicit flag, then $ATLAS_OBSIDIAN_HOME, then ~/.atlas-obsidian.
func Resolve(explicit string) Home {
	value := explicit
	if value == "" {
		value = os.Getenv(EnvHome)
	}
	if value == "" {
		return Home{Root: Expand(defaultHome)}
	}
	abs, err := filepath.Abs(Expand(value))
	if err != nil {
		abs = Expand(value)
	}
	return Home{Root: abs}
}

func (h Home) ConfigPath() string { return filepath.Join(h.Root, "config.json") }

// StateDir holds the derived registry file. Safe to delete.
func (h Home) StateDir() string { return filepath.Join(h.Root, "state") }

func (h Home) Exists() bool {
	info, err := os.Stat(h.ConfigPath())
	return err == nil && info.Mode().IsRegular()
}

func defaultPlugin() PluginConfig {
	return PluginConfig{ID: DefaultPluginID, Source: DefaultPluginSource}
}

func defaultLaunch() LaunchConfig {
	return LaunchConfig{Command: "claude", SessionContext: true}
}

// Default is the config a fresh setup starts from.
func (h Home) Default() *Config {
	return &Config{
		Schema:     ConfigSchema,
		Plugin:     defaultPlugin(),
		ClaudeCode: defaultLaunch(),
		Heat:       &HeatConfig{NewDays: DefaultNewDays},
	}
}

var ErrNoAtlas = errors.New("no atlas here; run `atlas-obsidian setup` first")

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
	for i, v := range cfg.Projects {
		cfg.Projects[i] = Expand(v)
	}
	// Configs written before these sections existed keep working with the defaults.
	if cfg.Plugin.ID == "" {
		cfg.Plugin = defaultPlugin()
	}
	cfg.Plugin.Source = Expand(cfg.Plugin.Source)
	if cfg.ClaudeCode.Command == "" {
		cfg.ClaudeCode = defaultLaunch()
	}
	if cfg.Heat != nil && cfg.Heat.NewDays < 0 {
		return nil, fmt.Errorf("%s: heat.new_days must be 0 or more", h.ConfigPath())
	}
	if err := cfg.SetPreferredHarness(cfg.Harness()); err != nil {
		return nil, err
	}
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
