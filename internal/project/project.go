// Package project knows what a atlas-obsidian project is: a folder atlas/<name>/ inside the
// user's work that holds everything the atlas knows about it. Two halves live there. The
// wiki, under wiki/ with its raw store and its source ledger, is the knowledge base: only
// an operation writes it, and every operation is one commit. The threads, under threads/,
// are the state of the work: the threads package writes their cards and the model writes
// their prose. The folder takes the project's name so that Obsidian, which names a vault
// after its folder, tells one project from another.
package project

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/home"
	"github.com/nathanaday/atlas-obsidian/internal/ledger"
	"github.com/nathanaday/atlas-obsidian/internal/links"
)

const (
	// Dir is the folder under the work folder that holds the project's folder.
	Dir = "atlas"
	// Marker is the identity file inside the project's folder. It is visible because the
	// folder is the user's and the file says what the folder is.
	Marker = "project.json"
	Schema = "atlas-obsidian.project.v1"
	// EnvProject names the project explicitly for the MCP server and the hooks. The
	// launcher sets it.
	EnvProject = "ATLAS_OBSIDIAN_PROJECT"

	// The wiki, and the engine's own paths.
	WikiDir      = "wiki"
	LogPage      = "wiki/log.md"
	HotPage      = "wiki/hot.md"
	IndexPage    = "wiki/index.md"
	OverviewPage = "wiki/overview.md"
	LedgerPath   = ledger.VaultPath
	RawDir       = ".raw"
	CapturedDir  = ".raw/captured"
	MetaDir      = ".vault-meta"
	// A folder's index page takes the folder's name, so no page shares the basename of
	// wiki/index.md.
	CanvasIndex = "wiki/canvases/canvases.md"

	// The threads: the cards and the board at the top, one folder per stage under it.
	ThreadsDir   = "threads"
	ThreadsIndex = "threads/threads.md"
	ArchiveDir   = "threads/archive"
	StubsDir     = "threads/stubs"
	SpecsDir     = "threads/specs"
	PlansDir     = "threads/plans"
	ReceiptsDir  = "threads/receipts"
	PhasesDir    = "threads/phases"

	// What the user drops in, and what nothing reads.
	InboxDir = "inbox"
	IdeasDir = "ideas"
)

// Folders are the folders every project holds, relative to its folder.
var Folders = []string{WikiDir, ThreadsDir, ArchiveDir, StubsDir, SpecsDir, PlansDir, ReceiptsDir, PhasesDir, InboxDir, IdeasDir}

// StageDirs are the folders a thread's documents sit in, in stage order.
var StageDirs = []string{StubsDir, SpecsDir, PlansDir, ReceiptsDir}

// EngineScope are the paths inside the project's folder that the engine owns. Every git
// command an operation runs is scoped to them, so an apply, the manual commit before it,
// and an undo never see the user's code or a thread document. The inbox is among them
// because an ingest removes the sources it has filed.
var EngineScope = []string{WikiDir + "/", RawDir + "/", InboxDir + "/", Marker}

// Mode is the filing methodology for new wiki pages.
type Mode string

const (
	Generic Mode = "generic"
	LYT     Mode = "lyt"
)

var Modes = []Mode{Generic, LYT}

// ParseMode validates a mode name.
func ParseMode(s string) (Mode, error) {
	for _, m := range Modes {
		if string(m) == s {
			return m, nil
		}
	}
	return "", fmt.Errorf("mode must be generic or lyt, not %q", s)
}

// Config is the content of the identity file: the facts that travel with the project. It
// never holds a path; paths are facts about one machine and live in the atlas config.
type Config struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	// Description says what the work is and what its wiki should remember. The ingest and
	// query skills read it to judge what belongs.
	Description string `json:"description,omitempty"`
	Mode        Mode   `json:"mode"`
	Created     string `json:"created"`
}

// Encode renders the identity file.
func (c Config) Encode() []byte {
	data, _ := json.MarshalIndent(c, "", "  ")
	return append(data, '\n')
}

// NewID mints a random UUID (version 4).
func NewID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NewOperationID mints `<kind>-<yyyymmdd>-<hhmmss>-<4 hex>`.
func NewOperationID(kind string, now time.Time) string {
	var b [2]byte
	rand.Read(b[:])
	return fmt.Sprintf("%s-%s-%s", kind, now.UTC().Format("20060102-150405"), hex.EncodeToString(b[:]))
}

// CommitMessage is the subject and trailer format every core commit uses.
func CommitMessage(kind, summary, operationID string) string {
	summary = strings.TrimSpace(strings.ReplaceAll(summary, "\n", " "))
	return fmt.Sprintf("%s: %s\n\natlas-operation: %s\n", kind, summary, operationID)
}

