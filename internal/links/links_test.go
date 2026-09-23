package links

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
)

func TestInspectRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("1"), 0o644)
	run("add", "f.txt")
	run("commit", "-q", "-m", "one")
	os.WriteFile(filepath.Join(dir, "g.txt"), []byte("2"), 0o644)
	link := Inspect(Repo, dir)
	if !link.OK || link.Branch != "main" || link.LastCommit == "" || link.Dirty == nil || *link.Dirty != 1 {
		t.Fatalf("got %+v", link)
	}
	plain := Inspect(Repo, t.TempDir())
	if plain.OK || plain.Error != "not a git repository" {
		t.Fatalf("got %+v", plain)
	}
}

func TestRemoteURLIsRepoAndCleanName(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	if IsRepo(dir) {
		t.Fatal("an empty folder is not a repository")
	}
	if err := (gitx.Repo{Dir: dir}).Init(); err != nil {
		t.Fatal(err)
	}
	if !IsRepo(dir) || IsRepo(filepath.Join(dir, "missing")) {
		t.Fatal("IsRepo")
	}
	if RemoteURL(dir) != "" {
		t.Fatal("no remote yet")
	}
	exec.Command("git", "-C", dir, "remote", "add", "origin", "git@example.com:a/x.git").Run()
	if RemoteURL(dir) != "git@example.com:a/x.git" {
		t.Fatalf("remote %q", RemoteURL(dir))
	}
	if CleanName("a/b:c") != "a-b-c" || CleanName("...") != "" || CleanName(" ok ") != "ok" {
		t.Fatal("CleanName")
	}
}

func TestInspectUpstream(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	commit := func(dir, name string) {
		t.Helper()
		os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644)
		git(dir, "add", name)
		git(dir, "commit", "-q", "-m", "add "+name)
	}
	bare, a, b := filepath.Join(root, "bare.git"), filepath.Join(root, "a"), filepath.Join(root, "b")
	git(root, "init", "-q", "--bare", "-b", "main", bare)
	git(root, "clone", "-q", bare, a)
	commit(a, "one")
	git(a, "push", "-q", "origin", "main")
	git(root, "clone", "-q", bare, b)

	link := Inspect(Repo, a)
	if link.Upstream != "origin/main" || link.Remote != bare || link.Head == "" || link.Subject != "add one" {
		t.Fatalf("got %+v", link)
	}
	if link.Diverged() || link.Changed() || *link.Ahead != 0 || *link.Behind != 0 {
		t.Fatalf("in sync, got %+v", link)
	}

	commit(a, "two")
	commit(b, "three")
	git(b, "push", "-q", "origin", "main")
	git(a, "fetch", "-q")
	link = Inspect(Repo, a)
	if *link.Ahead != 1 || *link.Behind != 1 || !link.Diverged() || link.Fetched == "" {
		t.Fatalf("one each way, got %+v", link)
	}

	lone := filepath.Join(root, "lone")
	git(root, "init", "-q", "-b", "main", lone)
	commit(lone, "x")
	if l := Inspect(Repo, lone); l.Upstream != "" || l.Ahead != nil || l.Remote != "" || l.Fetched != "" || l.Diverged() {
		t.Fatalf("no upstream, got %+v", l)
	}
}
