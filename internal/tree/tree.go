// Package tree reads and writes the node tree: node.md (authored) and state.json (derived).
package tree

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/nathanaday/claude-atlas/internal/home"
)

const (
	NodeSchema  = "atlas.node.v1"
	StateSchema = "atlas.state.v1"
	NodeFile    = "node.md"
	StateFile   = "state.json"
)

var (
	Priorities = []string{"high", "normal", "low", "someday"}
	States     = []string{"active", "paused", "blocked", "archived"}
)

// Frontmatter is the authored half of a node, edited by hand or in Obsidian.
type Frontmatter struct {
	Schema           string   `yaml:"schema"`
	Kind             string   `yaml:"kind"`
	Name             string   `yaml:"name"`
	Vault            string   `yaml:"vault,omitempty"`
	Purpose          string   `yaml:"purpose"`
	DefinitionOfDone string   `yaml:"definition_of_done"`
	Priority         string   `yaml:"priority"`
	State            string   `yaml:"state"`
	BlockedOn        string   `yaml:"blocked_on"`
	ReviewAfter      string   `yaml:"review_after"`
	Repos            []string `yaml:"repos"`
}

// Node is one directory under the tree root.
type Node struct {
	Dir  string
	Rel  string // path relative to the tree root, forward slashes
	Body string // markdown after the frontmatter; not interpreted
	Frontmatter
}

