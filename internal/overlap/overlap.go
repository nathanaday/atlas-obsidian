// Package overlap finds what the origins of a hub's wiki hold in common: the hub's own
// pages and the mirror of each member under wiki/projects/. It reads the pages as lint
// reads them and scores every pair of pages from two different origins by name, by
// content, and by the names they link, so a merge can read the few pages that look like
// the same thing, or like neighbours, instead of the whole wiki. The report is
// deterministic for one wiki and needs no model to produce.
package overlap

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/nathanaday/atlas-obsidian/internal/lint"
	"github.com/nathanaday/atlas-obsidian/internal/project"
)

// The thresholds. A pair appears when its combined score reaches ScoreFloor; it is a
// duplicate at NameDuplicate or ContentDuplicate. A name scores 1 when it equals the
// other, NameDuplicate when it is a typing distance away, and its word overlap when at
// least NameOverlap of the words of the two names coincide.
const (
	ScoreFloor       = 0.1
	NameDuplicate    = 0.8
	ContentDuplicate = 0.6
	NameOverlap      = 0.5
	// The weights that combine the three scores into one.
	NameWeight    = 0.5
	ContentWeight = 0.35
	LinksWeight   = 0.15
	// DefaultLimit bounds each list of the report.
	DefaultLimit = 30
	// Emphasis is how many times a title, alias, or heading token counts.
	Emphasis = 3
	// MaxEvidence bounds the shared terms and links shown for one pair.
	MaxEvidence = 8
	// MaxLinkedFrom bounds the pages listed for one shared name.
	MaxLinkedFrom = 6
)

// The two kinds of pair.
const (
	Duplicate = "duplicate"
	Related   = "related"
)

// MovedToProjectKey marks a page whose content moved into a hub; overlap leaves it out.
const MovedToProjectKey = "moved_to_project"

// Options narrow a report.
type Options struct {
	// Member keeps only the pairs, names, and tags that involve this origin: a mirror's
	// folder name, or the hub's own name.
	Member string
	// Limit bounds each list; zero means DefaultLimit.
	Limit int
	// Within compares pages of one origin with each other as well, so one wiki's own
	// near-duplicates show; without it only pages of two origins pair.
	Within bool
}

// Origin is one wiki the hub reads: its own, or a member's mirror.
type Origin struct {
	Name string `json:"name"`
	Own  bool   `json:"own,omitempty"`
	// Project is the member's id, from the mirror's frontmatter.
	Project string `json:"project,omitempty"`
	Pages   int    `json:"pages"`
}

// Page names one page of the report.
type Page struct {
	Origin string `json:"origin"`
	Path   string `json:"path"`
	Title  string `json:"title"`
	Type   string `json:"type,omitempty"`
}

// Pair is two pages from two origins that look like the same thing, or like neighbours,
// with the scores that put them here and the hub pages that already cover them.
type Pair struct {
	Kind    string  `json:"kind"`
	Score   float64 `json:"score"`
	A       Page    `json:"a"`
	B       Page    `json:"b"`
	Name    float64 `json:"name"`
	Content float64 `json:"content"`
	Links   float64 `json:"links"`
	// SharedTerms are the terms that weigh most in both pages; SharedLinks the names
	// both pages link.
	SharedTerms []string `json:"shared_terms,omitempty"`
	SharedLinks []string `json:"shared_links,omitempty"`
	// UpgradedTo is a hub page named like one of the two; BridgedBy are the hub pages
	// that link to both. Either settles the pair.
	UpgradedTo string   `json:"upgraded_to,omitempty"`
	BridgedBy  []string `json:"bridged_by,omitempty"`
	Settled    bool     `json:"settled"`
}

// Name is a link target that pages of two or more origins name, and that resolves to no
// page in the hub or to a different page in each origin: a bridge that has a name.
type Name struct {
	Name       string   `json:"name"`
	Origins    []string `json:"origins"`
	LinkedFrom []string `json:"linked_from"`
	// Pages are the pages the name resolves to, one per origin at most, when it resolves
	// at all.
	Pages []string `json:"pages,omitempty"`
	// Page is the hub's own page the name resolves to, which settles it.
	Page    string `json:"page,omitempty"`
	Settled bool   `json:"settled"`
}

