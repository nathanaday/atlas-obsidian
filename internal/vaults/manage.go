package vaults

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/links"
	"github.com/nathanaday/claude-atlas/internal/tree"
	"github.com/nathanaday/claude-atlas/internal/vault"
)

// Edit is a set of changes to one project. Empty strings mean "unchanged" except
// Purpose, which may be cleared by setting ClearPurpose. Pointer fields are unchanged
// when nil and cleared when they point at "".
type Edit struct {
	Name             string
	Purpose          string
	ClearPurpose     bool
	Priority         string
	State            string
	BlockedOn        *string
	ReviewAfter      *string // YYYY-MM-DD or ""
	DefinitionOfDone *string
	Category         *string // nil: unchanged; "" : top level
	Vault            string  // new vault path; "" unchanged
	MoveVault        bool    // move the directory on disk to Vault
	Repos            *[]string
	Materials        *[]string
}

// ValidReviewDate reports whether s is empty or a YYYY-MM-DD date.
func ValidReviewDate(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// Update applies an Edit: frontmatter first, then the vault, then the page's category.
func Update(cfg *home.Config, p *tree.Project, edit Edit) error {
	fields := map[string]any{}
	if edit.Priority != "" && !contains(tree.Priorities, edit.Priority) {
		return fmt.Errorf("priority must be one of %s", strings.Join(tree.Priorities, ", "))
	}
	if edit.State != "" && !contains(tree.States, edit.State) {
		return fmt.Errorf("state must be one of %s", strings.Join(tree.States, ", "))
	}
	if edit.ReviewAfter != nil && !ValidReviewDate(*edit.ReviewAfter) {
		return fmt.Errorf("review_after must be a date like 2026-10-01")
	}
	if edit.BlockedOn != nil && *edit.BlockedOn != p.BlockedOn {
		fields["blocked_on"] = *edit.BlockedOn
	}
	if edit.ReviewAfter != nil && *edit.ReviewAfter != p.ReviewAfter {
		fields["review_after"] = *edit.ReviewAfter
	}
	if edit.DefinitionOfDone != nil && *edit.DefinitionOfDone != p.DefinitionOfDone {
		fields["definition_of_done"] = *edit.DefinitionOfDone
	}
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
	if edit.Repos != nil {
		fields["repos"] = orEmpty(*edit.Repos)
	}
	if edit.Materials != nil {
		fields["materials"] = orEmpty(*edit.Materials)
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
			} else if !vault.IsVault(target) && !vault.IsLegacy(target) {
				return fmt.Errorf("%s is not a claude-atlas vault", home.Display(target))
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

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func orEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// SetLinks replaces a project's repo and materials lists.
func SetLinks(p *tree.Project, repos, materials []string) error {
	if repos == nil {
		repos = []string{}
	}
	if materials == nil {
		materials = []string{}
	}
	return tree.UpdateFrontmatter(p.Path, map[string]any{"repos": repos, "materials": materials})
}

// AddLink records a folder on a project page; kind is links.Repo or links.Materials.
func AddLink(p *tree.Project, kind, path string) error {
	abs, err := filepath.Abs(home.Expand(path))
	if err != nil {
		return err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a directory", home.Display(abs))
	}
	for _, existing := range append(append([]string{}, p.Repos...), p.Materials...) {
		if filepath.Clean(home.Expand(existing)) == abs {
			return fmt.Errorf("%s is already linked", home.Display(abs))
		}
	}
	repos, materials := p.Repos, p.Materials
	if kind == links.Repo {
		repos = append(repos, abs)
	} else {
		materials = append(materials, abs)
	}
	return SetLinks(p, repos, materials)
}

// RemoveLink drops a folder from a project page, whichever list holds it.
func RemoveLink(p *tree.Project, path string) error {
	target := filepath.Clean(home.Expand(path))
	keep := func(list []string) ([]string, bool) {
		var out []string
		found := false
		for _, item := range list {
			if filepath.Clean(home.Expand(item)) == target {
				found = true
				continue
			}
			out = append(out, item)
		}
		return out, found
	}
	repos, inRepos := keep(p.Repos)
	materials, inMaterials := keep(p.Materials)
	if !inRepos && !inMaterials {
		return fmt.Errorf("%s is not linked to %s", home.Display(target), p.Name)
	}
	return SetLinks(p, repos, materials)
}

// Unlink removes a project from the atlas. The vault stays on disk.
func Unlink(p *tree.Project) error {
	return tree.Unlink(p)
}
