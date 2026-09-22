package overlap

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nathanaday/atlas-obsidian/internal/lint"
)

// page renders a wiki page. project stamps it as a mirror of that member.
func page(title, typ, project, extra, body string) string {
	var b strings.Builder
	b.WriteString("---\ntitle: " + title + "\ntype: " + typ + "\nstatus: seed\ncreated: 2026-01-01\nupdated: 2026-01-01\n")
	if !strings.Contains(extra, "tags:") {
		b.WriteString("tags:\n  - " + typ + "\n")
	}
	if project != "" {
		b.WriteString("project: \"" + project + "\"\nmirror_of: \"wiki/x.md\"\n")
	}
	b.WriteString(extra + "---\n# " + title + "\n\n" + body)
	return b.String()
}

func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, text := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(text), 0o644)
	}
	return root
}

const os1 = "The kernel schedules every process and the shell runs commands over the filesystem. Drivers talk to hardware; the scheduler shares the processor; permissions guard files.\n"
const os2 = "Apple ships the kernel with a shell and a filesystem, and every process gets a window server and a launch daemon. Notarization signs the binaries.\n"

func hub(t *testing.T) string {
	return fixture(t, map[string]string{
		"wiki/index.md":                                page("Index", "meta", "", "", "- [[Widget]]\n- [[Configuration]]\n"),
		"wiki/overview.md":                             page("Overview", "overview", "", "", "kernel shell filesystem process scheduler drivers\n"),
		"wiki/hot.md":                                  page("Hot", "meta", "", "", "kernel shell filesystem process scheduler drivers\n"),
		"wiki/entities/Widget.md":                      page("Widget", "entity", "", "", "The hub's widget page.\n"),
		"wiki/concepts/Configuration.md":               page("Configuration", "concept", "", "", "Joins [[wiki/projects/a/entities/Config|Config]] and [[wiki/projects/b/entities/Config|Config]].\n"),
		"wiki/projects/projects.md":                    page("Projects", "meta", "", "", "| a | b |\n"),
		"wiki/projects/a/a.md":                         page("a", "meta", "id-a", "", "- [[wiki/projects/a/entities/Widget|Widget]]\n- kernel shell filesystem process\n"),
		"wiki/projects/a/overview.md":                  page("Overview", "overview", "id-a", "", "kernel shell filesystem process scheduler drivers\n"),
		"wiki/projects/a/entities/Widget.md":           page("Widget", "entity", "id-a", "", "A gadget that counts beans in project a.\n"),
		"wiki/projects/a/entities/Config.md":           page("Config", "entity", "id-a", "", "Settings for a. See [[Operating Systems]] and [[wiki/projects/a/concepts/Linux|Linux]].\n"),
		"wiki/projects/a/concepts/Gradient Descent.md": page("Gradient Descent", "concept", "id-a", "aliases:\n  - GD\n", "Walk downhill along the gradient.\n"),
		"wiki/projects/a/concepts/Linux.md":            page("Linux", "concept", "id-a", "tags:\n  - concept\n  - ops\n", os1+"\nSee [[Operating Systems]] and [[Shell]].\n"),
		"wiki/projects/a/concepts/Moved.md":            page("Moved", "concept", "id-a", "", "Left behind in a: a page that b moved.\n"),
		"wiki/projects/b/b.md":                         page("b", "meta", "id-b", "", "- [[wiki/projects/b/entities/Widget|Widget]]\n"),
		"wiki/projects/b/entities/Widget.md":           page("Widget", "entity", "id-b", "", "A gizmo that counts peas in project b.\n"),
		"wiki/projects/b/entities/Config.md":           page("Config", "entity", "id-b", "", "Settings for b. See [[Operating Systems]] and [[wiki/projects/b/concepts/macOS|macOS]].\n"),
		"wiki/projects/b/concepts/GD.md":               page("GD", "concept", "id-b", "", "Step against the slope, again and again.\n"),
		"wiki/projects/b/concepts/macOS.md":            page("macOS", "concept", "id-b", "tags:\n  - concept\n  - ops\n", os2+"\nSee [[Operating Systems]] and [[Shell]].\n"),
		"wiki/projects/b/concepts/Moved.md":            page("Moved", "concept", "id-b", "moved_to: \"wiki/concepts/Moved.md\"\nmoved_to_project: \"hub-id\"\n", "Left behind in a: a page that b moved.\n"),
		"threads/threads.md":                           "# Threads\n",
	})
}

