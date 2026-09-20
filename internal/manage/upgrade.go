package manage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/project"
	"github.com/nathanaday/claude-atlas/internal/registry"
	"github.com/nathanaday/claude-atlas/internal/threads"
)

// The migration to 4.0, where a project holds its own wiki. It moves the user's files, so
// nothing here runs without `claude-atlas upgrade`, and every case either does the whole
// move or refuses and says why.

// Upgrade is what one upgrade did or would do.
type Upgrade struct {
	// Path is the folder the upgrade acts on: a project's work folder, or a knowledge
	// base's folder that becomes one.
	Path string
	Name string
	// Absorb is the knowledge base whose folders become the project's wiki, or "".
	Absorb string
	// Did lists what changed, in order. Empty means the folder was current.
	Did []string
	// Problems are the pages a migration could not read; they stay where they are.
	Problems []threads.Problem
	// Refuse says why the upgrade did nothing. The other fields still say what it found.
	Refuse string
	// Users are the work folders of the projects that used this knowledge base. Upgrading
	// one of them absorbs it, so an upgrade of the knowledge base itself is refused.
	Users []string
	at    time.Time
}

// PlanUpgrade decides what upgrading path would do, and writes nothing. path is a work
// folder or a 3.x knowledge base's folder.
func PlanUpgrade(cfg *home.Config, path string, now time.Time) (*Upgrade, error) {
	abs, err := filepath.Abs(home.Expand(path))
	if err != nil {
		return nil, err
	}
	up := &Upgrade{Path: abs, Name: filepath.Base(abs), at: now}
	switch {
	case project.IsProject(abs):
		return planProject(cfg, up, abs)
	case project.IsKnowledge(abs):
		kb, ok := project.ReadKnowledge(abs)
		if !ok {
			return nil, fmt.Errorf("%s: the identity file is not readable; repair it, then upgrade", home.Display(abs))
		}
		up.Name = kb.Name
		if users := projectsUsing(cfg, kb.ID); len(users) > 0 {
			up.Users = users
			up.Refuse = fmt.Sprintf("%s is the knowledge base of %d project%s (%s); upgrade one of them, which absorbs it, then upgrade the rest", kb.Name, len(users), plural(len(users)), namesOf(users))
			return up, nil
		}
		up.Did = append(up.Did, "make this folder a project with its wiki")
		return up, nil
	}
	return nil, fmt.Errorf("%s is neither a project nor a knowledge base of an earlier version", home.Display(abs))
}

// planProject fills in what a project's upgrade would do.
func planProject(cfg *home.Config, up *Upgrade, work string) (*Upgrade, error) {
	if _, err := project.Locate(work); errors.Is(err, project.ErrFlat) {
		up.Did = append(up.Did, "move "+project.Dir+"/ into "+project.Dir+"/<name>/")
	}
	v3, isV3 := project.ReadV3(work)
	p, err := project.OpenV3(work)
	if err == nil {
		up.Name = p.Name()
	}
	if isV3 && v3.Knowledge != nil && v3.Knowledge.ID != "" {
		found := knowledgeAt(cfg, v3.Knowledge.ID)
		switch {
		case found == "":
			up.Refuse = fmt.Sprintf("the knowledge base %s (%s) this project used is not on this machine; the atlas cannot absorb it. Pass --absorb PATH to name its folder, or --no-knowledge to upgrade with an empty wiki", v3.Knowledge.Name, v3.Knowledge.ID)
			return up, nil
		default:
			up.Absorb = found
			up.Did = append(up.Did, "absorb the knowledge base "+v3.Knowledge.Name+" from "+home.Display(found))
		}
	}
	if isV3 {
		up.Did = append(up.Did, "raise the identity file to "+project.Schema)
	}
	if p != nil {
		for _, dir := range project.StageDirs {
			if info, err := os.Stat(p.Path(filepath.Base(dir))); err == nil && info.IsDir() {
				up.Did = append(up.Did, "move the stage folders under "+project.ThreadsDir+"/")
				break
			}
		}
		if threads.Legacy(p) {
			up.Did = append(up.Did, "turn the task pages of 2.x into threads")
		}
	}
	return up, nil
}

