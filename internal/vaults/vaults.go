// Package vaults creates and adopts vaults and registers them as project pages in the tree.
package vaults

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/claude-atlas/internal/console"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/tree"
	"github.com/nathanaday/claude-atlas/internal/vault"
)

var ErrCancelled = errors.New("cancelled")

// ResolveNewPath puts a bare name under the vaults directory; anything path-like is a path.
func ResolveNewPath(arg, vaultsDir string) (string, error) {
	if strings.Contains(arg, string(filepath.Separator)) || strings.HasPrefix(arg, "~") || strings.HasPrefix(arg, ".") {
		return filepath.Abs(home.Expand(arg))
	}
	return filepath.Abs(filepath.Join(vaultsDir, arg))
}

// Create makes a new vault at path after showing what it will contain.
func Create(path string, mode vault.Mode, c *console.Console, confirm bool) (*vault.InitResult, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("%s already exists; use `claude-atlas adopt` for an existing vault", home.Display(path))
	}
	if mode == "" {
		mode = vault.Generic
	}
	if confirm {
		files := append(vault.TemplateFiles(), vault.Marker, vault.LedgerPath)
		c.Say("claude-atlas will create %s (%s mode) with %d files and a git repository:", home.Display(path), mode, len(files))
		for _, item := range files {
			c.Say("    %s", item)
		}
		c.Say("")
		ok, err := c.Confirm("Create this vault?", true)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrCancelled
		}
	}
	return vault.Init(path, mode, time.Now())
}

// RegisterOptions are the authored fields for a new project.
type RegisterOptions struct {
	Name     string
	Category string
	Purpose  string
	Priority string
}

// Register adds a project page pointing at an existing vault.
func Register(cfg *home.Config, root string, opts RegisterOptions) (*tree.Project, error) {
	root, err := filepath.Abs(home.Expand(root))
	if err != nil {
		return nil, err
	}
	if !vault.IsVault(root) && !vault.IsLegacy(root) {
		return nil, fmt.Errorf("%s is not a claude-atlas vault (no %s); create one with `claude-atlas new-vault` or adopt it with `claude-atlas adopt`", home.Display(root), vault.Marker)
	}
	treeRoot := cfg.TreeRoot()
	projects, _, err := tree.Walk(treeRoot)
	if err != nil {
		return nil, err
	}
	if existing := tree.FindByVault(projects, root); existing != nil {
		return nil, fmt.Errorf("%s is already registered as %s", home.Display(root), existing.Rel)
	}
	name := opts.Name
	if name == "" {
		name = filepath.Base(root)
	}
	id, err := tree.Slugify(name)
	if err != nil {
		return nil, err
	}
	path, err := tree.Create(treeRoot, tree.ProjectOptions{
		ID: id, Name: name, Vault: root, Category: opts.Category, Purpose: opts.Purpose, Priority: opts.Priority,
	})
	if err != nil {
		return nil, err
	}
	return tree.Load(path, treeRoot)
}