func find(t *testing.T, r *Report, a, b string) Pair {
	t.Helper()
	for _, p := range r.Pairs {
		if strings.HasSuffix(p.A.Path, a) && strings.HasSuffix(p.B.Path, b) {
			return p
		}
	}
	t.Fatalf("no pair %s / %s in %s", a, b, r.Markdown())
	return Pair{}
}

func TestReportFindsDuplicatesRelatedPagesNamesAndTags(t *testing.T) {
	root := hub(t)
	r, err := Run(root, "hub", Options{})
	if err != nil {
		t.Fatal(err)
	}
	var origins []string
	for _, o := range r.Origins {
		origins = append(origins, o.Name+":"+o.Project+":"+itoa(o.Pages))
	}
	if strings.Join(origins, " ") != "hub::2 a:id-a:5 b:id-b:4" {
		t.Fatalf("origins %v", origins)
	}
	if r.Summary.PagesCompared != 11 || r.Note != "" {
		t.Fatalf("summary %+v note %q", r.Summary, r.Note)
	}
	// The same name in two members, and the hub has one by that name already.
	w := find(t, r, "a/entities/Widget.md", "b/entities/Widget.md")
	if w.Kind != Duplicate || w.Name != 1 || w.UpgradedTo != "wiki/entities/Widget.md" || !w.Settled {
		t.Fatalf("widget %+v", w)
	}
	// The hub's own Widget against a member's is a pair too, and nothing settles it.
	hw := find(t, r, "wiki/entities/Widget.md", "a/entities/Widget.md")
	if hw.A.Origin != "hub" || hw.Kind != Duplicate || hw.Settled {
		t.Fatalf("hub widget %+v", hw)
	}
	// An alias of one is the title of the other.
	gd := find(t, r, "a/concepts/Gradient Descent.md", "b/concepts/GD.md")
	if gd.Kind != Duplicate || gd.Name != 1 || gd.Settled {
		t.Fatalf("gd %+v", gd)
	}
	// Two names, one vocabulary, and a link name in common.
	os := find(t, r, "a/concepts/Linux.md", "b/concepts/macOS.md")
	if os.Kind != Related || os.Name != 0 || os.Content < ScoreFloor || os.Links == 0 || os.Settled {
		t.Fatalf("os %+v", os)
	}
	if len(os.SharedTerms) == 0 || strings.Join(os.SharedLinks, ",") != "Operating Systems,Shell" {
		t.Fatalf("os evidence %+v", os)
	}
	// A hub page links both Config pages: bridged.
	c := find(t, r, "a/entities/Config.md", "b/entities/Config.md")
	if !c.Settled || strings.Join(c.BridgedBy, ",") != "wiki/concepts/Configuration.md" || c.UpgradedTo != "" {
		t.Fatalf("config %+v", c)
	}
	// The moved page is out, so nothing pairs with a's Moved.
	for _, p := range r.Pairs {
		if strings.HasSuffix(p.A.Path, "Moved.md") || strings.HasSuffix(p.B.Path, "Moved.md") {
			t.Fatalf("moved page compared: %+v", p)
		}
	}
	// The shared names: one wanted everywhere, one that resolves nowhere, and none for a
	// hub page's own links into the mirrors.
	var names []string
	for _, n := range r.Names {
		names = append(names, n.Name+"="+strings.Join(n.Origins, "+")+":"+itoa(len(n.LinkedFrom)))
	}
	if strings.Join(names, " ") != "Operating Systems=a+b:4 Shell=a+b:2" {
		t.Fatalf("names %v", names)
	}
	if len(r.Tags) != 1 || r.Tags[0].Tag != "ops" || strings.Join(r.Tags[0].Origins, "+") != "a+b" || r.Tags[0].Pages != 2 {
		t.Fatalf("tags %+v", r.Tags)
	}
	// Ranked: score descending, ties by path.
	for i := 1; i < len(r.Pairs); i++ {
		if r.Pairs[i].Score > r.Pairs[i-1].Score {
			t.Fatalf("unsorted at %d: %+v", i, r.Pairs)
		}
	}
	md := r.Markdown()
	for _, want := range []string{"## Pairs", "## Shared names", "## Shared tags", "upgraded: wiki/entities/Widget.md", "bridged by wiki/concepts/Configuration.md"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
	// The same wiki gives the same report.
	again, err := Run(root, "hub", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.JSON(), again.JSON()) {
		t.Fatalf("not deterministic:\n%s\n%s", r.JSON(), again.JSON())
	}
}

func TestMemberNarrowsAndLimitBounds(t *testing.T) {
	root := hub(t)
	r, err := Run(root, "hub", Options{Member: "b", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Pairs) != 2 {
		t.Fatalf("pairs %d", len(r.Pairs))
	}
	all, _ := Run(root, "hub", Options{Member: "b"})
	for _, p := range all.Pairs {
		if p.A.Origin != "b" && p.B.Origin != "b" {
			t.Fatalf("pair outside b: %+v", p)
		}
	}
	if len(all.Pairs) >= len(must(Run(root, "hub", Options{})).Pairs) {
		t.Fatal("member did not narrow")
	}
	if _, err := Run(root, "hub", Options{Member: "nope"}); err == nil || !strings.Contains(err.Error(), "hub, a, b") {
		t.Fatalf("err %v", err)
	}
}

func TestAHubWithoutMirrorsSaysSo(t *testing.T) {
	root := fixture(t, map[string]string{
		"wiki/index.md":          page("Index", "meta", "", "", "- [[Alpha]]\n"),
		"wiki/concepts/Alpha.md": page("Alpha", "concept", "", "", "text\n"),
	})
	r, err := Run(root, "solo", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Pairs) != 0 || !strings.Contains(r.Note, "no mirrors") || len(r.Origins) != 1 || r.Origins[0].Name != "solo" {
		t.Fatalf("report %+v", r)
	}
	empty := fixture(t, map[string]string{"wiki/index.md": page("Index", "meta", "", "", "")})
	if r, _ := Run(empty, "solo", Options{}); r.Note == "" {
		t.Fatalf("empty %+v", r)
	}
}

func TestNameScoreCountsSharedWords(t *testing.T) {
	mk := func(names ...string) *doc {
		d := &doc{names: names}
		for _, n := range names {
			d.keys = append(d.keys, lint.NameKey(n))
			d.runes = append(d.runes, []rune(lint.NameKey(n)))
			d.words = append(d.words, distinct(tokens(n)))
		}
		return d
	}
	for _, c := range []struct {
		a, b []string
		want float64
	}{
		{[]string{"CS513 Course Project"}, []string{"cs513-project", "CS513 group project"}, 2.0 / 3},
		{[]string{"Widget"}, []string{"Widget (source)"}, 0.5},
		{[]string{"Gradient Descent"}, []string{"GD"}, 0},
		{[]string{"Linux"}, []string{"macOS"}, 0},
		{[]string{"Config"}, []string{"config"}, 1},
		{[]string{"Backpropagation"}, []string{"Backpropogation"}, NameDuplicate},
	} {
		if got := nameScore(mk(c.a...), mk(c.b...)); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("nameScore(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestTokensAndStem(t *testing.T) {
	got := strings.Join(tokens("The Kernels' schedulers are scheduling processes, and the filesystems were flushed; CS513 studies analysis."), " ")
	want := "kernel scheduler schedul process filesystem flush cs513 study analysis"
	if got := stem("running") + " " + stem("planned") + " " + stem("classes") + " " + stem("seeing"); got != "run plan class see" {
		t.Fatalf("stem %q", got)
	}
	if got != want {
		t.Fatalf("tokens %q", got)
	}
	if got := plain("See [[wiki/projects/a/concepts/Linux|Linux]] and [[Shell#Top]] and [text](wiki/x.md)."); got != "See  Linux  and  Shell  and  text ." {
		t.Fatalf("plain %q", got)
	}
}

func must(r *Report, err error) *Report {
	if err != nil {
		panic(err)
	}
	return r
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestWithinComparesOneWikisOwnPages(t *testing.T) {
	root := fixture(t, map[string]string{
		"wiki/index.md":             page("Index", "meta", "", "", "- [[Linux]]\n"),
		"wiki/concepts/Linux.md":    page("Linux", "concept", "", "", os1),
		"wiki/concepts/Linux OS.md": page("Linux OS", "concept", "", "", os1),
		"wiki/concepts/Gradient.md": page("Gradient", "concept", "", "", "Slopes and descent.\n"),
	})
	r, err := Run(root, "solo", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Pairs) != 0 || !strings.Contains(r.Note, "no mirrors") {
		t.Fatalf("without within: %+v", r)
	}
	r, err = Run(root, "solo", Options{Within: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Pairs) == 0 || r.Note != "" {
		t.Fatalf("within: %+v", r)
	}
	p := r.Pairs[0]
	if p.A.Path != "wiki/concepts/Linux OS.md" || p.B.Path != "wiki/concepts/Linux.md" || p.Kind != Duplicate || p.A.Origin != "solo" || p.B.Origin != "solo" {
		t.Fatalf("pair %+v", p)
	}
}
