// Package links inspects the git repositories a project points at besides its vault.
// Atlas only reads them.
package links

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/home"
)

const Repo = "repo"

// Link is the derived view of one linked repository.
type Link struct {
	Kind  string `json:"kind"`
	Name  string `json:"name,omitempty"` // the link page, when the folder has one
	Path  string `json:"path"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// Repo facts.
	Branch     string `json:"branch,omitempty"`
	LastCommit string `json:"last_commit,omitempty"`
	Dirty      *int   `json:"dirty,omitempty"`
	Head       string `json:"head,omitempty"`    // the short sha of HEAD
	Subject    string `json:"subject,omitempty"` // HEAD's subject line
	Remote     string `json:"remote,omitempty"`  // the origin URL
	// Upstream is the branch HEAD tracks. Ahead and Behind count against it as of the last
	// fetch, which is Fetched; nothing here reaches the network.
	Upstream string `json:"upstream,omitempty"`
	Ahead    *int   `json:"ahead,omitempty"`
	Behind   *int   `json:"behind,omitempty"`
	Fetched  string `json:"fetched,omitempty"`
}

// Diverged reports whether the branch is ahead of or behind its upstream.
func (l Link) Diverged() bool {
	return l.Ahead != nil && *l.Ahead > 0 || l.Behind != nil && *l.Behind > 0
}

// Changed reports whether the work has uncommitted changes.
func (l Link) Changed() bool { return l.Dirty != nil && *l.Dirty > 0 }

// Touched is the latest date the link shows activity on, if any.
func (l Link) Touched() (time.Time, bool) {
	if t, err := time.ParseInLocation("2006-01-02", l.LastCommit, time.Local); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// Inspect derives the facts for one linked path.
func Inspect(kind, path string) Link {
	link := Link{Kind: kind, Path: path}
	abs := home.Expand(path)
	info, err := os.Stat(abs)
	if err != nil {
		link.Error = "not found"
		return link
	}
	if !info.IsDir() {
		link.Error = "not a directory"
		return link
	}
	link.OK = true
	inspectRepo(&link, abs)
	return link
}

func git(dir string, args ...string) (string, bool) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return strings.TrimSpace(out.String()), true
}

func inspectRepo(link *Link, dir string) {
	if _, err := exec.LookPath("git"); err != nil {
		return
	}
	if _, ok := git(dir, "rev-parse", "--is-inside-work-tree"); !ok {
		link.OK = false
		link.Error = "not a git repository"
		return
	}
	if branch, ok := git(dir, "rev-parse", "--abbrev-ref", "HEAD"); ok {
		link.Branch = branch
	}
	if head, ok := git(dir, "log", "-1", "--format=%h%x00%cs%x00%s"); ok {
		if f := strings.SplitN(head, "\x00", 3); len(f) == 3 {
			link.Head, link.LastCommit, link.Subject = f[0], f[1], f[2]
		}
	}
	if url, ok := git(dir, "remote", "get-url", "origin"); ok {
		link.Remote = url
	}
	if up, ok := git(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); ok {
		link.Upstream = up
		if counts, ok := git(dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}"); ok {
			var ahead, behind int
			if _, err := fmt.Sscan(counts, &ahead, &behind); err == nil {
				link.Ahead, link.Behind = &ahead, &behind
			}
		}
	}
	if p, ok := git(dir, "rev-parse", "--git-path", "FETCH_HEAD"); ok {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if info, err := os.Stat(p); err == nil {
			link.Fetched = info.ModTime().Format("2006-01-02")
		}
	}
	if status, ok := git(dir, "status", "--porcelain"); ok {
		n := 0
		if status != "" {
			n = strings.Count(status, "\n") + 1
		}
		link.Dirty = &n
	}
}
