package registry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/project"
)

var now = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

// fixture lists three projects in the config and leaves one out of it, and adds the
// folders the scan cannot use: a work folder that is gone, one with no project in it, a
// project in the flat layout of 2.2.0, a 3.x project, an identity file that is not JSON,
// one from a later version, and a 3.x knowledge base.
func fixture(t *testing.T) (*home.Config, map[string]*project.Project, string) {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	cfg := &home.Config{}
	ps := map[string]*project.Project{}
	mk := func(rel string, opts project.Options, list bool) string {
		work := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(work, 0o755); err != nil {
			t.Fatal(err)
		}
		res, err := project.Init(work, opts, now)
		if err != nil {
			t.Fatal(err)
		}
		ps[res.Project.Name()] = res.Project
		if list {
			cfg.Projects = append(cfg.Projects, work)
		}
		return work
	}
	mk("Code/webapp", project.Options{Name: "webapp", Description: "The web app."}, true)
	mk("Code/deep/nested/firmware", project.Options{Name: "firmware"}, true)
	mk("Docs/thesis", project.Options{Name: "thesis", Mode: project.LYT}, true)
	mk("Code/unlisted", project.Options{Name: "unlisted"}, false)

	bad := func(rel string, write func(dir string)) string {
		dir := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(dir, 0o755)
		write(dir)
		cfg.Projects = append(cfg.Projects, dir)
		return dir
	}
	bad("Bad/gone", func(dir string) { os.RemoveAll(dir) })
	bad("Bad/plain", func(dir string) {})
	bad("Bad/flat", func(dir string) {
		os.MkdirAll(filepath.Join(dir, project.Dir), 0o755)
		os.WriteFile(filepath.Join(dir, project.Dir, project.Marker), []byte(`{"schema":"claude-atlas.project.v3","id":"flat"}`), 0o644)
	})
	bad("Bad/split", func(dir string) {
		os.MkdirAll(filepath.Join(dir, project.Dir, "split"), 0o755)
		os.WriteFile(filepath.Join(dir, project.Dir, "split", project.Marker), []byte(`{"schema":"claude-atlas.project.v3","id":"split","name":"split","knowledge":{"id":"k1","name":"notes"}}`), 0o644)
	})
	bad("Bad/broken", func(dir string) {
		os.MkdirAll(filepath.Join(dir, project.Dir, "broken"), 0o755)
		os.WriteFile(filepath.Join(dir, project.Dir, "broken", project.Marker), []byte(`{not json`), 0o644)
	})
	bad("Bad/later", func(dir string) {
		os.MkdirAll(filepath.Join(dir, project.Dir, "later"), 0o755)
		os.WriteFile(filepath.Join(dir, project.Dir, "later", project.Marker), []byte(`{"schema":"atlas-obsidian.project.v9","id":"later"}`), 0o644)
	})
	kb := filepath.Join(root, "Vaults", "notes")
	os.MkdirAll(kb, 0o755)
	os.WriteFile(filepath.Join(kb, project.KnowledgeMarker), []byte(`{"schema":"claude-atlas.vault.v3","id":"k1","kind":"knowledge","name":"notes"}`), 0o644)
	cfg.Knowledge = []string{kb}
	return cfg, ps, kb
}

func TestScanReadsEveryProjectAndSortsThem(t *testing.T) {
	cfg, ps, _ := fixture(t)
	ix, err := Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var good []string
	for _, e := range ix.Projects() {
		good = append(good, e.Name)
	}
	if strings.Join(good, ",") != "firmware,thesis,webapp" {
		t.Fatalf("valid entries first, by name: %v", good)
	}
	// A project the config does not list is not in the atlas.
	if _, err := ix.Find("unlisted"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unlisted: %v", err)
	}
	web := ix.ByID(ps["webapp"].Config.ID)
	if web == nil || web.Description != "The web app." || web.Mode != project.Generic || web.Created != "2026-09-15" {
		t.Fatalf("webapp %+v", web)
	}
	if thesis, _ := ix.Find("thesis"); thesis == nil || thesis.Mode != project.LYT {
		t.Fatalf("the mode travels with the entry: %+v", thesis)
	}
	// The wiki and the project's own folder come off the entry.
	if filepath.Base(web.Atlas()) != "webapp" || filepath.Base(web.Wiki()) != project.WikiDir {
		t.Fatalf("atlas %q wiki %q", web.Atlas(), web.Wiki())
	}
	if web.Rel() != "projects/webapp" {
		t.Fatalf("rel %q", web.Rel())
	}
}

