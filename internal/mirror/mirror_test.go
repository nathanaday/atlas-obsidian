package mirror

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
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

func TestSyncMirrorsTheThreadsOfMembersThatTrackThem(t *testing.T) {
	needGit(t)
	hub := initProject(t, "hub", "hub")
	a := initProject(t, "svc-a", "svc-a")
	b := initProject(t, "svc-b", "svc-b")
	// b tracks no threads; a has one with a stub and a spec that cites a wiki page, and a
	// wiki page that cites the thread.
	if err := project.UpdateConfig(b.Root, "threads", now, func(c *project.Config) error { c.Threads = false; return nil }); err != nil {
		t.Fatal(err)
	}
	b.Config.Threads = false
	th, err := threads.Start(a, threads.New{Title: "Fix it", Text: "Do it."}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := threads.File(a, th.ID, threads.Filing{Stage: threads.Spec, Text: "See [[Config]] and [the stub](<../stubs/Fix it.md>)."}, now); err != nil {
		t.Fatal(err)
	}
	writePage(t, a, "wiki/entities/Config.md", "---\ntitle: Config\ntype: entity\nstatus: evergreen\ncreated: 2026-09-01\nupdated: 2026-09-01\ntags: []\n---\n# Config\n\nTracked in [[threads/stubs/Fix it]].\n")
	setMembers(t, hub, a.Config.ID, b.Config.ID)
	ix := index(entryOf(hub), entryOf(a), entryOf(b))

	res, err := Sync(hub, ix, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Members[0].Threads == 0 || res.Members[1].Threads != 0 {
		t.Fatalf("members %+v", res.Members)
	}
	card := readPage(t, hub, "threads/projects/svc-a/Fix it.md")
	for _, want := range []string{
		"project: \"" + a.Config.ID + "\"",
		"mirror_of: \"threads/Fix it.md\"",
		"thread_id: " + th.ID,
		"[Stub](threads/projects/svc-a/stubs/Fix%20it.md)",
		"![[threads/projects/svc-a/stubs/Fix it|threads/stubs/Fix it]]",
	} {
		if !strings.Contains(card, want) {
			t.Fatalf("mirrored card lacks %q:\n%s", want, card)
		}
	}
	if spec := readPage(t, hub, "threads/projects/svc-a/specs/Fix it.md"); !strings.Contains(spec, "[[wiki/projects/svc-a/entities/Config|Config]]") || !strings.Contains(spec, "[the stub](threads/projects/svc-a/stubs/Fix%20it.md)") || strings.Contains(spec, "commit:") {
		t.Fatalf("mirrored spec:\n%s", spec)
	}
	if page := readPage(t, hub, "wiki/projects/svc-a/entities/Config.md"); !strings.Contains(page, "[[threads/projects/svc-a/stubs/Fix it|threads/stubs/Fix it]]") {
		t.Fatalf("mirrored wiki page:\n%s", page)
	}
	board := readPage(t, hub, project.ThreadsIndex)
	if !strings.Contains(board, "## svc-a\n") || !strings.Contains(board, "![[threads/projects/svc-a/threads]]") || strings.Contains(board, "## svc-b") {
		t.Fatalf("hub board:\n%s", board)
	}
	if _, err := os.Stat(hub.Path("threads/projects/svc-b")); err == nil {
		t.Fatal("a member with threads off was mirrored")
	}
	if _, err := os.Stat(hub.Path("threads/projects/svc-a/threads.md")); err != nil {
		t.Fatal("the member's board was not mirrored")
	}
	// The hub's own threads are untouched by the mirror, and Load sees none of it.
	hubBoard, err := threads.Load(hub)
	if err != nil || len(hubBoard.Threads) != 0 || len(hubBoard.Problems) != 0 {
		t.Fatalf("hub board %+v %v", hubBoard, err)
	}

	// Idempotent.
	again, err := Sync(hub, ix, now)
	if err != nil || again.Creates+again.Updates+again.Removes != 0 || again.Commit != "" {
		t.Fatalf("second sync %+v %v", again, err)
	}

	// A thread filed in the member, and a stray file in the mirror: the thread half
	// changes with no wiki commit.
	if _, err := threads.File(a, th.ID, threads.Filing{Stage: threads.Plan, Text: "1. Do it."}, now); err != nil {
		t.Fatal(err)
	}
	writePage(t, hub, "threads/projects/svc-a/stubs/Stray.md", "---\ntype: stub\nthread: thr-20260101-0000\n---\nstray\n")
	third, err := SyncThreads(hub, ix, now)
	if err != nil || third.Creates != 1 || third.Removes != 1 || third.Updates == 0 {
		t.Fatalf("third sync %+v %v", third, err)
	}
	if _, err := os.Stat(hub.Path("threads/projects/svc-a/plans/Fix it.md")); err != nil {
		t.Fatal("the plan was not mirrored")
	}
	if _, err := os.Stat(hub.Path("threads/projects/svc-a/stubs/Stray.md")); err == nil {
		t.Fatal("the stray file stayed")
	}
	if ops, _ := txn.History(hub, 1, true); len(ops) != 1 || ops[0].ID != res.OperationID {
		t.Fatalf("SyncThreads committed: %+v", ops)
	}

	// The member leaves: its thread mirror goes, and the board no longer embeds it.
	setMembers(t, hub, b.Config.ID)
	ix = index(entryOf(hub), entryOf(a), entryOf(b))
	if _, err := Sync(hub, ix, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(hub.Path("threads/projects/svc-a")); err == nil {
		t.Fatal("svc-a's thread mirror stayed")
	}
	if strings.Contains(readPage(t, hub, project.ThreadsIndex), "## svc-a") {
		t.Fatal("the board still embeds svc-a")
	}
}

func TestAHubWithThreadsOffMirrorsNoThreads(t *testing.T) {
	needGit(t)
	hub := initProject(t, "hub", "hub")
	a := initProject(t, "svc-a", "svc-a")
	if err := project.UpdateConfig(hub.Root, "threads", now, func(c *project.Config) error { c.Threads = false; return nil }); err != nil {
		t.Fatal(err)
	}
	hub.Config.Threads = false
	if _, err := threads.Start(a, threads.New{Title: "Fix it"}, now); err != nil {
		t.Fatal(err)
	}
	setMembers(t, hub, a.Config.ID)
	res, err := Sync(hub, index(entryOf(hub), entryOf(a)), now)
	if err != nil || res.Members[0].Threads != 0 || res.Members[0].Pages == 0 {
		t.Fatalf("result %+v %v", res, err)
	}
	if _, err := os.Stat(hub.Path(project.ThreadMirrorDir)); err == nil {
		t.Fatal("threads were mirrored into a hub with threads off")
	}
}

func TestFindThreadReachesIntoTheMembers(t *testing.T) {
	needGit(t)
	hub := initProject(t, "hub", "hub")
	a := initProject(t, "svc-a", "svc-a")
	b := initProject(t, "svc-b", "svc-b")
	own, _ := threads.Start(hub, threads.New{Title: "Ecosystem"}, now)
	inA, _ := threads.Start(a, threads.New{Title: "Fix it"}, now)
	inB, _ := threads.Start(b, threads.New{Title: "Fix it"}, now)
	setMembers(t, hub, a.Config.ID, b.Config.ID)
	ix := index(entryOf(hub), entryOf(a), entryOf(b))

	if p, th, err := FindThread(hub, ix, "eco"); err != nil || p.Root != hub.Root || th.ID != own.ID {
		t.Fatalf("own thread: %v %v %v", p, th, err)
	}
	if p, th, err := FindThread(hub, ix, inA.ID); err != nil || p.Root != a.Root || th.ID != inA.ID {
		t.Fatalf("by id: %v %v %v", p, th, err)
	}
	if _, _, err := FindThread(hub, ix, "fix"); err == nil || !strings.Contains(err.Error(), "2 projects") || !strings.Contains(err.Error(), inB.ID) {
		t.Fatalf("a title in two members: %v", err)
	}
	if _, _, err := FindThread(hub, ix, "nope"); err == nil || !strings.Contains(err.Error(), "or the 2 projects it mirrors") {
		t.Fatalf("no thread: %v", err)
	}
	// A member is not a hub: it sees its own threads only, and says so plainly.
	if _, _, err := FindThread(a, ix, own.ID); err == nil || !errors.Is(err, threads.ErrNoThread) {
		t.Fatalf("from a member: %v", err)
	}
	// A hub with threads off reaches nothing.
	hub.Config.Threads = false
	if _, _, err := FindThread(hub, ix, inA.ID); !errors.Is(err, threads.ErrOff) {
		t.Fatalf("threads off: %v", err)
	}
}

func TestSyncSendsLinksToAPageThatMovedIntoTheHub(t *testing.T) {
	needGit(t)
	hub := initProject(t, "hub", "hub")
	a := initProject(t, "svc-a", "svc-a")
	other := initProject(t, "other", "other")
	front := "---\ntitle: %s\ntype: concept\nstatus: evergreen\ncreated: 2026-09-01\nupdated: 2026-09-01\ntags: []\n%s---\n"
	// The hub holds the merged page; a keeps a pointer to it and a page that links the
	// pointer; a second pointer names a hub the page is not in, and a third names a page
	// the hub does not hold.
	writePage(t, hub, "wiki/concepts/Widget.md", fmt.Sprintf(front, "Widget", "")+"# Widget\n\nMerged.\n")
	writePage(t, a, "wiki/concepts/Widget.md", fmt.Sprintf(front, "Widget", "moved_to: \"wiki/concepts/Widget.md\"\nmoved_to_project: \""+hub.Config.ID+"\"\n")+"# Widget\n\nMoved to hub.\n")
	writePage(t, a, "wiki/concepts/Gadget.md", fmt.Sprintf(front, "Gadget", "moved_to: \"wiki/concepts/Gadget.md\"\nmoved_to_project: \""+other.Config.ID+"\"\n")+"# Gadget\n\nMoved to other.\n")
	writePage(t, a, "wiki/concepts/Gone.md", fmt.Sprintf(front, "Gone", "moved_to: \"wiki/concepts/Gone.md\"\nmoved_to_project: \""+hub.Config.ID+"\"\n")+"# Gone\n\nMoved to a page the hub lacks.\n")
	writePage(t, a, "wiki/concepts/Uses.md", fmt.Sprintf(front, "Uses", "")+"# Uses\n\nSee [[Widget]], [[Widget|the widget]], [[Gadget]], [[Gone]], and [w](Widget.md).\n")
	writePage(t, a, "wiki/canvases/map.canvas", "{\"nodes\":[{\"id\":\"1\",\"type\":\"file\",\"file\":\"wiki/concepts/Widget.md\",\"x\":0,\"y\":0,\"width\":100,\"height\":50}],\"edges\":[]}\n")
	setMembers(t, hub, a.Config.ID)
	ix := index(entryOf(hub), entryOf(a), entryOf(other))
	if _, err := Sync(hub, ix, now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(hub.Path("wiki/projects/svc-a/concepts/Widget.md")); err == nil {
		t.Fatal("the moved page was mirrored")
	}
	for _, rel := range []string{"wiki/projects/svc-a/concepts/Gadget.md", "wiki/projects/svc-a/concepts/Gone.md"} {
		if _, err := os.Stat(hub.Path(rel)); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
	}
	uses := readPage(t, hub, "wiki/projects/svc-a/concepts/Uses.md")
	for _, want := range []string{
		"[[wiki/concepts/Widget|Widget]]",
		"[[wiki/concepts/Widget|the widget]]",
		"[[wiki/projects/svc-a/concepts/Gadget|Gadget]]",
		"[[wiki/projects/svc-a/concepts/Gone|Gone]]",
		"[w](wiki/concepts/Widget.md)",
	} {
		if !strings.Contains(uses, want) {
			t.Fatalf("mirrored page lacks %q:\n%s", want, uses)
		}
	}
	if canvas := readPage(t, hub, "wiki/projects/svc-a/canvases/map.canvas"); !strings.Contains(canvas, "wiki/concepts/Widget.md") || strings.Contains(canvas, "projects/svc-a") {
		t.Fatalf("canvas node not sent to the hub's page:\n%s", canvas)
	}
}
