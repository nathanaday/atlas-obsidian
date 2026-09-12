package vaults

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/tree"
)

// Edit is a set of changes to one project. Empty strings mean "unchanged" except
// Purpose, which may be cleared by setting ClearPurpose.
type Edit struct {
	Name         string
	Purpose      string
	ClearPurpose bool
	Priority     string
	State        string
	Category     *string // nil: unchanged; "" : top level
	Vault        string  // new vault path; "" unchanged
	MoveVault    bool    // move the directory on disk to Vault
}

// Update applies an Edit: frontmatter first, then the vault, then the page's category.
func Update(cfg *home.Config, p *tree.Project, edit Edit) error {
	fields := map[string]string{}
	if edit.Name != "" && edit.Name != p.Name {
		fields["name"] = edit.Name
	}
	if edit.ClearPurpose {
		fields["purpose"] = ""
	} else if edit.Purpose != "" && edit.Purpose != p.Purpose {
		fields["purpose"] = edit.Purpose
	}
	if edit.Priority != "" && edit.Priority != p.Priority {
		fields["priority"] = edit.Priority
	}
	if edit.State != "" && edit.State != p.State {
		fields["state"] = edit.State
	}
	if edit.Vault != "" {
		target, err := filepath.Abs(home.Expand(edit.Vault))
		if err != nil {
			return err
		}
		if target != p.VaultPath() {
			if edit.MoveVault {
				if _, err := os.Stat(target); err == nil {
					return fmt.Errorf("%s already exists; cannot move the vault there", home.Display(target))
				}
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				if err := os.Rename(p.VaultPath(), target); err != nil {
					return fmt.Errorf("move vault: %w", err)
				}
			} else if _, err := os.Stat(filepath.Join(target, ".claude-obsidian.json")); err != nil {
				return fmt.Errorf("%s is not a claude-obsidian vault", home.Display(target))
			}
			fields["vault"] = target
		}
	}
	if len(fields) > 0 {
		if err := tree.UpdateFrontmatter(p.Path, fields); err != nil {
			return err
		}
	}
	if edit.Category != nil && *edit.Category != p.Category() {
		if _, err := tree.Move(cfg.TreeRoot(), p, *edit.Category); err != nil {
			return err
		}
	}
	return nil
}

// Unlink removes a project from the atlas. The vault stays on disk.
func Unlink(p *tree.Project) error {
	return tree.Unlink(p)
}
