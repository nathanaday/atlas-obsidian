// Package pages writes the static orientation pages into the atlas vault.
//
// The pages are markdown templates under templates/. Placeholders: %REPO%, %VERSION%,
// %VAULTS%, %ATLAS%, %HOME%.
package pages

import (
	"embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/nathanaday/claude-atlas/internal/home"
)

// Repository is the project's home on GitHub.
const Repository = "https://github.com/nathanaday/claude-atlas"

//go:embed templates/*.md
var templates embed.FS

// Write renders every template into the atlas vault.
func Write(cfg *home.Config, homeRoot, version string) error {
	r := strings.NewReplacer(
		"%REPO%", Repository,
		"%VERSION%", version,
		"%VAULTS%", home.Display(cfg.VaultsDir),
		"%ATLAS%", home.Display(cfg.AtlasVault),
		"%HOME%", home.Display(homeRoot),
	)
	entries, err := templates.ReadDir("templates")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		text, err := templates.ReadFile("templates/" + entry.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(cfg.AtlasVault, entry.Name()), []byte(r.Replace(string(text))), 0o644); err != nil {
			return err
		}
	}
	return nil
}
