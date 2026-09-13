package gitx

import (
	"os"
	"path/filepath"
	"testing"
)

func repo(t *testing.T) Repo {
	t.Helper()
	if !Available() {
		t.Skip("git is not installed")
	}
	r := Repo{Dir: t.TempDir()}
	if err := r.Init(); err != nil {
		t.Fatal(err)
	}
	return r
}

func write(t *testing.T, r Repo, rel, text string) {
	t.Helper()
	path := filepath.Join(r.Dir, rel)
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInitCommitLogAndTrailers(t *testing.T) {
	r := repo(t)
	if !r.IsRepo() || r.HasHead() {
		t.Fatal("fresh repo should be a repo without HEAD")
	}
	write(t, r, "wiki/a.md", "a")
	write(t, r, "wiki/sub/b.md", "b")
	entries, err := r.Status()
	if err != nil || len(entries) != 2 {
		t.Fatalf("status %v %v", entries, err)
	}
	if err := r.AddAll(); err != nil {
		t.Fatal(err)
	}
	sha, err := r.Commit("setup: init\n\natlas-operation: setup-1")
	if err != nil || len(sha) != 40 {
		t.Fatalf("commit %q %v", sha, err)
	}
	if dirty, _ := r.Dirty(); dirty {
		t.Fatal("tree should be clean after commit")
	}
	commits, err := r.Log(0)
	if err != nil || len(commits) != 1 {
		t.Fatalf("log %v %v", commits, err)
	}
	c := commits[0]
	if c.SHA != sha || c.Subject != "setup: init" || c.Trailers["atlas-operation"] != "setup-1" || c.Date.IsZero() {
		t.Fatalf("commit %+v", c)
	}
	if !r.Tracked("wiki/a.md") || r.Tracked("wiki/nope.md") {
		t.Fatal("tracked check")
	}
	paths, _ := r.ChangedPaths(sha)
	if len(paths) != 2 {
		t.Fatalf("changed %v", paths)
	}
}

func TestRestoreAndRevert(t *testing.T) {
	r := repo(t)
	write(t, r, "wiki/a.md", "one")
	r.AddAll()
	first, _ := r.Commit("first")
	write(t, r, "wiki/a.md", "two")
	if err := r.RestoreFromHead("wiki/a.md"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(r.Dir, "wiki/a.md")); string(data) != "one" {
		t.Fatalf("restore gave %q", data)
	}
	write(t, r, "wiki/a.md", "two")
	r.AddAll()
	second, _ := r.Commit("second")
	if err := r.RevertNoCommit(second); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(r.Dir, "wiki/a.md")); string(data) != "one" {
		t.Fatalf("revert gave %q", data)
	}
	if _, err := r.Commit("undo"); err != nil {
		t.Fatal(err)
	}
	shown, err := r.ShowFile(first, "wiki/a.md")
	if err != nil || string(shown) != "one" {
		t.Fatalf("show %q %v", shown, err)
	}
	// Reverting the first commit now conflicts with nothing but would delete the file; it must not leave a half revert behind.
	write(t, r, "wiki/a.md", "three")
	r.AddAll()
	r.Commit("third")
	if err := r.RevertNoCommit(second); err == nil {
		t.Fatal("reverting a commit whose changes were overwritten should conflict")
	}
	if dirty, _ := r.Dirty(); dirty {
		t.Fatal("a failed revert must be aborted cleanly")
	}
}

func TestNestedRepoIsNotARepo(t *testing.T) {
	r := repo(t)
	inner := Repo{Dir: filepath.Join(r.Dir, "vault")}
	os.MkdirAll(inner.Dir, 0o755)
	if inner.IsRepo() || !inner.InsideOtherRepo() {
		t.Fatal("a directory inside another repo is not its own repo")
	}
}