func TestScanNamesEveryFolderItCannotUse(t *testing.T) {
	cfg, _, kb := fixture(t)
	ix, err := Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{}
	for _, e := range ix.Entries {
		if e.Error != "" {
			reasons[filepath.Base(e.Path)] = e.Reason
		}
	}
	want := map[string]string{
		"gone":   ReasonMissing,
		"plain":  ReasonNotProject,
		"flat":   ReasonFlat,
		"split":  ReasonV3Split,
		"broken": ReasonUnreadable,
		"later":  ReasonSchema,
		"notes":  ReasonV3Split,
	}
	for name, reason := range want {
		if reasons[name] != reason {
			t.Errorf("%s: reason %q, want %q", name, reasons[name], reason)
		}
	}
	if len(reasons) != len(want) {
		t.Fatalf("reasons %+v", reasons)
	}
	// Every unreadable folder is a problem as well, so a command names each one once.
	if len(ix.Problems) != len(want) {
		t.Fatalf("problems %+v", ix.Problems)
	}
	// A 3.x knowledge base's message names the command that makes it a project.
	e := ix.ByPath(kb)
	if e == nil || !strings.Contains(e.Error, "atlas-obsidian upgrade") {
		t.Fatalf("the knowledge base: %+v", e)
	}
	// A project the atlas cannot read has a path, an error, and a reason, and nothing else.
	flat := ix.ByPath(filepath.Join(filepath.Dir(kb), "..", "Bad", "flat"))
	if flat != nil && (flat.Name != "" || flat.ID != "") {
		t.Fatalf("an unreadable entry carries nothing else: %+v", flat)
	}
}

func TestFindByNameIDAndPath(t *testing.T) {
	cfg, ps, _ := fixture(t)
	ix, _ := Scan(cfg)
	web := ps["webapp"]
	for _, arg := range []string{"webapp", "WEBAPP", web.Config.ID, web.Config.ID[:8], web.Root} {
		found, err := ix.Find(arg)
		if err != nil || found.Name != "webapp" {
			t.Fatalf("Find(%q): %+v %v", arg, found, err)
		}
	}
	if _, err := ix.Find("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a name nothing carries: %v", err)
	}
	if _, err := ix.Find("/nowhere/at/all"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a path nothing carries: %v", err)
	}
	// Two entries with one name are ambiguous, and the error names both paths.
	ix.Entries = append(ix.Entries, Entry{ID: "second00", Name: "webapp", Path: filepath.Join(t.TempDir(), "webapp")})
	_, err := ix.Find("webapp")
	if !errors.Is(err, ErrAmbiguous) || !strings.Contains(err.Error(), "2 entries") {
		t.Fatalf("ambiguous: %v", err)
	}
}

func TestFindAmbiguousIDPrefix(t *testing.T) {
	ix := &Index{Entries: []Entry{
		{ID: "abcdefgh1111", Name: "one", Path: "/one"},
		{ID: "abcdefgh2222", Name: "two", Path: "/two"},
	}}
	if _, err := ix.Find("abcdefgh"); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("an id prefix two entries share: %v", err)
	}
	if found, err := ix.Find("abcdefgh1111"); err != nil || found.Name != "one" {
		t.Fatalf("the full id: %+v %v", found, err)
	}
}

func TestStateFileRoundTrips(t *testing.T) {
	cfg, _, _ := fixture(t)
	ix, _ := Scan(cfg)
	dir := filepath.Join(t.TempDir(), "state")
	if _, _, err := Read(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read before write: %v", err)
	}
	four := 4
	ix.Entries[0].State = &State{GeneratedAt: "2026-09-15T12:00:00Z", OK: true, Pages: &four, Heat: "new", HotTopics: []string{}}
	if err := Write(dir, ix.Entries, "2026-09-15T12:00:00Z"); err != nil {
		t.Fatal(err)
	}
	entries, generated, err := Read(dir)
	if err != nil || generated != "2026-09-15T12:00:00Z" || len(entries) != len(ix.Entries) || entries[0].State == nil || *entries[0].State.Pages != 4 {
		t.Fatalf("round trip %v %s %+v", err, generated, entries)
	}
	data, _ := os.ReadFile(File(dir))
	if !strings.Contains(string(data), `"schema": "atlas-obsidian.registry.v4"`) {
		t.Fatalf("file:\n%s", data)
	}
	os.WriteFile(File(dir), []byte(`{"schema":"claude-atlas.registry.v3","entries":[]}`), 0o644)
	if _, _, err := Read(dir); !errors.Is(err, ErrStale) {
		t.Fatalf("read another schema: %v", err)
	}
}

