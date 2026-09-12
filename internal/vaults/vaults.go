// Package vaults creates claude-obsidian vaults and registers them as tree leaves.
package vaults

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nathanaday/claude-atlas/internal/console"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/product"
	"github.com/nathanaday/claude-atlas/internal/tree"
)

var ErrCancelled = errors.New("cancelled")

// ResolveNewPath puts a bare name under the vaults directory; anything path-like is a path.
func ResolveNewPath(arg, vaultsDir string) (string, error) {
	if strings.Contains(arg, string(filepath.Separator)) || strings.HasPrefix(arg, "~") || strings.HasPrefix(arg, ".") {
		return filepath.Abs(home.Expand(arg))
	}
	return filepath.Abs(filepath.Join(vaultsDir, arg))
}

// Create runs claude-obsidian's plan-then-apply init as one reviewed step.
func Create(p *product.Product, path string, c *console.Console, confirm bool) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; use `claude-atlas vault add` to register an existing vault", home.Display(path))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	generatedAt := product.NowUTC()
	operation := product.OperationID("init", generatedAt)
	plan, err := p.InitPlan(path, generatedAt, operation)
	if err != nil {
		return err
	}
	if plan.Status != "dry-run" {
		return fmt.Errorf("unexpected init plan status %q", plan.Status)
	}
	if confirm {
		c.Say("claude-obsidian will create %s with %d files:", home.Display(path), len(plan.ChangedPaths))
		for _, item := range plan.ChangedPaths {
			c.Say("    %s", item)
		}
		c.Say("")
		ok, err := c.Confirm("Create this vault?", true)
		if err != nil {
			return err
		}
		if !ok {
			return ErrCancelled
		}
	}
	return p.InitApply(path, generatedAt, operation, plan.Approval)
}

// RegisterOptions are the authored fields for a new leaf.
type RegisterOptions struct {
	Name     string
	Parent   string
	Purpose  string
	Priority string
}

// Register adds a leaf pointing at an existing claude-obsidian vault.
func Register(cfg *home.Config, vault string, opts RegisterOptions) (*tree.Node, error) {
	vault, err := filepath.Abs(home.Expand(vault))
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(vault, ".claude-obsidian.json")); err != nil {
		return nil, fmt.Errorf("%s is not a claude-obsidian vault (no .claude-obsidian.json)", home.Display(vault))
	}
	root := cfg.TreeRoot()
	nodes, err := tree.Walk(root)
	if err != nil {
		return nil, err
	}
	if existing := tree.FindByVault(nodes, vault); existing != nil {
		return nil, fmt.Errorf("%s is already registered as %s", home.Display(vault), existing.Rel)
	}
	name := opts.Name
	if name == "" {
		name = filepath.Base(vault)
	}
	id, err := tree.Slugify(name)
	if err != nil {
		return nil, err
	}
	dir, err := tree.CreateLeaf(root, tree.LeafOptions{
		ID: id, Name: name, Vault: vault, Parent: opts.Parent, Purpose: opts.Purpose, Priority: opts.Priority,
	})
	if err != nil {
		return nil, err
	}
	return tree.Load(dir, root)
}
