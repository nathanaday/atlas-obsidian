// Package cli parses arguments and dispatches subcommands.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/claude-atlas/internal/claudecode"
	"github.com/nathanaday/claude-atlas/internal/console"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/pages"
	"github.com/nathanaday/claude-atlas/internal/product"
	"github.com/nathanaday/claude-atlas/internal/refresh"
	"github.com/nathanaday/claude-atlas/internal/tree"
	"github.com/nathanaday/claude-atlas/internal/vaults"
	"github.com/nathanaday/claude-atlas/internal/wizard"
)

// Version is set at build time with -ldflags "-X .../cli.Version=v1.2.3".
var Version = "dev"

const usage = `claude-atlas: one view across many claude-obsidian vaults.

Usage:
  claude-atlas [--home DIR] [-y] <command> [options]

Commands:
  setup            install claude-obsidian, create the atlas, and your first vault
  vault new NAME   create a claude-obsidian vault and register it
  vault add PATH   register an existing claude-obsidian vault
  vault list       list registered vaults
  refresh          recompute every state.json and rewrite Overview.md
  info             show every path and version the atlas uses
  doctor           check the installation and every registered vault
  version          print the version

Global options:
  --home DIR       atlas home (default ~/.claude-atlas or $CLAUDE_ATLAS_HOME)
  -y, --yes        answer yes to every prompt
`

type env struct {
	home    home.Home
	console *console.Console
	stdout  io.Writer
	stderr  io.Writer
}

// Main runs the CLI and returns the exit code.
func Main(args []string) int {
	return run(args, os.Stdin, os.Stdout, os.Stderr, nil)
}

