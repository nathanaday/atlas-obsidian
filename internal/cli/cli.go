// Package cli parses arguments and dispatches subcommands.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/actions"
	"github.com/nathanaday/atlas-obsidian/internal/capture"
	"github.com/nathanaday/atlas-obsidian/internal/claudecode"
	"github.com/nathanaday/atlas-obsidian/internal/codex"
	"github.com/nathanaday/atlas-obsidian/internal/console"
	"github.com/nathanaday/atlas-obsidian/internal/describe"
	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/hooks"
	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/manage"
	"github.com/nathanaday/atlas-obsidian/internal/mcpserver"
	"github.com/nathanaday/atlas-obsidian/internal/obsidian"
	"github.com/nathanaday/atlas-obsidian/internal/place"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/refresh"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
	"github.com/nathanaday/atlas-obsidian/internal/tui"
	"github.com/nathanaday/atlas-obsidian/internal/txn"
	"github.com/nathanaday/atlas-obsidian/internal/wizard"
)

// Version is set at build time with -ldflags "-X .../cli.Version=v1.2.3".
var Version = "dev"

const usage = `atlas-obsidian: a wiki and the state of the work, in every project.

Usage:
  atlas-obsidian                       open the view: every project on one screen
  atlas-obsidian [--home DIR] [-y] <command> [options]

Getting started:
  setup                     install the plugin; --agent claude (default) or codex
  init [PATH]               make the current folder (or PATH) a project: an atlas/<name>/ folder
                            inside your work, with its wiki and its threads
                            --name N, --description TEXT, --mode generic|lyt;
                            a folder in no git repository becomes one unless --no-git

Projects (PROJECT is a name, a path, or nothing for the project you are in):
  list                      every project
  show NAME                 everything the atlas knows about one
  edit NAME                 change it: --name N, --description TEXT, --mode generic|lyt
  describe PROJECT          stage a snapshot of the work; the describe skill writes its page
  forget PROJECT            drop a project from the atlas; its atlas/<name>/ folder stays
  open-ide NAME            open the work folder in the preferred IDE
  open-agent NAME          start the preferred harness in the work folder
  open-vault [PROJECT]      open the project's folder in Obsidian
  open-claude NAME          start Claude Code in the work; --thread ID continues a thread
  open-codex NAME           start Codex in the work; --thread ID continues a thread

Threads (ID is a thread's id or title):
  threads [PROJECT]         list open threads by stage; --all adds the closed ones, --stage S, --json
  thread PROJECT ACTION ... new TEXT... [--title T] [--priority P] [--phase NAME]: a card and a stub
                            show ID [--json]: its state and the path of each document
                            file ID STAGE: file the spec, plan, or receipt, which moves the thread there;
                              the text comes from --text T, --file PATH, or stdin; a receipt takes --outcome
                            close ID TEXT... [--killed]: file the receipt; completed unless --killed
                            set ID [--title T] [--priority P] [--phase NAME] [--blocked TEXT]
                            reopen ID: delete the receipt
  phase PROJECT ACTION ...  create TITLE [--goal TEXT] [--order N], rename TITLE --to NEW,
                            reorder TITLE --order N, remove TITLE

The wiki (PROJECT is a name, a path, or nothing for the project you are in):
  ingest PROJECT [PATH...]  stage new files into the inbox, then ingest them
  lint [PROJECT]            run the wiki health check
  stub PROJECT [TITLE...]   create seed pages for the pages your links name but nobody has written
  history [PROJECT]         list operations, newest first
  undo PROJECT OPERATION    take back one operation
  recover [PROJECT]         restore the wiki after an interrupted operation
  apply PROJECT PLAN.json   apply a plan file, for scripts

Across the atlas:
  view                      the interactive screen; the same as no command at all
  refresh                   read everything again and rewrite the registry
  config [KEY VALUE]        show or set new-days / preferred-harness / preferred-ide
  info                      show every path and version the atlas uses
  doctor                    check the installation and every project

Plugin:
  mcp                       serve the atlas tools over stdio; Claude Code runs this
  hook EVENT                run a plugin hook: session-start, guard, touched, stop
  version                   print the version

Global options:
  --home DIR       atlas home (default ~/.atlas-obsidian or $ATLAS_OBSIDIAN_HOME)
  -y, --yes        answer yes to every prompt
`

