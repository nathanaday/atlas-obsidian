package obsidian

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func registry(t *testing.T, content string) string {
	dir := t.TempDir()
	t.Setenv("OBSIDIAN_CONFIG_DIR", dir)
	if content != "" {
		os.WriteFile(filepath.Join(dir, "obsidian.json"), []byte(content), 0o644)
	}
	return filepath.Join(dir, "obsidian.json")
}

func TestRegisterAddsOneEntryAndKeepsTheRestVerbatim(t *testing.T) {
	path := registry(t, `{"vaults":{"abc":{"path":"/v/one","ts":1,"open":true,"future":"x"}},"insider":true}`)
	reg, err := LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Find("/v/one/"); !ok {
		t.Fatal("existing vault not found")
	}
	id, err := reg.Register("/v/two")
	if err != nil || len(id) != 16 {
		t.Fatalf("register: %q %v", id, err)
	}
	if again, _ := reg.Register("/v/two"); again != id {
		t.Fatal("second register should return the same id")
	}
	data, _ := os.ReadFile(path)
	var back map[string]any
	json.Unmarshal(data, &back)
	if back["insider"] != true {
		t.Fatal("unknown top-level field was dropped")
	}
	vaults := back["vaults"].(map[string]any)
	one := vaults["abc"].(map[string]any)
	if len(vaults) != 2 || one["open"] != true || one["future"] != "x" {
		t.Fatalf("vaults after save: %v", vaults)
	}
	two := vaults[id].(map[string]any)
	if two["path"] != "/v/two" || two["ts"] == nil {
		t.Fatalf("new entry: %v", two)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatal("backup missing")
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Fatal("temp file left behind")
	}
}

func TestLoadRefusesUnexpectedShapes(t *testing.T) {
	for _, content := range []string{
		`not json`,
		`{"vaults":[]}`,
		`{"vaults":{"abc":{"ts":1}}}`,
		`{"vaults":{"abc":{"path":"/v"}}}`,
		`{"vaults":{"abc":"nope"}}`,
	} {
		registry(t, content)
		_, err := LoadRegistry()
		if !errors.Is(err, ErrUnexpected) {
			t.Errorf("%s: expected ErrUnexpected, got %v", content, err)
		}
	}
}

func TestMissingRegistry(t *testing.T) {
	registry(t, "")
	if _, err := LoadRegistry(); !errors.Is(err, ErrNoRegistry) {
		t.Fatalf("expected ErrNoRegistry, got %v", err)
	}
}

func TestOpenURI(t *testing.T) {
	if got := OpenURI("/Users/me/My Vault"); got != "obsidian://open?path=%2FUsers%2Fme%2FMy+Vault" {
		t.Fatalf("got %s", got)
	}
}
