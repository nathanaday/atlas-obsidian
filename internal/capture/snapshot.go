package capture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/describe"
	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
)

// ProjectStage is what staging a project's snapshot did.
type ProjectStage struct {
	Project  registry.Entry     `json:"project"`
	Snapshot *describe.Snapshot `json:"-"`
	Commit   string             `json:"commit,omitempty"`
	Since    string             `json:"since,omitempty"`
	To       string             `json:"to"` // vault-relative inbox path
	// New is false when the same snapshot already waits in the inbox or its bytes were
	// captured before; nothing was written then.
	New bool `json:"new"`
	// Described is the page that describes the project today, when one does.
	Described *registry.Description `json:"described,omitempty"`
}

// StageProject writes a snapshot of the work of project e into its own inbox, with the log
// since the commit the page describing it was written from. The snapshot is a source like
// any file; nothing is remembered for a later stage with no paths.
func StageProject(v *project.Project, e registry.Entry, now time.Time) (*ProjectStage, error) {
	if e.Path != v.Root {
		return nil, fmt.Errorf("%s is not the project at %s", e.Name, v.Root)
	}
	described := describe.Page(e)
	since := ""
	if described != nil {
		since = described.Commit
	}
	snap, err := describe.TakeSnapshot(e, since, now)
	if err != nil {
		return nil, err
	}
	out := &ProjectStage{Project: e, Snapshot: snap, Commit: snap.Commit, Since: snap.Since, Described: described}
	known, err := knownHashes(v, now)
	if err != nil {
		return nil, err
	}
	taken := map[string]bool{}
	files, _ := ListInbox(v, now)
	for _, f := range files {
		taken[strings.ToLower(f.Path)] = true
	}
	to := "inbox/" + snap.FileName
	if taken[strings.ToLower(to)] {
		out.To = to
		return out, nil
	}
	sum := hashBytes(snap.Content)
	if known[sum] {
		out.To = to
		return out, nil
	}
	if err := os.MkdirAll(filepath.Dir(v.Path(to)), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(v.Path(to), snap.Content, 0o644); err != nil {
		return nil, err
	}
	out.To, out.New = to, true
	return out, nil
}
