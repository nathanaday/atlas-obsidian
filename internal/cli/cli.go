// Package cli parses arguments and dispatches subcommands.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/claude-atlas/internal/claudecode"
	"github.com/nathanaday/claude-atlas/internal/console"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/obsidian"
	"github.com/nathanaday/claude-atlas/internal/pages"
	"github.com/nathanaday/claude-atlas/internal/product"
	"github.com/nathanaday/claude-atlas/internal/refresh"
	"github.com/nathanaday/claude-atlas/internal/tree"
	"github.com/nathanaday/claude-atlas/internal/tui"
	"github.com/nathanaday/claude-atlas/internal/vaults"
	"github.com/nathanaday/claude-atlas/internal/wizard"
)

// Version is set at build time with -ldflags "-X .../cli.Version=v1.2.3".
var Version = "dev"

const usage = `claude-atlas: one view across many claude-obsidian vaults.

Usage:
  claude-atlas [--home DIR] [-y] <command> [options]

Commands:
  setup                  install claude-obsidian, create the atlas, and your first vault
  new-vault              create a vault and its project page, step by step
  new-vault NAME         create a vault without prompts
  new-vault --from PATH  register a claude-obsidian vault that already exists
  view                   navigate the atlas as a tree; open a project for every detail
  manage-vaults          browse every project by category; rename, move, repoint, or remove
  open-vault [NAME]      open the atlas, or a project's vault, in Obsidian
  open-claude NAME       start Claude Code inside a project's vault
  list                   list every project
  refresh                read every vault and rewrite Overview.md
  info                   show every path and version the atlas uses
  doctor                 check the installation and every registered vault
  version                print the version

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
	case "new-vault":
		code, err = e.newVault(rest[1:])
	case "view":
		code, err = e.view(rest[1:])
	case "manage-vaults":
		code, err = e.manageVaults(rest[1:])
	case "open-vault":
		code, err = e.openVault(rest[1:])
	case "open-claude":
		code, err = e.openClaude(rest[1:])
	case "list":
		code, err = e.list(rest[1:])
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
func (e *env) refreshAll(cfg *home.Config, prod *product.Product) (string, *refresh.Result, error) {
	if err := pages.Write(cfg, e.home.Root, Version); err != nil {
		return "", nil, err
	}
	return refresh.Run(cfg, e.home.StateDir(), prod, time.Now())
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
	fs.StringVar(&opts.Category, "category", "", "directory under tree/ to file the project in, e.g. university/cs566")
	fs.StringVar(&opts.Purpose, "purpose", "", "one paragraph: why this vault exists")
	fs.StringVar(&opts.Priority, "priority", "normal", "high, normal, low, or someday")
	return opts
}

func (e *env) newVault(args []string) (int, error) {
	fs := newFlags("new-vault", e.stderr)
	opts := nodeFlags(fs)
	fs.StringVar(&opts.Name, "name", "", "display name (default: the vault's directory name)")
	from := fs.String("from", "", "register a claude-obsidian vault that already exists at this path")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	switch {
	case *from != "" && len(positional) == 0:
		return e.registerExisting(*from, *opts)
	case *from == "" && len(positional) == 0:
		return e.newVaultInteractive()
	case *from == "" && len(positional) == 1:
		return e.createVault(positional[0], *opts)
	}
	return 2, errors.New("usage: claude-atlas new-vault [NAME | --from PATH] [--name N] [--category DIR] [--purpose TEXT] [--priority P]")
}

func (e *env) createVault(arg string, opts vaults.RegisterOptions) (int, error) {
	cfg, prod, err := e.load()
	if err != nil {
		return 1, err
	}
	path, err := vaults.ResolveNewPath(arg, cfg.VaultsDir)
	if err != nil {
		return 1, err
	}
	if err := vaults.Create(prod, path, e.console, true); err != nil {
		return 1, err
	}
	return e.finishVault(cfg, prod, path, opts)
}

func (e *env) registerExisting(path string, opts vaults.RegisterOptions) (int, error) {
	cfg, prod, err := e.load()
	if err != nil {
		return 1, err
	}
	project, err := vaults.Register(cfg, path, opts)
	if err != nil {
		return 1, err
	}
	page, _, err := e.refreshAll(cfg, prod)
	if err != nil {
		return 1, err
	}
	e.console.Step(console.OK, "registered", fmt.Sprintf("%s → tree/%s.md", home.Display(project.VaultPath()), project.Rel))
	e.console.Step(console.OK, "refreshed", home.Display(page))
	return 0, nil
}

// finishVault registers a vault that was just created, refreshes, and reports.
func (e *env) finishVault(cfg *home.Config, prod *product.Product, path string, opts vaults.RegisterOptions) (int, error) {
	project, err := vaults.Register(cfg, path, opts)
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
	c.Step(console.OK, "registered", "tree/"+project.Rel+".md")
	c.Step(console.OK, "refreshed", home.Display(page))
	c.Say("")
	c.Say("  Open it in Obsidian with \"Open folder as vault\", or start working:")
	c.Say("  cd %s && claude    # then /claude-obsidian:wiki", home.Display(path))
	c.Say("")
	return 0, nil
}

// newVaultInteractive walks the user through name, category, and purpose, then creates the vault.
func (e *env) newVaultInteractive() (int, error) {
	if !e.console.Interactive() {
		return 2, errors.New("usage: claude-atlas new-vault NAME (the interactive screen needs a terminal)")
	}
	cfg, prod, err := e.load()
	if err != nil {
		return 1, err
	}
	choice, err := tui.RunAddVault(cfg.VaultsDir, tui.Categories(cfg.TreeRoot()))
	if err != nil {
		return 1, err
	}
	if choice == nil {
		return 1, vaults.ErrCancelled
	}
	if err := vaults.Create(prod, choice.Path, e.console, false); err != nil {
		return 1, err
	}
	return e.finishVault(cfg, prod, choice.Path, vaults.RegisterOptions{
		Name: choice.Name, Category: choice.Category, Purpose: choice.Purpose,
	})
}

func (e *env) view(args []string) (int, error) {
	if !e.console.Interactive() {
		return 2, errors.New("view is an interactive screen and needs a terminal")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	projects, _, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return 1, err
	}
	items := make([]tui.Item, 0, len(projects))
	for _, p := range projects {
		state, _ := tree.ReadState(e.home.StateDir(), p.Rel)
		items = append(items, tui.Item{Project: p, State: state})
	}
	opener := tui.Opener{
		Status:          obsidian.Status,
		Open:            obsidian.Open,
		RegisterAndOpen: obsidian.RegisterAndOpen,
		Claude:          func(vault string) (*exec.Cmd, error) { return claudecode.LaunchCommand(cfg.ClaudeCode, vault) },
	}
	if err := tui.RunView(items, opener); err != nil {
		return 1, err
	}
	return 0, nil
}

func (e *env) manageVaults(args []string) (int, error) {
	if !e.console.Interactive() {
		return 2, errors.New("manage-vaults is an interactive screen and needs a terminal")
	}
	cfg, prod, err := e.load()
	if err != nil {
		return 1, err
	}
	hooks := tui.Hooks{
		Load: func() ([]*tree.Project, error) {
			projects, _, err := tree.Walk(cfg.TreeRoot())
			return projects, err
		},
		Categories: func() []string { return tui.Categories(cfg.TreeRoot()) },
		State: func(rel string) *tree.State {
			state, err := tree.ReadState(e.home.StateDir(), rel)
			if err != nil {
				return nil
			}
			return state
		},
		Update: func(p *tree.Project, edit vaults.Edit) error { return vaults.Update(cfg, p, edit) },
		Unlink: vaults.Unlink,
	}
	changed, err := tui.RunManage(hooks)
	if err != nil {
		return 1, err
	}
	if !changed {
		return 0, nil
	}
	page, _, err := e.refreshAll(cfg, prod)
	if err != nil {
		return 1, err
	}
	e.console.Step(console.OK, "refreshed", home.Display(page))
	return 0, nil
}

// resolveVault turns an open-vault argument into a directory: nothing means the atlas,
// a project name or tree path means its vault, and anything else is taken as a path.
func resolveVault(cfg *home.Config, projects []*tree.Project, arg string) (string, string, error) {
	if arg == "" {
		return cfg.AtlasVault, "the atlas", nil
	}
	if p := tree.FindByRel(projects, arg); p != nil {
		return p.VaultPath(), p.Name, nil
	}
	path, err := filepath.Abs(home.Expand(arg))
	if err != nil {
		return "", "", err
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path, filepath.Base(path), nil
	}
	return "", "", fmt.Errorf("%q is neither a project nor a directory", arg)
}

func (e *env) openVault(args []string) (int, error) {
	fs := newFlags("open-vault", e.stderr)
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) > 1 {
		return 2, errors.New("usage: claude-atlas open-vault [NAME | PATH]")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	projects, _, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return 1, err
	}
	arg := ""
	if len(positional) == 1 {
		arg = positional[0]
	}
	vault, label, err := resolveVault(cfg, projects, arg)
	if err != nil {
		return 1, err
	}
	c := e.console
	registered, running, err := obsidian.Status(vault)
	if err != nil {
		c.Say("%v", err)
		c.Say("Open it by hand: in Obsidian choose \"Open folder as vault\" and pick %s.", home.Display(vault))
		obsidian.Reveal(vault)
		return 1, nil
	}
	if registered {
		if err := obsidian.Open(vault); err != nil {
			return 1, err
		}
		c.Step(console.OK, "opened", fmt.Sprintf("%s in Obsidian", label))
		return 0, nil
	}
	c.Say("Obsidian does not know %s yet (%s).", label, home.Display(vault))
	question := "Register it as a vault and open it?"
	if running {
		question = "Register it as a vault? Obsidian will quit and relaunch so it sees the new entry."
	}
	ok, err := c.Confirm(question, true)
	if err != nil {
		return 1, err
	}
	if !ok {
		c.Say("Open it by hand: in Obsidian choose \"Open folder as vault\" and pick %s.", home.Display(vault))
		obsidian.Reveal(vault)
		return 1, vaults.ErrCancelled
	}
	if err := obsidian.RegisterAndOpen(vault); err != nil {
		if errors.Is(err, obsidian.ErrManualRestart) {
			c.Say("%v", err)
			return 1, nil
		}
		return 1, err
	}
	c.Step(console.OK, "registered", home.Display(vault))
	if running {
		c.Step(console.OK, "restarted", "Obsidian")
	}
	c.Step(console.OK, "opened", fmt.Sprintf("%s in Obsidian", label))
	return 0, nil
}

// skillHint is printed before handing the terminal to Claude Code.
const skillHint = "claude-obsidian skills: /claude-obsidian:wiki  wiki-ingest  wiki-query  wiki-retrieve  wiki-lint  wiki-fold  canvas  save"

func (e *env) openClaude(args []string) (int, error) {
	if len(args) != 1 {
		return 2, errors.New("usage: claude-atlas open-claude NAME")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	projects, _, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return 1, err
	}
	project := tree.FindByRel(projects, args[0])
	if project == nil {
		return 1, fmt.Errorf("no project named %q; see `claude-atlas list`", args[0])
	}
	if !e.console.Interactive() {
		return 2, errors.New("open-claude starts an interactive Claude Code session and needs a terminal")
	}
	cmd, err := claudecode.LaunchCommand(cfg.ClaudeCode, project.VaultPath())
	if err != nil {
		return 1, err
	}
	e.console.Say("  %s", home.Display(project.VaultPath()))
	e.console.Say("  %s", skillHint)
	e.console.Say("")
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

func (e *env) list(args []string) (int, error) {
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	projects, problems, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return 1, err
	}
	if len(projects) == 0 && len(problems) == 0 {
		e.console.Say("no projects yet; run `claude-atlas new-vault`")
		return 0, nil
	}
	for _, p := range projects {
		heat := "?"
		if state, err := tree.ReadState(e.home.StateDir(), p.Rel); err == nil {
			heat = state.Heat
			if heat == "" {
				heat = "off"
			}
		}
		e.console.Say("  %-5s %-7s %-8s %-32s %s", heat, p.Priority, p.State, p.Rel, home.Display(p.VaultPath()))
	}
	for _, problem := range problems {
		e.console.Step(console.Fail, "tree/"+problem.Rel+".md", problem.Reason)
	}
	return 0, nil
}

func (e *env) refresh(args []string) (int, error) {
	cfg, prod, err := e.load()
	if err != nil {
		return 1, err
	}
	page, res, err := e.refreshAll(cfg, prod)
	if err != nil {
		return 1, err
	}
	for _, problem := range res.Problems {
		e.console.Step(console.Fail, "tree/"+problem.Rel+".md", problem.Reason)
	}
	for _, r := range res.Rows {
		if !r.State.VaultOK {
			e.console.Step(console.Fail, r.Project.Rel, r.State.VaultError)
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
		e.console.Step(console.OK, r.Project.Rel, fmt.Sprintf("%s, idle %sd", heat, days))
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
	row("state", home.Display(e.home.StateDir()))
	row("vaults dir", home.Display(cfg.VaultsDir))
	if prod, err := product.Locate(cfg.ClaudeObsidian); err != nil {
		row("claude-obsidian", err.Error())
	} else {
		row("claude-obsidian", fmt.Sprintf("v%s (tested with v%s)", prod.Version, product.TestedVersion))
		row("  cli", home.Display(filepath.Join(prod.Root, "scripts", "claude-obsidian.py")))
		row("  source", ternary(prod.Source == "plugin", "Claude Code plugin "+cfg.ClaudeObsidian.Plugin, "config path"))
	}
	row("claude config", home.Display(claudecode.ConfigDir()))
	projects, _, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return 1, err
	}
	row("projects", fmt.Sprintf("%d registered", len(projects)))
	for _, p := range projects {
		row("  "+p.Rel, home.Display(p.VaultPath()))
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
	projects, problems, err := tree.Walk(cfg.TreeRoot())
	if err != nil {
		return 1, err
	}
	c.Say("  %-16s %d registered", "projects", len(projects))
	for _, p := range projects {
		_, err := os.Stat(filepath.Join(p.VaultPath(), ".claude-obsidian.json"))
		reachable := err == nil
		ok = ok && reachable
		c.Say("    %-3s %-24s %s", ternary(reachable, "ok", "off"), p.Rel, home.Display(p.VaultPath()))
	}
	for _, problem := range problems {
		ok = false
		c.Say("    %-3s %-24s %s", "bad", "tree/"+problem.Rel+".md", problem.Reason)
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
