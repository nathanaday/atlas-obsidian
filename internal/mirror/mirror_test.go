package mirror

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/txn"
)

var now = time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)

func entry(id, name string, members ...string) registry.Entry {
	return registry.Entry{ID: id, Name: name, Path: "/" + name, Members: members}
}

func index(entries ...registry.Entry) *registry.Index {
	return &registry.Index{Entries: entries}
}

func TestClosureIsFlatEachProjectOnceSelfExcluded(t *testing.T) {
	// hub -> a -> c, hub -> b -> c: c appears once, and hub never appears.
	ix := index(entry("hub", "hub", "a", "b"), entry("a", "a", "c"), entry("b", "b", "c"), entry("c", "c"))
	got, err := Closure(ix, *ix.ByID("hub"))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range got {
		if m.Error != "" {
			t.Fatalf("%s: %s", m.ID, m.Error)
		}
		ids = append(ids, m.ID)
	}
	if strings.Join(ids, ",") != "a,b,c" {
		t.Fatalf("closure %v", ids)
	}
}

func TestClosureRefusesACycle(t *testing.T) {
	ix := index(entry("hub", "hub", "a"), entry("a", "a", "b"), entry("b", "b", "hub"))
	if _, err := Closure(ix, *ix.ByID("hub")); !errors.Is(err, ErrCycle) {
		t.Fatalf("err %v", err)
	}
	// A cycle that does not pass through the hub is one as well.
	ix = index(entry("hub", "hub", "a"), entry("a", "a", "b"), entry("b", "b", "a"))
	if _, err := Closure(ix, *ix.ByID("hub")); !errors.Is(err, ErrCycle) {
		t.Fatalf("err %v", err)
	}
}