// Tag is a tag pages of two or more origins carry.
type Tag struct {
	Tag     string   `json:"tag"`
	Origins []string `json:"origins"`
	Pages   int      `json:"pages"`
}

// Summary counts the work.
type Summary struct {
	PagesCompared int `json:"pages_compared"`
	PairsScored   int `json:"pairs_scored"`
	Pairs         int `json:"pairs"`
	Names         int `json:"names"`
	Tags          int `json:"tags"`
}

// Report is the overlap between the origins.
type Report struct {
	Project string   `json:"project"`
	Origins []Origin `json:"origins"`
	Pairs   []Pair   `json:"pairs"`
	Names   []Name   `json:"names"`
	Tags    []Tag    `json:"tags"`
	Summary Summary  `json:"summary"`
	// Note says why the lists are empty when they are.
	Note string `json:"note,omitempty"`
}

// doc is one page as the index holds it.
type doc struct {
	info     lint.PageInfo
	origin   int
	names    []string          // the title and the aliases
	keys     []string          // their name keys
	runes    [][]rune          // the same keys as runes, for Near
	words    [][]string        // the distinct words of each name, stemmed and sorted
	terms    []term            // sorted by id, unit length
	links    map[string]string // link name key -> display name
	linkKeys []string          // the link name keys, sorted
	linked   map[string]bool   // resolved paths
}

type term struct {
	id     int
	weight float64
}

// Run reads the hub at root, whose own name is hub, and reports its overlap.
func Run(root, hub string, opts Options) (*Report, error) {
	vault, err := lint.LoadVault(root)
	if err != nil {
		return nil, err
	}
	if opts.Limit <= 0 {
		opts.Limit = DefaultLimit
	}
	report := &Report{Project: root, Pairs: []Pair{}, Names: []Name{}, Tags: []Tag{}}
	ix := newIndex(hub, vault.Pages())
	report.Origins = ix.origins()
	report.Summary.PagesCompared = len(ix.docs)
	if opts.Member != "" {
		if _, ok := ix.originID[opts.Member]; !ok {
			var names []string
			for _, o := range report.Origins {
				names = append(names, o.Name)
			}
			return nil, fmt.Errorf("no origin named %q; the origins are %s", opts.Member, strings.Join(names, ", "))
		}
	}
	withPages := 0
	for _, o := range report.Origins {
		if o.Pages > 0 {
			withPages++
		}
	}
	switch {
	case opts.Within && len(ix.docs) < 2:
		report.Note = "fewer than two pages to compare"
		return report, nil
	case opts.Within:
	case len(report.Origins) == 1 && report.Origins[0].Own:
		report.Note = "no mirrors under " + project.MirrorDir + "/; the project tool's sync action mirrors the members, and overlap compares the pages across them"
		return report, nil
	case len(report.Origins) == 1:
		report.Note = "one origin only: the hub has no pages of its own and mirrors one member, so there is nothing to compare across"
		return report, nil
	case withPages < 2:
		report.Note = "one origin holds pages, so there is nothing to compare across; within compares the pages of one wiki with each other"
		return report, nil
	}
	pairs, scored := ix.pairs(opts)
	report.Pairs, report.Summary.PairsScored = append(report.Pairs, pairs...), scored
	report.Names = append(report.Names, ix.names(opts)...)
	report.Tags = append(report.Tags, ix.tags(opts)...)
	report.Summary.Pairs, report.Summary.Names, report.Summary.Tags = len(report.Pairs), len(report.Names), len(report.Tags)
	if len(report.Pairs)+len(report.Names)+len(report.Tags) == 0 {
		report.Note = "no page looks like another"
	}
	return report, nil
}

// index is every comparable page, tokenized and weighted.
type index struct {
	hub      string
	docs     []*doc
	origins_ []Origin
	originID map[string]int
	termID   map[string]int
	// vocabulary names each term by id.
	vocabulary []string
	// The hub's own pages, and the same by name key.
	hub_     []int
	hubByKey map[string][]int
}

