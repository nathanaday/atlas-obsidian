package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nathanaday/atlas-obsidian/internal/actions"
	"github.com/nathanaday/atlas-obsidian/internal/capture"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/manage"
	"github.com/nathanaday/atlas-obsidian/internal/mirror"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

// The atlas tools: the whole atlas as one read, and the writes that configure it. They
// bind the same functions the view calls, once per call over a config loaded for that
// call, because the CLI in another process may change config.json between two calls. A
// write ends in a refresh, so the stored registry `atlas-obsidian list` and the view read
// carries the change, and the tool answers from the index that refresh derived. They
// work with no place at all, because init is how a plain folder becomes a project.

// bind loads the config and binds the atlas actions for one call.
func (s *Server) bind() (actions.Atlas, *home.Config, error) {
	cfg, err := s.home().Load()
	if err != nil {
		return actions.Atlas{}, nil, err
	}
	return actions.Bind(s.home(), cfg, nil), cfg, nil
}

// entryOf resolves a project a tool names: by name, by id, or by path. One the atlas
// cannot read is an error that says why.
func entryOf(ix *registry.Index, arg string) (registry.Entry, error) {
	en, err := anyEntryOf(ix, arg)
	if err != nil {
		return registry.Entry{}, err
	}
	if en.Error != "" {
		return registry.Entry{}, fmt.Errorf("%s: %s", home.Display(en.Path), en.Error)
	}
	return en, nil
}

// anyEntryOf is entryOf for a project the atlas cannot read too, so forget can drop it.
func anyEntryOf(ix *registry.Index, arg string) (registry.Entry, error) {
	if strings.TrimSpace(arg) == "" {
		return registry.Entry{}, errors.New("name a project: its name, id, or path")
	}
	found, err := ix.Find(arg)
	if err == nil {
		return *found, nil
	}
	if !errors.Is(err, registry.ErrNotFound) {
		return registry.Entry{}, err
	}
	abs, aerr := filepath.Abs(home.Expand(arg))
	for i := range ix.Entries {
		e := &ix.Entries[i]
		if e.Error == "" {
			continue
		}
		if (aerr == nil && e.Path == abs) || strings.EqualFold(filepath.Base(e.Path), arg) {
			return *e, nil
		}
	}
	return registry.Entry{}, fmt.Errorf("no project named %q; the atlas tool lists them", arg)
}

// Settings are the atlas settings a session may read and set.
type Settings struct {
	NewDays int `json:"new_days"`
}

func settingsOf(cfg *home.Config) Settings {
	return Settings{NewDays: cfg.NewDays()}
}

type AtlasArgs struct {
	Refresh bool `json:"refresh,omitempty" jsonschema:"also rewrite the registry: what atlas-obsidian refresh does"`
}

// AtlasOut is the whole atlas: every project with its state, the folders the atlas cannot
// read, and the settings.
type AtlasOut struct {
	Projects []registry.Entry   `json:"projects"`
	Problems []registry.Problem `json:"problems"`
	Settings Settings           `json:"settings"`
}

func (s *Server) atlasTool(ctx context.Context, req *mcp.CallToolRequest, a AtlasArgs) (*mcp.CallToolResult, AtlasOut, error) {
	acts, cfg, err := s.bind()
	if err != nil {
		return nil, AtlasOut{}, err
	}
	var ix *registry.Index
	if a.Refresh {
		ix, err = acts.Refresh()
	} else {
		ix, err = acts.Scan()
	}
	if err != nil {
		return nil, AtlasOut{}, err
	}
	out := AtlasOut{Projects: ix.Projects(), Problems: ix.Problems, Settings: settingsOf(cfg)}
	if out.Projects == nil {
		out.Projects = []registry.Entry{}
	}
	if out.Problems == nil {
		out.Problems = []registry.Problem{}
	}
	return nil, out, nil
}

type ProjectToolArgs struct {
	Action        string   `json:"action" jsonschema:"init, edit, sync, or forget"`
	Work          string   `json:"work,omitempty" jsonschema:"the project's folder: on init the folder that becomes one, default the session's folder; otherwise the project by name, id, or path, default the session's project"`
	Name          string   `json:"name,omitempty" jsonschema:"init: the project's name, default the folder's; edit: the new name, which renames atlas/<name>/ too"`
	Description   *string  `json:"description,omitempty" jsonschema:"init, edit: one to three sentences saying what the work is and what its wiki should remember; the ingest and query skills read it. On edit an empty string clears it"`
	Mode          string   `json:"mode,omitempty" jsonschema:"init, edit: the filing mode for new wiki pages, generic (default, a folder per type) or lyt (atomic notes and Maps of Content)"`
	NoGit         bool     `json:"no_git,omitempty" jsonschema:"init: leave a folder that is in no git repository without one. The wiki then has no history and no operation can run"`
	Threads       *bool    `json:"threads,omitempty" jsonschema:"init, edit: whether the project tracks threads (default true on init). Off, the thread tools refuse and the session hook lists none; a threads/ folder that exists stays as it is"`
	AddMembers    []string `json:"add_members,omitempty" jsonschema:"edit: projects whose wikis this one mirrors under wiki/projects/, each by name, id, or path. Refused when one is this project, is unknown, or would close a cycle"`
	RemoveMembers []string `json:"remove_members,omitempty" jsonschema:"edit: members to drop, each by name, id, or path; the next sync removes their mirrors"`
}

