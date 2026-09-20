package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRouteAndSkeleton(t *testing.T) {
	p := newProject(t)
	r, err := p.RouteFor("concept", "Contextual Retrieval: a/b?", now)
	if err != nil || r.Path != "wiki/concepts/Contextual Retrieval a b.md" || r.Exists {
		t.Fatalf("route %+v %v", r, err)
	}
	if !strings.HasPrefix(r.Skeleton, "---\ntype: concept\ntitle: \"Contextual Retrieval a b\"\nstatus: seed\ncreated: 2026-09-19\n") || !strings.Contains(r.Skeleton, "## Definition") {
		t.Fatalf("skeleton:\n%s", r.Skeleton)
	}
	os.MkdirAll(p.Path("wiki/concepts"), 0o755)
	os.WriteFile(p.Path(r.Path), []byte("x"), 0o644)
	if r, _ := p.RouteFor("concept", "Contextual Retrieval: a/b?", now); !r.Exists {
		t.Fatal("an existing page should be reported")
	}
	if _, err := p.RouteFor("moc", "Maps", now); err == nil {
		t.Fatal("moc is not a generic type")
	}
	if _, err := p.RouteFor("question", "Why", now); err == nil {
		t.Fatal("questions are not filed")
	}
	p.Config.Mode = LYT
	if r, _ := p.RouteFor("moc", "AI", now); r.Path != "wiki/mocs/AI.md" {
		t.Fatalf("lyt moc %+v", r)
	}
	if r, _ := p.RouteFor("source", "Paper", now); r.Path != "wiki/notes/Paper.md" {
		t.Fatalf("lyt note %+v", r)
	}
	if SanitizeTitle("  ...  ") != "Untitled" || SanitizeTitle("a\x00b") != "ab" {
		t.Fatal("sanitize edge cases")
	}
}

func TestRoutableTypesByMode(t *testing.T) {
	if got := strings.Join(RoutableTypes(Generic), ","); got != "source,entity,concept" {
		t.Fatalf("generic: %s", got)
	}
	if got := strings.Join(RoutableTypes(LYT), ","); got != "note,moc,source,entity,concept" {
		t.Fatalf("lyt: %s", got)
	}
}

