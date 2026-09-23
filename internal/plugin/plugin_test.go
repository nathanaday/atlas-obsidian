// Package plugin holds the tests that keep the plugin's own files consistent: the skills,
// the agents, and every place the code and the docs name one. See docs/skills.md.
package plugin

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/claudecode"
	"github.com/nathanaday/atlas-obsidian/internal/hooks"
	"github.com/nathanaday/atlas-obsidian/internal/project"
)

const root = "../.."

// current are the files that must name only skills that exist: the plugin's own files,
// the current docs, and the code. The design docs of earlier versions are history.
func current(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"skills", "agents", "internal", "hooks"} {
		filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if strings.HasSuffix(p, "_test.go") {
				return nil
			}
			if ext := filepath.Ext(p); ext == ".md" || ext == ".go" || ext == ".json" {
				files = append(files, p)
			}
			return nil
		})
	}
	for _, f := range []string{"README.md", "CLAUDE.md", "docs/usage.md", "docs/skills.md", "docs/members-design.md", "docs/threads-design.md"} {
		files = append(files, filepath.Join(root, f))
	}
	return files
}

func read(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// frontmatter returns the value of key in a file's YAML frontmatter, one line or folded.
func frontmatter(text, key string) string {
	front, _, ok, err := project.SplitFrontmatter(text)
	if !ok || err != nil {
		return ""
	}
	lines := strings.Split(front, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, key+":") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		if v == ">" || v == "|" {
			var parts []string
			for _, next := range lines[i+1:] {
				if !strings.HasPrefix(next, " ") {
					break
				}
				parts = append(parts, strings.TrimSpace(next))
			}
			v = strings.Join(parts, " ")
		}
		return strings.Trim(v, `"`)
	}
	return ""
}

func skillDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

func agents(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "agents"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			out = append(out, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return out
}

func TestEverySkillIsAFolderWithItsNameAndADescription(t *testing.T) {
	for _, dir := range skillDirs(t) {
		text := read(t, filepath.Join(root, "skills", dir, "SKILL.md"))
		if name := frontmatter(text, "name"); name != dir {
			t.Errorf("skills/%s: name %q", dir, name)
		}
		desc := frontmatter(text, "description")
		if desc == "" || len(desc) > 1024 {
			t.Errorf("skills/%s: description of %d characters", dir, len(desc))
		}
		if !strings.Contains(desc, "Use for") {
			t.Errorf("skills/%s: the description names no triggers (Use for ...)", dir)
		}
	}
}

func TestTheSkillMapIsTheFolders(t *testing.T) {
	var mapped []string
	homes := map[string]bool{}
	for _, c := range hooks.SkillMap {
		mapped = append(mapped, c.Home)
		homes[c.Home] = true
		for _, s := range c.Skills {
			if !strings.HasPrefix(s, c.Home+"-") {
				t.Errorf("%s is filed under %s", s, c.Home)
			}
			mapped = append(mapped, s)
		}
	}
	sort.Strings(mapped)
	dirs := skillDirs(t)
	if strings.Join(mapped, " ") != strings.Join(dirs, " ") {
		t.Fatalf("the skill map\n  %v\nis not the folders\n  %v", mapped, dirs)
	}
	if len(homes) != 3 {
		t.Fatalf("homes %v", homes)
	}
}

func TestEveryAgentHasItsName(t *testing.T) {
	for _, a := range agents(t) {
		text := read(t, filepath.Join(root, "agents", a+".md"))
		if name := frontmatter(text, "name"); name != a {
			t.Errorf("agents/%s.md: name %q", a, name)
		}
		if frontmatter(text, "description") == "" || frontmatter(text, "tools") == "" {
			t.Errorf("agents/%s.md: needs a description and tools", a)
		}
	}
}

var mdLink = regexp.MustCompile(`\]\(([^)\s]+)\)`)

func TestEveryRelativeLinkInThePluginResolves(t *testing.T) {
	for _, dir := range []string{"skills", "agents", "docs/skills.md", "docs/usage.md"} {
		filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(p) != ".md" {
				return nil
			}
			for _, m := range mdLink.FindAllStringSubmatch(read(t, p), -1) {
				target := m[1]
				if strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
					continue
				}
				target, _, _ = strings.Cut(target, "#")
				if _, err := os.Stat(filepath.Join(filepath.Dir(p), target)); err != nil {
					t.Errorf("%s links %s, which is not there", p, m[1])
				}
			}
			return nil
		})
	}
}

// named finds the skills a text names: a slash command, a skill picked in Codex, a skill
// called by name in prose, and a category-prefixed name in backticks.
var named = []*regexp.Regexp{
	regexp.MustCompile(`/atlas-obsidian:([a-z][a-z-]*)`),
	regexp.MustCompile("`\\$([a-z][a-z-]*)`"),
	regexp.MustCompile("(?:the|The) `([a-z][a-z-]*)` skill\\b"),
	regexp.MustCompile("`((?:wiki|thread|atlas)-[a-z][a-z-]*)`"),
	regexp.MustCompile(`\b((?:wiki|thread|atlas)-[a-z]+(?:-[a-z]+)?) skill\b`),
	regexp.MustCompile(`\bthe ([a-z]+) skill\b`),
}

// notSkills are hyphenated names in backticks that are not skills.
var notSkills = map[string]bool{"atlas-obsidian": true, "stage": true, "same": true, "next": true, "right": true, "calling": true, "writing": true, "owning": true}

// retired are the skill names 5.6.0 replaced. Only the record of that change in
// docs/skills.md may name them.
var retired = map[string]bool{"save": true, "describe": true, "canvas": true, "obsidian-bases": true, "obsidian-markdown": true, "work": true, "wiki-merge": true, "wiki-lint": true, "wiki-mode": true, "think": true}

func TestEverySkillNamedExists(t *testing.T) {
	known := map[string]bool{}
	for _, s := range skillDirs(t) {
		known[s] = true
	}
	for _, a := range agents(t) {
		known[a] = true
	}
	for _, f := range current(t) {
		text := read(t, f)
		for _, re := range named {
			for _, m := range re.FindAllStringSubmatch(text, -1) {
				name := m[1]
				if known[name] || notSkills[name] || strings.HasPrefix(name, "atlas-obsidian") {
					continue
				}
				if retired[name] && strings.HasSuffix(f, "docs/skills.md") {
					continue
				}
				t.Errorf("%s names %q, which is no skill or agent", strings.TrimPrefix(f, root+"/"), name)
			}
		}
	}
}

func TestTheLaunchPromptsNameSkills(t *testing.T) {
	known := map[string]bool{}
	for _, s := range skillDirs(t) {
		known[s] = true
	}
	prompts := []string{claudecode.IngestPrompt, claudecode.DescribePrompt}
	for _, stage := range []string{"stub", "spec", "plan", "receipt", ""} {
		prompts = append(prompts, claudecode.ThreadPrompt(stage, "thr-1"))
	}
	for _, p := range prompts {
		name, _, _ := strings.Cut(strings.TrimPrefix(p, "/atlas-obsidian:"), " ")
		if !known[name] {
			t.Errorf("launch prompt %q names no skill", p)
		}
	}
}

// TestEverySkillSaysWhereItSits holds each skill to the form docs/skills.md sets: the
// tools it uses and the skills around it.
func TestEverySkillSaysWhereItSits(t *testing.T) {
	for _, dir := range skillDirs(t) {
		text := read(t, filepath.Join(root, "skills", dir, "SKILL.md"))
		if !strings.Contains(text, "Tools:") {
			t.Errorf("skills/%s names no tools (a line starting Tools:)", dir)
		}
	}
}