type env struct {
	home    home.Home
	console *console.Console
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

// Main runs the CLI and returns the exit code.
func Main(args []string) int {
	return run(args, os.Stdin, os.Stdout, os.Stderr, nil)
}

// run is Main with injectable streams; console may be nil to build one from stdin.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, c *console.Console) int {
	global := flag.NewFlagSet("atlas-obsidian", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	homeFlag := global.String("home", "", "")
	yes := global.Bool("yes", false, "")
	global.BoolVar(yes, "y", false, "")
	if err := global.Parse(args); err != nil {
		fmt.Fprint(stderr, usage)
		return 2
	}
	rest := global.Args()
	if len(rest) > 0 && (rest[0] == "help" || rest[0] == "--help" || rest[0] == "-h") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if c == nil {
		c = console.New(*yes)
		c.Out = stdout
	} else {
		c.AssumeYes = c.AssumeYes || *yes
	}
	e := &env{home: home.Resolve(*homeFlag), console: c, stdin: stdin, stdout: stdout, stderr: stderr}
	if len(rest) == 0 {
		switch {
		case !c.Interactive():
			fmt.Fprint(stdout, usage)
			return 0
		case !e.home.Exists():
			fmt.Fprint(stdout, usage)
			fmt.Fprintf(stdout, "\nNo atlas yet; run `atlas-obsidian setup`, then `atlas-obsidian init` in your work.\n")
			return 0
		}
		rest = []string{"view"}
	}

	var err error
	var code int
	switch rest[0] {
	case "setup":
		code, err = e.setup(rest[1:])
	case "init":
		code, err = e.initProject(rest[1:])
	case "describe":
		code, err = e.describe(rest[1:])
	case "forget":
		code, err = e.forget(rest[1:])
	case "threads":
		code, err = e.threads(rest[1:])
	case "thread":
		code, err = e.thread(rest[1:])
	case "phase":
		code, err = e.phase(rest[1:])
	case "view":
		code, err = e.view(rest[1:])
	case "open-vault":
		code, err = e.openVault(rest[1:])
	case "open-ide":
		code, err = e.openIDE(rest[1:])
	case "open-agent":
		code, err = e.openPreferredAgent(rest[1:])
	case "open-claude":
		code, err = e.openClaude(rest[1:])
	case "open-codex":
		code, err = e.openAgent(rest[1:], "codex")
	case "ingest":
		code, err = e.ingest(rest[1:])
	case "list":
		code, err = e.list(rest[1:])
	case "show":
		code, err = e.show(rest[1:])
	case "edit":
		code, err = e.edit(rest[1:])
	case "refresh":
		code, err = e.refresh(rest[1:])
	case "lint":
		code, err = e.lint(rest[1:])
	case "stub":
		code, err = e.stub(rest[1:])
	case "history":
		code, err = e.history(rest[1:])
	case "undo":
		code, err = e.undo(rest[1:])
	case "recover":
		code, err = e.recover(rest[1:])
	case "config":
		code, err = e.config(rest[1:])
	case "apply":
		code, err = e.apply(rest[1:])
	case "mcp":
		code, err = e.mcp(rest[1:])
	case "hook":
		code, err = e.hook(rest[1:])
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
		if errors.Is(err, manage.ErrCancelled) {
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

// setFlags names the flags the user actually gave, so "" can mean "clear this field".
func setFlags(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}

func first(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// entry resolves one project by name, id, or path. Every command scans afresh: the
// registry file is derived state, and a stale one must never decide what a command acts on.
func (e *env) entry(cfg *home.Config, arg string) (registry.Entry, error) {
	ix, err := registry.Scan(cfg)
	if err != nil {
		return registry.Entry{}, err
	}
	return findEntry(ix, arg)
}

// anyEntry resolves one entry, including one the atlas could not read. Only forget acts on
// those; every other command wants entry.
func (e *env) anyEntry(cfg *home.Config, arg string) (registry.Entry, error) {
	ix, err := registry.Scan(cfg)
	if err != nil {
		return registry.Entry{}, err
	}
	return findAnyEntry(ix, arg)
}

// findEntry is entry over an index the caller already has. An entry the scan could not
// read carries its own reason, which says more than "no such".
func findEntry(ix *registry.Index, arg string) (registry.Entry, error) {
	found, err := ix.Find(arg)
	if err == nil {
		return *found, nil
	}
	if !errors.Is(err, registry.ErrNotFound) {
		return registry.Entry{}, err
	}
	if bad := badEntry(ix, arg); bad != nil {
		return registry.Entry{}, fmt.Errorf("%s: %s", home.Display(bad.Path), bad.Error)
	}
	return registry.Entry{}, fmt.Errorf("no project named %q; see `atlas-obsidian list`", arg)
}

// findAnyEntry resolves an entry by name, id, or path, and one the atlas could not read
// by its path or its folder's name.
func findAnyEntry(ix *registry.Index, arg string) (registry.Entry, error) {
	found, err := ix.Find(arg)
	if err == nil {
		return *found, nil
	}
	if !errors.Is(err, registry.ErrNotFound) {
		return registry.Entry{}, err
	}
	if bad := badEntry(ix, arg); bad != nil {
		return *bad, nil
	}
	return registry.Entry{}, fmt.Errorf("nothing named %q in the atlas; see `atlas-obsidian list`", arg)
}

// uncoveredProblems lists the scan's problems that no entry carries, so a command names
// each one once.
func uncoveredProblems(ix *registry.Index) []registry.Problem {
	covered := map[string]bool{}
	for _, en := range ix.Entries {
		covered[en.Path] = true
	}
	var out []registry.Problem
	for _, p := range ix.Problems {
		if !covered[p.Path] {
			out = append(out, p)
		}
	}
	return out
}

// badEntry finds an entry the scan could not read, by its path or its folder's name.
// Such an entry has no name of its own.
func badEntry(ix *registry.Index, arg string) *registry.Entry {
	abs, err := filepath.Abs(home.Expand(arg))
	for i := range ix.Entries {
		e := &ix.Entries[i]
		if e.Error == "" {
			continue
		}
		if (err == nil && e.Path == abs) || strings.EqualFold(filepath.Base(e.Path), arg) {
			return e
		}
	}
	return nil
}

// refreshAll rebuilds the registry from a scan.
func (e *env) refreshAll(cfg *home.Config) ([]registry.Entry, *registry.Index, error) {
	return refresh.All(e.home, cfg, time.Now())
}

// registryEntries reads the registry the last refresh wrote, writing one first when no
// refresh has run yet.
func (e *env) registryEntries(cfg *home.Config) ([]registry.Entry, error) {
	return refresh.Entries(e.home, cfg, time.Now())
}

// refreshed is what a command says after rewriting the registry.
func refreshed(entries []registry.Entry) string {
	n := 0
	for _, en := range entries {
		if en.Error == "" {
			n++
		}
	}
	return fmt.Sprintf("%d project%s", n, plural(n))
}

// entryName is the name to show: an entry the scan could not read has only its folder.
func entryName(en registry.Entry) string {
	if en.Error != "" {
		return filepath.Base(en.Path)
	}
	return en.Name
}

// cwd is the working directory, or "" when it cannot be read.
func cwd() string { return place.Cwd() }

// projectArg resolves a project for the thread and project commands: nothing means the
// project at or above the current directory; a path opens that folder; a name goes
// through the registry. It returns the registry entry when the atlas knows it.
func (e *env) projectArg(arg string) (*project.Project, *registry.Entry, error) {
	var work string
	switch {
	case arg == "" || arg == ".":
		work = project.FindAbove(cwd())
		if work == "" {
			return nil, nil, fmt.Errorf("%w: the current directory is not inside a project; name one or run `atlas-obsidian init`", project.ErrNotProject)
		}
	case strings.ContainsAny(arg, `/\`) || strings.HasPrefix(arg, "~"):
		abs, err := filepath.Abs(home.Expand(arg))
		if err != nil {
			return nil, nil, err
		}
		work = project.FindAbove(abs)
		if work == "" {
			return nil, nil, fmt.Errorf("%w: %s", project.ErrNotProject, abs)
		}
	}
	var entry *registry.Entry
	if cfg, err := e.home.Load(); err == nil {
		if ix, err := registry.Scan(cfg); err == nil {
			if work == "" {
				found, err := ix.Find(arg)
				if err != nil {
					if errors.Is(err, registry.ErrNotFound) {
						return nil, nil, fmt.Errorf("no project named %q; see `atlas-obsidian list`", arg)
					}
					return nil, nil, err
				}
				work = found.Path
			}
			if found := ix.ByPath(work); found != nil && found.Error == "" {
				entry = found
			}
		}
	}
	if work == "" {
		return nil, nil, fmt.Errorf("no atlas config; name the project by its path, or run `atlas-obsidian setup`")
	}
	p, err := project.Open(work)
	if err != nil {
		return nil, nil, err
	}
	return p, entry, nil
}

// vaultArg resolves the project a wiki command acts on: a name, a path, or the current
// directory, from anywhere inside the work. It does not need the atlas to be set up when a
// path is given.
func (e *env) vaultArg(arg string) (*project.Project, error) {
	if arg == "" || arg == "." {
		work := project.FindAbove(cwd())
		if work == "" {
			return nil, fmt.Errorf("%w: the current directory is not inside one; name a project or give a path", project.ErrNotProject)
		}
		return project.Open(work)
	}
	if strings.ContainsAny(arg, `/\`) || strings.HasPrefix(arg, "~") {
		abs, err := filepath.Abs(home.Expand(arg))
		if err != nil {
			return nil, err
		}
		work := project.FindAbove(abs)
		if work == "" {
			return nil, fmt.Errorf("%w: %s", project.ErrNotProject, home.Display(abs))
		}
		return project.Open(work)
	}
	cfg, err := e.home.Load()
	if err != nil {
		return nil, err
	}
	en, err := e.entry(cfg, arg)
	if err != nil {
		return nil, err
	}
	return project.Open(en.Path)
}

func (e *env) setup(args []string) (int, error) {
	fs := newFlags("setup", e.stderr)
	source := fs.String("plugin-source", "", "install the plugin from this marketplace source, e.g. a local checkout")
	noPlugin := fs.Bool("no-plugin", false, "do not install an agent plugin")
	agent := fs.String("agent", "claude", "plugin host: claude or codex")
	if err := fs.Parse(args); err != nil {
		return 2, nil
	}
	if *agent != "claude" && *agent != "codex" {
		return 2, fmt.Errorf("unknown agent %q; choose claude or codex", *agent)
	}
	opts := wizard.Options{Version: Version, PluginSource: *source, WithPlugin: !*noPlugin, Agent: *agent}
	return wizard.Run(e.home, e.console, opts)
}

func (e *env) initProject(args []string) (int, error) {
	fs := newFlags("init", e.stderr)
	name := fs.String("name", "", "the project's name (default: the folder's name)")
	description := fs.String("description", "", "what the work is and what its wiki should remember")
	mode := fs.String("mode", "", "the filing mode for new wiki pages: generic (default) or lyt")
	noGit := fs.Bool("no-git", false, "leave a folder that is in no git repository without one")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) > 1 {
		return 2, errors.New("usage: atlas-obsidian init [PATH] [--name N] [--description TEXT] [--mode generic|lyt] [--no-git]")
	}
	work := cwd()
	if len(positional) == 1 {
		if work, err = filepath.Abs(home.Expand(positional[0])); err != nil {
			return 1, err
		}
	}
	if err := project.CheckNew(work); err != nil {
		return 1, err
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	c := e.console
	set := setFlags(fs)
	if *name == "" {
		*name = filepath.Base(work)
	}
	if c.Interactive() {
		if !set["name"] {
			*name = c.Ask("Name", *name)
		}
		if !set["description"] {
			*description = c.Ask("Description: what the work is, and what its wiki should remember", "")
		}
	}
	acts := actions.Bind(e.home, cfg, c)
	made, err := acts.InitProject(actions.InitProject{Work: work, Name: *name, Description: *description, Mode: *mode, NoGit: *noGit})
	if err != nil {
		return 1, err
	}
	p := made.Project
	c.Say("")
	c.Step(console.OK, "project", fmt.Sprintf("%s at %s", p.Name(), home.Display(p.Root)))
	c.Step(console.OK, "wrote", p.Rel()+"/: "+strings.Join(made.Written, ", "))
	switch made.Git {
	case project.GitCreated:
		c.Step(console.OK, "git", "initialized a repository on main")
	case project.GitExisting:
		c.Step(console.Skip, "git", "a repository already")
	case project.GitEnclosed:
		c.Step(console.Skip, "git", "inside another repository, which keeps its history")
	case project.GitSkipped:
		c.Step(console.Skip, "git", "none; --no-git, so no operation can run until the work is a repository")
	}
	if made.Commit != "" {
		c.Step(console.OK, "committed", made.Commit[:12])
	}
	c.Step(console.OK, "registered", "in "+home.Display(e.home.ConfigPath()))
	entries, ix, err := e.refreshAll(cfg)
	if err != nil {
		return 1, err
	}
	c.Step(console.OK, "refreshed", refreshed(entries))
	ref, here := ".", work == cwd()
	if !here {
		ref = entryArg(ix, p.Root)
	}
	next := [][2]string{
		{"atlas-obsidian describe " + ref, "a page in its wiki that says what the work is"},
		{"atlas-obsidian thread " + ref + ` new "..."`, "open a thread, or drop a note in " + p.Rel() + "/" + project.InboxDir + "/"},
	}
	if here {
		next = append(next, [2]string{"claude", "Claude Code here sees the project, its wiki, and its threads"})
	} else {
		next = append(next, [2]string{"atlas-obsidian open-claude " + ref, "Claude Code in the project"})
	}
	next = append(next, [2]string{"atlas-obsidian open-vault " + ref, "open " + p.Rel() + "/ in Obsidian"})
	c.Say("")
	c.Say("  Next:")
	sayCommands(c, next)
	c.Say("")
	return 0, nil
}

// sayCommands prints commands with their comments in one column.
func sayCommands(c *console.Console, lines [][2]string) {
	width := 0
	for _, l := range lines {
		width = max(width, len(l[0]))
	}
	for _, l := range lines {
		c.Say("  %-*s  # %s", width, l[0], l[1])
	}
}

// entryArg is the shortest command argument that names the entry at path: its name
// when the name finds it, else its path. It comes quoted for a shell when it must be.
func entryArg(ix *registry.Index, path string) string {
	en := ix.ByPath(path)
	if en != nil && en.Error == "" {
		if found, err := ix.Find(en.Name); err == nil && found.Path == path {
			return shellArg(en.Name)
		}
	}
	display := home.Display(path)
	if rest, ok := strings.CutPrefix(display, "~/"); ok {
		return "~/" + shellArg(rest)
	}
	return shellArg(display)
}

// shellArg quotes s for a POSIX shell unless every byte is safe bare.
func shellArg(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-+/=:@,%") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (e *env) forget(args []string) (int, error) {
	if len(args) != 1 {
		return 2, errors.New("usage: atlas-obsidian forget PROJECT")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	entry, err := e.anyEntry(cfg, args[0])
	if err != nil {
		return 1, err
	}
	name, where := entryName(entry), home.Display(entry.Path)
	gone := entry.Reason == registry.ReasonMissing
	question := fmt.Sprintf("Forget %s? The folder %s and everything in it stay.", name, where)
	if gone {
		question = fmt.Sprintf("Forget %s? Its folder %s is already gone.", name, where)
	}
	ok, err := e.console.Confirm(question, false)
	if err != nil {
		return 1, err
	}
	if !ok {
		return 1, manage.ErrCancelled
	}
	if err := manage.Forget(e.home, cfg, entry.Path); err != nil {
		return 1, err
	}
	entries, _, err := e.refreshAll(cfg)
	if err != nil {
		return 1, err
	}
	e.console.Step(console.OK, "forgot", name)
	e.console.Step(console.OK, "refreshed", refreshed(entries))
	return 0, nil
}

func (e *env) describe(args []string) (int, error) {
	fs := newFlags("describe", e.stderr)
	noClaude := fs.Bool("no-claude", false, "stage the snapshot but do not start Claude Code")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) > 1 {
		return 2, errors.New("usage: atlas-obsidian describe [PROJECT] [--no-claude]")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	p, entry, err := e.projectArg(first(positional))
	if err != nil {
		return 1, err
	}
	if entry == nil {
		return 1, fmt.Errorf("%s is not in the atlas; run `atlas-obsidian init` there", p.Name())
	}
	snap, err := actions.Bind(e.home, cfg, e.console).StageProject(*entry)
	if err != nil {
		return 1, err
	}
	c := e.console
	if snap.Described != nil {
		c.Say("  %-10s %s", "page", snap.Described.Summary())
	}
	at := snap.Commit
	if len(at) > 7 {
		at = at[:7]
	}
	if at == "" {
		at = "no commit"
	}
	switch {
	case snap.New:
		c.Step(console.OK, "staged", fmt.Sprintf("%s: %s at %s, in %s/%s/", strings.TrimPrefix(snap.To, "inbox/"), p.Name(), at, p.Rel(), project.InboxDir))
	default:
		c.Say("  %-10s %s already waits or was captured", "unchanged", strings.TrimPrefix(snap.To, "inbox/"))
	}
	return e.offerClaude(cfg, p.Root, *noClaude, claudecode.DescribePrompt)
}

// offerClaude ends a staging command: it names the skill to run next, and in a terminal
// offers to start Claude Code on it in dir.
func (e *env) offerClaude(cfg *home.Config, dir string, noClaude bool, skill string) (int, error) {
	c := e.console
	if noClaude || !c.Interactive() {
		c.Say("  Next: start Claude Code in %s, then %s", home.Display(dir), skill)
		return 0, nil
	}
	c.Say("  %s", trustNote)
	ok, err := c.Confirm("Start Claude Code now and run "+skill+"?", true)
	if err != nil {
		return 1, err
	}
	if !ok {
		c.Say("  Next: start Claude Code in %s, then %s", home.Display(dir), skill)
		return 0, nil
	}
	cmd, err := claudecode.LaunchCommand(cfg.ClaudeCode, dir, skill)
	if err != nil {
		return 1, err
	}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

func (e *env) threads(args []string) (int, error) {
	fs := newFlags("threads", e.stderr)
	all := fs.Bool("all", false, "include the closed threads")
	stage := fs.String("stage", "", "only threads at this stage: stub, spec, plan, or receipt")
	asJSON := fs.Bool("json", false, "print the board as JSON")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) > 1 {
		return 2, errors.New("usage: atlas-obsidian threads [PROJECT] [--all] [--stage S] [--json]")
	}
	if *stage != "" && threads.Dir(*stage) == "" {
		return 2, fmt.Errorf("stage must be one of %s", strings.Join(threads.Stages, ", "))
	}
	now := time.Now()
	var projects []*project.Project
	// every means no project was named and the current folder is in none: list them all.
	every := false
	p, _, err := e.projectArg(first(positional))
	switch {
	case err == nil:
		projects = append(projects, p)
	case len(positional) == 0 && errors.Is(err, project.ErrNotProject):
		every = true
		cfg, err := e.home.Load()
		if err != nil {
			return 1, err
		}
		ix, err := registry.Scan(cfg)
		if err != nil {
			return 1, err
		}
		for _, en := range ix.Projects() {
			if p, err := project.Open(en.Path); err == nil {
				projects = append(projects, p)
			}
		}
	default:
		return 1, err
	}
	shown := 0
	boards := map[string]*threads.Board{}
	for _, p := range projects {
		board, err := threads.Load(p)
		if err != nil {
			return 1, err
		}
		if *asJSON {
			boards[p.Name()] = board
			continue
		}
		list := board.Open()
		if *all || *stage == threads.Receipt {
			list = append(list, board.Closed()...)
		}
		if every && len(list) == 0 {
			continue
		}
		shown += len(list)
		if every {
			e.console.Say("%s", p.Name())
		}
		e.printThreads(list, now, *stage)
		if notes := threads.Notes(p); len(notes) > 0 {
			e.console.Say("  %d note%s waiting in %s/%s/: %s", len(notes), plural(len(notes)), p.Rel(), project.InboxDir, strings.Join(notes, ", "))
		}
		for _, pr := range board.Problems {
			e.console.Step(console.Fail, pr.Path, pr.Reason)
		}
	}
	if *asJSON {
		return 0, e.printJSON(boards)
	}
	if every && shown == 0 {
		e.console.Say("no open threads in any project; open one with `atlas-obsidian thread PROJECT new \"...\"`")
	}
	return 0, nil
}

func (e *env) printJSON(v any) error {
	enc := json.NewEncoder(e.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printThreads lists threads under a heading per stage, the furthest stage first.
func (e *env) printThreads(list []threads.Thread, now time.Time, only string) {
	if len(list) == 0 {
		e.console.Say("  no open threads")
		return
	}
	last := ""
	for _, t := range list {
		if only != "" && t.Stage != only {
			continue
		}
		if t.Stage != last {
			e.console.Say("  %s", t.Stage)
			last = t.Stage
		}
		notes := ""
		if t.Closed() {
			notes += "  " + t.Outcome
		}
		if t.Phase != "" {
			notes += "  " + t.Phase
		}
		if t.Blocked != "" {
			notes += "  blocked: " + t.Blocked
		}
		if threads.Stale(t, now) {
			notes += "  stale"
		}
		e.console.Say("    %-8s %-44s %s  %s%s", t.Priority, t.Title, t.ID, dash(t.Updated), notes)
	}
}

// threadText is a document's text: the flag, the file, the words on the command line, or
// stdin when it is not a terminal.
func (e *env) threadText(text, file string, words []string) (string, error) {
	switch {
	case text != "":
		return text, nil
	case file == "-":
	case file != "":
		data, err := os.ReadFile(home.Expand(file))
		return string(data), err
	case len(words) > 0:
		return strings.Join(words, " "), nil
	case e.console.Interactive():
		return "", nil
	}
	data, err := io.ReadAll(e.stdin)
	return string(data), err
}

const threadUsage = "usage: atlas-obsidian thread PROJECT new TEXT... | show ID | file ID STAGE | close ID TEXT... | set ID | reopen ID"

func (e *env) thread(args []string) (int, error) {
	fs := newFlags("thread", e.stderr)
	title := fs.String("title", "", "the thread's title; on new, taken from the text when omitted")
	priority := fs.String("priority", "", strings.Join(threads.Priorities, ", "))
	phase := fs.String("phase", "", "the phase the thread belongs to; \"\" clears it")
	blocked := fs.String("blocked", "", "what the thread waits on; \"\" unblocks it")
	text := fs.String("text", "", "the document's text")
	file := fs.String("file", "", "read the document's text from this file; - is stdin")
	outcome := fs.String("outcome", "", "with file ID receipt: completed or killed")
	killed := fs.Bool("killed", false, "with close: the thread was killed, not completed")
	asJSON := fs.Bool("json", false, "with show: print the thread as JSON")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) < 2 {
		return 2, errors.New(threadUsage)
	}
	p, _, err := e.projectArg(positional[0])
	if err != nil {
		return 1, err
	}
	action, rest := positional[1], positional[2:]
	if action != "new" && len(rest) == 0 {
		return 2, errors.New(threadUsage)
	}
	set := setFlags(fs)
	now := time.Now()
	var t *threads.Thread
	verb := ""
	switch action {
	case "new":
		body, err := e.threadText(*text, *file, rest)
		if err != nil {
			return 1, err
		}
		t, err = threads.Start(p, threads.New{Title: *title, Text: body, Priority: *priority, Phase: *phase}, now)
		if err != nil {
			return 1, err
		}
		verb = "opened"
	case "show":
		board, err := threads.Load(p)
		if err != nil {
			return 1, err
		}
		if t, err = board.Resolve(rest[0]); err != nil {
			return 1, err
		}
		if *asJSON {
			return 0, e.printJSON(t)
		}
		verb = "thread"
	case "file":
		if len(rest) != 2 {
			return 2, errors.New("usage: atlas-obsidian thread PROJECT file ID STAGE [--text T | --file PATH] [--outcome completed|killed]")
		}
		body, err := e.threadText(*text, *file, nil)
		if err != nil {
			return 1, err
		}
		if t, err = threads.File(p, rest[0], threads.Filing{Stage: rest[1], Text: body, Outcome: *outcome}, now); err != nil {
			return 1, err
		}
		verb = "filed"
	case "close":
		body, err := e.threadText(*text, *file, rest[1:])
		if err != nil {
			return 1, err
		}
		f := threads.Filing{Stage: threads.Receipt, Text: body, Outcome: threads.Completed}
		if *killed {
			f.Outcome = threads.Killed
		}
		if t, err = threads.File(p, rest[0], f, now); err != nil {
			return 1, err
		}
		verb = f.Outcome
	case "set":
		var ch threads.Changes
		if set["title"] {
			ch.Title = title
		}
		if set["priority"] {
			ch.Priority = priority
		}
		if set["phase"] {
			ch.Phase = phase
		}
		if set["blocked"] {
			ch.Blocked = blocked
		}
		if ch.Empty() {
			return 2, errors.New("usage: atlas-obsidian thread PROJECT set ID [--title T] [--priority P] [--phase NAME] [--blocked TEXT]")
		}
		if t, err = threads.Set(p, rest[0], ch, now); err != nil {
			return 1, err
		}
		verb = "set"
	case "reopen":
		if t, err = threads.Reopen(p, rest[0], now); err != nil {
			return 1, err
		}
		verb = "reopened"
	default:
		return 2, fmt.Errorf("unknown action %q; new, show, file, close, set, or reopen", action)
	}
	e.printThread(p, t, verb)
	return 0, nil
}

// printThread shows one thread: its state on one line, then the path of the document of
// its stage, or of every document for show.
func (e *env) printThread(p *project.Project, t *threads.Thread, verb string) {
	state := t.Stage
	if t.Closed() {
		state = t.Outcome
	}
	facts := []string{state, t.Priority}
	if t.Phase != "" {
		facts = append(facts, t.Phase)
	}
	if t.Blocked != "" {
		facts = append(facts, "blocked: "+t.Blocked)
	}
	e.console.Step(console.OK, verb, fmt.Sprintf("%s (%s) in %s · %s", t.Title, t.ID, p.Name(), strings.Join(facts, " · ")))
	for _, d := range t.Docs {
		if verb == "thread" || d.Stage == t.Stage {
			e.console.Say("  %-8s %s", d.Stage, home.Display(p.Path(d.Path)))
		}
	}
}

func (e *env) phase(args []string) (int, error) {
	fs := newFlags("phase", e.stderr)
	goal := fs.String("goal", "", "what the phase delivers")
	order := fs.Int("order", 0, "where the phase sits in the timeline")
	to := fs.String("to", "", "the new title, for rename")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	set := setFlags(fs)
	if len(positional) != 3 {
		return 2, errors.New("usage: atlas-obsidian phase PROJECT create|rename|reorder|remove TITLE [--goal TEXT] [--order N] [--to NEW]")
	}
	p, _, err := e.projectArg(positional[0])
	if err != nil {
		return 1, err
	}
	title := positional[2]
	now := time.Now()
	switch positional[1] {
	case "create":
		var n *int
		if set["order"] {
			n = order
		}
		ph, err := threads.CreatePhase(p, title, *goal, n, now)
		if err != nil {
			return 1, err
		}
		e.console.Step(console.OK, "created", fmt.Sprintf("phase %s (order %d) · %s", ph.Title, ph.Order, home.Display(p.Path(ph.Path))))
	case "rename":
		if *to == "" {
			return 2, errors.New("rename needs --to NEW")
		}
		ph, err := threads.RenamePhase(p, title, *to, now)
		if err != nil {
			return 1, err
		}
		e.console.Step(console.OK, "renamed", fmt.Sprintf("%s is now %s", title, ph.Title))
	case "reorder":
		if !set["order"] {
			return 2, errors.New("reorder needs --order N")
		}
		ph, err := threads.ReorderPhase(p, title, *order, now)
		if err != nil {
			return 1, err
		}
		e.console.Step(console.OK, "reordered", fmt.Sprintf("%s is now order %d", ph.Title, ph.Order))
	case "remove":
		if err := threads.RemovePhase(p, title, now); err != nil {
			return 1, err
		}
		e.console.Step(console.OK, "removed", "phase "+title)
	default:
		return 2, fmt.Errorf("unknown action %q; create, rename, reorder, or remove", positional[1])
	}
	return 0, nil
}

func (e *env) view(args []string) (int, error) {
	if !e.console.Interactive() {
		return 2, errors.New("view is an interactive screen and needs a terminal")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	acts := actions.Bind(e.home, cfg, e.console)
	entries, err := acts.Load()
	if err != nil {
		return 1, err
	}
	opener := tui.Opener{
		Obsidian: func(path string) error { return obsidian.RegisterAndOpen(path) },
		Agent: func(harness, path string) error {
			cmd, err := claudecode.LaunchCommand(cfg.HarnessLaunch(harness), path, "")
			if err != nil {
				return err
			}
			return cmd.Run()
		},
	}
	changed, err := tui.RunView(tui.Items(entries), opener, acts)
	if err != nil {
		return 1, err
	}
	if !changed {
		return 0, nil
	}
	entries, _, err = e.refreshAll(cfg)
	if err != nil {
		return 1, err
	}
	e.console.Step(console.OK, "refreshed", refreshed(entries))
	return 0, nil
}

func (e *env) openVault(args []string) (int, error) {
	fs := newFlags("open-vault", e.stderr)
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) > 1 {
		return 2, errors.New("usage: atlas-obsidian open-vault [PROJECT | PATH]")
	}
	v, err := e.vaultArg(first(positional))
	if err != nil {
		return 1, err
	}
	root, label := v.Atlas(), v.Name()
	c := e.console
	registered, running, err := obsidian.Status(root)
	if err != nil {
		c.Say("%v", err)
		c.Say("Open it by hand: in Obsidian choose \"Open folder as vault\" and pick %s.", home.Display(root))
		obsidian.Reveal(root)
		return 1, nil
	}
	if registered {
		if err := obsidian.Open(root); err != nil {
			return 1, err
		}
		c.Step(console.OK, "opened", fmt.Sprintf("%s in Obsidian", label))
		return 0, nil
	}
	c.Say("Obsidian does not know %s yet (%s).", label, home.Display(root))
	question := "Register it as a vault and open it?"
	if running {
		question = "Register it as a vault? Obsidian will quit and relaunch so it sees the new entry."
	}
	ok, err := c.Confirm(question, true)
	if err != nil {
		return 1, err
	}
	if !ok {
		c.Say("Open it by hand: in Obsidian choose \"Open folder as vault\" and pick %s.", home.Display(root))
		obsidian.Reveal(root)
		return 1, manage.ErrCancelled
	}
	if err := obsidian.RegisterAndOpen(root); err != nil {
		if errors.Is(err, obsidian.ErrManualRestart) {
			c.Say("%v", err)
			return 1, nil
		}
		return 1, err
	}
	c.Step(console.OK, "registered", home.Display(root))
	if running {
		c.Step(console.OK, "restarted", "Obsidian")
	}
	c.Step(console.OK, "opened", fmt.Sprintf("%s in Obsidian", label))
	return 0, nil
}

// skillHint is printed before handing the terminal to Claude Code.
func skillHint() string { return "skills: " + hooks.Skills }

// trustNote explains Claude Code's own first-run dialog, whose default answer quits.
const trustNote = "The first time in a folder, Claude Code asks whether you trust it; choose Yes."

func (e *env) openIDE(args []string) (int, error) {
	fs := newFlags("open-ide", e.stderr)
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) != 1 {
		return 2, errors.New("usage: atlas-obsidian open-ide NAME")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	entry, err := e.entry(cfg, positional[0])
	if err != nil {
		return 1, err
	}
	if err := actions.Bind(e.home, cfg, e.console).OpenIDE(entry); err != nil {
		return 1, err
	}
	e.console.Step(console.OK, "opened", entry.Name+" in VS Code")
	return 0, nil
}

func (e *env) openPreferredAgent(args []string) (int, error) {
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	return e.openAgent(args, cfg.Harness())
}

func (e *env) openClaude(args []string) (int, error) {
	return e.openAgent(args, "claude")
}

func (e *env) openAgent(args []string, agent string) (int, error) {
	fs := newFlags("open-"+agent, e.stderr)
	threadID := fs.String("thread", "", "continue this thread: start with the skill for its next stage as the first message")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) != 1 {
		return 2, fmt.Errorf("usage: atlas-obsidian open-%s NAME [--thread ID]", agent)
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	entry, err := e.entry(cfg, positional[0])
	if err != nil {
		return 1, err
	}
	if !e.console.Interactive() {
		return 2, fmt.Errorf("open-%s starts an interactive session and needs a terminal", agent)
	}
	prompt := ""
	if *threadID != "" {
		p, err := project.Open(entry.Path)
		if err != nil {
			return 1, err
		}
		board, err := threads.Load(p)
		if err != nil {
			return 1, err
		}
		t, err := board.Resolve(*threadID)
		if err != nil {
			return 1, err
		}
		if t.Closed() {
			return 1, fmt.Errorf("%s is closed (%s); `atlas-obsidian thread %s reopen %s` opens it again", t.Title, t.Outcome, entry.Name, t.ID)
		}
		prompt = claudecode.ThreadPrompt(t.Stage, t.ID)
		e.console.Say("  thread: %s (%s)", t.Title, t.Stage)
	}
	launch := cfg.HarnessLaunch(agent)
	if agent == "codex" {
		prompt = strings.Replace(prompt, "/atlas-obsidian:", "$", 1)
	}
	cmd, err := claudecode.LaunchCommand(launch, entry.Path, prompt)
	if err != nil {
		return 1, err
	}
	e.console.Say("  %s", home.Display(cmd.Dir))
	if agent == "codex" {
		e.console.Say("  Use $wiki to orient; review and trust Atlas hooks in /hooks before relying on them.")
	} else {
		e.console.Say("  %s", skillHint())
		e.console.Say("  %s", trustNote)
	}
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

func (e *env) ingest(args []string) (int, error) {
	fs := newFlags("ingest", e.stderr)
	dryRun := fs.Bool("dry-run", false, "show what would be staged and stop")
	noClaude := fs.Bool("no-claude", false, "stage the files but do not start Claude Code")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) == 0 {
		return 2, errors.New("usage: atlas-obsidian ingest [PROJECT] [PATH ...] [--dry-run] [--no-claude]")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	v, err := e.vaultArg(positional[0])
	if err != nil {
		return 1, err
	}
	entry, err := e.entry(cfg, v.Root)
	if err != nil {
		return 1, err
	}
	c := e.console
	sources, err := capture.SourcesFor(v, positional[1:])
	if err != nil {
		return 1, err
	}
	plan, err := capture.PlanStage(v, sources, time.Now())
	if err != nil {
		return 1, err
	}
	for _, src := range plan.Sources {
		c.Say("  %-10s %s", "source", home.Display(src))
	}
	for _, f := range plan.New {
		c.Say("  %-10s %s", "new", strings.TrimPrefix(f.To, "inbox/"))
	}
	if n := len(plan.Unchanged); n > 0 {
		c.Say("  %-10s %d file%s already ingested or waiting", "unchanged", n, plural(n))
	}
	for _, sk := range plan.Skipped {
		c.Say("  %-10s %s (%s)", "skipped", home.Display(sk.From), sk.Reason)
	}
	if plan.Waiting > 0 {
		c.Say("  %-10s %d file%s waiting to be ingested", "inbox", plan.Waiting, plural(plan.Waiting))
	}
	if *dryRun {
		return 0, nil
	}
	if len(plan.New) == 0 && plan.Waiting == 0 {
		c.Say("  nothing to ingest")
		return 0, nil
	}
	if len(plan.New) > 0 {
		question := fmt.Sprintf("Stage %d file%s into inbox/", len(plan.New), plural(len(plan.New)))
		ok, err := c.Confirm(question+"?", true)
		if err != nil {
			return 1, err
		}
		if !ok {
			return 1, manage.ErrCancelled
		}
		res, remembered, err := actions.Bind(e.home, cfg, e.console).Stage(entry, plan)
		if err != nil {
			return 1, err
		}
		c.Step(console.OK, "staged", fmt.Sprintf("%d file%s in %s", len(res.Staged), plural(len(res.Staged)), home.Display(v.Path(project.InboxDir))))
		for _, dir := range remembered {
			c.Step(console.OK, "remembered", home.Display(dir)+"; `atlas-obsidian ingest "+entry.Name+"` stages what is new there next time")
		}
	} else {
		c.Say("  nothing new to stage; %d file%s already waiting", plan.Waiting, plural(plan.Waiting))
	}
	return e.offerClaude(cfg, v.Root, *noClaude, claudecode.IngestPrompt)
}

func (e *env) list(args []string) (int, error) {
	if len(args) != 0 {
		return 2, errors.New("usage: atlas-obsidian list")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	entries, err := e.registryEntries(cfg)
	if err != nil {
		return 1, err
	}
	if len(entries) == 0 {
		e.console.Say("no projects yet; run `atlas-obsidian init` in your work")
		return 0, nil
	}
	var good, bad []registry.Entry
	for _, en := range entries {
		if en.Error != "" {
			bad = append(bad, en)
		} else {
			good = append(good, en)
		}
	}
	width := nameWidth(good)
	for _, en := range good {
		e.console.Say("  %-5s %-*s  %s", listHeat(en), width, en.Name, home.Display(en.Path))
	}
	if len(bad) > 0 && len(bad) < len(entries) {
		e.console.Say("")
	}
	for _, en := range bad {
		e.console.Step(console.Fail, entryName(en), home.Display(en.Path)+": "+en.Error)
	}
	return 0, nil
}

// unreadable is the one word for an entry the atlas knows but could not read.
func unreadable(en registry.Entry) string {
	switch en.Reason {
	case registry.ReasonMissing:
		return "missing"
	case registry.ReasonNotProject:
		return "none"
	default:
		return "bad"
	}
}

// nameWidth is the width of a name column that holds every entry's name.
func nameWidth(entries []registry.Entry) int {
	width := 0
	for _, en := range entries {
		width = max(width, len(entryName(en)))
	}
	return width
}

// listHeat is the heat column: what the last refresh found, or why there is nothing.
func listHeat(en registry.Entry) string {
	if en.State == nil {
		return "?"
	}
	if en.State.Heat == "" {
		return "off"
	}
	return en.State.Heat
}

func (e *env) show(args []string) (int, error) {
	if len(args) != 1 {
		return 2, errors.New("usage: atlas-obsidian show NAME")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	ix, err := registry.Scan(cfg)
	if err != nil {
		return 1, err
	}
	entry, err := findEntry(ix, args[0])
	if err != nil {
		return 1, err
	}
	if stored, _, err := registry.Read(e.home.StateDir()); err == nil {
		for _, s := range stored {
			if s.ID == entry.ID {
				entry.State = s.State
			}
		}
	}
	c := e.console
	row := func(k, val string) {
		if val == "" {
			val = "—"
		}
		c.Say("  %-16s %s", k, val)
	}
	row("Name", entry.Name)
	row("Id", entry.ID)
	row("Path", home.Display(entry.Path))
	row("Wiki", home.Display(entry.Wiki()))
	row("Created", entry.Created)
	row("Mode", string(entry.Mode))
	row("Description", entry.Description)
	if d := describe.Page(entry); d != nil {
		row("Page", d.Summary())
	} else {
		row("Page", registry.NotDescribed)
	}
	state := entry.State
	if state == nil {
		row("Refreshed", "never; run `atlas-obsidian refresh`")
		return 0, nil
	}
	if state.OK {
		row("Check", "ok")
	} else {
		row("Check", state.Error)
	}
	heat := state.Heat
	if heat == "" {
		heat = "unknown"
	}
	row("Heat", heat)
	row("Last touched", state.LastTouched)
	if state.DaysIdle != nil {
		row("Idle", fmt.Sprintf("%d day%s", *state.DaysIdle, plural(*state.DaysIdle)))
	}
	if state.Git != nil {
		row("Git", refresh.LinkSummary(*state.Git))
	}
	row("Last operation", state.LastOperation)
	if state.Pages != nil {
		row("Pages", fmt.Sprint(*state.Pages))
	}
	if state.Inbox != nil {
		row("Inbox", fmt.Sprintf("%d waiting", *state.Inbox))
	}
	row("Unfinished", state.Unfinished.Text())
	for i, t := range state.HotTopics {
		label := "Hot topics"
		if i > 0 {
			label = ""
		}
		c.Say("  %-16s - %s", label, refresh.PlainText(t))
	}
	// The threads are cheap to read and change without a refresh, so they are read now
	// rather than from the registry.
	if p, err := project.Open(entry.Path); err == nil {
		if board, err := threads.Load(p); err == nil {
			counts := board.Counts(time.Now())
			counts.Notes = len(threads.Notes(p))
			row("Threads", threadCounts(counts))
			var phases []string
			for _, ph := range board.Phases {
				name := ph.Title
				if board.Finished(ph.Title) {
					name += " (finished)"
				}
				phases = append(phases, name)
			}
			if len(phases) > 0 {
				row("Phases", strings.Join(phases, ", "))
			}
		}
	}
	row("Refreshed", state.GeneratedAt)
	for _, signal := range refresh.Signals(entry, time.Now()) {
		row("Signal", signal)
	}
	return 0, nil
}

// threadCounts is one line of a project's threads.
func threadCounts(c threads.Counts) string {
	line := fmt.Sprintf("%d open (plan %d, spec %d, stub %d)", c.Open, c.Plan, c.Spec, c.Stub)
	if c.Blocked > 0 {
		line += fmt.Sprintf(" · %d blocked", c.Blocked)
	}
	if c.Completed+c.Killed > 0 {
		line += fmt.Sprintf(" · %d closed", c.Completed+c.Killed)
	}
	if c.Notes > 0 {
		line += fmt.Sprintf(" · %d note%s waiting", c.Notes, plural(c.Notes))
	}
	return line
}

func (e *env) edit(args []string) (int, error) {
	fs := newFlags("edit", e.stderr)
	name := fs.String("name", "", "the project's name; it renames atlas/<name>/ too")
	description := fs.String("description", "", "what the work is and what its wiki should remember; \"\" clears it")
	mode := fs.String("mode", "", "the filing mode for new wiki pages: generic or lyt")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	set := setFlags(fs)
	if len(positional) != 1 || len(set) == 0 {
		return 2, errors.New("usage: atlas-obsidian edit NAME [--name N] [--description TEXT] [--mode generic|lyt]")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	entry, err := e.entry(cfg, positional[0])
	if err != nil {
		return 1, err
	}
	change := manage.Edit{Name: *name}
	if set["description"] {
		change.Description = description
	}
	if set["mode"] {
		m, err := project.ParseMode(*mode)
		if err != nil {
			return 2, err
		}
		change.Mode = m
	}
	if err := actions.Bind(e.home, cfg, e.console).EditProject(entry, change); err != nil {
		return 1, err
	}
	entries, _, err := e.refreshAll(cfg)
	if err != nil {
		return 1, err
	}
	shown := entry.Name
	if *name != "" {
		shown = *name
	}
	e.console.Step(console.OK, "edited", shown+": "+strings.Join(change.Fields(), ", "))
	if p, err := project.Open(entry.Path); err == nil {
		e.console.Step(console.OK, "folder", p.Rel()+"/")
	}
	e.console.Step(console.OK, "refreshed", refreshed(entries))
	return 0, nil
}

func (e *env) refresh(args []string) (int, error) {
	if len(args) != 0 {
		return 2, errors.New("usage: atlas-obsidian refresh")
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	entries, ix, err := refresh.Registry(e.home, cfg, e.home.StateDir(), time.Now())
	if err != nil {
		return 1, err
	}
	for _, en := range entries {
		switch {
		case en.Error != "":
			e.console.Step(console.Fail, entryName(en), en.Error)
		case en.State == nil:
			e.console.Step(console.Fail, en.Name, "not read")
		case !en.State.OK:
			e.console.Step(console.Fail, en.Name, en.State.Error)
		default:
			heat := en.State.Heat
			if heat == "" {
				heat = "-"
			}
			days := "?"
			if en.State.DaysIdle != nil {
				days = fmt.Sprint(*en.State.DaysIdle)
			}
			e.console.Step(console.OK, en.Name, fmt.Sprintf("%s, idle %sd", heat, days))
		}
	}
	for _, problem := range uncoveredProblems(ix) {
		e.console.Step(console.Fail, home.Display(problem.Path), problem.Reason)
	}
	e.console.Say("  wrote %s", home.Display(registry.File(e.home.StateDir())))
	return 0, nil
}

func (e *env) lint(args []string) (int, error) {
	fs := newFlags("lint", e.stderr)
	asJSON := fs.Bool("json", false, "print the report as JSON")
	strict := fs.Bool("strict", false, "exit 1 when there are findings")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) > 1 {
		return 2, errors.New("usage: atlas-obsidian lint [PROJECT] [--json] [--strict]")
	}
	v, err := e.vaultArg(first(positional))
	if err != nil {
		return 1, err
	}
	report, err := lint.Run(v.Atlas(), lint.Options{AsOf: time.Now()})
	if err != nil {
		return 1, err
	}
	if *asJSON {
		e.stdout.Write(report.JSON())
	} else {
		io.WriteString(e.stdout, report.Markdown())
	}
	if *strict && report.Summary.IssuesFound > 0 {
		return 1, nil
	}
	return 0, nil
}

func (e *env) history(args []string) (int, error) {
	fs := newFlags("history", e.stderr)
	limit := fs.Int("n", 20, "how many operations to show")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) > 1 {
		return 2, errors.New("usage: atlas-obsidian history [PROJECT] [-n N]")
	}
	v, err := e.vaultArg(first(positional))
	if err != nil {
		return 1, err
	}
	ops, err := txn.History(v, *limit, false)
	if err != nil {
		return 1, err
	}
	if len(ops) == 0 {
		e.console.Say("no operations yet")
		return 0, nil
	}
	for _, op := range ops {
		e.console.Say("  %s  %-34s %-9s %s", op.Date.Local().Format("2006-01-02 15:04"), op.ID, op.Kind, op.Summary)
	}
	return 0, nil
}

func (e *env) stub(args []string) (int, error) {
	fs := newFlags("stub", e.stderr)
	pageType := fs.String("type", "", "the type of every stub: concept or entity; in lyt mode note or moc")
	positional, err := parse(fs, args)
	if err != nil {
		return 2, nil
	}
	if len(positional) < 1 {
		return 2, errors.New("usage: atlas-obsidian stub [PROJECT] [TITLE...] [--type T]")
	}
	v, err := e.vaultArg(positional[0])
	if err != nil {
		return 1, err
	}
	var titles []txn.StubTitle
	for _, title := range positional[1:] {
		titles = append(titles, txn.StubTitle{Title: title})
	}
	res, err := txn.StubPages(v, titles, *pageType, time.Now())
	if err != nil {
		return 1, err
	}
	for _, s := range res.Skipped {
		e.console.Step(console.Skip, "skipped", fmt.Sprintf("%s: %s", s.Title, s.Reason))
	}
	if len(res.Stubs) == 0 {
		if len(res.Skipped) == 0 {
			e.console.Step(console.Skip, "nothing to stub", "every page the wiki links to exists")
		}
		return 0, nil
	}
	for _, s := range res.Stubs {
		e.console.Step(console.OK, "stubbed", fmt.Sprintf("%s (%s)", s.Path, s.Type))
	}
	e.console.Step(console.OK, "committed", res.OperationID)
	return 0, nil
}

func (e *env) undo(args []string) (int, error) {
	if len(args) != 2 {
		return 2, errors.New("usage: atlas-obsidian undo PROJECT OPERATION")
	}
	v, err := e.vaultArg(args[0])
	if err != nil {
		return 1, err
	}
	op, err := txn.Find(v, args[1])
	if err != nil {
		return 1, err
	}
	ok, err := e.console.Confirm(fmt.Sprintf("Undo %s (%s: %s)?", op.ID, op.Kind, op.Summary), false)
	if err != nil {
		return 1, err
	}
	if !ok {
		return 1, manage.ErrCancelled
	}
	res, err := txn.UndoOperation(v, args[1], time.Now())
	if err != nil {
		return 1, err
	}
	e.console.Step(console.OK, "undone", fmt.Sprintf("%s in commit %s", op.ID, res.Commit[:12]))
	for _, p := range res.ChangedPaths {
		e.console.Say("    %s", p)
	}
	return 0, nil
}

func (e *env) recover(args []string) (int, error) {
	if len(args) > 1 {
		return 2, errors.New("usage: atlas-obsidian recover [PROJECT]")
	}
	v, err := e.vaultArg(first(args))
	if err != nil {
		return 1, err
	}
	res, err := txn.Recover(v)
	if err != nil {
		return 1, err
	}
	if res == nil {
		e.console.Step(console.Skip, "recover", "nothing was interrupted")
		return 0, nil
	}
	e.console.Step(console.OK, "recovered", fmt.Sprintf("%s; restored %s", res.OperationID, strings.Join(res.Restored, ", ")))
	return 0, nil
}

// planFile is the JSON shape `apply` reads: the plan tool's arguments, with content_file
// allowed in place of content.
type planFile struct {
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
	Writes  []struct {
		Path        string `json:"path"`
		Mode        string `json:"mode"`
		Content     string `json:"content"`
		ContentFile string `json:"content_file"`
		BaseSHA256  string `json:"base_sha256"`
	} `json:"writes"`
	Sources []struct {
		ID        string   `json:"id"`
		Ingested  bool     `json:"ingested"`
		Pages     []string `json:"pages"`
		Authority string   `json:"authority"`
		Title     string   `json:"title"`
		Notes     string   `json:"notes"`
	} `json:"sources"`
}

func (e *env) apply(args []string) (int, error) {
	if len(args) != 2 {
		return 2, errors.New("usage: atlas-obsidian apply PROJECT PLAN.json")
	}
	v, err := e.vaultArg(args[0])
	if err != nil {
		return 1, err
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		return 1, err
	}
	var pf planFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return 1, fmt.Errorf("%s: %w", args[1], err)
	}
	req := txn.Request{Kind: txn.Kind(pf.Kind), Summary: pf.Summary}
	for _, w := range pf.Writes {
		content := []byte(w.Content)
		if w.ContentFile != "" {
			content, err = os.ReadFile(filepath.Join(filepath.Dir(args[1]), w.ContentFile))
			if err != nil {
				return 1, err
			}
		}
		req.Writes = append(req.Writes, txn.Write{Path: w.Path, Mode: txn.WriteMode(w.Mode), Content: content, BaseSHA256: w.BaseSHA256})
	}
	for _, s := range pf.Sources {
		req.Sources = append(req.Sources, ledgerUpdate(s.ID, s.Ingested, s.Pages, s.Authority, s.Title, s.Notes))
	}
	plan, err := txn.Prepare(v, req, time.Now())
	if err != nil {
		return 1, err
	}
	c := e.console
	for _, ch := range plan.Preview.Creates {
		c.Say("  create   %s", ch.Path)
	}
	for _, ch := range plan.Preview.Replaces {
		c.Say("  replace  %s", ch.Path)
	}
	for _, ch := range plan.Preview.Deletes {
		c.Say("  delete   %s", ch.Path)
	}
	for _, w := range plan.Warnings {
		c.Step(console.Fail, "warning", w)
	}
	ok, err := c.Confirm("Apply "+plan.Summary+"?", true)
	if err != nil {
		return 1, err
	}
	if !ok {
		return 1, manage.ErrCancelled
	}
	res, err := txn.Apply(v, plan, time.Now())
	if err != nil {
		return 1, err
	}
	c.Step(console.OK, "applied", fmt.Sprintf("%s in commit %s", res.OperationID, res.Commit[:12]))
	return 0, nil
}

func (e *env) mcp(args []string) (int, error) {
	if len(args) != 0 {
		return 2, errors.New("usage: atlas-obsidian mcp")
	}
	dir := os.Getenv("CLAUDE_PROJECT_DIR")
	if dir == "" {
		dir = cwd()
	}
	err := mcpserver.Run(context.Background(), mcpserver.Options{
		Version: Version, PluginRoot: pluginRoot(), ProjectDir: dir,
	})
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func pluginRoot() string {
	if root := os.Getenv("PLUGIN_ROOT"); root != "" {
		return root
	}
	return os.Getenv("CLAUDE_PLUGIN_ROOT")
}

func (e *env) hook(args []string) (int, error) {
	if len(args) != 1 {
		return 2, errors.New("usage: atlas-obsidian hook session-start|guard|touched|stop")
	}
	switch args[0] {
	case "session-start":
		enabled := true
		if cfg, err := e.home.Load(); err == nil {
			enabled = cfg.ClaudeCode.SessionContext
		}
		return 0, hooks.SessionStart(e.stdin, e.stdout, os.Getenv, enabled, time.Now())
	case "guard":
		return 0, hooks.Guard(e.stdin, e.stdout)
	case "touched":
		// A thread that cannot be marked is no reason to interrupt the session.
		hooks.Touched(e.stdin, time.Now())
		return 0, nil
	case "stop":
		return 0, hooks.Stop(e.stdin, e.stdout, os.Getenv)
	}
	return 2, fmt.Errorf("unknown hook %q", args[0])
}

// config shows the settings, or sets one and refreshes so the registry follows.
func (e *env) config(args []string) (int, error) {
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	c := e.console
	if len(args) == 0 {
		row := func(label, value string) { c.Say("  %-18s %s", label, value) }
		row("preferred-harness", cfg.Harness())
		row("preferred-ide", cfg.IDE())
		row("new-days", fmt.Sprintf("%d  (an entry is new for this many days after its creation; 0 turns it off)", cfg.NewDays()))
		row("projects", fmt.Sprintf("%d registered", len(cfg.Projects)))
		row("claude command", cfg.ClaudeCode.Command)
		row("plugin source", cfg.Plugin.Source)
		row("file", home.Display(e.home.ConfigPath()))
		return 0, nil
	}
	if len(args) != 2 {
		return 2, errors.New("usage: atlas-obsidian config [KEY VALUE]; keys: new-days, preferred-harness, preferred-ide")
	}
	switch args[0] {
	case "preferred-ide":
		if err := actions.Bind(e.home, cfg, c).SetPreferredIDE(args[1]); err != nil {
			return 1, err
		}
		c.Step(console.OK, args[0], args[1])
		return 0, nil
	case "preferred-harness":
		if err := actions.Bind(e.home, cfg, c).SetPreferredHarness(args[1]); err != nil {
			return 1, err
		}
		c.Step(console.OK, args[0], args[1])
		return 0, nil
	case "new-days":
		days, err := strconv.Atoi(args[1])
		if err != nil {
			return 2, fmt.Errorf("new-days takes a number of days, got %q", args[1])
		}
		if err := cfg.SetNewDays(days); err != nil {
			return 2, err
		}
	default:
		return 2, fmt.Errorf("unknown setting %q; keys: new-days, preferred-harness, preferred-ide", args[0])
	}
	if err := e.home.Save(cfg); err != nil {
		return 1, err
	}
	entries, _, err := e.refreshAll(cfg)
	if err != nil {
		return 1, err
	}
	c.Step(console.OK, args[0], args[1])
	c.Step(console.OK, "refreshed", refreshed(entries))
	return 0, nil
}

func (e *env) info(args []string) (int, error) {
	c := e.console
	row := func(label, value string) { c.Say("  %-18s %s", label, value) }
	row("atlas-obsidian", Version)
	if exe, err := os.Executable(); err == nil {
		row("binary", home.Display(exe))
	}
	row("home", home.Display(e.home.Root))
	row("config", home.Display(e.home.ConfigPath()))
	if !e.home.Exists() {
		row("status", "not set up; run `atlas-obsidian setup`")
		return 0, nil
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	row("registry", home.Display(registry.File(e.home.StateDir())))
	row("state", home.Display(e.home.StateDir()))
	if inst, _ := claudecode.InstalledPlugin(cfg.Plugin.ID); inst != nil {
		row("plugin", fmt.Sprintf("%s v%s", cfg.Plugin.ID, inst.Version))
		row("  path", home.Display(inst.InstallPath))
	} else {
		row("plugin", cfg.Plugin.ID+" (not installed; run `atlas-obsidian setup`)")
	}
	row("  source", cfg.Plugin.Source)
	row("claude config", home.Display(claudecode.ConfigDir()))
	ix, err := registry.Scan(cfg)
	if err != nil {
		return 1, err
	}
	row("entries", fmt.Sprintf("%d found", len(ix.Entries)))
	for _, en := range ix.Entries {
		row("  "+entryName(en), home.Display(en.Path))
	}
	for _, p := range uncoveredProblems(ix) {
		row("  "+filepath.Base(p.Path), home.Display(p.Path)+": "+p.Reason)
	}
	return 0, nil
}

func (e *env) doctor(args []string) (int, error) {
	fs := newFlags("doctor", e.stderr)
	agent := fs.String("agent", "claude", "plugin host: claude or codex")
	if err := fs.Parse(args); err != nil {
		return 2, nil
	}
	if fs.NArg() != 0 || (*agent != "claude" && *agent != "codex") {
		return 2, errors.New("usage: atlas-obsidian doctor [--agent claude|codex]")
	}
	c := e.console
	line := func(label, value string) { c.Say("  %-16s %s", label, value) }
	line("home", home.Display(e.home.Root)+"  "+ternary(e.home.Exists(), "ok", "missing"))
	if !e.home.Exists() {
		return 1, nil
	}
	cfg, err := e.home.Load()
	if err != nil {
		return 1, err
	}
	ok := true
	line("atlas-obsidian", Version)
	if gitx.Available() {
		line("git", "on PATH")
	} else {
		ok = false
		line("git", "missing; every wiki operation needs it")
	}
	if *agent == "codex" {
		inst, err := codex.InstalledPlugin(cfg.Plugin.ID)
		switch {
		case err != nil:
			ok = false
			line("Codex plugin", err.Error())
		case inst == nil:
			ok = false
			line("Codex plugin", "not installed; run `atlas-obsidian setup --agent codex`")
		case !inst.Enabled:
			ok = false
			line("Codex plugin", "disabled; enable it in /plugins")
		default:
			note := ""
			if inst.Version != "" && Version != "dev" && inst.Version != Version {
				note = fmt.Sprintf(" (binary is %s; keep them in step)", Version)
			}
			line("Codex plugin", cfg.Plugin.ID+" v"+inst.Version+note)
			line("hooks", "review trust in Codex /hooks")
		}
	} else {
		if claudecode.CLI() == "" {
			line("Claude Code", "`claude` is not on PATH")
		} else {
			line("Claude Code", "on PATH")
		}
		if inst, _ := claudecode.InstalledPlugin(cfg.Plugin.ID); inst != nil {
			note := ""
			if inst.Version != "" && Version != "dev" && inst.Version != Version {
				note = fmt.Sprintf("  (binary is %s; keep them in step)", Version)
			}
			line("plugin", fmt.Sprintf("v%s at %s%s", inst.Version, home.Display(inst.InstallPath), note))
		} else {
			ok = false
			line("plugin", cfg.Plugin.ID+" is not installed; run `atlas-obsidian setup`")
		}
	}
	ix, err := registry.Scan(cfg)
	if err != nil {
		return 1, err
	}
	line("entries", fmt.Sprintf("%d found", len(ix.Entries)))
	width := nameWidth(ix.Entries)
	for _, en := range ix.Entries {
		status := entryStatus(en)
		if status != "ok" {
			ok = false
		}
		c.Say("    %-7s %-*s  %s", status, width, entryName(en), home.Display(en.Path))
		if en.Error != "" {
			c.Say("            %s", en.Error)
		}
	}
	for _, p := range uncoveredProblems(ix) {
		ok = false
		c.Step(console.Fail, filepath.Base(p.Path), home.Display(p.Path)+": "+p.Reason)
	}
	if !ok {
		return 1, nil
	}
	return 0, nil
}

// entryStatus is doctor's one word for an entry: what stands between it and working.
func entryStatus(en registry.Entry) string {
	if en.Error != "" {
		return unreadable(en)
	}
	p, err := project.Open(en.Path)
	if err != nil {
		return "bad"
	}
	if pending, _ := txn.Pending(p); pending != nil {
		return "recover"
	}
	if !p.Repo().IsRepo() {
		return "no git"
	}
	return "ok"
}