func newIndex(hub string, pages []lint.PageInfo) *index {
	ix := &index{hub: hub, originID: map[string]int{}, termID: map[string]int{}, hubByKey: map[string][]int{}}
	ix.origin(hub, true, "")
	counts := map[int]int{}
	type raw struct {
		d      *doc
		counts map[string]int
	}
	var raws []raw
	for _, info := range pages {
		origin, ok := originOf(info.Path)
		if !ok {
			continue
		}
		oi := ix.origin(origin, origin == "", project.StringField(info.Fields, "project"))
		if !comparable(info) {
			continue
		}
		counts[oi]++
		d := &doc{info: info, origin: oi, links: map[string]string{}, linked: map[string]bool{}}
		d.names = append([]string{info.Title}, info.Aliases...)
		if stem := strings.TrimSuffix(path.Base(info.Path), path.Ext(info.Path)); lint.NameKey(stem) != lint.NameKey(info.Title) {
			d.names = append(d.names, stem)
		}
		for _, n := range d.names {
			if k := lint.NameKey(n); len(k) >= 2 {
				d.keys = append(d.keys, k)
				d.runes = append(d.runes, []rune(k))
			}
			if w := distinct(tokens(n)); len(w) > 0 {
				d.words = append(d.words, w)
			}
		}
		for _, l := range info.Links {
			if l.Resolved != "" {
				d.linked[l.Resolved] = true
			}
			name := linkName(l)
			if k := lint.NameKey(name); len(k) >= 3 {
				if _, seen := d.links[k]; !seen {
					d.links[k] = name
				}
			}
		}
		for k := range d.links {
			d.linkKeys = append(d.linkKeys, k)
		}
		sort.Strings(d.linkKeys)
		tf := map[string]int{}
		for _, n := range d.names {
			for _, t := range tokens(n) {
				tf[t] += Emphasis
			}
		}
		for _, h := range info.Headings {
			if templateHeadings[h] {
				continue
			}
			for _, t := range tokens(h) {
				tf[t] += Emphasis
			}
		}
		for _, t := range tokens(plain(info.Text)) {
			tf[t]++
		}
		raws = append(raws, raw{d: d, counts: tf})
		ix.docs = append(ix.docs, d)
	}
	for oi := range ix.origins_ {
		ix.origins_[oi].Pages = counts[oi]
	}
	// Drop the origins that hold no comparable page, so a mirror of nothing is not an
	// origin; the hub itself always stays.
	df := map[string]int{}
	for _, r := range raws {
		for t := range r.counts {
			df[t]++
		}
	}
	// Term ids follow the alphabet, so a tie between two terms breaks the same way on
	// every run.
	for t := range df {
		ix.vocabulary = append(ix.vocabulary, t)
	}
	sort.Strings(ix.vocabulary)
	for id, t := range ix.vocabulary {
		ix.termID[t] = id
	}
	n := float64(len(raws))
	for _, r := range raws {
		var norm float64
		for t, c := range r.counts {
			id := ix.termID[t]
			w := (1 + math.Log(float64(c))) * (math.Log((n+1)/(float64(df[t])+1)) + 1)
			r.d.terms = append(r.d.terms, term{id: id, weight: w})
			norm += w * w
		}
		norm = math.Sqrt(norm)
		for i := range r.d.terms {
			r.d.terms[i].weight /= norm
		}
		sort.Slice(r.d.terms, func(i, j int) bool { return r.d.terms[i].id < r.d.terms[j].id })
	}
	for i, d := range ix.docs {
		if d.origin != 0 {
			continue
		}
		for _, k := range d.keys {
			ix.hubByKey[k] = append(ix.hubByKey[k], i)
		}
		ix.hub_ = append(ix.hub_, i)
	}
	return ix
}

func (ix *index) origin(name string, own bool, id string) int {
	if name == "" {
		name = ix.hub
	}
	if oi, ok := ix.originID[name]; ok {
		if id != "" && ix.origins_[oi].Project == "" {
			ix.origins_[oi].Project = id
		}
		return oi
	}
	ix.originID[name] = len(ix.origins_)
	ix.origins_ = append(ix.origins_, Origin{Name: name, Own: own, Project: id})
	return ix.originID[name]
}

func (ix *index) origins() []Origin {
	var out []Origin
	for _, o := range ix.origins_ {
		if o.Pages > 0 || o.Own {
			out = append(out, o)
		}
	}
	return out
}

// originOf says which origin a wiki path belongs to: "" for the hub's own wiki, the
// folder name for a mirror. A file outside wiki/ belongs to none.
func originOf(rel string) (string, bool) {
	if !strings.HasPrefix(rel, project.WikiDir+"/") {
		return "", false
	}
	if !lint.Mirrored(rel) {
		return "", true
	}
	folder, _, _ := strings.Cut(strings.TrimPrefix(rel, project.MirrorDir+"/"), "/")
	return folder, folder != ""
}

