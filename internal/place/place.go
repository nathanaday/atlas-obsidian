// Package place finds where a session is: in a project, anywhere inside its work. The
// hooks, the MCP server, and the CLI resolve a session the same way through it, and a
// session heals its own entry in the atlas config.
package place

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/manage"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

// EnvPlace names the project explicitly for the MCP server and the hooks. It is the
// variable the launcher sets.
const EnvPlace = project.EnvProject

// Place is the project a session is in, with what the atlas knows about it.
type Place struct {
	Project *project.Project
	// Entry is the registry's entry for the project, with the whole index behind it; nil
	// when there is no atlas config or the scan failed.
	Entry *registry.Entry
	Index *registry.Index
	// Heal is what registering the project did to the config, when asked to.
	Heal manage.Heal
	// ConfigError says why Entry is nil when the config could not be read.
	ConfigError string
}

// ErrNoPlace means the session is not in a project.
var ErrNoPlace = errors.New("not in a atlas-obsidian project")

// Resolve finds the project: the explicit path, then the environment, then the nearest
// project at or above start. An explicit path is the work folder, or anywhere inside it.
// With register set, the session heals the atlas config so it lists the project at this
// path.
func Resolve(h home.Home, explicit, envValue, start string, register bool) (*Place, error) {
	for _, given := range []string{explicit, envValue} {
		if given == "" {
			continue
		}
		abs, err := filepath.Abs(home.Expand(given))
		if err != nil {
			return nil, err
		}
		return from(h, abs, register)
	}
	if start == "" {
		return nil, ErrNoPlace
	}
	return from(h, start, register)
}

// from resolves the project at or above dir.
func from(h home.Home, dir string, register bool) (*Place, error) {
	if work := project.FindAbove(dir); work != "" {
		return resolve(h, work, register)
	}
	if known := project.KnowledgeAbove(dir); known != "" {
		return nil, fmt.Errorf("%w: %s is a knowledge base of 3.x; run atlas-obsidian upgrade %s to make it a project", ErrNoPlace, home.Display(known), home.Display(known))
	}
	return nil, fmt.Errorf("%w: none at or above %s", ErrNoPlace, home.Display(dir))
}

func resolve(h home.Home, work string, register bool) (*Place, error) {
	p, err := project.Open(work)
	if err != nil {
		return nil, err
	}
	out := &Place{Project: p}
	cfg, err := h.Load()
	if err != nil {
		if errors.Is(err, home.ErrNoAtlas) {
			out.ConfigError = "no atlas config on this machine; run atlas-obsidian setup"
			return out, nil
		}
		return nil, err
	}
	if register {
		if out.Heal, err = manage.RegisterProject(h, cfg, p); err != nil {
			return nil, err
		}
	}
	ix, err := registry.Scan(cfg)
	if err != nil {
		return nil, err
	}
	out.Index = ix
	if e := ix.ByPath(work); e != nil && e.Error == "" {
		out.Entry = e
	}
	return out, nil
}

// Cwd is the working directory, or "" when it cannot be read.
func Cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}
