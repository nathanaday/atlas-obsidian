package project

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nathanaday/atlas-obsidian/internal/gitx"
	"github.com/nathanaday/atlas-obsidian/internal/ledger"
)

//go:embed all:templates
var templates embed.FS

const templateDir = "templates/project"

// TemplateFiles lists the paths the template provides, relative to the project's folder.
func TemplateFiles() []string {
	var out []string
	fs.WalkDir(templates, templateDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, strings.TrimPrefix(path, templateDir+"/"))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func renderTemplate(rel string, now time.Time) ([]byte, error) {
	data, err := templates.ReadFile(templateDir + "/" + rel)
	if err != nil {
		return nil, err
	}
	return bytes.ReplaceAll(data, []byte("{{generated_date}}"), []byte(now.Format("2006-01-02"))), nil
}

// AppearanceFile is Obsidian's appearance settings, where CSS snippets are enabled.
const AppearanceFile = ".obsidian/appearance.json"

// AppFile is Obsidian's app settings, where the folder for new notes is set.
const AppFile = ".obsidian/app.json"

// Snippet is the project's CSS snippet: the wiki's folder colors and every thread stage's
// callout color and icon.
const Snippet = ".obsidian/snippets/atlas-obsidian.css"

// SnippetName is the snippet's name in Obsidian's settings.
const SnippetName = "atlas-obsidian"

// settingsMerges are the Obsidian settings files an existing folder keeps, with the change
// each one needs.
var settingsMerges = map[string]func(settings map[string]any) bool{
	AppearanceFile: enableSnippet,
	AppFile:        newNotesInWiki,
}

// checkSettings reads every Obsidian settings file the folder holds and refuses one that is
// not a JSON object.
func checkSettings(root string) error {
	for _, rel := range TemplateFiles() {
		if _, ok := settingsMerges[rel]; !ok {
			continue
		}
		if _, _, err := readSettings(root, rel); err != nil {
			return err
		}
	}
	return nil
}

// readSettings parses an existing Obsidian settings file. It reports whether the file is
// there; an empty one parses as no settings at all.
func readSettings(root, rel string) (map[string]any, bool, error) {
	existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	settings := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		var holds map[string]any
		if err := json.Unmarshal(existing, &holds); err != nil || holds == nil {
			return nil, true, fmt.Errorf("%s is not a JSON object; Obsidian wrote it, so fix or remove the file, then try again", rel)
		}
		settings = holds
	}
	return settings, true, nil
}

// mergeSettings applies merge to an existing Obsidian settings file, keeping every other
// setting, and writes the template when there is none. It reports whether the file changed.
func mergeSettings(root, rel string, template []byte, merge func(map[string]any) bool) (bool, error) {
	settings, found, err := readSettings(root, rel)
	if err != nil {
		return false, err
	}
	if !found {
		return true, writeFile(root, rel, template)
	}
	if !merge(settings) {
		return false, nil
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), append(data, '\n'), 0o644)
}

// enableSnippet turns on the project's CSS snippet and turns off the one the tool enabled
// under its earlier name.
func enableSnippet(settings map[string]any) bool {
	var enabled []any
	if list, ok := settings["enabledCssSnippets"].([]any); ok {
		enabled = list
	}
	for _, item := range enabled {
		if item == SnippetName {
			return false
		}
	}
	settings["enabledCssSnippets"] = append(enabled, SnippetName)
	return true
}

// newNotesInWiki puts the notes Obsidian creates, from a click on a link to a missing page
// or from a new note, under wiki/, unless the user chose a location.
func newNotesInWiki(settings map[string]any) bool {
	if _, chosen := settings["newFileLocation"]; chosen {
		return false
	}
	settings["newFileLocation"] = "folder"
	settings["newFileFolderPath"] = WikiDir
	return true
}

