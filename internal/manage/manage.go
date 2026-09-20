// Package manage creates projects, changes their own facts, and keeps the atlas config
// pointing at them. Every write to the config on this machine goes through it.
package manage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/claude-atlas/internal/console"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/project"
)

var ErrCancelled = errors.New("cancelled")

// ResolvePath is the absolute path a project argument names. A bare name is a folder in
// the current directory, as for any other command that takes a path.
func ResolvePath(arg string) (string, error) {
	if strings.TrimSpace(arg) == "" {
		return "", errors.New("a project needs a path")
	}
	return filepath.Abs(home.Expand(arg))
}

// Init makes work a project and lists it in the config. With confirm set it shows what it
// will write and asks first.
func Init(h home.Home, cfg *home.Config, work string, opts project.Options, c *console.Console, confirm bool) (*project.InitResult, error) {
	abs, err := ResolvePath(work)
	if err != nil {
		return nil, err
	}
	if err := project.CheckNew(abs); err != nil {
		return nil, err
	}
	if opts.Mode == "" {
		opts.Mode = project.Generic
	}
	if confirm {
		if err := preview(c, abs, opts); err != nil {
			return nil, err
		}
	}
	res, err := project.Init(abs, opts, time.Now())
	if err != nil {
		return nil, err
	}
	if cfg.AddProject(abs) {
		if err := h.Save(cfg); err != nil {
			return res, err
		}
	}
	return res, nil
}

// preview lists what a new project will hold and asks to go ahead.
func preview(c *console.Console, work string, opts project.Options) error {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = filepath.Base(work)
	}
	folder := project.Dir + "/" + project.FolderName(name)
	files := append(project.TemplateFiles(), project.Marker, project.LedgerPath)
	c.Say("claude-atlas will make %s a project (%s mode), as %s/ with %d files %s:", home.Display(work), opts.Mode, folder, len(files), repoNote(work, opts))
	for _, item := range files {
		c.Say("    %s/%s", folder, item)
	}
	c.Say("")
	ok, err := c.Confirm("Make this folder a project?", true)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCancelled
	}
	return nil
}

// repoNote says where the project's wiki will commit: into the repository that holds the
// work, or into a new one the work gets.
func repoNote(work string, opts project.Options) string {
	if host, _ := project.HostFor(filepath.Join(work, project.Dir)); host != "" {
		return "in the git repository " + home.Display(host)
	}
	if opts.NoGit {
		return "with no git history, so no operation can run until the work is a repository"
	}
	return "and a git repository at " + home.Display(work)
}

// Edit changes a project's own facts. A zero field means unchanged.
type Edit struct {
	Name        string
	Description *string
	Mode        project.Mode
}

// Fields names what the edit touches, for the commit message.
func (e Edit) Fields() []string {
	var out []string
	if e.Name != "" {
		out = append(out, "name")
	}
	if e.Description != nil {
		out = append(out, "description")
	}
	if e.Mode != "" {
		out = append(out, "mode")
	}
	return out
}

// EditProject rewrites a project's identity file as one setup commit. A new name moves
// atlas/<name>/ to match; the work folder, which the config holds, never moves.
func EditProject(work string, edit Edit, now time.Time) error {
	fields := edit.Fields()
	if len(fields) == 0 {
		return nil
	}
	return project.UpdateConfig(work, "edit "+strings.Join(fields, ", "), now, func(c *project.Config) error {
		if edit.Name != "" {
			c.Name = edit.Name
		}
		if edit.Description != nil {
			c.Description = *edit.Description
		}
		if edit.Mode != "" {
			c.Mode = edit.Mode
		}
		return nil
	})
}

// Forget drops a path from the config: a project's work folder, or a knowledge base 3.x
// left behind. The folder itself stays.
func Forget(h home.Home, cfg *home.Config, path string) error {
	abs, err := filepath.Abs(home.Expand(path))
	if err != nil {
		return err
	}
	if !cfg.RemoveProject(abs) && !cfg.RemoveKnowledge(abs) {
		return fmt.Errorf("%s is not in the atlas", home.Display(abs))
	}
	return h.Save(cfg)
}

// Heal is what RegisterProject did to the config for the project a session started in.
type Heal string

const (
	HealNone  Heal = ""      // the config listed it at this path already
	HealMoved Heal = "moved" // the config listed its id, or its gone folder, at another path
	HealAdded Heal = "added" // the config did not list it
)

// RegisterProject makes sure the config lists the project at p.Root: it adds one the
// config does not know, as after a clone, and moves one whose id the config knows at
// another path, as after the work folder moved. A listed folder that is gone is taken
// for this project's old home when it is the only one gone; otherwise it stays until
// forget or a later heal, because it may be another project on a drive that is not
// mounted. It reports what it did.
func RegisterProject(h home.Home, cfg *home.Config, p *project.Project) (Heal, error) {
	if cfg.HasProject(p.Root) {
		return HealNone, nil
	}
	drop, heal := healPaths(cfg.Projects, p.Config.ID, func(path string) (string, bool) {
		c, ok := project.ReadConfig(path)
		return c.ID, ok
	})
	for _, path := range drop {
		cfg.RemoveProject(path)
	}
	cfg.AddProject(p.Root)
	return heal, h.Save(cfg)
}

// healPaths decides which listed paths an entry with id, found at a path the list does
// not hold, replaces: every path whose identity carries the id, else the one path that
// is gone when exactly one is.
func healPaths(listed []string, id string, readID func(string) (string, bool)) ([]string, Heal) {
	var same, gone []string
	for _, path := range listed {
		if other, ok := readID(path); ok {
			if other == id {
				same = append(same, path)
			}
			continue
		}
		if _, err := os.Stat(path); err != nil {
			gone = append(gone, path)
		}
	}
	switch {
	case len(same) > 0:
		return same, HealMoved
	case len(gone) == 1:
		return gone, HealMoved
	}
	return nil, HealAdded
}
