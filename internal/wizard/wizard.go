// Package wizard is the guided `atlas-obsidian setup` flow.
package wizard

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/claudecode"
	"github.com/nathanaday/atlas-obsidian/internal/console"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/refresh"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

// Options come from setup's flags.
type Options struct {
	Version      string
	PluginSource string // marketplace source override, e.g. a local checkout
	WithPlugin   bool
	Agent        string // claude (default) or codex
}

func plan(c *console.Console, label, action, target string) {
	c.Say("  %-16s %-10s %s", label, action, target)
}

// Run executes setup. It returns 1 when the user declines the plan.
func Run(h home.Home, c *console.Console, opts Options) (int, error) {
	if opts.Agent == "" {
		opts.Agent = "claude"
	}
	if opts.Agent != "claude" && opts.Agent != "codex" {
		return 1, fmt.Errorf("unknown agent %q; choose claude or codex", opts.Agent)
	}
	fresh := !h.Exists()
	var cfg *home.Config
	if fresh {
		cfg = h.Default()
	} else {
		loaded, err := h.Load()
		if err != nil {
			return 1, err
		}
		cfg = loaded
	}
	if opts.PluginSource != "" {
		cfg.Plugin.Source = home.Expand(opts.PluginSource)
	}
	if !gitx.Available() {
		return 1, fmt.Errorf("git is required and is not on PATH; install it (on macOS: xcode-select --install) and run setup again")
	}

	var installed *claudecode.Install
	hostCLI, _ := exec.LookPath(opts.Agent)
	if opts.Agent == "claude" && opts.WithPlugin {
		installed, _ = claudecode.InstalledPlugin(cfg.Plugin.ID)
	}
	ix, err := registry.Scan(cfg)
	if err != nil {
		return 1, err
	}
	c.Say("")
	c.Say("atlas-obsidian setup")
	c.Say("")
	plan(c, "home", ternary(fresh, "create", "exists"), home.Display(h.Root))
	switch {
	case !opts.WithPlugin:
		plan(c, "plugin", "skip", "--no-plugin")
	case installed != nil:
		plan(c, "plugin", "installed", fmt.Sprintf("%s v%s", cfg.Plugin.ID, installed.Version))
	case hostCLI == "":
		plan(c, "plugin", "skip", fmt.Sprintf("`%s` is not on PATH; manual commands will be printed", opts.Agent))
	default:
		plan(c, "plugin", "install", fmt.Sprintf("%s from %s via `%s plugin`", cfg.Plugin.ID, cfg.Plugin.Source, opts.Agent))
	}
	plan(c, "projects", "keep", fmt.Sprintf("%d listed", len(ix.Projects())))
	c.Say("")
	ok, err := c.Confirm("Proceed?", true)
	if err != nil {
		return 1, err
	}
	if !ok {
		return 1, nil
	}
	c.Say("")

	if err := h.Save(cfg); err != nil {
		return 1, err
	}
	c.Step(console.OK, "home", home.Display(h.Root))

	if opts.Agent == "codex" && opts.WithPlugin && hostCLI != "" {
		for _, args := range [][]string{{"plugin", "marketplace", "add", cfg.Plugin.Source}, {"plugin", "add", cfg.Plugin.ID}} {
			out, err := exec.Command(hostCLI, args...).CombinedOutput()
			if err != nil {
				return 1, fmt.Errorf("codex %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
			}
			c.Step(console.OK, "ran", "codex "+strings.Join(args, " "))
		}
		c.Say("Start a new Codex session, review and trust the plugin hooks in /hooks, then restart the session.")
	} else if installed == nil && opts.WithPlugin && hostCLI != "" {
		ran, err := claudecode.InstallPlugin(cfg.Plugin.Source, cfg.Plugin.ID)
		for _, cmd := range ran {
			c.Step(console.OK, "ran", cmd)
		}
		if err != nil {
			c.Step(console.Fail, "plugin", err.Error())
		} else {
			installed, _ = claudecode.InstalledPlugin(cfg.Plugin.ID)
		}
	}
	switch {
	case !opts.WithPlugin || opts.Agent == "codex":
		// The selected host's result was reported above, or installation was skipped.
	case installed != nil && installed.Version != "" && installed.Version != opts.Version && opts.Version != "dev":
		c.Step(console.OK, "plugin", fmt.Sprintf("v%s installed; this binary is %s. Keep them in step.", installed.Version, opts.Version))
	case installed != nil:
		c.Step(console.OK, "plugin", fmt.Sprintf("v%s", installed.Version))
	default:
		c.Step(console.Skip, "plugin", "not installed; Claude Code will not have the atlas tools until it is")
	}

	entries, _, err := refresh.Registry(h, cfg, h.StateDir(), time.Now())
	if err != nil {
		return 1, err
	}
	read := 0
	for _, e := range entries {
		if e.Error != "" {
			c.Step(console.Fail, "project", home.Display(e.Path)+": "+e.Error)
			continue
		}
		read++
	}
	c.Step(console.OK, "refreshed", fmt.Sprintf("%d entr%s", read, map[bool]string{true: "y", false: "ies"}[read == 1]))

	c.Say("")
	c.Say("Setup complete.")
	c.Say("")
	c.Say("Next:")
	c.Say("  cd <your work> && atlas-obsidian init   make a folder or a repository a project")
	c.Say("  atlas-obsidian open-vault NAME          open a project in Obsidian")
	c.Say("  cd <your work> && %-6s                start an agent session", opts.Agent)
	c.Say("  atlas-obsidian refresh                  read everything again")
	if opts.WithPlugin && installed == nil && (opts.Agent == "claude" || hostCLI == "") {
		c.Say("")
		c.Say("The plugin is not installed. Install it, then run setup again:")
		for _, cmd := range claudecode.Commands(cfg.Plugin.Source, cfg.Plugin.ID) {
			if opts.Agent == "codex" {
				cmd = strings.Replace(cmd, "claude plugin", "codex plugin", 1)
				cmd = strings.Replace(cmd, "plugin install", "plugin add", 1)
			}
			c.Say("  %s", cmd)
		}
	}
	c.Say("")
	return 0, nil
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
