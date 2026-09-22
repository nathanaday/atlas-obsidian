package mirror

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nathanaday/atlas-obsidian/internal/project"
	"github.com/nathanaday/atlas-obsidian/internal/registry"
	"github.com/nathanaday/atlas-obsidian/internal/threads"
)

// Owner is a project in a hub's closure whose threads the hub can reach: the project,
// opened, and its board as it is now.
type Owner struct {
	Project *project.Project
	Board   *threads.Board
}

// ThreadMembers opens every project in p's closure that tracks threads, with its board,
// in folder order. A hub that tracks no threads, or lists no members, has none. A member
// that cannot be read is left out; sync reports it.
func ThreadMembers(p *project.Project, ix *registry.Index) ([]Owner, error) {
	if !p.Config.Threads || len(p.Config.Members) == 0 || ix == nil {
		return nil, nil
	}
	root := registry.Entry{ID: p.Config.ID, Name: p.Name(), Path: p.Root, Members: p.Config.Members}
	members, err := Closure(ix, root)
	if err != nil {
		return nil, err
	}
	var out []Owner
	for _, m := range members {
		if m.Error != "" {
			continue
		}
		mp, err := project.Open(m.Path)
		if err != nil || !mp.Config.Threads {
			continue
		}
		board, err := threads.Load(mp)
		if err != nil {
			continue
		}
		out = append(out, Owner{Project: mp, Board: board})
	}
	return out, nil
}

// FindThread resolves a thread by its id, its title, or the start of its title: in p
// first, then, when p is a hub, in the projects it mirrors. It returns the project that
// owns the thread, which is where a change to it is made. A key that matches in more than
// one project is refused with the ids.
func FindThread(p *project.Project, ix *registry.Index, key string) (*project.Project, *threads.Thread, error) {
	board, err := threads.Load(p)
	if err != nil {
		return nil, nil, err
	}
	t, err := board.Resolve(key)
	if err == nil {
		return p, t, nil
	}
	if !errors.Is(err, threads.ErrNoThread) {
		return nil, nil, err
	}
	owners, err := ThreadMembers(p, ix)
	if err != nil {
		return nil, nil, err
	}
	type match struct {
		p *project.Project
		t *threads.Thread
	}
	var found []match
	for _, o := range owners {
		t, err := o.Board.Resolve(key)
		if err == nil {
			found = append(found, match{o.Project, t})
			continue
		}
		if !errors.Is(err, threads.ErrNoThread) {
			return nil, nil, fmt.Errorf("in %s: %w", o.Project.Name(), err)
		}
	}
	switch len(found) {
	case 0:
		if len(owners) == 0 {
			return nil, nil, fmt.Errorf("%w %q in %s", threads.ErrNoThread, key, p.Name())
		}
		return nil, nil, fmt.Errorf("%w %q in %s or the %d project%s it mirrors", threads.ErrNoThread, key, p.Name(), len(owners), plural(len(owners)))
	case 1:
		return found[0].p, found[0].t, nil
	}
	var names []string
	for _, m := range found {
		names = append(names, fmt.Sprintf("%s in %s", m.t.ID, m.p.Name()))
	}
	sort.Strings(names)
	return nil, nil, fmt.Errorf("%q matches a thread in %d projects (%s); use the id", key, len(found), strings.Join(names, ", "))
}
