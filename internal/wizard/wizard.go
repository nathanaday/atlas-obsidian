// Package wizard is the guided `claude-atlas setup` flow.
package wizard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nathanaday/claude-atlas/internal/claudecode"
	"github.com/nathanaday/claude-atlas/internal/console"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/product"
	"github.com/nathanaday/claude-atlas/internal/refresh"
	"github.com/nathanaday/claude-atlas/internal/tree"
	"github.com/nathanaday/claude-atlas/internal/vaults"
)

// Options come from setup's flags.
type Options struct {
	VaultsDir   string
	FirstVault  string
	ProductPath string // dev override written into config
	WithPlugin  bool
}

func plan(c *console.Console, label, action, target string) {
	c.Say("  %-16s %-10s %s", label, action, target)
}

// EnsureAtlasVault creates the root Obsidian vault if missing; it reports whether it did.
func EnsureAtlasVault(cfg *home.Config) (bool, error) {
	obsidianDir := filepath.Join(cfg.AtlasVault, ".obsidian")
	_, err := os.Stat(obsidianDir)
	created := err != nil
	if err := os.MkdirAll(obsidianDir, 0o755); err != nil {
		return false, err
	}
	app := filepath.Join(obsidianDir, "app.json")
	if _, err := os.Stat(app); err != nil {
		data, _ := json.MarshalIndent(map[string]any{"newLinkFormat": "absolute"}, "", "  ")
		if err := os.WriteFile(app, append(data, '\n'), 0o644); err != nil {
			return false, err
		}
	}
	return created, os.MkdirAll(cfg.TreeRoot(), 0o755)
}

