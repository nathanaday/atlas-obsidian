package capture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/project"
	"github.com/nathanaday/claude-atlas/internal/registry"
)

func TestStageProjectWritesOneSnapshotIntoTheInbox(t *testing.T) {
	p := newProject(t)
	r := p.Work()
	os.WriteFile(filepath.Join(p.Root, "README.md"), []byte("# code\n"), 0o644)
	r.AddAll()
	first, _ := r.Commit("feat: one")
	en := registry.Entry{ID: p.Config.ID, Name: p.Name(), Path: p.Root}

	elsewhere := registry.Entry{ID: "p-2", Name: "other", Path: t.TempDir()}
	if _, err := StageProject(p, elsewhere, now); err == nil || !strings.Contains(err.Error(), "is not the project at") {
		t.Fatalf("another project: %v", err)
	}
	out, err := StageProject(p, en, now)
	if err != nil {
		t.Fatal(err)
	}
	if !out.New || out.To != "inbox/"+p.Name()+"-"+first[:7]+".md" || out.Commit != first || out.Since != "" || out.Described != nil {
		t.Fatalf("first stage: %+v", out)
	}
	if _, err := os.Stat(p.Path(out.To)); err != nil {
		t.Fatal("the snapshot should be in the inbox")
	}
	files, _ := ListInbox(p, now)
	if len(files) != 1 || files[0].Path != out.To || files[0].Captured {
		t.Fatalf("inbox %+v", files)
	}
	// A snapshot carries frontmatter, so it reads as a source and not as a note.
	if files[0].Hint != HintSource || LooksLikeNote(files[0]) {
		t.Fatalf("hint %q", files[0].Hint)
	}
	// The same commit again: nothing new.
	if again, err := StageProject(p, en, now); err != nil || again.New || again.To != out.To {
		t.Fatalf("again: %+v %v", again, err)
	}
	if len(Sources(p)) != 0 {
		t.Fatal("a snapshot remembers no folder")
	}
	// The snapshot leaves the project's own folder out of the work it lists.
	text, _ := os.ReadFile(p.Path(out.To))
	if strings.Contains(string(text), project.Dir+"/") {
		t.Fatalf("the snapshot lists the project's own folder:\n%s", text)
	}
	// A page describing the work sets since, and a new commit makes a new snapshot.
	os.MkdirAll(p.Path("wiki/entities"), 0o755)
	os.WriteFile(p.Path("wiki/entities/code.md"), []byte("---\ntitle: code\ntype: entity\nentity_type: project\nproject: "+p.Config.ID+"\ncommit: "+first+"\nstatus: developing\ncreated: 2026-09-17\nupdated: 2026-09-17\ntags:\n  - entity\n---\n\n# code\n"), 0o644)
	os.WriteFile(filepath.Join(p.Root, "b.txt"), []byte("b"), 0o644)
	r.AddAll()
	second, _ := r.Commit("feat: two")
	out, err = StageProject(p, en, now)
	if err != nil {
		t.Fatal(err)
	}
	if !out.New || out.To != "inbox/"+p.Name()+"-"+second[:7]+".md" || out.Since != first || out.Described == nil {
		t.Fatalf("second stage: %+v", out)
	}
	if text, _ := os.ReadFile(p.Path(out.To)); !strings.Contains(string(text), "## Changes since "+first[:7]) {
		t.Fatal("the log since the described commit")
	}
}

func TestStageProjectOfAFolderThatIsNotARepository(t *testing.T) {
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	docs := filepath.Join(t.TempDir(), "thesis")
	os.MkdirAll(docs, 0o755)
	os.WriteFile(filepath.Join(docs, "chapter1.md"), []byte("# One\n"), 0o644)
	res, err := project.Init(docs, project.Options{NoGit: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	p := res.Project
	en := registry.Entry{ID: p.Config.ID, Name: "thesis", Path: docs}
	out, err := StageProject(p, en, now)
	if err != nil || !out.New || out.To != "inbox/thesis-2026-09-12.md" || out.Commit != "" {
		t.Fatalf("plain folder: %+v %v", out, err)
	}
}