// comparable says whether a page is content: not an index, an overview, the log or the
// hot cache, a meta or fold page, a mirror's root, or a page that moved into a hub.
func comparable(info lint.PageInfo) bool {
	switch info.Path {
	case project.IndexPage, project.OverviewPage, project.HotPage, project.LogPage, project.MirrorIndex, project.CanvasIndex:
		return false
	}
	base := strings.ToLower(path.Base(info.Path))
	if base == "index.md" || base == "_index.md" || base == "overview.md" {
		return false
	}
	if lint.Mirrored(info.Path) {
		folder, rest, _ := strings.Cut(strings.TrimPrefix(info.Path, project.MirrorDir+"/"), "/")
		if rest == folder+".md" {
			return false
		}
	}
	switch info.Type {
	case "meta", "fold":
		return false
	}
	if project.StringField(info.Fields, MovedToProjectKey) != "" {
		return false
	}
	return true
}

// linkName is the name a link shows: the last segment of the target as written, without
// its extension.
func linkName(l lint.Link) string {
	t := strings.TrimSpace(l.Target)
	t = strings.TrimSuffix(t, "/")
	name := path.Base(t)
	if ext := path.Ext(name); strings.EqualFold(ext, ".md") {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}

var (
	wikiLinkRE = regexp.MustCompile(`!?\[\[([^\]\r\n]+?)\]\]`)
	mdLinkRE   = regexp.MustCompile(`!?\[([^\]\r\n]*)\]\([^\r\n)]*\)`)
)

// plain replaces every link with its visible text, so a rewritten path under
// wiki/projects/ adds no terms of its own.
func plain(text string) string {
	text = wikiLinkRE.ReplaceAllStringFunc(text, func(m string) string {
		body := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(m, "!"), "[["), "]]")
		if _, alias, ok := strings.Cut(body, "|"); ok {
			return " " + alias + " "
		}
		target, _, _ := strings.Cut(body, "#")
		return " " + path.Base(target) + " "
	})
	return mdLinkRE.ReplaceAllString(text, " $1 ")
}

// tokens splits text into lowercase words of three letters or more, minus stop words,
// each lightly stemmed.
func tokens(text string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		w := b.String()
		b.Reset()
		if len(w) < 3 || stopwords[w] {
			return
		}
		if w = stem(w); len(w) >= 3 {
			out = append(out, w)
		}
	}
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// stem strips the common English suffixes: plural s, ies, ing, ed, and the doubled
// letter they leave (running, run).
func stem(w string) string {
	switch {
	case strings.HasSuffix(w, "ies") && len(w) > 4:
		return w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "sses"):
		return w[:len(w)-2]
	case strings.HasSuffix(w, "ss"), strings.HasSuffix(w, "us"), strings.HasSuffix(w, "is"):
		return w
	case strings.HasSuffix(w, "ing") && len(w) > 5:
		return undouble(w[:len(w)-3])
	case strings.HasSuffix(w, "ed") && len(w) > 4:
		return undouble(w[:len(w)-2])
	case strings.HasSuffix(w, "s") && len(w) > 3:
		return w[:len(w)-1]
	}
	return w
}

func undouble(w string) string {
	if n := len(w); n > 3 && w[n-1] == w[n-2] && !strings.ContainsRune("aeiousz", rune(w[n-1])) {
		return w[:n-1]
	}
	return w
}

// pairs scores every cross-origin pair and keeps the ones that clear a floor.
func (ix *index) pairs(opts Options) ([]Pair, int) {
	member, filter := ix.originID[opts.Member]
	var out []Pair
	scored := 0
	for i, a := range ix.docs {
		for j := i + 1; j < len(ix.docs); j++ {
			b := ix.docs[j]
			if a.origin == b.origin && !opts.Within {
				continue
			}
			if filter && a.origin != member && b.origin != member {
				continue
			}
			scored++
			p, ok := ix.score(i, j)
			if ok {
				out = append(out, p)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].A.Path != out[j].A.Path {
			return out[i].A.Path < out[j].A.Path
		}
		return out[i].B.Path < out[j].B.Path
	})
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out, scored
}