// Run executes setup. It returns 1 when the user declines the plan.
func Run(h home.Home, c *console.Console, opts Options) (int, error) {
	fresh := !h.Exists()
	var cfg *home.Config
	if fresh {
		dir := opts.VaultsDir
		if dir == "" {
			dir = c.Ask("Where should new vaults live?", "~/Documents/Vaults")
		}
		cfg = h.Default(dir)
	} else {
		loaded, err := h.Load()
		if err != nil {
			return 1, err
		}
		cfg = loaded
		if opts.VaultsDir != "" {
			cfg.VaultsDir = home.Expand(opts.VaultsDir)
		}
	}
	if opts.ProductPath != "" {
		cfg.ClaudeObsidian.Path = home.Expand(opts.ProductPath)
	}

	prod, locateErr := product.Locate(cfg.ClaudeObsidian)
	claude := claudecode.CLI()
	atlasReady := false
	if _, err := os.Stat(filepath.Join(cfg.AtlasVault, ".obsidian")); err == nil {
		atlasReady = true
	}
	var registered []*tree.Node
	if atlasReady {
		nodes, err := tree.Walk(cfg.TreeRoot())
		if err != nil {
			return 1, err
		}
		registered = tree.Leaves(nodes)
	}
	firstPath := ""
	if len(registered) == 0 {
		name := opts.FirstVault
		if name == "" {
			name = c.Ask("Name for your first vault", "welcome")
		}
		path, err := vaults.ResolveNewPath(name, cfg.VaultsDir)
		if err != nil {
			return 1, err
		}
		if _, err := os.Stat(path); err == nil {
			return 1, fmt.Errorf("%s already exists; choose another name or register it with `claude-atlas vault add`", home.Display(path))
		}
		firstPath = path
	}

	c.Say("")
	c.Say("claude-atlas setup")
	c.Say("")
	plan(c, "home", ternary(fresh, "create", "exists"), home.Display(h.Root))
	switch {
	case prod != nil:
		note := ""
		if !prod.Tested() {
			note = fmt.Sprintf(" (atlas was tested with v%s)", product.TestedVersion)
		}
		plan(c, "claude-obsidian", "installed", fmt.Sprintf("v%s at %s%s", prod.Version, home.Display(prod.Root), note))
	case !opts.WithPlugin:
		plan(c, "claude-obsidian", "skip", "--no-plugin; "+locateErr.Error())
	case claude == "":
		plan(c, "claude-obsidian", "skip", "`claude` is not on PATH; manual commands will be printed")
	default:
		plan(c, "claude-obsidian", "install", fmt.Sprintf("%s from %s via `claude plugin`", cfg.ClaudeObsidian.Plugin, cfg.ClaudeObsidian.Marketplace))
	}
	plan(c, "atlas vault", ternary(atlasReady, "exists", "create"), home.Display(cfg.AtlasVault))
	plan(c, "vaults dir", "use", home.Display(cfg.VaultsDir))
	if firstPath != "" {
		plan(c, "first vault", "create", home.Display(firstPath)+" (claude-obsidian init)")
	} else {
		plan(c, "vaults", "keep", fmt.Sprintf("%d registered", len(registered)))
	}
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

	if prod == nil && opts.WithPlugin && claude != "" {
		ran, err := claudecode.InstallPlugin(cfg.ClaudeObsidian.Marketplace, cfg.ClaudeObsidian.Plugin)
		for _, cmd := range ran {
			c.Step(console.OK, "ran", cmd)
		}
		if err != nil {
			c.Step(console.Fail, "claude-obsidian", err.Error())
		} else if prod, locateErr = product.Locate(cfg.ClaudeObsidian); locateErr != nil {
			c.Step(console.Fail, "claude-obsidian", locateErr.Error())
		}
	}
	switch {
	case prod != nil && prod.Tested():
		c.Step(console.OK, "claude-obsidian", fmt.Sprintf("v%s (%s)", prod.Version, prod.Source))
	case prod != nil:
		c.Step(console.OK, "claude-obsidian", fmt.Sprintf("v%s; atlas was tested with v%s", prod.Version, product.TestedVersion))
	default:
		c.Step(console.Skip, "claude-obsidian", "not installed; the first vault and refresh are skipped")
	}

	created, err := EnsureAtlasVault(cfg)
	if err != nil {
		return 1, err
	}
	c.Step(ternary(created, console.OK, console.Skip), "atlas vault", ternary(created, home.Display(cfg.AtlasVault), "already present"))

	if firstPath != "" && prod != nil {
		if err := vaults.Create(prod, firstPath, c, false); err != nil {
			return 1, err
		}
		node, err := vaults.Register(cfg, firstPath, vaults.RegisterOptions{
			Purpose: "Created by claude-atlas setup to verify the installation.",
		})
		if err != nil {
			return 1, err
		}
		c.Step(console.OK, "first vault", fmt.Sprintf("%s → node %s", home.Display(firstPath), node.Rel))
	}
	if prod != nil {
		page, _, err := refresh.Run(cfg, prod, time.Now())
		if err != nil {
			return 1, err
		}
		c.Step(console.OK, "refreshed", home.Display(page))
	}

	c.Say("")
	c.Say("Setup complete.")
	c.Say("")
	c.Say("  Atlas        %s", home.Display(cfg.AtlasVault))
	if firstPath != "" && prod != nil {
		c.Say("  First vault  %s", home.Display(firstPath))
	}
	c.Say("")
	c.Say("Open each in Obsidian with \"Open folder as vault\". In the macOS file dialog,")
	c.Say("press Cmd+Shift+. to show hidden folders such as %s.", home.Display(h.Root))
	c.Say("")
	c.Say("Next:")
	c.Say("  claude-atlas vault new <name>    create another vault")
	c.Say("  claude-atlas vault add <path>    register an existing claude-obsidian vault")
	c.Say("  claude-atlas refresh             rebuild Atlas.md from every vault")
	c.Say("  claude-atlas info                show every path the atlas uses")
	if prod == nil {
		c.Say("")
		c.Say("claude-obsidian is not installed. Install it, then run setup again:")
		for _, cmd := range claudecode.Commands(cfg.ClaudeObsidian.Marketplace, cfg.ClaudeObsidian.Plugin) {
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
