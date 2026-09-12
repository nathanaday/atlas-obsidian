// Package testutil locates a real claude-obsidian for integration tests.
package testutil

import (
	"os"
	"testing"

	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/product"
)

// EnvProduct names a claude-obsidian checkout for tests to use.
const EnvProduct = "CLAUDE_ATLAS_TEST_PRODUCT"

// Product returns an installed claude-obsidian or skips the test. Tests never install one.
func Product(t *testing.T) *product.Product {
	t.Helper()
	cfg := home.Home{}.Default("", "").ClaudeObsidian
	cfg.Path = os.Getenv(EnvProduct)
	p, err := product.Locate(cfg)
	if err != nil {
		t.Skipf("no claude-obsidian available (%v); set %s", err, EnvProduct)
	}
	return p
}