// mergeGitignore appends the lines of the template ignore file that are missing.
func mergeGitignore(root string, template []byte) error {
	path := filepath.Join(root, ".gitignore")
	existing, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return os.WriteFile(path, template, 0o644)
	}
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		have[strings.TrimSpace(line)] = true
	}
	var missing []string
	for _, line := range strings.Split(string(template), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || have[trimmed] {
			continue
		}
		missing = append(missing, trimmed)
	}
	if len(missing) == 0 {
		return nil
	}
	var b bytes.Buffer
	b.Write(existing)
	if !bytes.HasSuffix(existing, []byte("\n")) {
		b.WriteString("\n")
	}
	b.WriteString("\n# added by atlas-obsidian\n")
	b.WriteString(strings.Join(missing, "\n") + "\n")
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// writeMissing writes the template, identity, and ledger files that are not there yet.
// With overwrite set (a fresh init) every file is written. Paths are relative to the
// project's folder.
func writeMissing(root string, cfg Config, now time.Time, overwrite bool) ([]string, error) {
	// A settings file the merge cannot read stops the pass before it writes anything, so a
	// refused refresh leaves the folder as it was.
	if !overwrite {
		if err := checkSettings(root); err != nil {
			return nil, err
		}
	}
	var written []string
	put := func(rel string, data []byte) error {
		if !overwrite {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
				return nil
			}
		}
		if err := writeFile(root, rel, data); err != nil {
			return err
		}
		written = append(written, rel)
		return nil
	}
	for _, rel := range TemplateFiles() {
		data, err := renderTemplate(rel, now)
		if err != nil {
			return nil, err
		}
		if rel == ".gitignore" && !overwrite {
			if err := mergeGitignore(root, data); err != nil {
				return nil, err
			}
			continue
		}
		// The snippet is the atlas's to keep current; the settings that enable it are the
		// user's to turn off.
		if rel == Snippet && !overwrite && !equalFile(root, rel, data) {
			if err := writeFile(root, rel, data); err != nil {
				return nil, err
			}
			written = append(written, rel)
			continue
		}
		if merge, ok := settingsMerges[rel]; ok && !overwrite {
			changed, err := mergeSettings(root, rel, data, merge)
			if err != nil {
				return nil, err
			}
			if changed {
				written = append(written, rel)
			}
			continue
		}
		if err := put(rel, data); err != nil {
			return nil, err
		}
	}
	if err := put(Marker, cfg.Encode()); err != nil {
		return nil, err
	}
	if err := put(LedgerPath, ledger.Empty(now).Encode()); err != nil {
		return nil, err
	}
	sort.Strings(written)
	return written, nil
}

// Git says what Init found or did about the work's repository.
type Git string

const (
	GitCreated  Git = "created"  // init made the work a repository
	GitExisting Git = "existing" // the work is a repository already
	GitEnclosed Git = "enclosed" // the work sits inside another repository, which holds its history
	GitSkipped  Git = "skipped"  // the caller asked for no repository
)

// Options say what to make. The name is the work folder's by default.
type Options struct {
	Name        string
	Description string
	Mode        Mode
	// NoGit leaves a work folder that is in no repository without one. The project then has
	// no history, and no operation can run until it gets one.
	NoGit bool
	// NoThreads leaves threads off: no thread folders, and the thread tools refuse.
	NoThreads bool
}

// InitResult reports what Init made.
type InitResult struct {
	Project *Project
	// Written are the paths it wrote, relative to the project's folder.
	Written     []string
	Git         Git
	OperationID string
	Commit      string
	// Host is the working tree the project commits into, or "" when its folder is its own.
	Host string
}

func requireGit() error {
	if !gitx.Available() {
		return errors.New("git is required and is not on PATH")
	}
	return nil
}

// newConfig is the identity file of a project made now.
func newConfig(work string, opts Options, now time.Time) (Config, error) {
	mode := opts.Mode
	if mode == "" {
		mode = Generic
	}
	if _, err := ParseMode(string(mode)); err != nil {
		return Config{}, err
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = filepath.Base(work)
	}
	return Config{
		Schema:      Schema,
		ID:          NewID(),
		Name:        name,
		Description: strings.TrimSpace(opts.Description),
		Mode:        mode,
		Created:     now.Format("2006-01-02"),
		Threads:     !opts.NoThreads,
	}, nil
}