// RunUpgrade carries out what PlanUpgrade found. absorb, when given, is the folder of the
// knowledge base to absorb, whatever the identity file names.
func RunUpgrade(h home.Home, cfg *home.Config, up *Upgrade, absorb string, now time.Time) error {
	if up.Refuse != "" && absorb == "" {
		return errors.New(up.Refuse)
	}
	if absorb != "" {
		abs, err := filepath.Abs(home.Expand(absorb))
		if err != nil {
			return err
		}
		if !project.IsKnowledge(abs) {
			return fmt.Errorf("%s holds no %s", home.Display(abs), project.KnowledgeMarker)
		}
		up.Absorb = abs
	}
	up.Did = nil
	if !project.IsProject(up.Path) {
		p, did, err := project.MakeProject(up.Path, now)
		if err != nil {
			return err
		}
		up.Did, up.Name = did, p.Name()
		if cfg.RemoveKnowledge(up.Path) {
			if err := h.Save(cfg); err != nil {
				return err
			}
			up.Did = append(up.Did, "dropped "+home.Display(up.Path)+" from the knowledge bases in the atlas config")
		}
		return finish(h, cfg, p, up, up.Path, now)
	}
	if moved, err := project.MoveFlat(up.Path); err != nil && !errors.Is(err, project.ErrNotProject) {
		return err
	} else if moved {
		up.Did = append(up.Did, "moved "+project.Dir+"/ into "+project.Dir+"/<name>/")
	}
	v3, isV3 := project.ReadV3(up.Path)
	p, err := project.OpenV3(up.Path)
	if err != nil {
		return err
	}
	up.Name = p.Name()
	mode, description := project.Mode(""), ""
	if up.Absorb != "" {
		kb, ok := project.ReadKnowledge(up.Absorb)
		if !ok {
			return fmt.Errorf("%s: the identity file is not readable", home.Display(up.Absorb))
		}
		did, err := p.Absorb(up.Absorb)
		if err != nil {
			return err
		}
		for _, one := range did {
			up.Did = append(up.Did, "absorbed "+one)
		}
		mode, description = kb.Mode, project.JoinText(v3.Description, kb.Scope)
		// A knowledge base that was inside the work is now an empty folder in the project's
		// own repository; its identity file would read as a second project's. One copied
		// from outside keeps everything, because the original stays as it was.
		if project.Under(p.Root, up.Absorb) {
			if err := os.Remove(filepath.Join(up.Absorb, project.KnowledgeMarker)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			gone, err := project.CleanAbsorbed(up.Absorb)
			if err != nil {
				return err
			}
			if gone {
				up.Did = append(up.Did, "removed the empty folder "+home.Display(up.Absorb))
			} else {
				up.Did = append(up.Did, "left "+home.Display(up.Absorb)+", which still holds files of yours")
			}
		}
		if cfg.RemoveKnowledge(up.Absorb) {
			if err := h.Save(cfg); err != nil {
				return err
			}
			up.Did = append(up.Did, "dropped "+home.Display(up.Absorb)+" from the atlas config")
		}
	}
	if isV3 {
		if err := p.RaiseSchema(mode, description); err != nil {
			return err
		}
		up.Did = append(up.Did, "raised the identity file to "+project.Schema)
	}
	return finish(h, cfg, p, up, up.Absorb, now)
}

// finish is the tail of every upgrade: the stage folders move under threads/, the task
// pages of 2.x become threads, the template files and the snippet are brought up to date,
// and the generated pages are rewritten.
func finish(h home.Home, cfg *home.Config, p *project.Project, up *Upgrade, from string, now time.Time) error {
	moved, err := project.MoveStages(p)
	if err != nil {
		return err
	}
	for _, one := range moved {
		up.Did = append(up.Did, "moved "+one)
	}
	if threads.Legacy(p) {
		n, left, err := threads.Migrate(p, now)
		if err != nil {
			return err
		}
		up.Did = append(up.Did, fmt.Sprintf("turned %d task%s into thread%s", n, plural(n), plural(n)))
		up.Problems = append(up.Problems, left...)
	}
	res, err := project.Refresh(p, now)
	if err != nil {
		return err
	}
	if len(res.Added) > 0 {
		up.Did = append(up.Did, "wrote "+joinShort(res.Added))
	}
	if _, err := threads.Sync(p, now); err != nil {
		return err
	}
	if cfg.AddProject(p.Root) {
		if err := h.Save(cfg); err != nil {
			return err
		}
		up.Did = append(up.Did, "added "+home.Display(p.Root)+" to the atlas config")
	}
	return commitMove(p, up, from)
}

// commitMove records the whole upgrade as one setup commit, scoped to the project's folder
// and to the folder it absorbed, so the user's code is left alone. A project in no
// repository commits nothing.
func commitMove(p *project.Project, up *Upgrade, from string) error {
	repo := gitx.At(p.Root)
	if !repo.IsRepo() {
		return nil
	}
	scope := []string{p.Rel() + "/"}
	if from != "" && project.Under(p.Root, from) {
		if rel, err := filepath.Rel(p.Root, from); err == nil && rel != "." {
			scope = append(scope, filepath.ToSlash(rel)+"/")
		}
	}
	repo = repo.Scoped(scope...)
	if err := repo.CheckIdle(); err != nil {
		return err
	}
	if err := repo.AddAll(); err != nil {
		return err
	}
	staged, err := repo.Staged()
	if err != nil || !staged {
		return err
	}
	sha, err := repo.Commit(project.CommitMessage("setup", "upgrade "+up.Name+" to "+project.Schema, project.NewOperationID("setup", nowOf(up))))
	if err != nil {
		return err
	}
	up.Did = append(up.Did, "committed "+sha[:12])
	return nil
}

// nowOf is the time the upgrade ran; the operation id carries it.
func nowOf(up *Upgrade) time.Time { return up.at }

// UpgradeTargets are the folders `upgrade --all` acts on: every project and every 3.x
// knowledge base the config lists, projects first, so a knowledge base is absorbed by the
// project that used it before it is made a project of its own.
func UpgradeTargets(cfg *home.Config) []string {
	var out []string
	seen := map[string]bool{}
	add := func(path string) {
		if abs, err := filepath.Abs(home.Expand(path)); err == nil && !seen[abs] {
			seen[abs] = true
			out = append(out, abs)
		}
	}
	for _, work := range cfg.Projects {
		add(work)
	}
	for _, root := range cfg.Knowledge {
		add(root)
	}
	return out
}

// projectsUsing lists the projects whose 3.x identity file names the knowledge base with
// id.
func projectsUsing(cfg *home.Config, id string) []string {
	var out []string
	for _, work := range cfg.Projects {
		if v3, ok := project.ReadV3(home.Expand(work)); ok && v3.Knowledge != nil && v3.Knowledge.ID == id {
			out = append(out, work)
		}
	}
	return out
}

// knowledgeAt is the folder of the knowledge base with id, from the config's list.
func knowledgeAt(cfg *home.Config, id string) string {
	for _, root := range cfg.Knowledge {
		abs := home.Expand(root)
		if kb, ok := project.ReadKnowledge(abs); ok && kb.ID == id {
			return abs
		}
	}
	return ""
}

// Stale lists the entries the atlas knows that upgrade has not reached yet.
func Stale(ix *registry.Index) []registry.Entry {
	var out []registry.Entry
	for _, e := range ix.Entries {
		if e.Reason == registry.ReasonV3Split || e.Reason == registry.ReasonFlat {
			out = append(out, e)
		}
	}
	return out
}

func namesOf(paths []string) string {
	var out []string
	for _, p := range paths {
		out = append(out, home.Display(p))
	}
	return joinShort(out)
}

func joinShort(list []string) string {
	if len(list) > 4 {
		list = append(list[:4], fmt.Sprintf("and %d more", len(list)-4))
	}
	out := ""
	for i, s := range list {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
