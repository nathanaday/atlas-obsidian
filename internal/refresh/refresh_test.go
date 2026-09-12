package refresh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nathanaday/claude-atlas/internal/console"
	"github.com/nathanaday/claude-atlas/internal/home"
	"github.com/nathanaday/claude-atlas/internal/testutil"
	"github.com/nathanaday/claude-atlas/internal/tree"
	"github.com/nathanaday/claude-atlas/internal/vaults"
)

var today = time.Date(2026, 9, 11, 12, 0, 0, 0, time.Local)

func fakeVault(t *testing.T, log, hot string, pages map[string]string) string {
	t.Helper()
	vault := filepath.Join(t.TempDir(), "vault")
	os.MkdirAll(filepath.Join(vault, "wiki"), 0o755)
	os.WriteFile(filepath.Join(vault, ".claude-obsidian.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(vault, "wiki", "log.md"), []byte(log), 0o644)
	os.WriteFile(filepath.Join(vault, "wiki", "hot.md"), []byte(hot), 0o644)
	for name, text := range pages {
		os.WriteFile(filepath.Join(vault, "wiki", name), []byte(text), 0o644)
	}
	return vault
}

func leaf(vault string) *tree.Project {
	return &tree.Project{Path: "/x.md", Rel: "x", Frontmatter: tree.Frontmatter{Name: "x", Vault: vault, Priority: "normal", State: "active"}}
}

func TestHeat(t *testing.T) {
	for days, want := range map[int]string{0: "hot", 6: "hot", 7: "warm", 29: "warm", 30: "cold"} {
		d := days
		if got := Heat(&d); got != want {
			t.Errorf("%d days: got %s want %s", days, got, want)
		}
	}
	if Heat(nil) != "" {
		t.Error("nil should be unknown")
	}
}

func TestNewestLogDate(t *testing.T) {
	vault := fakeVault(t, "# Log\n\n## 2026-09-04 — a\n\n## 2026-09-10 — b\n\n## not-a-date\n", "", nil)
	got, ok := NewestLogDate(vault)
	if !ok || got.Format("2006-01-02") != "2026-09-10" {
		t.Fatalf("got %v %v", got, ok)
	}
	if _, ok := NewestLogDate(filepath.Join(t.TempDir(), "missing")); ok {
		t.Fatal("missing vault should report no date")
	}
}

func TestActiveThreadsJoinsContinuationLines(t *testing.T) {
	vault := fakeVault(t, "", "# R\n\n## Recent Changes\n\n- ignored\n\n## Active Threads\n\n- First thread\n  continues here.\n- Second\n\n## Later\n\n- no\n", nil)
	got := ActiveThreads(vault)
	if strings.Join(got, "|") != "First thread continues here.|Second" {
		t.Fatalf("got %v", got)
	}
}

func TestSeedPagesCountsFrontmatterStatus(t *testing.T) {
	vault := fakeVault(t, "", "", map[string]string{
		"a.md": "---\ntitle: A\nstatus: seed\n---\n# A\n",
		"b.md": "---\nstatus: \"seed\"\n---\n",
		"c.md": "---\nstatus: evergreen\n---\n",
		"d.md": "no frontmatter\nstatus: seed\n",
	})
	if got := SeedPages(vault); got != 2 {
		t.Fatalf("got %d", got)
	}
}

func TestPlainTextStripsWikilinks(t *testing.T) {
	if got := PlainText("See [[Spec]] and [[Long Name|alias]]."); got != "See Spec and alias." {
		t.Fatalf("got %q", got)
	}
}

func TestDeriveMarksMissingVault(t *testing.T) {
	state := Derive(nil, leaf(filepath.Join(t.TempDir(), "nope")), today, "t")
	if state.VaultOK || state.VaultError != "not found" || state.Heat != "" {
		t.Fatalf("got %+v", state)
	}
	plain := t.TempDir()
	if state := Derive(nil, leaf(plain), today, "t"); state.VaultError != "not a claude-obsidian vault" {
		t.Fatalf("got %+v", state)
	}
}

func TestDeriveTakesLaterOfLogAndMtime(t *testing.T) {
	vault := fakeVault(t, "## 2026-08-01 — old\n", "", nil)
	state := Derive(nil, leaf(vault), time.Now(), "t")
	if state.LastOperation != "2026-08-01" || state.LastTouched != time.Now().Format("2006-01-02") {
		t.Fatalf("got %+v", state)
	}
	if state.DaysIdle == nil || *state.DaysIdle != 0 || state.Heat != "hot" {
		t.Fatalf("idle %v heat %s", state.DaysIdle, state.Heat)
	}
	if state.VaultOK || state.VaultError != "claude-obsidian is not installed" {
		t.Fatalf("got %+v", state)
	}
}

func p(v int) *int { return &v }

func TestSignals(t *testing.T) {
	node := leaf("/v")
	node.Priority, node.ReviewAfter = "high", "2026-01-01"
	notes := strings.Join(Signals(node, &tree.State{VaultOK: true, Heat: "cold", DaysIdle: p(45)}, today), "\n")
	if !strings.Contains(notes, "priority high") || !strings.Contains(notes, "45 days") || !strings.Contains(notes, "review date 2026-01-01") {
		t.Fatalf("got %q", notes)
	}
	blocked := leaf("/v")
	blocked.State, blocked.BlockedOn = "blocked", "hardware"
	if got := Signals(blocked, &tree.State{VaultOK: true, Heat: "hot"}, today); len(got) != 1 || got[0] != "blocked on: hardware" {
		t.Fatalf("got %v", got)
	}
}

func TestRenderListsRowsAndSignals(t *testing.T) {
	node := leaf("/Users/me/v")
	node.Rel, node.Purpose = "work/v", "Why."
	state := &tree.State{VaultOK: true, LastTouched: "2026-08-01", DaysIdle: p(41), Heat: "cold", Pages: p(3), OpenThreads: []string{"thread [[one]]"}, Unfinished: tree.Unfinished{EmptySections: p(1), SeedPages: p(2), DeadLinks: p(0)}}
	res := &Result{Rows: []Row{{node, state}}, Problems: []tree.Problem{{Rel: "stray", Reason: "missing frontmatter"}}}
	page := Render(res, "2026-09-11T20:00:00Z", today)
	for _, want := range []string{
		"| ❄️ cold | [[tree/work/v\\|x]] | work | normal | active | 41d | 3 | 1 | 3 |",
		"| Vault | `/Users/me/v` |",
		"## Signals", "> [!failure] tree/stray.md\n> Not a project: missing frontmatter.", "> [!warning] work/v", "cold for 41 days",
		"## work\n\n### x\n\n> [!abstract] Purpose\n> Why.", "> - thread one",
		"| Unfinished | 1 empty sections · 2 seed pages · 0 dead links |",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q in:\n%s", want, page)
		}
	}
}

func TestRunAgainstARealVault(t *testing.T) {
	prod := testutil.Product(t)
	root := t.TempDir()
	cfg := &home.Config{Schema: home.ConfigSchema, VaultsDir: filepath.Join(root, "Vaults"), AtlasVault: filepath.Join(root, "atlas")}
	os.MkdirAll(cfg.TreeRoot(), 0o755)
	vault := filepath.Join(cfg.VaultsDir, "fresh")
	if err := vaults.Create(prod, vault, console.NewWith(true, strings.NewReader(""), os.Stderr, false), false); err != nil {
		t.Fatal(err)
	}
	project, err := vaults.Register(cfg, vault, vaults.RegisterOptions{Category: "area"})
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(root, "state")
	os.MkdirAll(stateDir, 0o755)
	os.WriteFile(filepath.Join(stateDir, "stale.json"), []byte("{}"), 0o644)
	page, res, err := Run(cfg, stateDir, prod, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	state, err := tree.ReadState(stateDir, project.Rel)
	if err != nil {
		t.Fatal(err)
	}
	if !state.VaultOK || *state.Pages != 4 || state.Heat != "hot" || len(state.OpenThreads) != 1 || *state.Unfinished.EmptySections != 0 || state.Project != "area/fresh" {
		t.Fatalf("state %+v", state)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "stale.json")); err == nil {
		t.Fatal("stale state should be pruned")
	}
	text, _ := os.ReadFile(page)
	if filepath.Base(page) != "Overview.md" || !strings.Contains(string(text), "[[tree/area/fresh\\|fresh]]") || len(res.Rows) != 1 {
		t.Fatalf("page:\n%s", text)
	}
}