// Init makes the folder at work a project: atlas/<name>/ with its wiki, its threads, its
// inbox, the Obsidian settings, and one setup commit. A work folder in no repository
// becomes one first, unless the caller asks for none, so the wiki has a history to commit
// into. It writes nothing outside atlas/<name>/.
func Init(work string, opts Options, now time.Time) (*InitResult, error) {
	if err := requireGit(); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(work)
	if err != nil {
		return nil, err
	}
	if err := CheckNew(abs); err != nil {
		return nil, err
	}
	cfg, err := newConfig(abs, opts, now)
	if err != nil {
		return nil, err
	}
	folder := FolderName(cfg.Name)
	if folder == "" {
		return nil, fmt.Errorf("%q leaves no usable folder name", cfg.Name)
	}
	atlas := filepath.Join(abs, Dir, folder)
	if entries, err := os.ReadDir(atlas); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("%s/%s/ holds something that is not a project; move it aside or choose another name", Dir, folder)
	}
	if _, err := HostFor(atlas); err != nil {
		return nil, err
	}
	res := &InitResult{Git: GitSkipped}
	host := gitx.Repo{Dir: abs}
	switch {
	case opts.NoGit:
	case host.IsRepo():
		res.Git = GitExisting
	case host.InsideOtherRepo():
		res.Git = GitEnclosed
	default:
		if err := host.Init(); err != nil {
			return nil, fmt.Errorf("git init %s: %w", abs, err)
		}
		res.Git = GitCreated
	}
	undo := func() {
		os.RemoveAll(atlas)
		if res.Git == GitCreated {
			os.RemoveAll(filepath.Join(abs, ".git"))
		}
	}
	p := &Project{Root: abs, Folder: folder, Config: cfg}
	if err := p.EnsureFolders(); err != nil {
		undo()
		return nil, err
	}
	written, err := writeMissing(atlas, cfg, now, true)
	if err != nil {
		undo()
		return nil, err
	}
	res.Project, res.Written, res.Host = p, written, Host(atlas)
	repo := p.Repo()
	if !repo.IsRepo() {
		return res, nil
	}
	if err := repo.CheckIdle(); err != nil {
		undo()
		return nil, err
	}
	if err := repo.AddAll(); err != nil {
		undo()
		return nil, err
	}
	res.OperationID = NewOperationID("setup", now)
	res.Commit, err = repo.Commit(CommitMessage("setup", fmt.Sprintf("initialize project %s (%s mode)", cfg.Name, cfg.Mode), res.OperationID))
	if err != nil {
		undo()
		return nil, err
	}
	return res, nil
}

// RefreshResult reports what Refresh added; an empty result means the project had every
// template file already.
type RefreshResult struct {
	Added []string
}

// Refresh adds the template files a project lacks and rewrites the CSS snippet, as one
// setup commit. A project whose template is complete gains nothing.
func Refresh(p *Project, now time.Time) (*RefreshResult, error) {
	res := &RefreshResult{}
	if err := p.EnsureFolders(); err != nil {
		return res, err
	}
	repo := p.Repo()
	if repo.IsRepo() {
		if err := repo.CheckIdle(); err != nil {
			return res, err
		}
	}
	added, err := writeMissing(p.Atlas(), p.Config, now, false)
	if err != nil {
		return res, err
	}
	res.Added = added
	if len(added) == 0 || !repo.IsRepo() {
		return res, nil
	}
	if err := repo.Add(added...); err != nil {
		return res, err
	}
	// A file rewritten to what the last commit already held leaves nothing to commit.
	staged, err := repo.Staged()
	if err != nil {
		return res, err
	}
	if !staged {
		return res, nil
	}
	if _, err := repo.Commit(CommitMessage("setup", "add "+strings.Join(added, ", "), NewOperationID("setup", now))); err != nil {
		return res, err
	}
	return res, nil
}

// UpdateConfig rewrites the identity file through change and commits it as one setup
// operation named by summary. An unchanged file makes no commit. It is the one way a
// project's own facts change, and the one writer of project.json.
func UpdateConfig(work, summary string, now time.Time, change func(*Config) error) error {
	p, err := Open(work)
	if err != nil {
		return err
	}
	unlock, err := Lock(p.Atlas())
	if err != nil {
		return err
	}
	defer unlock()
	repo := p.Engine()
	if repo.IsRepo() {
		if err := repo.CheckIdle(); err != nil {
			return err
		}
	}
	cfg := p.Config
	if err := change(&cfg); err != nil {
		return err
	}
	if _, err := ParseMode(string(cfg.Mode)); err != nil {
		return err
	}
	if cfg.ID != p.Config.ID {
		return errors.New("a project's id does not change")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("name must not be blank")
	}
	cfg.Schema = Schema
	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Description = strings.TrimSpace(cfg.Description)
	if equalFile(p.Atlas(), Marker, cfg.Encode()) {
		return nil
	}
	// A new name moves the folder, so the identity file and the folder never disagree.
	p.Config = cfg
	if err := p.Save(); err != nil {
		return err
	}
	repo = p.Engine()
	if !repo.IsRepo() {
		return nil
	}
	if err := repo.AddAll(); err != nil {
		return err
	}
	_, err = repo.Commit(CommitMessage("setup", summary, NewOperationID("setup", now)))
	return err
}
