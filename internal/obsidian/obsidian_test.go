package obsidian

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryRoundTripKeepsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OBSIDIAN_CONFIG_DIR", dir)
	os.WriteFile(filepath.Join(dir, "obsidian.json"), []byte(`{"vaults":{"abc":{"path":"/v/one","ts":1,"open":true}},"insider":true}`), 0o644)
	reg, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := reg.Find("/v/one/"); !ok {
		t.Fatal("existing vault not found")
	}
	id, err := reg.Register("/v/two")
	if err != nil || len(id) != 16 {
		t.Fatalf("register: %q %v", id, err)
	}
	again, err := reg.Register("/v/two")
	if err != nil || again != id {
		t.Fatalf("second register should return the same id: %q %v", again, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "obsidian.json"))
	var back map[string]any
	json.Unmarshal(data, &back)
	if back["insider"] != true {
		t.Fatal("unknown top-level field was dropped")
	}
	vaults := back["vaults"].(map[string]any)
	if len(vaults) != 2 || vaults["abc"].(map[string]any)["open"] != true {
		t.Fatalf("vaults after save: %v", vaults)
	}
}

func TestMissingRegistry(t *testing.T) {
	t.Setenv("OBSIDIAN_CONFIG_DIR", t.TempDir())
	if _, err := LoadRegistry(); err == nil {
		t.Fatal("expected an error for a missing registry")
	}
}

func TestOpenURI(t *testing.T) {
	if got := OpenURI("/Users/me/My Vault"); got != "obsidian://open?path=%2FUsers%2Fme%2FMy+Vault" {
		t.Fatalf("got %s", got)
	}
}