func TestByPathFollowsASymlinkedAncestor(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "Code")
	if err := os.MkdirAll(filepath.Join(real, "webapp"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}
	ix := &Index{Entries: []Entry{{ID: "abcdefgh1111", Name: "webapp", Path: filepath.Join(real, "webapp")}}}
	if e := ix.ByPath(filepath.Join(link, "webapp")); e == nil || e.Name != "webapp" {
		t.Fatalf("ByPath through a symlinked parent: %+v", e)
	}
	linked := &Index{Entries: []Entry{{ID: "abcdefgh1111", Name: "webapp", Path: filepath.Join(link, "webapp")}}}
	if linked.ByPath(filepath.Join(real, "webapp")) == nil {
		t.Fatal("ByPath with a resolved path found nothing")
	}
	if ix.ByPath(filepath.Join(link, "other")) != nil {
		t.Fatal("ByPath matched a path that is no entry")
	}
}

// A project the config lists twice, once under a spelling that reaches it through a
// symlink, is one entry.
func TestScanDedupesAnEntryRegisteredThroughASymlink(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	work := filepath.Join(root, "work")
	os.MkdirAll(work, 0o755)
	if _, err := project.Init(work, project.Options{Name: "work"}, now); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "work-link")
	if err := os.Symlink(work, link); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}
	ix, err := Scan(&home.Config{Projects: []string{work, link}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Entries) != 1 || ix.Entries[0].Path != work {
		t.Fatalf("one entry, with the first spelling: %+v", ix.Entries)
	}
	for _, path := range []string{work, link} {
		if e := ix.ByPath(path); e == nil || e.Name != "work" {
			t.Fatalf("ByPath(%s): %+v", path, e)
		}
	}
}

func TestDescriptionSummaryAndUnfinished(t *testing.T) {
	cases := map[Description]string{
		{Page: "wiki/entities/x.md"}:                                   "described in wiki/entities/x.md",
		{Page: "wiki/entities/x.md", Commit: "abcdef0123", Behind: -1}: "described in wiki/entities/x.md at abcdef0, not in the repository's history",
		{Page: "wiki/entities/x.md", Commit: "abcdef0123", Behind: 0}:  "described in wiki/entities/x.md at abcdef0, current",
		{Page: "wiki/entities/x.md", Commit: "abcdef0123", Behind: 1}:  "described in wiki/entities/x.md at abcdef0, 1 commit behind",
		{Page: "wiki/entities/x.md", Commit: "abcdef0123", Behind: 12}: "described in wiki/entities/x.md at abcdef0, 12 commits behind",
	}
	for d, want := range cases {
		if got := d.Summary(); got != want {
			t.Errorf("%+v: %q", d, got)
		}
	}
	one, two := 1, 2
	u := Unfinished{Stubs: &one, DeadLinks: &two}
	if u.Text() != "1 stubs · 2 dead links" || *u.Total() != 3 || (Unfinished{}).Total() != nil {
		t.Fatalf("unfinished %q %v", u.Text(), u.Total())
	}
}

// The scan reads a project made before the rename as an ordinary entry, not a problem.
func TestScanAcceptsTheSchemaWrittenBeforeTheRename(t *testing.T) {
	work := t.TempDir()
	dir := filepath.Join(work, project.Dir, "webapp")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, project.Marker),
		[]byte(`{"schema":"claude-atlas.project.v4","id":"p1","name":"webapp","created":"2026-09-17"}`), 0o644)
	cfg := &home.Config{}
	cfg.AddProject(work)
	ix, err := Scan(cfg)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(ix.Entries) != 1 || ix.Entries[0].Error != "" {
		t.Fatalf("entries %+v", ix.Entries)
	}
}
