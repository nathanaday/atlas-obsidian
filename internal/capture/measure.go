package capture

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// MaxOutline bounds the headings an outline lists.
const MaxOutline = 200

// Heading is one line of a source's outline: a markdown heading and where it starts.
type Heading struct {
	Line  int    `json:"line"`
	Level int    `json:"level"`
	Title string `json:"title"`
}

// Measure is how big a captured source is, in the units a reader splits it by: pages for
// a PDF, lines for text, and the headings of a markdown file. The wiki-ingest skill reads
// it to decide whether a source is read in full or split across workers.
type Measure struct {
	// Pages is a PDF's page count, or 0 when the file does not say.
	Pages int `json:"pages,omitempty"`
	// Lines is a text or markdown file's line count.
	Lines   int       `json:"lines,omitempty"`
	Outline []Heading `json:"outline,omitempty"`
	// OutlineTruncated says the outline stopped at MaxOutline headings.
	OutlineTruncated bool `json:"outline_truncated,omitempty"`
}

// Measure reads data as a file of kind.
func measure(kind string, data []byte) Measure {
	switch kind {
	case "pdf":
		return Measure{Pages: PDFPages(data)}
	case "markdown":
		m := Measure{Lines: lineCount(data)}
		m.Outline, m.OutlineTruncated = outline(data)
		return m
	case "text", "html", "data":
		return Measure{Lines: lineCount(data)}
	}
	return Measure{}
}

func lineCount(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	n := bytes.Count(data, []byte("\n"))
	if data[len(data)-1] != '\n' {
		n++
	}
	return n
}

var atxLine = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.+?)[ \t#]*$`)

// outline lists a markdown file's headings outside code fences and frontmatter.
func outline(data []byte) ([]Heading, bool) {
	var out []Heading
	lines := strings.Split(string(data), "\n")
	inFence, fence := false, ""
	inFront := len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if inFront {
			if i > 0 && strings.TrimSpace(line) == "---" {
				inFront = false
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			switch {
			case !inFence:
				inFence, fence = true, marker
			case marker == fence:
				inFence = false
			}
			continue
		}
		if inFence {
			continue
		}
		if m := atxLine.FindStringSubmatch(line); m != nil {
			if len(out) == MaxOutline {
				return out, true
			}
			out = append(out, Heading{Line: i + 1, Level: len(m[1]), Title: m[2]})
		}
	}
	return out, false
}

var (
	pagesNode  = regexp.MustCompile(`/Type\s*/Pages\b`)
	pageLeaf   = regexp.MustCompile(`/Type\s*/Page(?:[^s]|$)`)
	countEntry = regexp.MustCompile(`/Count\s+(\d+)`)
	streamBody = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)
)

// PDFPages counts a PDF's pages from its page tree: the largest /Count of a /Pages node,
// or, when no node says, the number of page objects. Page objects inside compressed
// object streams are read by inflating each stream. It returns 0 when neither is found.
func PDFPages(data []byte) int {
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return 0
	}
	parts := [][]byte{data}
	for _, m := range streamBody.FindAllSubmatch(data, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue
		}
		inflated, err := io.ReadAll(io.LimitReader(r, 64<<20))
		r.Close()
		if err == nil && (pagesNode.Match(inflated) || pageLeaf.Match(inflated)) {
			parts = append(parts, inflated)
		}
	}
	best, leaves := 0, 0
	for _, part := range parts {
		for _, loc := range pagesNode.FindAllIndex(part, -1) {
			start := bytes.LastIndex(part[:loc[0]], []byte("<<"))
			if start < 0 {
				start = loc[0]
			}
			end := min(len(part), loc[1]+4096)
			window := part[start:end]
			if close := dictEnd(window); close > 0 {
				window = window[:close]
			}
			if m := countEntry.FindSubmatch(window); m != nil {
				if n, err := strconv.Atoi(string(m[1])); err == nil && n > best {
					best = n
				}
			}
		}
		leaves += len(pageLeaf.FindAllIndex(part, -1))
	}
	if best > 0 {
		return best
	}
	return leaves
}

// dictEnd is the offset just past the >> that closes the dictionary window opens with,
// or 0 when the window does not hold it.
func dictEnd(window []byte) int {
	depth := 0
	for i := 0; i+1 < len(window); i++ {
		switch {
		case window[i] == '<' && window[i+1] == '<':
			depth++
			i++
		case window[i] == '>' && window[i+1] == '>':
			depth--
			i++
			if depth == 0 {
				return i + 1
			}
		}
	}
	return 0
}