// run is Main with injectable streams; console may be nil to build one from stdin.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, c *console.Console) int {
	global := flag.NewFlagSet("claude-atlas", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	homeFlag := global.String("home", "", "")
	yes := global.Bool("yes", false, "")
	global.BoolVar(yes, "y", false, "")
	if err := global.Parse(args); err != nil {
		fmt.Fprint(stderr, usage)
		return 2
	}
	rest := global.Args()
	if len(rest) == 0 || rest[0] == "help" || rest[0] == "--help" || rest[0] == "-h" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if c == nil {
		c = console.New(*yes)
		c.Out = stdout
	} else {
		c.AssumeYes = c.AssumeYes || *yes
	}
	e := &env{home: home.Resolve(*homeFlag), console: c, stdout: stdout, stderr: stderr}

	var err error
	var code int
	switch rest[0] {
	case "setup":
		code, err = e.setup(rest[1:])
	case "vault":
		code, err = e.vault(rest[1:])
	case "refresh":
		code, err = e.refresh(rest[1:])
	case "info":
		code, err = e.info(rest[1:])
	case "doctor":
		code, err = e.doctor(rest[1:])
	case "version":
		fmt.Fprintln(stdout, Version)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", rest[0], usage)
		return 2
	}
	if err != nil {
		if errors.Is(err, vaults.ErrCancelled) {
			fmt.Fprintln(stderr, "cancelled")
			return 1
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// parse accepts flags before and after positional arguments, unlike flag.Parse.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

// refreshAll rewrites every derived page in the atlas vault.
func (e *env) refreshAll(cfg *home.Config, prod *product.Product) (string, []refresh.Row, error) {
	if err := pages.Write(cfg, e.home.ConfigPath(), Version); err != nil {
		return "", nil, err
	}
	return refresh.Run(cfg, prod, time.Now())
}

func (e *env) load() (*home.Config, *product.Product, error) {
	cfg, err := e.home.Load()
	if err != nil {
		return nil, nil, err
	}
	prod, err := product.Locate(cfg.ClaudeObsidian)
	if err != nil {
		return nil, nil, fmt.Errorf("%w; run `claude-atlas setup`", err)
	}
	return cfg, prod, nil
}

func (e *env) setup(args []string) (int, error) {
	fs := newFlags("setup", e.stderr)
	vaultsDir := fs.String("vaults-dir", "", "where new vaults are created (default ~/Documents/Vaults)")
	atlasVault := fs.String("atlas-vault", "", "where the atlas vault lives (default ~/Documents/Atlas)")
	first := fs.String("first-vault", "", "name or path of the first vault (default welcome)")
	productPath := fs.String("claude-obsidian", "", "use a claude-obsidian checkout at this path instead of the plugin")
	noPlugin := fs.Bool("no-plugin", false, "do not run `claude plugin`")
	if err := fs.Parse(args); err != nil {
		return 2, nil
	}
	opts := wizard.Options{Version: Version, VaultsDir: *vaultsDir, AtlasVault: *atlasVault, FirstVault: *first, ProductPath: *productPath, WithPlugin: !*noPlugin}
	return wizard.Run(e.home, e.console, opts)
}

func nodeFlags(fs *flag.FlagSet) *vaults.RegisterOptions {
	opts := &vaults.RegisterOptions{}
	fs.StringVar(&opts.Parent, "parent", "", "cluster path to place the node under, e.g. work")
	fs.StringVar(&opts.Purpose, "purpose", "", "one paragraph: why this vault exists")
	fs.StringVar(&opts.Priority, "priority", "normal", "high, normal, low, or someday")
	return opts
}

func (e *env) vault(args []string) (int, error) {
	if len(args) == 0 {
		fmt.Fprint(e.stderr, "usage: claude-atlas vault new|add|list\n")
		return 2, nil
	}
	switch args[0] {
	case "new":
		return e.vaultNew(args[1:])
	case "add":
		return e.vaultAdd(args[1:])
	case "list":
		return e.vaultList(args[1:])
	}
	fmt.Fprintf(e.stderr, "unknown vault command %q\n", args[0])
	return 2, nil
}

func (e *env) vaultNew(args []string) (int, error) {
	fs := newFlags("vault new", e.stderr)
	opts := nodeFlags(fs)
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) != 1 {
		return 2, errors.New("usage: claude-atlas vault new NAME [--parent P] [--purpose TEXT] [--priority P]")
	}
	cfg, prod, err := e.load()
	if err != nil {
		return 1, err
	}
	path, err := vaults.ResolveNewPath(positional[0], cfg.VaultsDir)
	if err != nil {
		return 1, err
	}
	if err := vaults.Create(prod, path, e.console, true); err != nil {
		return 1, err
	}
	node, err := vaults.Register(cfg, path, *opts)
	if err != nil {
		return 1, err
	}
	page, _, err := e.refreshAll(cfg, prod)
	if err != nil {
		return 1, err
	}
	c := e.console
	c.Say("")
	c.Step(console.OK, "created", home.Display(path))
	c.Step(console.OK, "registered", "node "+node.Rel)
	c.Step(console.OK, "refreshed", home.Display(page))
	c.Say("")
	c.Say("  Open it in Obsidian with \"Open folder as vault\", or start working:")
	c.Say("  cd %s && claude    # then /claude-obsidian:wiki", home.Display(path))
	c.Say("")
	return 0, nil
}

func (e *env) vaultAdd(args []string) (int, error) {
	fs := newFlags("vault add", e.stderr)
	opts := nodeFlags(fs)
	fs.StringVar(&opts.Name, "name", "", "display name (default: directory name)")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) != 1 {
		return 2, errors.New("usage: claude-atlas vault add PATH [--name N] [--parent P] [--purpose TEXT] [--priority P]")
	}
	cfg, prod, err := e.load()
	if err != nil {
		return 1, err
	}
	node, err := vaults.Register(cfg, positional[0], *opts)
	if err != nil {
		return 1, err
	}
	page, _, err := e.refreshAll(cfg, prod)
	if err != nil {
		return 1, err
	}
	e.console.Step(console.OK, "registered", fmt.Sprintf("%s → node %s", home.Display(node.VaultPath()), node.Rel))
	e.console.Step(console.OK, "refreshed", home.Display(page))
	return 0, nil
}

func (e *env) vaultList(args []string) (int, error) {
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	nodes, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return 1, err
	}
	if len(nodes) == 0 {
		e.console.Say("no vaults registered; run `claude-atlas vault new <name>`")
		return 0, nil
	}
	for _, n := range nodes {
		heat := "?"
		if state, err := tree.ReadState(n.Dir); err == nil {
			heat = state.Heat
			if heat == "" {
				heat = "off"
			}
		}
		target := "(cluster)"
		if v := n.VaultPath(); v != "" {
			target = home.Display(v)
		}
		e.console.Say("  %-5s %-7s %-8s %-32s %s", heat, n.Priority, n.State, n.Rel, target)
	}
	return 0, nil
}

