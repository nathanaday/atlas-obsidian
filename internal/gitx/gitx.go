// Package gitx runs the few git commands the core needs and parses their output.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Repo is a git working tree rooted at Dir. Prefix is the path inside it that the caller
// owns, with a trailing slash ("atlas/webapp/"), or "" for the whole tree: every command is
// scoped to it, and every path a method takes or returns is relative to it. Scope narrows
// a command further, to the given paths under Prefix, without changing what a path means:
// the engine owns a few folders of a project and must not see, stage, or commit the rest.
type Repo struct {
	Dir    string
	Prefix string
	Scope  []string
}

// Scoped is the same repository narrowed to the given paths under Prefix. A folder ends
// in a slash. With no paths it is the repository as it was.
func (r Repo) Scoped(paths ...string) Repo {
	r.Scope = paths
	return r
}

// Available reports whether the git command is on PATH.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// ErrNotRepo is returned when Dir is not the top level of a git working tree.
var ErrNotRepo = errors.New("not a git repository")

func (r Repo) cmd(args ...string) *exec.Cmd {
	return r.cmdEnv(nil, args...)
}

// cmdEnv is cmd with more environment variables, for a command that needs its own index.
func (r Repo) cmdEnv(env []string, args ...string) *exec.Cmd {
	full := append([]string{"-c", "core.quotePath=false", "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	cmd.Env = append(cmd.Env, env...)
	return cmd
}

func (r Repo) run(args ...string) (string, error) {
	return r.runEnv(nil, args...)
}

func (r Repo) runEnv(env []string, args ...string) (string, error) {
	cmd := r.cmdEnv(env, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git %s: %s", args[0], detail)
	}
	return stdout.String(), nil
}

// in returns p under Prefix.
func (r Repo) in(p string) string {
	return r.Prefix + p
}

// out strips Prefix from p. It returns false when p does not carry the prefix.
func (r Repo) out(p string) (string, bool) {
	if r.Prefix == "" {
		return p, true
	}
	return strings.CutPrefix(p, r.Prefix)
}

// pathspecs are the arguments that scope a command: Scope under Prefix, else Prefix, else
// "." for the whole tree.
func (r Repo) pathspecs() []string {
	switch {
	case len(r.Scope) > 0:
		out := make([]string, 0, len(r.Scope))
		for _, s := range r.Scope {
			out = append(out, r.Prefix+s)
		}
		return out
	case r.Prefix != "":
		return []string{r.Prefix}
	}
	return []string{"."}
}

// limit is "--" and the pathspecs, or nothing when the command may cover the whole tree.
func (r Repo) limit() []string {
	if r.Prefix == "" && len(r.Scope) == 0 {
		return nil
	}
	return append([]string{"--"}, r.pathspecs()...)
}

// At is the repository that holds dir: dir itself when it is the top of a working tree or
// in none, else the working tree above it, scoped to dir.
func At(dir string) Repo {
	out, err := Repo{Dir: dir}.run("rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{Dir: dir}
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(out))
	if err != nil {
		return Repo{Dir: dir}
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return Repo{Dir: dir}
	}
	rel, err := filepath.Rel(top, real)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return Repo{Dir: dir}
	}
	return Repo{Dir: top, Prefix: filepath.ToSlash(rel) + "/"}
}

// Init creates a repository at Dir with main as its first branch.
func (r Repo) Init() error {
	if _, err := r.run("init", "-q"); err != nil {
		return err
	}
	_, err := r.run("symbolic-ref", "HEAD", "refs/heads/main")
	return err
}

// Clone clones url into Dir, which must not exist or must be empty.
func (r Repo) Clone(url string) error {
	if err := os.MkdirAll(filepath.Dir(r.Dir), 0o755); err != nil {
		return err
	}
	parent := Repo{Dir: filepath.Dir(r.Dir)}
	_, err := parent.run("clone", "--quiet", url, r.Dir)
	return err
}

// RemoteURL is the fetch URL of origin, or "" when there is none.
func (r Repo) RemoteURL() string {
	out, err := r.run("remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// IsRepo reports whether Dir itself is the top level of a working tree.
// A vault inside another repository does not count: its history must be its own.
func (r Repo) IsRepo() bool {
	out, err := r.run("rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	top, err := filepath.EvalSymlinks(strings.TrimSpace(out))
	if err != nil {
		return false
	}
	dir, err := filepath.EvalSymlinks(r.Dir)
	if err != nil {
		return false
	}
	return top == dir
}

// InsideOtherRepo reports whether Dir is inside a working tree whose top level is above it.
func (r Repo) InsideOtherRepo() bool {
	out, err := r.run("rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	return !r.IsRepo() && strings.TrimSpace(out) != ""
}

// hasCommit reports whether the repository holds any commit at all.
func (r Repo) hasCommit() bool {
	_, err := r.run("rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

// HasHead reports whether there is history to work with. With a prefix, only a commit that
// touched it counts: a repository whose commits never reached the prefix has none there.
func (r Repo) HasHead() bool {
	if !r.hasCommit() {
		return false
	}
	if r.Prefix == "" {
		return true
	}
	commits, err := r.Log(1)
	return err == nil && len(commits) > 0
}

// Ignored reports whether the repository's ignore rules cover path. A directory needs a
// trailing slash, because a rule that ends in one matches directories only.
func (r Repo) Ignored(path string) bool {
	_, err := r.run("check-ignore", "-q", "--", r.in(path))
	return err == nil
}

// Head returns the current commit.
func (r Repo) Head() (string, error) {
	out, err := r.run("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Entry is one line of git status: a two-letter code and a path relative to Dir.
type Entry struct {
	Code string
	Path string
}

// Status lists every changed or untracked path. Untracked directories are expanded to files.
func (r Repo) Status() ([]Entry, error) {
	args := append([]string{"status", "--porcelain=v1", "-z", "-uall"}, r.limit()...)
	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}
	var entries []Entry
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if len(f) < 4 {
			continue
		}
		code, path := f[:2], f[3:]
		if code[0] == 'R' || code[0] == 'C' {
			// A rename carries the source path in the next field.
			i++
		}
		stripped, ok := r.out(path)
		if !ok {
			continue
		}
		entries = append(entries, Entry{Code: code, Path: stripped})
	}
	return entries, nil
}

// Dirty reports whether the working tree has any change git would notice.
func (r Repo) Dirty() (bool, error) {
	entries, err := r.Status()
	return len(entries) > 0, err
}

// AddAll stages every change in the tree, deletions included. A scoped path that names
// nothing, on disk or in the index, is left out, because git add refuses a pathspec that
// matches nothing and a project holds folders an operation may never have written.
func (r Repo) AddAll() error {
	specs := r.pathspecs()
	if len(r.Scope) > 0 {
		specs = r.matching(specs)
		if len(specs) == 0 {
			return nil
		}
	}
	_, err := r.run(append([]string{"add", "-A", "--"}, specs...)...)
	return err
}

// matching keeps the pathspecs that name something: a path on disk, or one git tracks.
func (r Repo) matching(specs []string) []string {
	var out []string
	for _, spec := range specs {
		if _, err := os.Stat(filepath.Join(r.Dir, filepath.FromSlash(strings.TrimSuffix(spec, "/")))); err == nil {
			out = append(out, spec)
			continue
		}
		if found, err := r.run("ls-files", "-z", "--", spec); err == nil && strings.TrimRight(found, "\x00") != "" {
			out = append(out, spec)
		}
	}
	return out
}

// Add stages the given paths, deletions included. A path that names nothing, on disk or in
// the index, is left out, because git add refuses a pathspec that matches nothing and a
// caller may name a file it removed that this repository never tracked.
func (r Repo) Add(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	specs := make([]string, 0, len(paths))
	for _, p := range paths {
		specs = append(specs, r.in(p))
	}
	specs = r.matching(specs)
	if len(specs) == 0 {
		return nil
	}
	_, err := r.run(append([]string{"add", "-A", "--"}, specs...)...)
	return err
}

func (r Repo) identityArgs() []string {
	out, err := r.run("config", "--get", "user.email")
	if err == nil && strings.TrimSpace(out) != "" {
		return nil
	}
	return []string{"-c", "user.name=atlas-obsidian", "-c", "user.email=atlas-obsidian@localhost"}
}

// Commit records the index with message and returns the new commit. It falls back to a
// local identity when the user has none configured, and skips commit hooks. With a prefix
// or a scope, it records the paths staged under it and leaves everything else alone.
func (r Repo) Commit(message string) (string, error) {
	if r.Prefix != "" || len(r.Scope) > 0 {
		return r.commitPrefix(message)
	}
	args := append(r.identityArgs(), "commit", "-q", "--no-verify", "-m", message)
	if _, err := r.run(args...); err != nil {
		return "", err
	}
	return r.Head()
}

// commitPrefix records the paths staged under the prefix, and under Scope when it is set,
// and nothing else.
// `git commit -- <prefix>` cannot do that: a pathspec puts git in --only mode, which
// records the working tree of every tracked file under the prefix, so a page the user
// changed by hand and never staged would land in the commit. The tree is built instead in
// a temporary index that starts at HEAD, and plumbing writes the commit, which runs no
// hooks. It records the working tree of the staged paths, which is what the engine
// staged a moment before, and it allows an empty commit; no caller reaches either case.
func (r Repo) commitPrefix(message string) (string, error) {
	hasHead := r.hasCommit()
	paths, err := r.stagedPaths(hasHead)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("nothing staged under %s", strings.Join(r.pathspecs(), ", "))
	}
	head := ""
	if hasHead {
		if head, err = r.Head(); err != nil {
			return "", err
		}
	}
	dir, err := os.MkdirTemp("", "atlas-obsidian-index")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	index := []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")}
	read := []string{"read-tree", "--empty"}
	if hasHead {
		read = []string{"read-tree", head}
	}
	if _, err := r.runEnv(index, read...); err != nil {
		return "", err
	}
	if err := r.updateIndex(index, paths); err != nil {
		return "", err
	}
	tree, err := r.runEnv(index, "write-tree")
	if err != nil {
		return "", err
	}
	args := append(r.identityArgs(), "commit-tree", strings.TrimSpace(tree))
	if hasHead {
		args = append(args, "-p", head)
	}
	out, err := r.run(append(args, "-m", message)...)
	if err != nil {
		return "", err
	}
	sha := strings.TrimSpace(out)
	subject, _, _ := strings.Cut(message, "\n")
	move := []string{"update-ref", "-m", "commit: " + subject, "HEAD", sha}
	if hasHead {
		move = append(move, head)
	}
	if _, err := r.run(move...); err != nil {
		return "", err
	}
	// The real index still holds what the caller staged. Reading those paths again makes
	// its entries match the new HEAD, so they read as clean.
	if err := r.updateIndex(nil, paths); err != nil {
		return "", err
	}
	return sha, nil
}

// stagedPaths lists the paths staged under the prefix, relative to Dir, with deletions.
// Renames are not detected, so a rename gives both the old path and the new one.
func (r Repo) stagedPaths(hasHead bool) ([]string, error) {
	args := append([]string{"diff", "--cached", "--name-only", "-z", "--no-renames"}, r.limit()...)
	if !hasHead {
		args = append([]string{"ls-files", "-z"}, r.limit()...)
	}
	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// updateIndex reads the given paths from the working tree into an index, adding the new
// ones and removing the ones that are gone. The paths go in on stdin, so their number and
// their characters do not matter.
func (r Repo) updateIndex(env, paths []string) error {
	cmd := r.cmdEnv(env, "update-index", "--add", "--remove", "-z", "--stdin")
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\x00") + "\x00")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git update-index: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

// unfinished are the files git leaves behind while an operation of several steps is still
// open, with the name of that operation.
var unfinished = []struct{ path, what string }{
	{"MERGE_HEAD", "merge"},
	{"rebase-merge", "rebase"},
	{"rebase-apply", "rebase"},
	{"CHERRY_PICK_HEAD", "cherry-pick"},
	{"REVERT_HEAD", "revert"},
}

// InProgress names the operation the repository is in the middle of: merge, rebase,
// cherry-pick, or revert. It is "" when there is none.
func (r Repo) InProgress() string {
	args := []string{"rev-parse"}
	for _, u := range unfinished {
		args = append(args, "--git-path", u.path)
	}
	out, err := r.run(args...)
	if err != nil {
		return ""
	}
	for i, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if i >= len(unfinished) {
			break
		}
		p := strings.TrimSpace(line)
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(r.Dir, p)
		}
		if _, err := os.Lstat(p); err == nil {
			return unfinished[i].what
		}
	}
	return ""
}

// CheckIdle refuses the write while the repository is in the middle of another operation.
// A commit made then joins that operation's work, or an abort throws it away.
func (r Repo) CheckIdle() error {
	if what := r.InProgress(); what != "" {
		return fmt.Errorf("%s is in the middle of a %s; finish or abort it first", r.Dir, what)
	}
	return nil
}

// RestoreFromHead puts the given tracked paths back to their HEAD content.
func (r Repo) RestoreFromHead(paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	args := []string{"checkout", "HEAD", "--"}
	for _, p := range paths {
		args = append(args, r.in(p))
	}
	_, err := r.run(args...)
	return err
}

// Move runs git mv, so a file keeps its history in this repository. from and to are
// relative to Dir, not to Prefix, because a move crosses the prefix.
func (r Repo) Move(from, to string) error {
	_, err := r.run("mv", "--", from, to)
	return err
}

// Tracked reports whether HEAD contains path.
func (r Repo) Tracked(path string) bool {
	_, err := r.run("cat-file", "-e", "HEAD:"+r.in(path))
	return err == nil
}

// Commit is one entry of the log with the trailers the core wrote.
type Commit struct {
	SHA      string
	Date     time.Time
	Subject  string
	Body     string
	Trailers map[string]string
}

// Log returns the newest n commits, or all of them when n is 0. Git counts n after it
// filters by path, so with a prefix Log(1) is the newest commit that touched the prefix.
func (r Repo) Log(n int) ([]Commit, error) {
	if !r.hasCommit() {
		return nil, nil
	}
	args := []string{"log", "--format=%H%x00%aI%x00%s%x00%b%x1e"}
	if n > 0 {
		args = append(args, fmt.Sprintf("-n%d", n))
	}
	args = append(args, r.limit()...)
	out, err := r.run(args...)
	if err != nil {
		return nil, err
	}
	return parseLog(out), nil
}

func parseLog(out string) []Commit {
	var commits []Commit
	for _, record := range strings.Split(out, "\x1e") {
		record = strings.TrimLeft(record, "\n")
		fields := strings.SplitN(record, "\x00", 4)
		if len(fields) < 4 {
			continue
		}
		date, _ := time.Parse(time.RFC3339, fields[1])
		body := strings.TrimSpace(fields[3])
		commits = append(commits, Commit{
			SHA: fields[0], Date: date, Subject: fields[2], Body: body, Trailers: trailers(body),
		})
	}
	return commits
}

// LogFollow lists the commits that touched one path, newest first, across renames.
func (r Repo) LogFollow(path string) ([]Commit, error) {
	if !r.hasCommit() {
		return nil, nil
	}
	out, err := r.run("log", "--follow", "--format=%H%x00%aI%x00%s%x00%b%x1e", "--", r.in(path))
	if err != nil {
		return nil, err
	}
	return parseLog(out), nil
}

// trailers parses `key: value` lines from the last paragraph of a commit body.
func trailers(body string) map[string]string {
	out := map[string]string{}
	paragraphs := strings.Split(strings.TrimSpace(body), "\n\n")
	if len(paragraphs) == 0 {
		return out
	}
	for _, line := range strings.Split(paragraphs[len(paragraphs)-1], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.ContainsAny(key, " \t") {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

// RestoreFrom puts each path back to what rev holds, in the working tree and the index: it
// writes the file rev has and removes the ones rev does not. It touches no other path, so
// it works while the rest of the working tree has changes of its own, where git revert
// refuses. Paths are relative to Prefix. An empty rev removes every path.
func (r Repo) RestoreFrom(rev string, paths ...string) error {
	var restore, remove []string
	for _, p := range paths {
		full := r.in(p)
		if rev != "" && r.has(rev+":"+full) {
			restore = append(restore, full)
			continue
		}
		remove = append(remove, full)
	}
	if len(restore) > 0 {
		if _, err := r.run(append([]string{"checkout", rev, "--"}, restore...)...); err != nil {
			return err
		}
	}
	if len(remove) > 0 {
		if _, err := r.run(append([]string{"rm", "-q", "-f", "--ignore-unmatch", "--"}, remove...)...); err != nil {
			return err
		}
	}
	return nil
}

// has reports whether an object exists, such as "HEAD:wiki/index.md".
func (r Repo) has(object string) bool {
	_, err := r.run("cat-file", "-e", object)
	return err == nil
}

// Parent is the first parent of rev, or "" when rev is a root commit.
func (r Repo) Parent(rev string) string {
	out, err := r.run("rev-parse", "--verify", "-q", rev+"^")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Staged reports whether anything is staged within the repository's scope. A commit with
// nothing staged is refused, so a caller that may have written nothing asks first.
func (r Repo) Staged() (bool, error) {
	if !r.hasCommit() {
		files, err := r.LsFiles()
		return len(files) > 0, err
	}
	paths, err := r.stagedPaths(true)
	return len(paths) > 0, err
}

// Unchanged reports whether every path is the same in the working tree as it is in rev.
// Paths are relative to Prefix.
func (r Repo) Unchanged(rev string, paths ...string) (bool, error) {
	if len(paths) == 0 {
		return true, nil
	}
	args := []string{"diff", "--quiet", rev, "--"}
	for _, p := range paths {
		args = append(args, r.in(p))
	}
	cmd := r.cmd(args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git diff: %s", strings.TrimSpace(stderr.String()))
}

// RevertNoCommit applies the inverse of sha to the index and tree without committing.
// A conflict undoes the attempt and returns an error.
func (r Repo) RevertNoCommit(sha string) error {
	if _, err := r.run("revert", "--no-commit", sha); err != nil {
		r.undoRevert()
		return err
	}
	return nil
}

// ClearRevert drops the state a revert leaves behind once its commit is made. A commit
// written by plumbing does not clear REVERT_HEAD, so the next write would find a revert
// in progress.
func (r Repo) ClearRevert() {
	r.run("revert", "--quit")
}

// undoRevert puts the tree back after a revert failed. With a prefix it restores only the
// files under it and then drops the sequencer state, because `git revert --abort` takes no
// pathspec: it would reset the whole tree and throw away work the user staged outside.
func (r Repo) undoRevert() {
	if r.Prefix == "" {
		r.run("revert", "--abort")
		return
	}
	r.run("checkout", "HEAD", "--", r.Prefix)
	r.run("revert", "--quit")
}

// ShowFile returns the content of path at rev.
func (r Repo) ShowFile(rev, path string) ([]byte, error) {
	full := r.in(path)
	cmd := r.cmd("show", rev+":"+full)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git show %s:%s: %s", rev, full, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// ChangedPaths lists the paths one commit touched.
func (r Repo) ChangedPaths(sha string) ([]string, error) {
	out, err := r.run("show", "--format=", "--name-only", "-z", sha)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p == "" {
			continue
		}
		stripped, ok := r.out(p)
		if !ok {
			continue
		}
		paths = append(paths, stripped)
	}
	return paths, nil
}

// HasCommit reports whether rev names a commit in the repository.
func (r Repo) HasCommit(rev string) bool {
	_, err := r.run("rev-parse", "--verify", "--quiet", rev+"^{commit}")
	return err == nil
}

// Behind counts the commits HEAD has that rev does not. It is 0 when rev is HEAD or a
// descendant of it, and an error when rev is not a commit here. A commit that touched only
// the excluded folders does not count.
func (r Repo) Behind(rev string, exclude ...string) (int, error) {
	out, err := r.run(append([]string{"rev-list", "--count", rev + "..HEAD"}, r.excluding(exclude)...)...)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("git rev-list: %q is not a count", strings.TrimSpace(out))
	}
	return n, nil
}

// Branch is the current branch's name, or "HEAD" when detached.
func (r Repo) Branch() (string, error) {
	out, err := r.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// excluding is the pathspec for Prefix without the given folders, or none when there are
// no folders to leave out.
func (r Repo) excluding(folders []string) []string {
	if len(folders) == 0 {
		return nil
	}
	args := append([]string{"--"}, r.pathspecs()...)
	for _, f := range folders {
		args = append(args, ":(exclude)"+r.in(strings.TrimSuffix(f, "/")))
	}
	return args
}

// Named lists every tracked path under Prefix whose base name is name, relative to it.
func (r Repo) Named(name string) ([]string, error) {
	out, err := r.run("ls-files", "-z", "--", ":(glob)"+r.in("**/"+name))
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if rel, ok := r.out(p); ok && p != "" {
			paths = append(paths, rel)
		}
	}
	return paths, nil
}

// LsFiles lists every tracked path under Prefix, relative to it, in git's order.
func (r Repo) LsFiles() ([]string, error) {
	out, err := r.run(append([]string{"ls-files", "-z"}, r.limit()...)...)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p == "" {
			continue
		}
		if rel, ok := r.out(p); ok {
			paths = append(paths, rel)
		}
	}
	return paths, nil
}

// LogStat is git log --stat for the commits after from up to HEAD, newest first, at most
// max of them when max is above 0, leaving out the excluded folders.
func (r Repo) LogStat(from string, max int, exclude ...string) (string, error) {
	args := []string{"log", "--stat", "--format=%h %as %s", from + "..HEAD"}
	if max > 0 {
		args = append(args, fmt.Sprintf("-n%d", max))
	}
	if len(exclude) > 0 {
		args = append(args, r.excluding(exclude)...)
	} else {
		args = append(args, r.limit()...)
	}
	return r.run(args...)
}
