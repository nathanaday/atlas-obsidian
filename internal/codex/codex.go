// Package codex reads plugin installation state through the Codex CLI.
package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Install struct {
	PluginID    string `json:"pluginId"`
	Name        string `json:"name"`
	Marketplace string `json:"marketplaceName"`
	Version     string `json:"version"`
	Enabled     bool   `json:"enabled"`
}

// InstalledPlugin asks only the named marketplace, avoiding unrelated remote catalogs.
func InstalledPlugin(id string) (*Install, error) {
	name, marketplace, ok := strings.Cut(id, "@")
	if !ok {
		return nil, fmt.Errorf("invalid plugin id %q: expected name@marketplace", id)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "codex", "plugin", "list", "--json", "--marketplace", marketplace).Output()
	if err != nil {
		return nil, fmt.Errorf("codex plugin list: %w", err)
	}
	var result struct {
		Installed []Install `json:"installed"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("codex plugin list: %w", err)
	}
	for _, inst := range result.Installed {
		if inst.PluginID == id || (inst.Name == name && inst.Marketplace == marketplace) {
			return &inst, nil
		}
	}
	return nil, nil
}
