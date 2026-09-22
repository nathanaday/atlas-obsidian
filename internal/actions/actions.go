// Package actions is every change to the atlas the view and the tools can make, as one
// struct of functions over the packages that own them. Bind builds it in one place; the
// CLI, the view, and the MCP server never bind a function a second time.
package actions

import (
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/capture"
	"github.com/nathanaday/atlas-obsidian/internal/console"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/ide"
	"github.com/nathanaday/atlas-obsidian/internal/manage"
	"github.com/nathanaday/atlas-obsidian/internal/mirror"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/refresh"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/terminal"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

// InitProject is what a caller chose for a new project.
type InitProject struct {
	Work        string // the folder that becomes the project
	Name        string
	Description string
	Mode        string // generic or lyt
	NoGit       bool   // leave a work folder that is in no repository without one
	NoThreads   bool   // leave threads off
}

// Atlas is every action the CLI, the view, and the tools reach. Each field is one
// function from the package that owns the action, bound to the atlas home and its config.
type Atlas struct {
	PreferredIDE        func() string
	SetPreferredIDE     func(string) error
	OpenIDE             func(registry.Entry) error
	OpenTerminal        func(registry.Entry) error
	PreferredHarness    func() string
	SetPreferredHarness func(string) error
	// Load reads the registry, refreshing it first when no refresh has run yet. Scan
	// reads everything afresh, with its state derived, and writes nothing. Refresh reads
	// everything again and rewrites the registry.
	Load    func() ([]registry.Entry, error)
	Scan    func() (*registry.Index, error)
	Refresh func() (*registry.Index, error)
	// The project calls: make a folder a project, change its name, description, mode,
	// or members, and drop it from the atlas. The folder stays. Sync rewrites the
	// mirrors of a project's members under its wiki/projects/.
	InitProject   func(InitProject) (*project.InitResult, error)
	EditProject   func(registry.Entry, manage.Edit) error
	ForgetProject func(registry.Entry) error
	Sync          func(registry.Entry) (*mirror.Result, error)
	// StagePlan says which files under the sources are new to a project's inbox; no
	// sources means the folders it staged from before. Stage copies a plan's files into
	// the inbox and reports the folders the project now remembers. Sources lists them.
	StagePlan func(registry.Entry, []string) (*capture.StagePlan, error)
	Stage     func(registry.Entry, *capture.StagePlan) (*capture.StageResult, []string, error)
	Sources   func(registry.Entry) []string
	// StageProject writes a snapshot of the work into the project's own inbox, for the
	// describe skill to ingest.
	StageProject func(registry.Entry) (*capture.ProjectStage, error)
	// The thread calls, on a project: read the board and the notes waiting in its inbox,
	// open a thread, file a document, change a card, reopen, and the phase calls.
	Threads     func(registry.Entry) (*threads.Board, []string, error)
	StartThread func(registry.Entry, threads.New) (*threads.Thread, error)
	FileThread  func(registry.Entry, string, threads.Filing) (*threads.Thread, error)
	SetThread   func(registry.Entry, string, threads.Changes) (*threads.Thread, error)
	Reopen      func(registry.Entry, string) (*threads.Thread, error)
	AddPhase    func(registry.Entry, string, string, *int) (*threads.Phase, error)
	RenamePhase func(registry.Entry, string, string) (*threads.Phase, error)
	OrderPhase  func(registry.Entry, string, int) (*threads.Phase, error)
	RemovePhase func(registry.Entry, string) error
}

// Bind builds the struct over an atlas home and its loaded config. The console is for
// InitProject's preview when a caller wants one; the view passes none.
func Bind(h home.Home, cfg *home.Config, c *console.Console) Atlas {
	openProject := func(en registry.Entry) (*project.Project, error) { return project.Open(en.Path) }
	return Atlas{
		PreferredIDE: cfg.IDE,
		SetPreferredIDE: func(preferred string) error {
			latest, err := h.Load()
			if err != nil {
				return err
			}
			if err := latest.SetPreferredIDE(preferred); err != nil {
				return err
			}
			if err := h.Save(latest); err != nil {
				return err
			}
			*cfg = *latest
			return nil
		},
		OpenIDE: func(en registry.Entry) error {
			p, err := openProject(en)
			if err != nil {
				return err
			}
			return ide.Open(cfg.IDE(), p.Root)
		},
		OpenTerminal: func(en registry.Entry) error {
			p, err := openProject(en)
			if err != nil {
				return err
			}
			return terminal.Open(p.Root)
		},
		PreferredHarness: cfg.Harness,
		SetPreferredHarness: func(harness string) error {
			latest, err := h.Load()
			if err != nil {
				return err
			}
			if err := latest.SetPreferredHarness(harness); err != nil {
				return err
			}
			if err := h.Save(latest); err != nil {
				return err
			}
			*cfg = *latest
			return nil
		},
		Load: func() ([]registry.Entry, error) { return refresh.Entries(h, cfg, time.Now()) },
		Scan: func() (*registry.Index, error) { return refresh.Derived(cfg, time.Now()) },
		Refresh: func() (*registry.Index, error) {
			_, ix, err := refresh.All(h, cfg, time.Now())
			return ix, err
		},
		InitProject: func(choice InitProject) (*project.InitResult, error) {
			mode := project.Mode("")
			if choice.Mode != "" {
				var err error
				if mode, err = project.ParseMode(choice.Mode); err != nil {
					return nil, err
				}
			}
			opts := project.Options{Name: choice.Name, Description: choice.Description, Mode: mode, NoGit: choice.NoGit, NoThreads: choice.NoThreads}
			return manage.Init(h, cfg, choice.Work, opts, c, c != nil)
		},
		EditProject: func(en registry.Entry, edit manage.Edit) error {
			return manage.EditProject(cfg, en.Path, edit, time.Now())
		},
		Sync: func(en registry.Entry) (*mirror.Result, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, err
			}
			ix, err := registry.Scan(cfg)
			if err != nil {
				return nil, err
			}
			return mirror.Sync(p, ix, time.Now())
		},
		ForgetProject: func(en registry.Entry) error { return manage.Forget(h, cfg, en.Path) },
		StagePlan: func(en registry.Entry, given []string) (*capture.StagePlan, error) {
			v, err := project.Open(en.Path)
			if err != nil {
				return nil, err
			}
			sources, err := capture.SourcesFor(v, given)
			if err != nil {
				return nil, err
			}
			return capture.PlanStage(v, sources, time.Now())
		},
		Stage: func(en registry.Entry, plan *capture.StagePlan) (*capture.StageResult, []string, error) {
			v, err := project.Open(en.Path)
			if err != nil {
				return nil, nil, err
			}
			res, err := capture.ApplyStage(v, plan, time.Now())
			if err != nil {
				return res, nil, err
			}
			return res, res.Remembered, nil
		},
		Sources: func(en registry.Entry) []string {
			v, err := project.Open(en.Path)
			if err != nil {
				return nil
			}
			return capture.Sources(v)
		},
		StageProject: func(en registry.Entry) (*capture.ProjectStage, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, err
			}
			return capture.StageProject(p, en, time.Now())
		},
		Threads: func(en registry.Entry) (*threads.Board, []string, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, nil, err
			}
			board, err := threads.Load(p)
			if err != nil {
				return nil, nil, err
			}
			return board, threads.Notes(p), nil
		},
		StartThread: func(en registry.Entry, n threads.New) (*threads.Thread, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, err
			}
			return threads.Start(p, n, time.Now())
		},
		FileThread: func(en registry.Entry, id string, f threads.Filing) (*threads.Thread, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, err
			}
			return threads.File(p, id, f, time.Now())
		},
		SetThread: func(en registry.Entry, id string, ch threads.Changes) (*threads.Thread, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, err
			}
			return threads.Set(p, id, ch, time.Now())
		},
		Reopen: func(en registry.Entry, id string) (*threads.Thread, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, err
			}
			return threads.Reopen(p, id, time.Now())
		},
		AddPhase: func(en registry.Entry, title, goal string, order *int) (*threads.Phase, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, err
			}
			return threads.CreatePhase(p, title, goal, order, time.Now())
		},
		RenamePhase: func(en registry.Entry, old, title string) (*threads.Phase, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, err
			}
			return threads.RenamePhase(p, old, title, time.Now())
		},
		OrderPhase: func(en registry.Entry, title string, order int) (*threads.Phase, error) {
			p, err := openProject(en)
			if err != nil {
				return nil, err
			}
			return threads.ReorderPhase(p, title, order, time.Now())
		},
		RemovePhase: func(en registry.Entry, title string) error {
			p, err := openProject(en)
			if err != nil {
				return err
			}
			return threads.RemovePhase(p, title, time.Now())
		},
	}
}
