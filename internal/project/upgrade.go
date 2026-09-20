package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
)

// The migration from 3.x, where a project and a knowledge base were two entities. Nothing
// here runs without `atlas-obsidian upgrade`.

// V3Config is what a 3.x identity file says: the same facts, plus the knowledge base the
// project used.
type V3Config struct {
	Schema      string `json:"schema"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Created     string `json:"created"`
	Knowledge   *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"knowledge,omitempty"`
}

// ReadV3 parses a 3.x identity file under work. ok is false when there is none, or its
// schema is another.
func ReadV3(work string) (V3Config, bool) {
	folder, err := Locate(work)
	if err != nil {
		return V3Config{}, false
	}
	data, err := os.ReadFile(filepath.Join(work, Dir, folder, Marker))
	if err != nil {
		return V3Config{}, false
	}
	var cfg V3Config
	if json.Unmarshal(data, &cfg) != nil || cfg.Schema != SchemaV3 {
		return V3Config{}, false
	}
	return cfg, true
}

// KnowledgeConfig is what a 3.x knowledge base's identity file says. Its mode and its
// scope are what upgrade takes into the project.
type KnowledgeConfig struct {
	Schema  string `json:"schema"`
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Mode    Mode   `json:"mode"`
	Created string `json:"created"`
	Scope   string `json:"scope,omitempty"`
}

// ReadKnowledge parses the identity file of a knowledge base at dir.
func ReadKnowledge(dir string) (KnowledgeConfig, bool) {
	data, err := os.ReadFile(filepath.Join(dir, KnowledgeMarker))
	if err != nil {
		return KnowledgeConfig{}, false
	}
	var cfg KnowledgeConfig
	if json.Unmarshal(data, &cfg) != nil || cfg.ID == "" {
		return KnowledgeConfig{}, false
	}
	return cfg, true
}

// AbsorbDirs are the folders of a knowledge base that become a project's own, in the
// order the report names them.
var AbsorbDirs = []string{WikiDir, RawDir, InboxDir, IdeasDir}

// Absorb moves the folders of the knowledge base at from into the project's folder. It
// uses git mv when one repository holds both sides, so the history follows the files;
// otherwise it copies, and leaves the source folder where it is. A folder the project
// already holds is merged into, and a file that exists on both sides stays as the
// project's. It returns what it moved, as "wiki/ (moved)" or "wiki/ (copied)".
func (p *Project) Absorb(from string) ([]string, error) {
	abs, err := filepath.Abs(from)
	if err != nil {
		return nil, err
	}
	if Same(abs, p.Atlas()) {
		return nil, fmt.Errorf("%s is the project's own folder", abs)
	}
	repo := p.Repo()
	// git mv works when one working tree holds both sides and the source is tracked.
	host := ""
	if repo.Prefix != "" {
		host = repo.Dir
	}
	var did []string
	for _, dir := range AbsorbDirs {
		src := filepath.Join(abs, filepath.FromSlash(dir))
		if info, err := os.Stat(src); err != nil || !info.IsDir() {
			continue
		}
		if empty, err := isEmptyDir(src); err == nil && empty {
			continue
		}
		dst := p.Path(dir)
		moved := false
		if host != "" && Under(host, abs) {
			if err := gitMove(host, src, dst); err == nil {
				moved = true
			}
		}
		if !moved {
			if err := copyTree(src, dst); err != nil {
				return did, err
			}
		}
		verb := "copied"
		if moved {
			verb = "moved"
		}
		did = append(did, fmt.Sprintf("%s/ (%s)", dir, verb))
	}
	return did, nil
}

// gitMove runs git mv, which keeps the file's history in one repository. A destination
// that exists already makes git refuse, so the move falls back to a copy.
func gitMove(host, src, dst string) error {
	// The repository reports its top with every symlink resolved; a caller's path may not
	// be, and a path relative to the wrong spelling of the same folder is nonsense.
	resolve := func(p string) string {
		if out, err := filepath.EvalSymlinks(p); err == nil {
			return out
		}
		return p
	}
	top := resolve(host)
	rel := func(p string) (string, error) {
		r, err := filepath.Rel(top, resolve(filepath.Dir(p)))
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(filepath.Join(r, filepath.Base(p))), nil
	}
	from, err := rel(src)
	if err != nil {
		return err
	}
	to, err := rel(dst)
	if err != nil {
		return err
	}
	// A folder init made and left empty is in the way of the move; git mv refuses a
	// destination that exists, so an empty one goes first.
	if info, err := os.Stat(dst); err == nil {
		if !info.IsDir() {
			return errors.New("the destination exists")
		}
		if empty, err := isEmptyDir(dst); err != nil || !empty {
			return errors.New("the destination is not empty")
		}
		if err := os.Remove(dst); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return gitx.Repo{Dir: host}.Move(from, to)
}

// CleanAbsorbed removes what the atlas itself wrote into a knowledge base whose folders
// have moved into a project: the ignore file, the Obsidian settings, and a folder that
// holds nothing but a .gitkeep. It then removes the folder when nothing is left. It stops
// and reports the path when it finds anything else, because everything else is the user's.
func CleanAbsorbed(from string) (bool, error) {
	abs, err := filepath.Abs(from)
	if err != nil {
		return false, err
	}
	if isFile(filepath.Join(abs, KnowledgeMarker)) {
		return false, nil
	}
	ours := map[string]bool{".gitignore": true, ".obsidian": true, MetaDir: true, ".DS_Store": true}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if ours[e.Name()] {
			continue
		}
		if e.IsDir() {
			if empty, err := isEmptyDir(filepath.Join(abs, e.Name())); err == nil && empty {
				continue
			}
		}
		return false, nil
	}
	if err := os.RemoveAll(abs); err != nil {
		return false, err
	}
	return true, nil
}