func TestFindPageByStemAndAlias(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "wiki/concepts"), 0o755)
	page := "---\ntitle: Backpropagation\ntype: concept\nstatus: seed\ncreated: 2026-09-12\nupdated: 2026-09-12\ntags:\n  - concept\naliases:\n  - backprop\n  - \"Back Propagation\"\n---\n\n# Backpropagation\n"
	if err := os.WriteFile(filepath.Join(root, "wiki/concepts/Backpropagation.md"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	// This name sorts before Backpropagation.md, so the walk reaches it first; a dangling
	// symlink must not stop FindPage from reaching the real page.
	if err := os.Symlink("does-not-exist.md", filepath.Join(root, "wiki/concepts/0gone.md")); err != nil {
		t.Fatal(err)
	}
	m, err := FindPage(root, "backpropagation")
	if err != nil || m == nil || m.Path != "wiki/concepts/Backpropagation.md" || m.ByAlias != "" {
		t.Fatalf("stem match past an unreadable file: %+v %v", m, err)
	}
	m, err = FindPage(root, "Back propagation")
	if err != nil || m == nil || m.Path != "wiki/concepts/Backpropagation.md" || m.ByAlias != "Back Propagation" {
		t.Fatalf("alias match: %+v %v", m, err)
	}
	if m, err := FindPage(root, "nope"); err != nil || m != nil {
		t.Fatalf("no match: %+v %v", m, err)
	}
	// A title with characters SanitizeTitle changes still stem-matches the sanitized file
	// name RouteFor would have created for it.
	stem := SanitizeTitle("A/B: C")
	os.WriteFile(filepath.Join(root, "wiki/concepts", stem+".md"), []byte("---\ntitle: "+stem+"\ntype: concept\nstatus: seed\ncreated: 2026-09-12\nupdated: 2026-09-12\ntags:\n  - concept\n---\n"), 0o644)
	if m, err := FindPage(root, "A/B: C"); err != nil || m == nil || m.Path != "wiki/concepts/"+stem+".md" {
		t.Fatalf("sanitized stem match: %+v %v", m, err)
	}
}

func TestFrontmatter(t *testing.T) {
	fields, body, err := Frontmatter("---\ntitle: A\ntags:\n  - x\n---\n\nBody\n")
	if err != nil || fields["title"] != "A" || body != "\nBody\n" || len(StringList(fields, "tags")) != 1 {
		t.Fatalf("%v %q %v", fields, body, err)
	}
	if missing := MissingFrontmatter(fields); len(missing) != 4 {
		t.Fatalf("missing %v", missing)
	}
	if _, _, err := Frontmatter("---\ntitle: A\n"); err == nil {
		t.Fatal("an unterminated block should error")
	}
	if fields, _, err := Frontmatter("no block"); err != nil || fields != nil {
		t.Fatal("no block should be nil, nil")
	}
	if _, _, err := Frontmatter("---\n: : :\n  bad: [\n---\n"); err == nil {
		t.Fatal("invalid yaml should error")
	}
}

func TestNewNotesGoUnderTheWikiUnlessTheUserChose(t *testing.T) {
	p := newProject(t)
	file := p.Path(AppFile)
	settings := func() map[string]any {
		t.Helper()
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var s map[string]any
		if err := json.Unmarshal(data, &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if s := settings(); s["newFileLocation"] != "folder" || s["newFileFolderPath"] != "wiki" {
		t.Fatalf("template settings %v", s)
	}
	repo := p.Work()
	commit := func(what string) {
		t.Helper()
		repo.AddAll()
		repo.Commit(CommitMessage("manual", what, NewOperationID("manual", now)))
	}
	os.WriteFile(file, []byte(`{"newLinkFormat": "absolute"}`), 0o644)
	commit("older settings")
	res, err := Refresh(p, now)
	if err != nil || len(res.Added) != 1 || res.Added[0] != AppFile {
		t.Fatalf("refresh %+v %v", res, err)
	}
	if s := settings(); s["newLinkFormat"] != "absolute" || s["newFileLocation"] != "folder" || s["newFileFolderPath"] != "wiki" {
		t.Fatalf("refresh keeps the settings and adds the folder: %v", s)
	}
	os.WriteFile(file, []byte(`{"newFileLocation": "current"}`), 0o644)
	commit("the user's choice")
	if res, err := Refresh(p, now); err != nil || len(res.Added) != 0 {
		t.Fatalf("refresh keeps a location the user chose: %+v %v", res, err)
	}
	if s := settings(); s["newFileLocation"] != "current" || s["newFileFolderPath"] != nil {
		t.Fatalf("settings %v", s)
	}
	// A settings file that is not a JSON object stops the pass before it writes anything.
	os.WriteFile(file, []byte("[]"), 0o644)
	if _, err := Refresh(p, now); err == nil || !strings.Contains(err.Error(), "not a JSON object") {
		t.Fatalf("a broken settings file: %v", err)
	}
}

func TestRefreshRewritesTheSnippetAndMergesTheIgnoreFile(t *testing.T) {
	p := newProject(t)
	os.WriteFile(p.Path(Snippet), []byte("/* the user replaced it */\n"), 0o644)
	os.WriteFile(p.Path(".gitignore"), []byte("build/\n"), 0o644)
	res, err := Refresh(p, now)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(res.Added, Snippet) {
		t.Fatalf("the snippet is the atlas's to keep current: %+v", res.Added)
	}
	if data, _ := os.ReadFile(p.Path(Snippet)); !strings.Contains(string(data), "--atlas-plan") {
		t.Fatalf("snippet:\n%s", data)
	}
	ignore, _ := os.ReadFile(p.Path(".gitignore"))
	if !strings.Contains(string(ignore), "build/") || !strings.Contains(string(ignore), MetaDir+"/") {
		t.Fatalf("the ignore file keeps what it had and gains what it lacked:\n%s", ignore)
	}
}

func TestLockIsExclusiveAndUpdateConfigTakesIt(t *testing.T) {
	p := newProject(t)
	unlock, err := Lock(p.Atlas())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- UpdateConfig(p.Root, "edit description", now, func(c *Config) error {
			c.Description = "x"
			return nil
		})
	}()
	select {
	case err := <-done:
		t.Fatalf("UpdateConfig ran while the project was locked: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if after, _ := Open(p.Root); after.Config.Description != "x" {
		t.Fatalf("description %+v", after.Config)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