func (e *env) refresh(args []string) (int, error) {
	cfg, prod, err := e.load()
	if err != nil {
		return 1, err
	}
	page, rows, err := e.refreshAll(cfg, prod)
	if err != nil {
		return 1, err
	}
	for _, r := range rows {
		if r.Node.IsLeaf() && !r.State.VaultOK {
			e.console.Step(console.Fail, r.Node.Rel, r.State.VaultError)
			continue
		}
		heat := r.State.Heat
		if heat == "" {
			heat = "-"
		}
		days := "?"
		if r.State.DaysIdle != nil {
			days = fmt.Sprint(*r.State.DaysIdle)
		}
		e.console.Step(console.OK, r.Node.Rel, fmt.Sprintf("%s, idle %sd", heat, days))
	}
	e.console.Say("  wrote %s", home.Display(page))
	return 0, nil
}

func (e *env) info(args []string) (int, error) {
	c := e.console
	row := func(label, value string) { c.Say("  %-18s %s", label, value) }
	row("claude-atlas", Version)
	row("home", home.Display(e.home.Root))
	row("config", home.Display(e.home.ConfigPath()))
	if !e.home.Exists() {
		row("status", "not set up; run `claude-atlas setup`")
		return 0, nil
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	row("atlas vault", home.Display(cfg.AtlasVault))
	row("overview", home.Display(filepath.Join(cfg.AtlasVault, "Overview.md")))
	row("tree", home.Display(cfg.TreeRoot()))
	row("vaults dir", home.Display(cfg.VaultsDir))
	if prod, err := product.Locate(cfg.ClaudeObsidian); err != nil {
		row("claude-obsidian", err.Error())
	} else {
		row("claude-obsidian", fmt.Sprintf("v%s (tested with v%s)", prod.Version, product.TestedVersion))
		row("  cli", home.Display(filepath.Join(prod.Root, "scripts", "claude-obsidian.py")))
		row("  source", ternary(prod.Source == "plugin", "Claude Code plugin "+cfg.ClaudeObsidian.Plugin, "config path"))
	}
	row("claude config", home.Display(claudecode.ConfigDir()))
	nodes, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return 1, err
	}
	leaves := tree.Leaves(nodes)
	row("vaults", fmt.Sprintf("%d registered", len(leaves)))
	for _, n := range leaves {
		row("  "+n.Rel, home.Display(n.VaultPath()))
	}
	return 0, nil
}

func (e *env) doctor(args []string) (int, error) {
	c := e.console
	c.Say("  %-16s %s  %s", "home", home.Display(e.home.Root), ternary(e.home.Exists(), "ok", "missing"))
	if !e.home.Exists() {
		return 1, nil
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	ok := true
	prod, err := product.Locate(cfg.ClaudeObsidian)
	if err != nil {
		ok = false
		c.Say("  %-16s %s", "claude-obsidian", err.Error())
	} else {
		note := ""
		if !prod.Tested() {
			note = fmt.Sprintf("  (atlas was tested with v%s)", product.TestedVersion)
		}
		c.Say("  %-16s v%s at %s (%s)%s", "claude-obsidian", prod.Version, home.Display(prod.Root), prod.Source, note)
	}
	if claudecode.CLI() == "" {
		c.Say("  %-16s `claude` is not on PATH", "Claude Code")
	} else {
		c.Say("  %-16s on PATH", "Claude Code")
	}
	_, statErr := os.Stat(strings.TrimSuffix(cfg.AtlasVault, "/") + "/.obsidian")
	c.Say("  %-16s %s  %s", "atlas vault", home.Display(cfg.AtlasVault), ternary(statErr == nil, "ok", "missing"))
	c.Say("  %-16s %s", "vaults dir", home.Display(cfg.VaultsDir))
	nodes, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return 1, err
	}
	leaves := tree.Leaves(nodes)
	c.Say("  %-16s %d registered", "vaults", len(leaves))
	for _, n := range leaves {
		_, err := os.Stat(n.VaultPath() + "/.claude-obsidian.json")
		reachable := err == nil
		ok = ok && reachable
		c.Say("    %-3s %-24s %s", ternary(reachable, "ok", "off"), n.Rel, home.Display(n.VaultPath()))
	}
	if !ok {
		return 1, nil
	}
	return 0, nil
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