func TestClosureKeepsAMemberItCannotRead(t *testing.T) {
	ix := index(entry("hub", "hub", "a", "gone"), entry("a", "a"))
	got, err := Closure(ix, *ix.ByID("hub"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[0].Error != "" || got[1].ID != "gone" || got[1].Error == "" {
		t.Fatalf("closure %+v", got)
	}
}

func TestClosureRefusesTwoProjectsWithOneFolder(t *testing.T) {
	needGit(t)
	a := initProject(t, "svc", "svc")
	b := initProject(t, "svc", "svc")
	ix := index(registry.Entry{ID: "hub", Name: "hub", Members: []string{a.Config.ID, b.Config.ID}}, entryOf(a), entryOf(b))
	if _, err := Closure(ix, *ix.ByID("hub")); err == nil || !strings.Contains(err.Error(), "rename one") {
		t.Fatalf("err %v", err)
	}
}

func TestValidateRefusesSelfUnknownAndCycle(t *testing.T) {
	ix := index(entry("hub", "hub"), entry("a", "a", "b"), entry("b", "b"))
	hub := *ix.ByID("hub")
	if err := Validate(ix, hub, []string{"hub"}); err == nil {
		t.Fatal("self accepted")
	}
	if err := Validate(ix, hub, []string{"nope"}); err == nil {
		t.Fatal("unknown accepted")
	}
	if err := Validate(ix, hub, []string{"a", "a"}); err == nil {
		t.Fatal("duplicate accepted")
	}
	if err := Validate(ix, hub, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	// b would list hub through a: hub -> a -> b -> hub.
	b := *ix.ByID("b")
	ix.Entries[0].Members = []string{"a"}
	if err := Validate(ix, b, []string{"hub"}); !errors.Is(err, ErrCycle) {
		t.Fatalf("err %v", err)
	}
	var many []string
	for i := 0; i <= project.MaxMembers; i++ {
		many = append(many, "x")
	}
	if err := Validate(ix, hub, many); err == nil {
		t.Fatal("limit accepted")
	}
}

func TestStampKeepsEveryOtherLine(t *testing.T) {
	in := "---\ntitle: Config\ntype: entity\ncommit: old\n  nested: keep\n---\nBody\n"
	got := stamp(in, [][2]string{{ProjectKey, "id-1"}, {MirrorOfKey, "wiki/entities/Config.md"}, {CommitKey, ""}})
	want := "---\ntitle: Config\ntype: entity\n  nested: keep\nproject: \"id-1\"\nmirror_of: \"wiki/entities/Config.md\"\n---\nBody\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if got := stamp("Body only\n", [][2]string{{ProjectKey, "id-1"}}); got != "---\nproject: \"id-1\"\n---\nBody only\n" {
		t.Fatalf("no frontmatter: %q", got)
	}
}

func TestSyncMirrorsTheClosureAndIsIdempotent(t *testing.T) {
	needGit(t)
	hub := initProject(t, "hub", "hub")
	a := initProject(t, "svc-a", "svc-a")
	b := initProject(t, "svc-b", "svc-b")
	writePage(t, a, "wiki/entities/Config.md", "---\ntitle: Config\ntype: entity\nstatus: evergreen\ncreated: 2026-09-01\nupdated: 2026-09-01\ntags: []\n---\n# Config\n\n## Setup\n\nSee [[Overview]], [[Config#Setup]], [[Config|the config]], ![[Config]], [[Missing]], and [the index](../index.md).\n`[[Overview]]` stays.\n\n![[projects/svc-b/entities/Other|Other]] points into a mirror already.\n")
	writePage(t, a, "wiki/canvases/map.canvas", "{\"nodes\":[{\"id\":\"1\",\"type\":\"file\",\"file\":\"wiki/entities/Config.md\",\"x\":0,\"y\":0,\"width\":100,\"height\":50}],\"edges\":[]}\n")
	writePage(t, b, "wiki/entities/Other.md", "---\ntitle: Other\ntype: entity\nstatus: evergreen\ncreated: 2026-09-01\nupdated: 2026-09-01\ntags: []\n---\n# Other\n\nText.\n")
	// b is a member of a; the hub lists a only and still mirrors both, flat.
	setMembers(t, a, b.Config.ID)
	setMembers(t, hub, a.Config.ID)
	ix := index(entryOf(hub), entryOf(a), entryOf(b))

	res, err := Sync(hub, ix, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Commit == "" || res.Creates == 0 || res.Updates != 0 || res.Removes != 0 {
		t.Fatalf("result %+v", res)
	}
	if len(res.Members) != 2 || res.Members[0].Folder != "svc-a" || res.Members[1].Folder != "svc-b" || res.Members[0].Pages == 0 {
		t.Fatalf("members %+v", res.Members)
	}
	config := readPage(t, hub, "wiki/projects/svc-a/entities/Config.md")
	for _, want := range []string{
		"project: \"" + a.Config.ID + "\"",
		"mirror_of: \"wiki/entities/Config.md\"",
		"commit: \"" + res.Members[0].Commit + "\"",
		"[[wiki/projects/svc-a/overview|Overview]]",
		"[[wiki/projects/svc-a/entities/Config#Setup|Config#Setup]]",
		"[[wiki/projects/svc-a/entities/Config|the config]]",
		"![[wiki/projects/svc-a/entities/Config|Config]]",
		"[[Missing]]",
		"[the index](wiki/projects/svc-a/svc-a.md)",
		"`[[Overview]]` stays",
		"![[projects/svc-b/entities/Other|Other]]",
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("mirrored page lacks %q:\n%s", want, config)
		}
	}
	if !strings.Contains(readPage(t, hub, "wiki/projects/svc-a/canvases/map.canvas"), `"file": "wiki/projects/svc-a/entities/Config.md"`) {
		t.Fatal("canvas node not rewritten")
	}
	for _, rel := range []string{"wiki/projects/svc-a/svc-a.md", "wiki/projects/svc-b/svc-b.md", "wiki/projects/svc-b/entities/Other.md", "wiki/projects/projects.md"} {
		if _, err := os.Stat(hub.Path(rel)); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
	}
	for _, rel := range []string{"wiki/projects/svc-a/index.md", "wiki/projects/svc-a/log.md", "wiki/projects/svc-a/hot.md", "wiki/projects/svc-a/meta/ledgers/source-ledger.json"} {
		if _, err := os.Stat(hub.Path(rel)); err == nil {
			t.Fatalf("%s was mirrored", rel)
		}
	}
	idx := readPage(t, hub, "wiki/projects/projects.md")
	if !strings.Contains(idx, "[[wiki/projects/svc-a/svc-a\\|svc-a]]") || !strings.Contains(idx, "`"+b.Config.ID+"`") {
		t.Fatalf("index:\n%s", idx)
	}
	ops, err := txn.History(hub, 1, true)
	if err != nil || len(ops) != 1 || ops[0].Kind != "sync" {
		t.Fatalf("history %+v %v", ops, err)
	}

	// Nothing changed: nothing to commit.
	again, err := Sync(hub, ix, now)
	if err != nil {
		t.Fatal(err)
	}
	if again.Commit != "" || again.Creates+again.Updates+again.Removes != 0 {
		t.Fatalf("second sync %+v", again)
	}

	// A member changed: one update. A member left: its folder goes.
	writePage(t, b, "wiki/entities/Other.md", "---\ntitle: Other\ntype: entity\nstatus: evergreen\ncreated: 2026-09-01\nupdated: 2026-09-02\ntags: []\n---\n# Other\n\nMore text.\n")
	third, err := Sync(hub, ix, now)
	if err != nil {
		t.Fatal(err)
	}
	if third.Updates != 1 || third.Creates != 0 || third.Removes != 0 {
		t.Fatalf("third sync %+v", third)
	}
	setMembers(t, a)
	ix = index(entryOf(hub), entryOf(a), entryOf(b))
	fourth, err := Sync(hub, ix, now)
	if err != nil {
		t.Fatal(err)
	}
	if fourth.Removes == 0 {
		t.Fatalf("fourth sync %+v", fourth)
	}
	if _, err := os.Stat(hub.Path("wiki/projects/svc-b")); err == nil {
		t.Fatal("svc-b's mirror stayed")
	}
	if _, err := os.Stat(hub.Path("wiki/projects/svc-a/entities/Config.md")); err != nil {
		t.Fatal("svc-a's mirror went")
	}

	// Undo restores the mirror the sync before wrote.
	if _, err := txn.UndoOperation(hub, fourth.OperationID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(hub.Path("wiki/projects/svc-b/entities/Other.md")); err != nil {
		t.Fatal("undo did not restore svc-b's mirror")
	}
}

func TestSyncKeepsTheMirrorOfAMemberItCannotRead(t *testing.T) {
	needGit(t)
	hub := initProject(t, "hub", "hub")
	a := initProject(t, "svc-a", "svc-a")
	setMembers(t, hub, a.Config.ID)
	if _, err := Sync(hub, index(entryOf(hub), entryOf(a)), now); err != nil {
		t.Fatal(err)
	}
	// The atlas no longer lists a: its mirror stays and the index says so.
	res, err := Sync(hub, index(entryOf(hub)), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Members) != 1 || res.Members[0].Error == "" || res.Removes != 0 {
		t.Fatalf("result %+v", res)
	}
	if _, err := os.Stat(hub.Path("wiki/projects/svc-a/svc-a.md")); err != nil {
		t.Fatal("the old mirror went")
	}
	if idx := readPage(t, hub, "wiki/projects/projects.md"); !strings.Contains(idx, "not read") {
		t.Fatalf("index:\n%s", idx)
	}
}

func TestSyncWithNoMembersRemovesEveryMirror(t *testing.T) {
	needGit(t)
	hub := initProject(t, "hub", "hub")
	a := initProject(t, "svc-a", "svc-a")
	setMembers(t, hub, a.Config.ID)
	if _, err := Sync(hub, index(entryOf(hub), entryOf(a)), now); err != nil {
		t.Fatal(err)
	}
	setMembers(t, hub)
	res, err := Sync(hub, index(entryOf(hub), entryOf(a)), now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Removes == 0 {
		t.Fatalf("result %+v", res)
	}
	if !strings.Contains(readPage(t, hub, "wiki/projects/projects.md"), "No members") {
		t.Fatal("index still lists members")
	}
}

func needGit(t *testing.T) {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
}

func initProject(t *testing.T, dir, name string) *project.Project {
	t.Helper()
	work := filepath.Join(t.TempDir(), dir)
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := project.Init(work, project.Options{Name: name}, now)
	if err != nil {
		t.Fatal(err)
	}
	return res.Project
}

func entryOf(p *project.Project) registry.Entry {
	return registry.Entry{ID: p.Config.ID, Name: p.Name(), Path: p.Root, Members: p.Config.Members}
}

func setMembers(t *testing.T, p *project.Project, ids ...string) {
	t.Helper()
	err := project.UpdateConfig(p.Root, "members", now, func(c *project.Config) error {
		c.Members = ids
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	p.Config.Members = ids
}

func writePage(t *testing.T, p *project.Project, rel, content string) {
	t.Helper()
	path := p.Path(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readPage(t *testing.T, p *project.Project, rel string) string {
	t.Helper()
	data, err := os.ReadFile(p.Path(rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