// ProjectToolOut is the project as the atlas sees it after the change, the files init
// wrote, or the path forget dropped.
type ProjectToolOut struct {
	Project   *registry.Entry `json:"project,omitempty"`
	Written   []string        `json:"written,omitempty" jsonschema:"init: what was written under atlas/<name>/"`
	Git       string          `json:"git,omitempty" jsonschema:"init: created (the work is now a repository), existing, enclosed (the work sits inside another repository), or skipped"`
	Commit    string          `json:"commit,omitempty" jsonschema:"init: the setup commit"`
	Forgotten string          `json:"forgotten,omitempty" jsonschema:"the path the atlas no longer lists; the folder and its atlas/<name>/ stay"`
	Sync      *mirror.Result  `json:"sync,omitempty" jsonschema:"sync: each project mirrored, with its pages and any error, and the counts and the commit when something changed"`
}

func (s *Server) projectTool(ctx context.Context, req *mcp.CallToolRequest, a ProjectToolArgs) (*mcp.CallToolResult, ProjectToolOut, error) {
	acts, _, err := s.bind()
	if err != nil {
		return nil, ProjectToolOut{}, err
	}
	if a.Action == "init" {
		work := a.Work
		if work == "" {
			work = s.opts.ProjectDir
		}
		if work == "" {
			return nil, ProjectToolOut{}, errors.New("init needs work: the folder that becomes a project")
		}
		choice := actions.InitProject{Work: home.Expand(work), Name: a.Name, Mode: a.Mode, NoGit: a.NoGit, NoThreads: a.Threads != nil && !*a.Threads}
		if a.Description != nil {
			choice.Description = *a.Description
		}
		made, err := acts.InitProject(choice)
		if err != nil {
			return nil, ProjectToolOut{}, err
		}
		res, out, err := s.projectOut(acts, made.Project.Root)
		out.Written, out.Git, out.Commit = made.Written, string(made.Git), made.Commit
		return res, out, err
	}
	ix, err := acts.Scan()
	if err != nil {
		return nil, ProjectToolOut{}, err
	}
	var en registry.Entry
	if a.Work != "" {
		if a.Action == "forget" {
			en, err = anyEntryOf(ix, a.Work)
		} else {
			en, err = entryOf(ix, a.Work)
		}
		if err != nil {
			return nil, ProjectToolOut{}, err
		}
	} else {
		pl, err := s.where()
		if err != nil || pl.Project == nil {
			return nil, ProjectToolOut{}, errors.New("name the project with work; this session is not in one")
		}
		found := ix.ByPath(pl.Project.Root)
		if found == nil || found.Error != "" {
			return nil, ProjectToolOut{}, fmt.Errorf("the atlas does not list %s; start a session in it, or pass work", pl.Project.Root)
		}
		en = *found
	}
	switch a.Action {
	case "sync":
		res, err := acts.Sync(en)
		if err != nil {
			return nil, ProjectToolOut{}, err
		}
		_, out, err := s.projectOut(acts, en.Path)
		out.Sync = res
		return nil, out, err
	case "edit":
		edit := manage.Edit{Name: a.Name, Description: a.Description, Threads: a.Threads, AddMembers: a.AddMembers, RemoveMembers: a.RemoveMembers}
		if a.Mode != "" {
			mode, err := project.ParseMode(a.Mode)
			if err != nil {
				return nil, ProjectToolOut{}, err
			}
			edit.Mode = mode
		}
		if len(edit.Fields()) == 0 {
			return nil, ProjectToolOut{}, errors.New("edit needs name, description, mode, threads, add_members, or remove_members")
		}
		if err := acts.EditProject(en, edit); err != nil {
			return nil, ProjectToolOut{}, err
		}
	case "forget":
		if err := acts.ForgetProject(en); err != nil {
			return nil, ProjectToolOut{}, err
		}
		if _, err := acts.Refresh(); err != nil {
			return nil, ProjectToolOut{}, err
		}
		return nil, ProjectToolOut{Forgotten: en.Path}, nil
	default:
		return nil, ProjectToolOut{}, fmt.Errorf("action must be init, edit, sync, or forget, not %q", a.Action)
	}
	return s.projectOut(acts, en.Path)
}