// MoveStages moves the stage folders of 3.x, which sat beside threads/, under it. It
// reports what it moved.
func MoveStages(p *Project) ([]string, error) {
	repo := p.Repo()
	host := repo.Dir
	var did []string
	for _, dir := range append([]string{}, StageDirs...) {
		name := filepath.Base(dir)
		src := p.Path(name)
		if info, err := os.Stat(src); err != nil || !info.IsDir() {
			continue
		}
		if Same(src, p.Path(dir)) {
			continue
		}
		dst := p.Path(dir)
		if repo.Prefix != "" {
			if err := gitMove(host, src, dst); err == nil {
				did = append(did, name+"/ → "+dir+"/")
				os.Remove(src)
				continue
			}
		}
		if err := copyTree(src, dst); err != nil {
			return did, err
		}
		if err := os.RemoveAll(src); err != nil {
			return did, err
		}
		did = append(did, name+"/ → "+dir+"/")
	}
	// The phase folder moves with them.
	name := filepath.Base(PhasesDir)
	if src := p.Path(name); !Same(src, p.Path(PhasesDir)) {
		if info, err := os.Stat(src); err == nil && info.IsDir() {
			dst := p.Path(PhasesDir)
			moved := false
			if repo.Prefix != "" {
				if err := gitMove(host, src, dst); err == nil {
					os.Remove(src)
					moved = true
				}
			}
			if !moved {
				if err := copyTree(src, dst); err != nil {
					return did, err
				}
				if err := os.RemoveAll(src); err != nil {
					return did, err
				}
			}
			did = append(did, name+"/ → "+PhasesDir+"/")
		}
	}
	return did, nil
}

// RaiseSchema rewrites the identity file at the current schema, with the mode and the
// description the caller decided. It makes no commit; upgrade commits the whole move.
func (p *Project) RaiseSchema(mode Mode, description string) error {
	p.Config.Schema = Schema
	if mode != "" {
		p.Config.Mode = mode
	}
	if p.Config.Mode == "" {
		p.Config.Mode = Generic
	}
	if description != "" {
		p.Config.Description = description
	}
	return os.WriteFile(p.Path(Marker), p.Config.Encode(), 0o644)
}

// JoinText joins a project's description and a knowledge base's scope into one
// description: the work first, then what its wiki holds.
func JoinText(description, scope string) string {
	description, scope = strings.TrimSpace(description), strings.TrimSpace(scope)
	switch {
	case scope == "":
		return description
	case description == "":
		return scope
	case strings.Contains(description, scope):
		return description
	}
	if !strings.HasSuffix(description, ".") {
		description += "."
	}
	return description + " Its wiki holds: " + scope
}

// OpenV3 reads a 3.x project as it is, so upgrade can work on it. Open refuses one.
func OpenV3(work string) (*Project, error) {
	abs, err := filepath.Abs(work)
	if err != nil {
		return nil, err
	}
	folder, err := Locate(abs)
	if err != nil {
		return nil, err
	}
	cfg, err := readConfig(filepath.Join(abs, Dir, folder, Marker))
	if err != nil {
		return nil, err
	}
	if cfg.Schema != SchemaV3 && !Current(cfg.Schema) {
		return nil, fmt.Errorf("%s: unsupported schema %q", filepath.Join(abs, Dir, folder, Marker), cfg.Schema)
	}
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = folder
	}
	return &Project{Root: abs, Folder: folder, Config: cfg}, nil
}

// MakeProject turns a 3.x knowledge base's folder into a project whose wiki is that
// knowledge base: the folder becomes the work, atlas/<name>/ is created inside it, and the
// knowledge base's own folders move in. It removes the old identity file.
func MakeProject(dir string, now time.Time) (*Project, []string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, nil, err
	}
	kb, ok := ReadKnowledge(abs)
	if !ok {
		return nil, nil, fmt.Errorf("%s holds no %s", abs, KnowledgeMarker)
	}
	if IsProject(abs) {
		return nil, nil, fmt.Errorf("%s is a project already", abs)
	}
	name := strings.TrimSpace(kb.Name)
	if name == "" {
		name = filepath.Base(abs)
	}
	folder := FolderName(name)
	if folder == "" {
		return nil, nil, fmt.Errorf("%q leaves no usable folder name", name)
	}
	mode := kb.Mode
	if mode == "" {
		mode = Generic
	}
	p := &Project{Root: abs, Folder: folder, Config: Config{
		Schema: Schema, ID: kb.ID, Name: name, Description: kb.Scope, Mode: mode, Created: kb.Created,
	}}
	if p.Config.Created == "" {
		p.Config.Created = now.Format("2006-01-02")
	}
	if err := os.MkdirAll(p.Atlas(), 0o755); err != nil {
		return nil, nil, err
	}
	did, err := p.Absorb(abs)
	if err != nil {
		return nil, did, err
	}
	if err := p.EnsureFolders(); err != nil {
		return nil, did, err
	}
	if _, err := writeMissing(p.Atlas(), p.Config, now, false); err != nil {
		return nil, did, err
	}
	if err := os.WriteFile(p.Path(Marker), p.Config.Encode(), 0o644); err != nil {
		return nil, did, err
	}
	if err := os.Remove(filepath.Join(abs, KnowledgeMarker)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, did, err
	}
	did = append(did, "removed "+KnowledgeMarker)
	return p, did, nil
}

// isEmptyDir reports whether a folder holds nothing but dot files.
func isEmptyDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			return false, nil
		}
	}
	return true, nil
}

// copyTree copies a folder into dst, keeping files dst already holds.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if _, err := os.Stat(target); err == nil {
			return nil
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
