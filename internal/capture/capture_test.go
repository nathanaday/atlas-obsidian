package capture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/claude-atlas/internal/gitx"
	"github.com/nathanaday/claude-atlas/internal/ledger"
	"github.com/nathanaday/claude-atlas/internal/project"
)

var now = time.Date(2026, 9, 12, 15, 0, 0, 0, time.UTC)

// newProject makes a project in a work folder of its own, which git init makes a
// repository, so the engine has a history.
func newProject(t *testing.T) *project.Project {
	t.Helper()
	if !gitx.Available() {
		t.Skip("git is not installed")
	}
	work := filepath.Join(t.TempDir(), "webapp")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := project.Init(work, project.Options{Mode: project.Generic}, now); err != nil {
		t.Fatal(err)
	}
	p, err := project.Open(work)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestListAndCapture(t *testing.T) {
	v := newProject(t)
	os.WriteFile(v.Path("inbox/paper.PDF"), []byte("%PDF fake"), 0o644)
	os.MkdirAll(v.Path("inbox/notes"), 0o755)
	os.WriteFile(v.Path("inbox/notes/a.md"), []byte("# a"), 0o644)
	os.WriteFile(v.Path("inbox/.hidden"), []byte("x"), 0o644)
	files, err := ListInbox(v, now)
	if err != nil || len(files) != 2 || files[0].Path != "inbox/notes/a.md" || files[1].Kind != "pdf" || files[1].Captured {
		t.Fatalf("list %+v %v", files, err)
	}
	res, err := Capture(v, []string{"paper.PDF", v.Path("inbox/notes/a.md")}, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Sources) != 2 || res.Commit == "" || !strings.HasPrefix(res.OperationID, "capture-") {
		t.Fatalf("result %+v", res)
	}
	pdf := res.Sources[0]
	if !strings.HasSuffix(pdf.StoredPath, ".pdf") || !strings.HasPrefix(pdf.StoredPath, ".raw/captured/") || pdf.AlreadyCaptured {
		t.Fatalf("pdf %+v", pdf)
	}
	if data, _ := os.ReadFile(v.Path(pdf.StoredPath)); string(data) != "%PDF fake" {
		t.Fatal("bytes must match")
	}
	if _, err := os.Stat(v.Path("inbox/paper.PDF")); err != nil {
		t.Fatal("capture must not remove inbox files")
	}
	l, _ := ledger.Load(v.Path(project.LedgerPath), now)
	rec, ok := l.Sources[pdf.SourceID]
	if !ok || rec.Title != "paper" || rec.ContentKind != "pdf" || rec.CapturedAt != "2026-09-12" || rec.ReviewStatus != "unreviewed" {
		t.Fatalf("ledger %+v %v", rec, ok)
	}
	if !v.Repo().Tracked(pdf.StoredPath) {
		t.Fatal("captured bytes are committed")
	}
	log, _ := os.ReadFile(v.Path(project.LogPage))
	if !strings.Contains(string(log), "capture paper.PDF, a.md") {
		t.Fatalf("log:\n%s", log)
	}
	again, err := Capture(v, []string{"inbox/paper.PDF"}, nil, now)
	if err != nil || len(again.Sources) != 1 || !again.Sources[0].AlreadyCaptured || again.Commit != "" {
		t.Fatalf("second capture %+v %v", again, err)
	}
	files, _ = ListInbox(v, now)
	if !files[1].Captured || files[1].SourceID != pdf.SourceID {
		t.Fatalf("list after %+v", files)
	}
	if _, err := Capture(v, []string{"../outside.md"}, nil, now); err == nil {
		t.Fatal("paths outside inbox must fail")
	}
	if _, err := Capture(v, []string{t.TempDir()}, nil, now); err == nil {
		t.Fatal("absolute paths outside the vault must fail")
	}
}

func TestCaptureRefusesAFileOverTheSizeCapWithoutReadingIt(t *testing.T) {
	v := newProject(t)
	f, err := os.Create(v.Path("inbox/huge.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxFileBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := v.Repo().Log(1)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Capture(v, []string{"huge.bin"}, nil, now)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("%d", MaxFileBytes)) {
		t.Fatalf("a file over the cap must be refused and name it: %v", err)
	}

	l, err := ledger.Load(v.Path(project.LedgerPath), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Sources) != 0 {
		t.Fatalf("a refused capture must not record a source, got %+v", l.Sources)
	}

	after, err := v.Repo().Log(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) || before[0].SHA != after[0].SHA {
		t.Fatal("a refused capture must not commit")
	}
}

func TestCaptureRecordsTheProjectItCameThrough(t *testing.T) {
	kb := newProject(t)
	os.WriteFile(kb.Path("inbox/paper.md"), []byte("# paper"), 0o644)
	via := ledger.Via{ID: "p-1", Name: "webapp"}
	res, err := Capture(kb, []string{"paper.md"}, &via, now)
	if err != nil || len(res.Sources) != 1 || res.Commit == "" {
		t.Fatalf("result %+v %v", res, err)
	}
	got := res.Sources[0]
	l, err := ledger.Load(kb.Path(project.LedgerPath), now)
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := l.Sources[got.SourceID]
	if !ok || rec.Via == nil || rec.Via.Name != "webapp" || rec.Via.ID != "p-1" {
		t.Fatalf("ledger record %+v %v", rec, ok)
	}
	again, err := Capture(kb, []string{"paper.md"}, &via, now)
	if err != nil || len(again.Sources) != 1 || !again.Sources[0].AlreadyCaptured {
		t.Fatalf("second capture %+v %v", again, err)
	}
	files, err := ListInbox(kb, now)
	if err != nil || len(files) != 1 || !files[0].Captured || files[0].SourceID != got.SourceID {
		t.Fatalf("list %+v %v", files, err)
	}
}

func TestCaptureAcceptsDotsInNamesAndRefusesTraversal(t *testing.T) {
	v := newProject(t)
	name := "L3.1 - Dynamical Sys Cont..md"
	os.WriteFile(v.Path("inbox/"+name), []byte("lecture"), 0o644)
	res, err := Capture(v, []string{name}, nil, now)
	if err != nil {
		t.Fatalf("a name with consecutive dots must capture: %v", err)
	}
	if len(res.Sources) != 1 || res.Sources[0].Path != "inbox/"+name || !strings.HasSuffix(res.Sources[0].StoredPath, ".md") {
		t.Fatalf("result %+v", res.Sources)
	}
	if _, err := Capture(v, []string{"inbox/" + name}, nil, now); err != nil {
		t.Fatalf("vault-relative form: %v", err)
	}
	if _, err := Capture(v, []string{v.Path("inbox/" + name)}, nil, now); err != nil {
		t.Fatalf("absolute form: %v", err)
	}
	for _, bad := range []string{"../wiki/index.md", "inbox/../wiki/index.md", "inbox/../../etc/passwd", filepath.Join(v.Root, "wiki", "index.md")} {
		if _, err := Capture(v, []string{bad}, nil, now); err == nil {
			t.Fatalf("%q must be refused", bad)
		}
	}
}
