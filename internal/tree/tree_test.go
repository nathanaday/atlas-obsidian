package tree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func rels(nodes []*Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Rel
	}
	return out
}

func equal(a, b []string) bool { return strings.Join(a, ",") == strings.Join(b, ",") }

func TestSlugify(t *testing.T) {
	got, err := Slugify("Sensor Triage (2026)")
	if err != nil || got != "sensor-triage-2026" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := Slugify("!!!"); err == nil {
		t.Fatal("expected an error for an empty slug")
	}
}

func TestCreateLeafUnderParentCreatesClusters(t *testing.T) {
	root := t.TempDir()
	dir, err := CreateLeaf(root, LeafOptions{ID: "triage", Name: "Triage", Vault: "/v", Parent: "technical/work"})
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(root, "technical", "work", "triage") {
		t.Fatalf("unexpected dir %s", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "outputs")); err != nil {
		t.Fatal("outputs directory missing")
	}
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	if !equal(rels(nodes), []string{"technical", "technical/work", "technical/work/triage"}) {
		t.Fatalf("order %v", rels(nodes))
	}
	if nodes[0].Kind != "cluster" || nodes[2].Kind != "leaf" || nodes[2].VaultPath() != "/v" {
		t.Fatalf("kinds %s %s vault %s", nodes[0].Kind, nodes[2].Kind, nodes[2].VaultPath())
	}
	if len(Leaves(nodes)) != 1 {
		t.Fatal("expected one leaf")
	}
}

func TestWalkOrdersParentsBeforeChildrenRegardlessOfNames(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateLeaf(root, LeafOptions{ID: "alpha", Name: "alpha", Vault: "/v1", Parent: "area"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateLeaf(root, LeafOptions{ID: "zeta", Name: "zeta", Vault: "/v2", Parent: "area/deep"}); err != nil {
		t.Fatal(err)
	}
	nodes, err := Walk(root)
	if err != nil {
		t.Fatal(err)
	}
	if !equal(rels(nodes), []string{"area", "area/alpha", "area/deep", "area/deep/zeta"}) {
		t.Fatalf("order %v", rels(nodes))
	}
}

func TestCreateLeafRefusesDuplicateAndWalkRejectsSharedVault(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateLeaf(root, LeafOptions{ID: "a", Name: "a", Vault: "/v"}); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateLeaf(root, LeafOptions{ID: "a", Name: "a", Vault: "/v2"}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if _, err := CreateLeaf(root, LeafOptions{ID: "b", Name: "b", Vault: "/v"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Walk(root); err == nil || !strings.Contains(err.Error(), "same vault") {
		t.Fatalf("expected shared vault error, got %v", err)
	}
}

func TestWalkOfMissingRootIsEmpty(t *testing.T) {
	nodes, err := Walk(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(nodes) != 0 {
		t.Fatalf("got %v, %v", nodes, err)
	}
}

func TestFindHelpers(t *testing.T) {
	root := t.TempDir()
	if _, err := CreateLeaf(root, LeafOptions{ID: "a", Name: "a", Vault: "/v", Parent: "x"}); err != nil {
		t.Fatal(err)
	}
	nodes, _ := Walk(root)
	if FindByVault(nodes, "/v").Rel != "x/a" || FindByRel(nodes, "x/a").ID() != "a" || FindByRel(nodes, "a").Rel != "x/a" {
		t.Fatal("lookup failed")
	}
	if FindByRel(nodes, "missing") != nil || FindByVault(nodes, "/nope") != nil {
		t.Fatal("expected nil for unknown")
	}
}

func TestLoadValidates(t *testing.T) {
	cases := map[string]string{
		"schema: nope\nkind: leaf\nvault: /v\n":                            "unsupported schema",
		"schema: atlas.node.v1\nkind: leaf\n":                              "must name a vault",
		"schema: atlas.node.v1\nkind: cluster\nvault: /v\n":                "must not name",
		"schema: atlas.node.v1\nkind: leaf\nvault: /v\npriority: urgent\n": "priority",
		"schema: atlas.node.v1\nkind: leaf\nvault: /v\nstate: done\n":      "state",
		"schema: atlas.node.v1\nkind: bucket\n":                            "kind",
	}
	for front, want := range cases {
		root := t.TempDir()
		dir := filepath.Join(root, "n")
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, NodeFile), []byte("---\n"+front+"---\n"), 0o644)
		_, err := Load(dir, root)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: expected %q, got %v", front, want, err)
		}
	}
}

func TestLoadDefaultsAndBody(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "n")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, NodeFile), []byte("---\nschema: atlas.node.v1\nkind: leaf\nvault: ~/v\n---\n\n# Notes\n"), 0o644)
	node, err := Load(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	if node.Name != "n" || node.Priority != "normal" || node.State != "active" {
		t.Fatalf("defaults not applied: %+v", node.Frontmatter)
	}
	if strings.HasPrefix(node.VaultPath(), "~") {
		t.Fatal("vault path not expanded")
	}
	if node.Body != "# Notes\n" {
		t.Fatalf("body %q", node.Body)
	}
}

func TestRenderRoundTrip(t *testing.T) {
	front := Frontmatter{Schema: NodeSchema, Kind: "leaf", Name: "N: with colon", Vault: "/v", Purpose: "Two\nlines", Priority: "high", State: "blocked", BlockedOn: "hw", Repos: []string{"https://x"}}
	data, err := Render(front, "body")
	if err != nil {
		t.Fatal(err)
	}
	head, body, ok := SplitFrontmatter(string(data))
	if !ok || body != "body\n" {
		t.Fatalf("split failed: ok=%v body=%q", ok, body)
	}
	var back Frontmatter
	if err := yaml.Unmarshal([]byte(head), &back); err != nil {
		t.Fatal(err)
	}
	if back.Name != front.Name || back.Purpose != front.Purpose || back.Repos[0] != "https://x" || back.State != "blocked" {
		t.Fatalf("round trip lost data: %+v", back)
	}
}

func TestSplitFrontmatter(t *testing.T) {
	if _, body, ok := SplitFrontmatter("no front\n"); ok || body != "no front\n" {
		t.Fatal("plain text should not split")
	}
	if front, body, ok := SplitFrontmatter("---\n---\nrest\n"); !ok || front != "" || body != "rest\n" {
		t.Fatalf("empty front: %q %q %v", front, body, ok)
	}
	if front, _, ok := SplitFrontmatter("---\na: 1\n---"); !ok || front != "a: 1" {
		t.Fatalf("eof front: %q %v", front, ok)
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	one := 1
	if err := WriteState(dir, &State{Schema: StateSchema, Heat: "hot", DaysIdle: &one}); err != nil {
		t.Fatal(err)
	}
	state, err := ReadState(dir)
	if err != nil || state.Heat != "hot" || *state.DaysIdle != 1 || state.OpenThreads == nil || state.Pages != nil {
		t.Fatalf("got %+v, %v", state, err)
	}
	if _, err := ReadState(t.TempDir()); err == nil {
		t.Fatal("expected error for missing state")
	}
}