// Project is an opened project. Root is the work folder; Folder is the name of the
// project's folder under Root/atlas/.
type Project struct {
	Root   string
	Folder string
	Config Config
}

// Name is the project's name from its identity file.
func (p *Project) Name() string { return p.Config.Name }

// Atlas is the project's folder, atlas/<name>/. It is the folder the user opens in
// Obsidian and the root every relative path in this package is measured from.
func (p *Project) Atlas() string { return filepath.Join(p.Root, Dir, p.Folder) }

// Rel is the project's folder relative to the work folder, with slashes.
func (p *Project) Rel() string { return Dir + "/" + p.Folder }

// Path joins a path relative to the project's folder onto it.
func (p *Project) Path(rel string) string { return filepath.Join(p.Atlas(), filepath.FromSlash(rel)) }

// Repo is the git repository of the project's folder: the working tree that holds the work,
// scoped to the folder, or a repository at the folder when the work is in none. Setup
// writes through it, because setup writes the whole folder.
func (p *Project) Repo() gitx.Repo { return gitx.At(p.Atlas()) }

// Engine is the repository an operation writes through: the project's folder narrowed to
// the paths the engine owns.
func (p *Project) Engine() gitx.Repo { return p.Repo().Scoped(EngineScope...) }

// Work is the repository of the work itself, which holds the code as well as the project's
// folder. Only what reports on the work reads it.
func (p *Project) Work() gitx.Repo { return gitx.At(p.Root) }

// Host is the top of the working tree that holds the project's folder, or "" when the
// folder is its own repository or in none.
func Host(atlas string) string {
	if repo := gitx.At(atlas); repo.Prefix != "" {
		return repo.Dir
	}
	return ""
}

// HostFor is the top of the working tree a project's folder at path would commit into, or
// "" when it would get a repository of its own. path need not exist. It refuses a path the
// working tree ignores, because no commit could record it.
func HostFor(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	dir := abs
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
	repo := gitx.At(dir)
	if repo.Prefix == "" && !repo.IsRepo() {
		return "", nil
	}
	rest, err := filepath.Rel(dir, abs)
	if err != nil {
		return "", err
	}
	if rest == "." {
		if repo.Prefix == "" {
			return "", nil
		}
		rest = ""
	} else {
		rest = filepath.ToSlash(rest) + "/"
	}
	if repo.Ignored(rest) {
		return "", fmt.Errorf("%s is ignored by the git repository %s; a project's wiki needs its history", abs, repo.Dir)
	}
	return repo.Dir, nil
}

var ErrNotProject = errors.New("not an atlas-obsidian project")

// FolderName is the name of the folder a project with this name sits in: the name cleaned
// so Obsidian can name a vault after it. It is empty when nothing usable is left.
func FolderName(name string) string { return links.CleanName(strings.TrimSpace(name)) }