// projectOut rewrites the registry after a write and returns the project at work as the
// atlas now sees it.
func (s *Server) projectOut(acts actions.Atlas, work string) (*mcp.CallToolResult, ProjectToolOut, error) {
	ix, err := acts.Refresh()
	if err != nil {
		return nil, ProjectToolOut{}, err
	}
	en := ix.ByPath(work)
	if en == nil {
		return nil, ProjectToolOut{}, fmt.Errorf("%s was written but the scan does not list it; call atlas with refresh", home.Display(work))
	}
	return nil, ProjectToolOut{Project: en}, nil
}

type SettingsArgs struct {
	NewDays *int `json:"new_days,omitempty" jsonschema:"an entry is new for this many days after its creation; 0 turns it off"`
}

func (s *Server) settingsTool(ctx context.Context, req *mcp.CallToolRequest, a SettingsArgs) (*mcp.CallToolResult, Settings, error) {
	acts, cfg, err := s.bind()
	if err != nil {
		return nil, Settings{}, err
	}
	if a.NewDays != nil {
		if err := cfg.SetNewDays(*a.NewDays); err != nil {
			return nil, Settings{}, err
		}
		if err := s.home().Save(cfg); err != nil {
			return nil, Settings{}, err
		}
		if _, err := acts.Refresh(); err != nil {
			return nil, Settings{}, err
		}
	}
	return nil, settingsOf(cfg), nil
}

type StageArgs struct {
	Paths    []string `json:"paths,omitempty" jsonschema:"files or folders outside the project; omit to stage what is new in the folders it staged from before"`
	Snapshot bool     `json:"snapshot,omitempty" jsonschema:"write a snapshot of the work into the inbox (its AGENTS.md, CLAUDE.md, README, file list, docs headings, TODO and FIXME lines, and the log since the page describing it was written) for the wiki-describe skill; not with paths"`
	DryRun   bool     `json:"dry_run,omitempty" jsonschema:"plan only: say what would be copied and copy nothing"`
}

// StageOut is the plan, and after a copy, what was copied and the folders the project now
// stages from when paths is omitted; or the snapshot a stage of the work wrote.
type StageOut struct {
	Project    string                `json:"project"`
	Plan       *capture.StagePlan    `json:"plan,omitempty"`
	Result     *capture.StageResult  `json:"result,omitempty"`
	Remembered []string              `json:"remembered,omitempty"`
	Snapshot   *capture.ProjectStage `json:"snapshot,omitempty"`
}

func (s *Server) stageTool(ctx context.Context, req *mcp.CallToolRequest, a StageArgs) (*mcp.CallToolResult, StageOut, error) {
	pl, p, err := s.place()
	if err != nil {
		return nil, StageOut{}, err
	}
	acts, _, err := s.bind()
	if err != nil {
		return nil, StageOut{}, err
	}
	if pl.Index == nil {
		return nil, StageOut{}, errors.New("no atlas config on this machine; run atlas-obsidian setup")
	}
	en := pl.Index.ByPath(p.Root)
	if en == nil || en.Error != "" {
		return nil, StageOut{}, fmt.Errorf("the atlas does not list %s; start a session in it, or run `atlas-obsidian init`", p.Name())
	}
	out := StageOut{Project: p.Root}
	if a.Snapshot {
		if len(a.Paths) > 0 {
			return nil, StageOut{}, errors.New("stage takes snapshot or paths, not both")
		}
		if a.DryRun {
			return nil, StageOut{}, errors.New("a snapshot has no dry run; it writes one file into the inbox or finds it already there")
		}
		snap, err := acts.StageProject(*en)
		if err != nil {
			return nil, StageOut{}, err
		}
		if snap.New {
			if _, err := acts.Refresh(); err != nil {
				return nil, StageOut{}, err
			}
		}
		out.Snapshot = snap
		return nil, out, nil
	}
	plan, err := acts.StagePlan(*en, a.Paths)
	if err != nil {
		return nil, StageOut{}, err
	}
	out.Plan = plan
	if a.DryRun {
		return nil, out, nil
	}
	res, remembered, err := acts.Stage(*en, plan)
	if err != nil {
		return nil, StageOut{}, err
	}
	// The inbox count is part of the derived state, so the registry is rewritten.
	if _, err := acts.Refresh(); err != nil {
		return nil, StageOut{}, err
	}
	out.Result, out.Remembered = res, remembered
	return nil, out, nil
}