func (ix *index) score(i, j int) (Pair, bool) {
	a, b := ix.docs[i], ix.docs[j]
	if a.origin > b.origin {
		a, b, i, j = b, a, j, i
	}
	name := nameScore(a, b)
	content := contentScore(a, b)
	links, sharedLinks := linkScore(a, b)
	score := NameWeight*name + ContentWeight*content + LinksWeight*links
	if score < ScoreFloor {
		return Pair{}, false
	}
	p := Pair{Kind: Related, A: ix.page(a), B: ix.page(b), Name: round(name), Content: round(content), Links: round(links), SharedTerms: ix.sharedTerms(a, b), SharedLinks: sharedLinks, Score: round(score)}
	if name >= NameDuplicate || content >= ContentDuplicate {
		p.Kind = Duplicate
	}
	for _, d := range []*doc{a, b} {
		if d.origin == 0 {
			continue
		}
		for _, k := range d.keys {
			for _, h := range ix.hubByKey[k] {
				if h == i || h == j {
					continue
				}
				if p.UpgradedTo == "" || ix.docs[h].info.Path < p.UpgradedTo {
					p.UpgradedTo = ix.docs[h].info.Path
				}
			}
		}
	}
	for _, h := range ix.hub_ {
		if h == i || h == j {
			continue
		}
		if linked := ix.docs[h].linked; linked[a.info.Path] && linked[b.info.Path] {
			p.BridgedBy = append(p.BridgedBy, ix.docs[h].info.Path)
		}
	}
	p.Settled = p.UpgradedTo != "" || len(p.BridgedBy) > 0
	return p, true
}

func (ix *index) page(d *doc) Page {
	return Page{Origin: ix.origins_[d.origin].Name, Path: d.info.Path, Title: d.info.Title, Type: d.info.Type}
}

// nameScore is 1 when a title or alias of one page equals one of the other, once reduced
// to letters and digits; NameDuplicate when one is a typing distance away; else the
// largest share of words two of the names have in common, when it reaches NameOverlap,
// so "CS513 Course Project" and "cs513-project" count; else 0.
func nameScore(a, b *doc) float64 {
	for _, ka := range a.keys {
		for _, kb := range b.keys {
			if ka == kb {
				return 1
			}
		}
	}
	for _, ra := range a.runes {
		for _, rb := range b.runes {
			if lint.NearKeys(ra, rb) {
				return NameDuplicate
			}
		}
	}
	best := 0.0
	for _, wa := range a.words {
		for _, wb := range b.words {
			if j, _ := jaccard(wa, wb); j > best {
				best = j
			}
		}
	}
	if best < NameOverlap {
		return 0
	}
	return best
}

// jaccard is the share of the strings two sorted, distinct lists have in common, with
// the ones in common.
func jaccard(a, b []string) (float64, []string) {
	var shared []string
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			i++
		case a[i] > b[j]:
			j++
		default:
			shared = append(shared, a[i])
			i++
			j++
		}
	}
	union := len(a) + len(b) - len(shared)
	if union == 0 {
		return 0, nil
	}
	return float64(len(shared)) / float64(union), shared
}

// distinct sorts words and drops repeats.
func distinct(words []string) []string {
	sort.Strings(words)
	return dedupe(words)
}

// contentScore is the cosine of the two pages' term vectors.
func contentScore(a, b *doc) float64 {
	var dot float64
	i, j := 0, 0
	for i < len(a.terms) && j < len(b.terms) {
		switch {
		case a.terms[i].id < b.terms[j].id:
			i++
		case a.terms[i].id > b.terms[j].id:
			j++
		default:
			dot += a.terms[i].weight * b.terms[j].weight
			i++
			j++
		}
	}
	return dot
}