// Locate returns the name of the project's folder under work/atlas/: the one child that
// holds the identity file. It refuses an atlas/ folder that holds more than one project.
func Locate(work string) (string, error) {
	atlas := filepath.Join(work, Dir)
	// A file named atlas, such as a binary, is not a project.
	if info, err := os.Stat(atlas); err == nil && !info.IsDir() {
		return "", fmt.Errorf("%w: %s has no %s/<name>/%s", ErrNotProject, work, Dir, Marker)
	}
	entries, err := os.ReadDir(atlas)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	var found []string
	for _, e := range entries {
		if isFile(filepath.Join(atlas, e.Name(), Marker)) {
			found = append(found, e.Name())
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf("%w: %s has no %s/<name>/%s", ErrNotProject, work, Dir, Marker)
	case 1:
		return found[0], nil
	}
	return "", fmt.Errorf("%s holds %d projects (%s); a work folder holds one", atlas, len(found), strings.Join(found, ", "))
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// IsProject reports whether work holds a project's identity file under atlas/; Open says
// whether it can be used.
func IsProject(work string) bool {
	_, err := Locate(work)
	return !errors.Is(err, ErrNotProject)
}

// ReadConfig parses the identity file without validating it, given the work folder. ok is
// false when there is none, the layout is not the current one, or it is not JSON.
func ReadConfig(work string) (Config, bool) {
	folder, err := Locate(work)
	if err != nil {
		return Config{}, false
	}
	return ReadMarker(filepath.Join(work, Dir, folder))
}

// ReadMarker parses the identity file inside a project's own folder without validating it.
// Lint reads a folder this way; everything else opens the project.
func ReadMarker(atlas string) (Config, bool) {
	cfg, err := readConfig(filepath.Join(atlas, Marker))
	return cfg, err == nil
}

func readConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Open reads the project whose work folder is work.
func Open(work string) (*Project, error) {
	abs, err := filepath.Abs(work)
	if err != nil {
		return nil, err
	}
	folder, err := Locate(abs)
	if err != nil {
		return nil, err
	}
	marker := filepath.Join(abs, Dir, folder, Marker)
	cfg, err := readConfig(marker)
	if err != nil {
		return nil, err
	}
	if cfg.Schema != Schema {
		return nil, fmt.Errorf("%s: unsupported schema %q", marker, cfg.Schema)
	}
	if cfg.ID == "" {
		return nil, fmt.Errorf("%s has no id", marker)
	}
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = folder
	}
	if cfg.Mode == "" {
		cfg.Mode = Generic
	}
	if _, err := ParseMode(string(cfg.Mode)); err != nil {
		return nil, fmt.Errorf("%s: %w", marker, err)
	}
	return &Project{Root: abs, Folder: folder, Config: cfg}, nil
}

// FindAbove returns the work folder of the nearest project at or above start, or "".
// A session anywhere inside the work belongs to the project.
func FindAbove(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if IsProject(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// Resolve picks a project: the explicit path, then the environment, then the nearest
// project at or above start. It fails closed when none applies.
func Resolve(explicit, envValue, start string) (*Project, error) {
	switch {
	case explicit != "":
		return Open(explicit)
	case envValue != "":
		return Open(envValue)
	}
	if work := FindAbove(start); work != "" {
		return Open(work)
	}
	return nil, fmt.Errorf("%w: none at or above %s; pass a project path or set %s", ErrNotProject, start, EnvProject)
}

// CheckNew says why a project cannot be made at work: the folder is not there, it is a
// project already, or it sits inside another project.
// An atlas/ folder that holds something else is fine; Init refuses only a taken
// atlas/<name>/.
func CheckNew(work string) error {
	abs, err := filepath.Abs(work)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("%s: not found", abs)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", abs)
	}
	if IsProject(abs) {
		return fmt.Errorf("%s is a project already", abs)
	}
	if outer := FindAbove(filepath.Dir(abs)); outer != "" {
		return fmt.Errorf("%s is inside the project %s; a project does not go inside another", abs, outer)
	}
	return nil
}

// Save rewrites the identity file. It is how edit changes a project. A new name moves the
// project's folder to match, and a taken folder refuses the save.
func (p *Project) Save() error {
	if strings.TrimSpace(p.Config.Name) == "" {
		return errors.New("name must not be blank")
	}
	if _, err := ParseMode(string(p.Config.Mode)); err != nil {
		return err
	}
	p.Config.Schema = Schema
	p.Config.Name = strings.TrimSpace(p.Config.Name)
	p.Config.Description = strings.TrimSpace(p.Config.Description)
	from := p.Folder
	if err := p.moveFolder(FolderName(p.Config.Name)); err != nil {
		return err
	}
	if err := os.WriteFile(p.Path(Marker), p.Config.Encode(), 0o644); err != nil {
		if p.Folder != from {
			// The name and the folder never disagree: put the folder back.
			os.Rename(p.Atlas(), filepath.Join(p.Root, Dir, from))
			p.Folder = from
		}
		return err
	}
	return nil
}

// moveFolder renames the project's folder under atlas/ to folder.
func (p *Project) moveFolder(folder string) error {
	if folder == "" {
		return fmt.Errorf("%q leaves no usable folder name", p.Config.Name)
	}
	if folder == p.Folder {
		return nil
	}
	target := filepath.Join(p.Root, Dir, folder)
	if taken, err := os.Stat(target); err == nil {
		// On a case-insensitive filesystem the project's own folder answers to the new
		// name already; renaming it to change its case is still a rename.
		here, err := os.Stat(p.Atlas())
		if err != nil || !os.SameFile(taken, here) {
			return fmt.Errorf("%s already exists; move it aside or choose another name", home.Display(target))
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(p.Atlas(), target); err != nil {
		return err
	}
	p.Folder = folder
	return nil
}

// EnsureFolders creates the folders a project should hold but may lack, as after a clone
// that did not carry empty ones.
func (p *Project) EnsureFolders() error {
	for _, dir := range Folders {
		if err := os.MkdirAll(p.Path(dir), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// Same reports whether two paths name one folder.
func Same(a, b string) bool {
	if a == b {
		return true
	}
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

// writeFile writes data at a path relative to root, making the folders it needs.
func writeFile(root, rel string, data []byte) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// equalFile reports whether the file at root/rel already holds data.
func equalFile(root, rel string, data []byte) bool {
	existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil && bytes.Equal(existing, data)
}
