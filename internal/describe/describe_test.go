package describe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

var now = time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

// repoWork makes a work folder that is a git repository with one commit and returns it
// with that commit.
func repoWork(t *testing.T) (string, string) {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	code := filepath.Join(t.TempDir(), "code")
	os.MkdirAll(filepath.Join(code, "docs"), 0o755)
	os.MkdirAll(filepath.Join(code, "atlas", "code", "tasks"), 0o755)
	os.WriteFile(filepath.Join(code, "README.md"), []byte("# code\n\nA thing.\n"), 0o644)
	os.WriteFile(filepath.Join(code, "CLAUDE.md"), []byte("# guide\n\n```go\nx\n```\n"), 0o644)
	os.WriteFile(filepath.Join(code, "docs", "design.md"), []byte("# The design\n\ntext\n"), 0o644)
	os.WriteFile(filepath.Join(code, "atlas", "code", "project.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(code, "atlas", "code", "tasks", "x.md"), []byte("task"), 0o644)
	os.WriteFile(filepath.Join(code, "main.go"), []byte("package main\n"), 0o644)
	r := gitx.Repo{Dir: code}
	if err := r.Init(); err != nil {
		t.Fatal(err)
	}
	r.AddAll()
	head, err := r.Commit("one")
	if err != nil {
		t.Fatal(err)
	}
	return code, head
}

func page(id, commit string) string {
	return "---\ntitle: code\ntype: entity\nentity_type: project\nproject: " + id + "\ncommit: " + commit + "\nstatus: developing\ncreated: 2026-09-17\nupdated: 2026-09-17\ntags:\n  - entity\n---\n\n# code\n"
}

func TestPageFindsTheProjectsPage(t *testing.T) {
	code, first := repoWork(t)
	kb := filepath.Join(code, project.Dir, "code")
	os.MkdirAll(filepath.Join(kb, "wiki", "entities"), 0o755)
	e := registry.Entry{ID: "p-1", Name: "code", Path: code}
	if Page(e) != nil {
		t.Fatal("no page yet")
	}
	if Page(registry.Entry{ID: "p-1", Path: filepath.Join(t.TempDir(), "nowhere")}) != nil {
		t.Fatal("no project, no page")
	}
	os.WriteFile(filepath.Join(kb, "wiki", "entities", "other.md"), []byte(page("p-2", first)), 0o644)
	os.WriteFile(filepath.Join(kb, "wiki", "entities", "repo.md"), []byte(strings.Replace(page("p-1", first), "entity_type: project", "entity_type: repository", 1)), 0o644)
	if Page(e) != nil {
		t.Fatal("another project's page, and a page of another entity type, do not match")
	}
	os.WriteFile(filepath.Join(kb, "wiki", "entities", "byname.md"), []byte(page("CODE", "")), 0o644)
	d := Page(e)
	if d == nil || d.Page != "wiki/entities/byname.md" || d.Commit != "" || d.Behind != -1 {
		t.Fatalf("a page naming the project by name: %+v", d)
	}
	os.WriteFile(filepath.Join(kb, "wiki", "entities", "code.md"), []byte(page("p-1", first)), 0o644)
	d = Page(e)
	if d == nil || d.Page != "wiki/entities/code.md" || d.Commit != first || d.Behind != 0 {
		t.Fatalf("a page at head wins over one without a commit: %+v", d)
	}
	os.WriteFile(filepath.Join(code, "main.go"), []byte("package main // two\n"), 0o644)
	r := gitx.Repo{Dir: code}
	r.AddAll()
	second, _ := r.Commit("two")
	os.WriteFile(filepath.Join(kb, "wiki", "entities", "newer.md"), []byte(page("p-1", second)), 0o644)
	d = Page(e)
	if d == nil || d.Page != "wiki/entities/newer.md" || d.Behind != 0 {
		t.Fatalf("the page nearest head wins: %+v", d)
	}
	os.WriteFile(filepath.Join(kb, "wiki", "entities", "lost.md"), []byte(page("p-1", "0000000000000000000000000000000000000000")), 0o644)
	if d = Page(e); d.Page != "wiki/entities/newer.md" {
		t.Fatalf("a commit not in the history never wins: %+v", d)
	}
	if ClaudeMD(code) != filepath.Join(code, "CLAUDE.md") || ClaudeMD(t.TempDir()) != "" {
		t.Fatal("ClaudeMD")
	}
	// A work folder that is not a repository: pages match, commits never count.
	plain := filepath.Join(t.TempDir(), "docs")
	os.MkdirAll(filepath.Join(plain, project.Dir, "code", "wiki", "entities"), 0o755)
	os.WriteFile(filepath.Join(plain, project.Dir, "code", project.Marker), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(plain, project.Dir, "code", "wiki", "entities", "code.md"), []byte(page("p-1", first)), 0o644)
	pe := registry.Entry{ID: "p-1", Name: "code", Path: plain}
	if d = Page(pe); d == nil || d.Behind != -1 {
		t.Fatalf("plain folder: %+v", d)
	}
}

func TestTakeSnapshotOverARepository(t *testing.T) {
	code, first := repoWork(t)
	e := registry.Entry{ID: "p-1", Name: "code", Path: code, Description: "A thing."}
	s, err := TakeSnapshot(e, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if s.Commit != first || s.Since != "" || s.FileName != "code-"+first[:7]+".md" || s.Project != "p-1" || s.Branch == "" {
		t.Fatalf("snapshot %+v", s)
	}
	text := string(s.Content)
	for _, want := range []string{"type: " + SnapshotType, "project: p-1", "commit: \"" + first + "\"", "The project describes itself as: A thing.", "## CLAUDE.md\n\n````markdown\n# guide", "## README.md", "## Files\n\n4 files:", "- main.go", "## Docs\n\n- docs/design.md — The design"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "atlas/") || strings.Contains(text, "## Changes since") {
		t.Fatalf("the atlas folder is left out and there is no log yet:\n%s", text)
	}
	os.WriteFile(filepath.Join(code, "b.txt"), []byte("b"), 0o644)
	r := gitx.Repo{Dir: code}
	r.AddAll()
	second, _ := r.Commit("two")
	s, err = TakeSnapshot(e, first, now)
	if err != nil || s.Commit != second || s.Since != first {
		t.Fatalf("second %+v %v", s, err)
	}
	if !strings.Contains(string(s.Content), "## Changes since "+first[:7]) || !strings.Contains(string(s.Content), "two") {
		t.Fatalf("the log since the first commit:\n%s", s.Content)
	}
	if s, _ := TakeSnapshot(e, second, now); s.Since != "" {
		t.Fatal("since at head is no since")
	}
	if s, _ := TakeSnapshot(e, "not-a-commit", now); s.Since != "" {
		t.Fatal("since not in the history is no since")
	}
	if _, err := TakeSnapshot(registry.Entry{Name: "x"}, "", now); err == nil {
		t.Fatal("no folder")
	}
}

func TestSnapshotIncludesCodexInstructions(t *testing.T) {
	code, _ := repoWork(t)
	if err := os.WriteFile(filepath.Join(code, "AGENTS.md"), []byte("# Codex guidance\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := TakeSnapshot(registry.Entry{ID: "p-1", Name: "code", Path: code}, "", now)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## AGENTS.md", "# Codex guidance", "## CLAUDE.md", "## README.md"} {
		if !strings.Contains(string(s.Content), want) {
			t.Errorf("snapshot missing %q", want)
		}
	}
}

func TestTakeSnapshotOverAPlainFolder(t *testing.T) {
	docs := filepath.Join(t.TempDir(), "thesis")
	os.MkdirAll(filepath.Join(docs, "atlas", "thesis"), 0o755)
	os.MkdirAll(filepath.Join(docs, ".hidden"), 0o755)
	os.MkdirAll(filepath.Join(docs, "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(docs, "atlas", "thesis", "project.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(docs, ".hidden", "x"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(docs, "node_modules", "x", "i.js"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(docs, "chapter1.md"), []byte("# One\n"), 0o644)
	os.WriteFile(filepath.Join(docs, "readme"), []byte("plain readme\n"), 0o644)
	e := registry.Entry{ID: "p-3", Name: "thesis", Path: docs}
	s, err := TakeSnapshot(e, "abc", now)
	if err != nil {
		t.Fatal(err)
	}
	if s.Commit != "" || s.Since != "" || s.Branch != "" || s.FileName != "thesis-2026-09-17.md" {
		t.Fatalf("snapshot %+v", s)
	}
	text := string(s.Content)
	for _, want := range []string{"title: \"thesis on 2026-09-17\"", "not a git repository", "## readme\n\n````markdown\nplain readme", "## Files\n\n2 files:", "- chapter1.md", "- readme"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	for _, absent := range []string{"atlas/", ".hidden", "node_modules"} {
		if strings.Contains(text, absent) {
			t.Errorf("%q should be left out:\n%s", absent, text)
		}
	}
}

// TestTheProjectsOwnFolderIsNotTheWork covers the wiki that commits into the work's
// repository: its commits do not put the page behind, and the snapshot leaves its files
// out.
func TestTheProjectsOwnFolderIsNotTheWork(t *testing.T) {
	code, first := repoWork(t)
	atlas := filepath.Join(code, "atlas", "code")
	os.MkdirAll(filepath.Join(atlas, "wiki", "entities"), 0o755)
	os.WriteFile(filepath.Join(atlas, "wiki", "entities", "code.md"), []byte(page("p-1", first)), 0o644)
	r := gitx.Repo{Dir: code}
	r.AddAll()
	if _, err := r.Commit("save: the page"); err != nil {
		t.Fatal(err)
	}
	e := registry.Entry{ID: "p-1", Name: "code", Path: code}
	if d := Page(e); d == nil || d.Behind != 0 {
		t.Fatalf("the wiki's commits are not the work's: %+v", d)
	}
	os.WriteFile(filepath.Join(code, "main.go"), []byte("package main // two\n"), 0o644)
	r.Add("main.go")
	if _, err := r.Commit("feat: two"); err != nil {
		t.Fatal(err)
	}
	if d := Page(e); d == nil || d.Behind != 1 {
		t.Fatalf("a commit to the work counts: %+v", d)
	}
	s, err := TakeSnapshot(e, first, now)
	if err != nil {
		t.Fatal(err)
	}
	text := string(s.Content)
	if strings.Contains(text, "atlas/") || strings.Contains(text, "save:") || !strings.Contains(text, "- main.go") || !strings.Contains(text, "feat: two") {
		t.Fatalf("the snapshot leaves the project's folder out:\n%s", text)
	}
}

func TestSnapshotListsTheMarkersOfUnfinishedWork(t *testing.T) {
	work := filepath.Join(t.TempDir(), "app")
	os.MkdirAll(filepath.Join(work, "src"), 0o755)
	os.WriteFile(filepath.Join(work, "src", "main.go"), []byte("package main\n\n// TODO: retry on timeout\nfunc main() {} // FIXME(nate) leaks\n// a todo in lower case is prose\n"), 0o644)
	os.WriteFile(filepath.Join(work, "logo.png"), []byte("PNG\x00TODO"), 0o644)
	os.WriteFile(filepath.Join(work, "NOTES.md"), []byte("XXX decide the schema\n"), 0o644)
	s, err := TakeSnapshot(registry.Entry{ID: "p-9", Name: "app", Path: work}, "", now)
	if err != nil {
		t.Fatal(err)
	}
	text := string(s.Content)
	for _, want := range []string{"## Markers", "- NOTES.md:1: XXX decide the schema", "- src/main.go:3: // TODO: retry on timeout", "- src/main.go:4: func main() {} // FIXME(nate) leaks"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "logo.png:") || strings.Contains(text, "lower case") {
		t.Errorf("a binary file or prose was listed:\n%s", text)
	}
}