func (n *Node) ID() string   { return filepath.Base(n.Dir) }
func (n *Node) IsLeaf() bool { return n.Kind == "leaf" }
func (n *Node) Depth() int   { return strings.Count(n.Rel, "/") }
func (n *Node) VaultPath() string {
	if n.Vault == "" {
		return ""
	}
	return home.Expand(n.Vault)
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify derives a node id from a display name.
func Slugify(name string) (string, error) {
	slug := strings.Trim(slugPattern.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		return "", fmt.Errorf("cannot derive a node id from %q", name)
	}
	return slug, nil
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// SplitFrontmatter separates a leading YAML block from the body.
func SplitFrontmatter(text string) (front, body string, ok bool) {
	if !strings.HasPrefix(text, "---\n") {
		return "", text, false
	}
	rest := text[4:]
	if strings.HasPrefix(rest, "---\n") {
		return "", strings.TrimLeft(rest[4:], "\n"), true
	}
	if strings.HasPrefix(rest, "---") && len(rest) == 3 {
		return "", "", true
	}
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", text, false
	}
	front = rest[:idx]
	body = strings.TrimLeft(rest[idx+4:], "\n")
	return front, body, true
}

// Load reads one node directory and validates it.
func Load(dir, root string) (*Node, error) {
	path := filepath.Join(dir, NodeFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	front, body, ok := SplitFrontmatter(string(data))
	if !ok {
		return nil, fmt.Errorf("%s: missing frontmatter", path)
	}
	node := &Node{Dir: dir, Body: body}
	if err := yaml.Unmarshal([]byte(front), &node.Frontmatter); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return nil, err
	}
	node.Rel = filepath.ToSlash(rel)
	if node.Schema != NodeSchema {
		return nil, fmt.Errorf("%s: unsupported schema %q", path, node.Schema)
	}
	if node.Kind != "leaf" && node.Kind != "cluster" {
		return nil, fmt.Errorf("%s: kind must be leaf or cluster", path)
	}
	if node.Kind == "leaf" && node.Vault == "" {
		return nil, fmt.Errorf("%s: a leaf must name a vault", path)
	}
	if node.Kind == "cluster" && node.Vault != "" {
		return nil, fmt.Errorf("%s: a cluster must not name a vault", path)
	}
	if node.Name == "" {
		node.Name = node.ID()
	}
	if node.Priority == "" {
		node.Priority = "normal"
	}
	if node.State == "" {
		node.State = "active"
	}
	if !contains(Priorities, node.Priority) {
		return nil, fmt.Errorf("%s: priority must be one of %s", path, strings.Join(Priorities, ", "))
	}
	if !contains(States, node.State) {
		return nil, fmt.Errorf("%s: state must be one of %s", path, strings.Join(States, ", "))
	}
	return node, nil
}

// Walk lists every node below root, parents before children, and rejects two leaves on one vault.
func Walk(root string) ([]*Node, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && path == root {
				return fs.SkipAll
			}
			return err
		}
		if !d.IsDir() && d.Name() == NodeFile && filepath.Dir(path) != root {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(dirs, func(i, j int) bool {
		return relParts(root, dirs[i]) < relParts(root, dirs[j])
	})
	nodes := make([]*Node, 0, len(dirs))
	byVault := map[string]*Node{}
	for _, dir := range dirs {
		node, err := Load(dir, root)
		if err != nil {
			return nil, err
		}
		if vault := node.VaultPath(); vault != "" {
			key := filepath.Clean(vault)
			if other, dup := byVault[key]; dup {
				return nil, fmt.Errorf("two leaves name the same vault %s: %s and %s", vault, other.Rel, node.Rel)
			}
			byVault[key] = node
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// relParts renders a path so that lexical order puts parents before children.
func relParts(root, dir string) string {
	rel, _ := filepath.Rel(root, dir)
	return strings.ReplaceAll(filepath.ToSlash(rel), "/", "\x00")
}

func Leaves(nodes []*Node) []*Node {
	var out []*Node
	for _, n := range nodes {
		if n.IsLeaf() {
			out = append(out, n)
		}
	}
	return out
}

func FindByVault(nodes []*Node, vault string) *Node {
	target := filepath.Clean(vault)
	for _, n := range nodes {
		if v := n.VaultPath(); v != "" && filepath.Clean(v) == target {
			return n
		}
	}
	return nil
}

func FindByRel(nodes []*Node, rel string) *Node {
	rel = strings.Trim(rel, "/")
	for _, n := range nodes {
		if n.Rel == rel || n.ID() == rel {
			return n
		}
	}
	return nil
}

// Render produces the node.md text for a frontmatter and body.
func Render(front Frontmatter, body string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(front); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	buf.WriteString("---\n")
	if body != "" {
		buf.WriteString("\n")
		buf.WriteString(strings.TrimRight(body, "\n"))
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

func writeNode(dir string, front Frontmatter, body string) error {
	data, err := Render(front, body)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, NodeFile), data, 0o644)
}

// EnsureClusters creates cluster nodes for each missing segment of parent and returns its directory.
func EnsureClusters(root, parent string) (string, error) {
	dir := root
	for _, segment := range strings.Split(strings.Trim(parent, "/"), "/") {
		if segment == "" {
			continue
		}
		dir = filepath.Join(dir, segment)
		if _, err := os.Stat(filepath.Join(dir, NodeFile)); err == nil {
			existing, err := Load(dir, root)
			if err != nil {
				return "", err
			}
			if existing.IsLeaf() {
				return "", fmt.Errorf("%s is a leaf and cannot hold children", existing.Rel)
			}
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		front := Frontmatter{Schema: NodeSchema, Kind: "cluster", Name: segment, Priority: "normal", State: "active", Repos: []string{}}
		if err := writeNode(dir, front, "# "+segment+"\n"); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// LeafOptions are the authored fields set when a leaf is created.
type LeafOptions struct {
	ID       string
	Name     string
	Vault    string
	Parent   string
	Purpose  string
	Priority string
}

// CreateLeaf writes a new leaf node and its outputs directory; it returns the node directory.
func CreateLeaf(root string, opts LeafOptions) (string, error) {
	if opts.Priority == "" {
		opts.Priority = "normal"
	}
	if !contains(Priorities, opts.Priority) {
		return "", fmt.Errorf("priority must be one of %s", strings.Join(Priorities, ", "))
	}
	container := root
	if opts.Parent != "" {
		var err error
		if container, err = EnsureClusters(root, opts.Parent); err != nil {
			return "", err
		}
	}
	dir := filepath.Join(container, opts.ID)
	if _, err := os.Stat(filepath.Join(dir, NodeFile)); err == nil {
		rel, _ := filepath.Rel(root, dir)
		return "", fmt.Errorf("node %s already exists", filepath.ToSlash(rel))
	}
	if err := os.MkdirAll(filepath.Join(dir, "outputs"), 0o755); err != nil {
		return "", err
	}
	front := Frontmatter{
		Schema:   NodeSchema,
		Kind:     "leaf",
		Name:     opts.Name,
		Vault:    opts.Vault,
		Purpose:  opts.Purpose,
		Priority: opts.Priority,
		State:    "active",
		Repos:    []string{},
	}
	body := "# " + opts.Name + "\n\nNotes about this project that belong to the atlas rather than the vault.\n"
	if err := writeNode(dir, front, body); err != nil {
		return "", err
	}
	return dir, nil
}

// Unfinished counts work the vault still owes. nil means unknown.
type Unfinished struct {
	EmptySections *int `json:"empty_sections"`
	SeedPages     *int `json:"seed_pages"`
	DeadLinks     *int `json:"dead_links"`
}

// State is the derived half of a node, regenerated in full by refresh.
type State struct {
	Schema        string     `json:"schema"`
	GeneratedAt   string     `json:"generated_at"`
	VaultOK       bool       `json:"vault_ok"`
	VaultError    string     `json:"vault_error"`
	LastOperation string     `json:"last_operation"`
	LastTouched   string     `json:"last_touched"`
	DaysIdle      *int       `json:"days_idle"`
	Heat          string     `json:"heat"`
	Pages         *int       `json:"pages"`
	OpenThreads   []string   `json:"open_threads"`
	Unfinished    Unfinished `json:"unfinished"`
	Leaves        *int       `json:"leaves,omitempty"`
}

func ReadState(dir string) (*State, error) {
	data, err := os.ReadFile(filepath.Join(dir, StateFile))
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func WriteState(dir string, state *State) error {
	if state.OpenThreads == nil {
		state.OpenThreads = []string{}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, StateFile), append(data, '\n'), 0o644)
}