// sharedTerms lists the terms that weigh most in both pages, for a pair the report
// keeps.
func (ix *index) sharedTerms(a, b *doc) []string {
	type hit struct {
		id int
		w  float64
	}
	var hits []hit
	i, j := 0, 0
	for i < len(a.terms) && j < len(b.terms) {
		switch {
		case a.terms[i].id < b.terms[j].id:
			i++
		case a.terms[i].id > b.terms[j].id:
			j++
		default:
			hits = append(hits, hit{a.terms[i].id, a.terms[i].weight * b.terms[j].weight})
			i++
			j++
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.Slice(hits, func(x, y int) bool {
		if hits[x].w != hits[y].w {
			return hits[x].w > hits[y].w
		}
		return hits[x].id < hits[y].id
	})
	if len(hits) > MaxEvidence {
		hits = hits[:MaxEvidence]
	}
	var shared []string
	for _, h := range hits {
		shared = append(shared, ix.vocabulary[h.id])
	}
	return shared
}

// linkScore is the Jaccard index of the names the two pages link, with the names both
// link.
func linkScore(a, b *doc) (float64, []string) {
	if len(a.linkKeys) == 0 || len(b.linkKeys) == 0 {
		return 0, nil
	}
	score, keys := jaccard(a.linkKeys, b.linkKeys)
	if len(keys) == 0 {
		return 0, nil
	}
	var shared []string
	for _, k := range keys {
		shared = append(shared, a.links[k])
	}
	sort.Strings(shared)
	if len(shared) > MaxEvidence {
		shared = shared[:MaxEvidence]
	}
	return score, shared
}

// names lists the link targets pages of two or more origins name.
func (ix *index) names(opts Options) []Name {
	member, filter := ix.originID[opts.Member]
	type group struct {
		name     string
		origins  map[int]bool
		from     []string
		resolved map[string]bool
	}
	groups := map[string]*group{}
	for _, d := range ix.docs {
		for _, l := range d.info.Links {
			name := linkName(l)
			k := lint.NameKey(name)
			if len(k) < 3 {
				continue
			}
			g := groups[k]
			if g == nil {
				g = &group{name: name, origins: map[int]bool{}, resolved: map[string]bool{}}
				groups[k] = g
			}
			g.origins[d.origin] = true
			g.from = append(g.from, d.info.Path)
			if l.Resolved != "" {
				g.resolved[l.Resolved] = true
			}
		}
	}
	var out []Name
	for _, g := range groups {
		if len(g.origins) < 2 {
			continue
		}
		if filter && !g.origins[member] {
			continue
		}
		var pages []string
		hubPage := ""
		for p := range g.resolved {
			if o, _ := originOf(p); o == "" {
				if hubPage == "" || p < hubPage {
					hubPage = p
				}
			}
			pages = append(pages, p)
		}
		sort.Strings(pages)
		// A hub page that links a member's page names it on purpose: fabric, not overlap.
		if len(pages) == 1 && hubPage == "" && g.origins[0] {
			continue
		}
		n := Name{Name: g.name, Pages: pages, Page: hubPage, Settled: hubPage != ""}
		for oi := range g.origins {
			n.Origins = append(n.Origins, ix.origins_[oi].Name)
		}
		sort.Strings(n.Origins)
		sort.Strings(g.from)
		n.LinkedFrom = dedupe(g.from)
		if len(n.LinkedFrom) > MaxLinkedFrom {
			n.LinkedFrom = n.LinkedFrom[:MaxLinkedFrom]
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Settled != out[j].Settled {
			return !out[i].Settled
		}
		if len(out[i].Origins) != len(out[j].Origins) {
			return len(out[i].Origins) > len(out[j].Origins)
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out
}

// templateHeadings are the headings the page skeletons give every page of a type,
// normalized as lint normalizes them; they say nothing about the page.
var templateHeadings = func() map[string]bool {
	m := map[string]bool{}
	for _, h := range project.TemplateHeadings() {
		m[strings.ToLower(strings.Join(strings.Fields(h), " "))] = true
	}
	return m
}()

// typeTags are the tags the page skeletons give every page of a type; they say nothing
// about what two origins share.
var typeTags = map[string]bool{"source": true, "entity": true, "concept": true, "comparison": true, "overview": true, "meta": true, "fold": true, "note": true, "moc": true, "project": true, "projects": true}

// tags lists the tags pages of two or more origins carry.
func (ix *index) tags(opts Options) []Tag {
	member, filter := ix.originID[opts.Member]
	type group struct {
		tag     string
		origins map[int]bool
		pages   int
	}
	groups := map[string]*group{}
	for _, d := range ix.docs {
		seen := map[string]bool{}
		for _, t := range d.info.Tags {
			k := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(t), "#"))
			if k == "" || typeTags[k] || seen[k] {
				continue
			}
			seen[k] = true
			g := groups[k]
			if g == nil {
				g = &group{tag: k, origins: map[int]bool{}}
				groups[k] = g
			}
			g.origins[d.origin] = true
			g.pages++
		}
	}
	var out []Tag
	for _, g := range groups {
		if len(g.origins) < 2 || (filter && !g.origins[member]) {
			continue
		}
		t := Tag{Tag: g.tag, Pages: g.pages}
		for oi := range g.origins {
			t.Origins = append(t.Origins, ix.origins_[oi].Name)
		}
		sort.Strings(t.Origins)
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Origins) != len(out[j].Origins) {
			return len(out[i].Origins) > len(out[j].Origins)
		}
		if out[i].Pages != out[j].Pages {
			return out[i].Pages > out[j].Pages
		}
		return out[i].Tag < out[j].Tag
	})
	if len(out) > opts.Limit {
		out = out[:opts.Limit]
	}
	return out
}

func dedupe(sorted []string) []string {
	var out []string
	for _, s := range sorted {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}

func round(f float64) float64 { return math.Round(f*100) / 100 }

// JSON renders the report.
func (r *Report) JSON() []byte {
	data, _ := json.MarshalIndent(r, "", "  ")
	return append(data, '\n')
}

// Markdown renders the report for a terminal.
func (r *Report) Markdown() string {
	var b strings.Builder
	b.WriteString("# Overlap\n\n")
	fmt.Fprintf(&b, "%d pages compared across %d origins; %d pairs scored.\n\n", r.Summary.PagesCompared, len(r.Origins), r.Summary.PairsScored)
	b.WriteString("| Origin | Pages | |\n|---|---|---|\n")
	for _, o := range r.Origins {
		note := ""
		if o.Own {
			note = "the hub's own pages"
		}
		fmt.Fprintf(&b, "| %s | %d | %s |\n", o.Name, o.Pages, note)
	}
	if r.Note != "" {
		b.WriteString("\n" + r.Note + "\n")
	}
	if len(r.Pairs) > 0 {
		b.WriteString("\n## Pairs\n\n| Kind | Score | A | B | Name | Content | Links | Evidence | Settled |\n|---|---|---|---|---|---|---|---|---|\n")
		for _, p := range r.Pairs {
			ev := strings.Join(p.SharedTerms, ", ")
			if len(p.SharedLinks) > 0 {
				ev += "; links " + strings.Join(p.SharedLinks, ", ")
			}
			settled := ""
			switch {
			case p.UpgradedTo != "":
				settled = "upgraded: " + p.UpgradedTo
			case len(p.BridgedBy) > 0:
				settled = "bridged by " + strings.Join(p.BridgedBy, ", ")
			}
			fmt.Fprintf(&b, "| %s | %.2f | %s: %s | %s: %s | %.2f | %.2f | %.2f | %s | %s |\n", p.Kind, p.Score, p.A.Origin, p.A.Path, p.B.Origin, p.B.Path, p.Name, p.Content, p.Links, strings.TrimPrefix(ev, "; "), settled)
		}
	}
	if len(r.Names) > 0 {
		b.WriteString("\n## Shared names\n\n| Name | Origins | Linked from | Resolves to | Settled |\n|---|---|---|---|---|\n")
		for _, n := range r.Names {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", n.Name, strings.Join(n.Origins, ", "), strings.Join(n.LinkedFrom, ", "), strings.Join(n.Pages, ", "), n.Page)
		}
	}
	if len(r.Tags) > 0 {
		b.WriteString("\n## Shared tags\n\n| Tag | Origins | Pages |\n|---|---|---|\n")
		for _, t := range r.Tags {
			fmt.Fprintf(&b, "| %s | %s | %d |\n", t.Tag, strings.Join(t.Origins, ", "), t.Pages)
		}
	}
	return b.String()
}

// stopwords are the English words that carry no topic.
var stopwords = func() map[string]bool {
	words := strings.Fields(`the and for are but not you all any can had her was one our out day get has him his how man new now old see two way who boy did its let put say she too use with this that from they what were when your their there have been will would could should about into than then them these those which while where after before also because does doing each few more most other over same some such only own very just here both between through during under until again further once why how being above below off down out very via per etc may might must shall can cannot need needs like make made makes take takes took give given gives used using uses use one two three first second third next last new old same different well much many still even much every another within without along across upon among`)
	m := map[string]bool{}
	for _, w := range words {
		m[w] = true
	}
	return m
}()
