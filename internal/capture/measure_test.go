package capture

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
	"testing"
)

// pdf builds a PDF whose page tree holds n pages, with the page objects in the body or,
// when packed, inside one flate-compressed object stream.
func pdf(n int, packed bool) []byte {
	var kids []string
	var objs strings.Builder
	for i := 0; i < n; i++ {
		kids = append(kids, fmt.Sprintf("%d 0 R", 3+i))
		fmt.Fprintf(&objs, "%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << >> >> >>\nendobj\n", 3+i)
	}
	tree := fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", strings.Join(kids, " "), n)
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	if !packed {
		b.WriteString(tree + objs.String())
	} else {
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		w.Write([]byte(tree + objs.String()))
		w.Close()
		fmt.Fprintf(&b, "9 0 obj\n<< /Type /ObjStm /Filter /FlateDecode /Length %d >>\nstream\n", z.Len())
		b.Write(z.Bytes())
		b.WriteString("\nendstream\nendobj\n")
	}
	b.WriteString("trailer\n<< /Root 1 0 R >>\n%%EOF\n")
	return b.Bytes()
}

func TestPDFPages(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want int
	}{
		{"plain", pdf(3, false), 3},
		{"packed", pdf(120, true), 120},
		{"no tree", []byte("%PDF-1.4\n1 0 obj << /Type /Page >> endobj\n2 0 obj << /Type /Page >> endobj\n"), 2},
		{"not a pdf", []byte("hello"), 0},
		{"empty pdf", []byte("%PDF-1.4\n%%EOF\n"), 0},
	} {
		if got := PDFPages(c.data); got != c.want {
			t.Errorf("%s: %d pages, want %d", c.name, got, c.want)
		}
	}
}

func TestMeasureMarkdownAndText(t *testing.T) {
	md := "---\ntitle: X\n# not a heading\n---\n# One\n\ntext\n```\n# not either\n```\n## Two ##\n  ### Three\n####### seven\n"
	m := measure("markdown", []byte(md))
	if m.Lines != 13 {
		t.Fatalf("lines %d", m.Lines)
	}
	var got []string
	for _, h := range m.Outline {
		got = append(got, fmt.Sprintf("%d:%d:%s", h.Line, h.Level, h.Title))
	}
	if strings.Join(got, "|") != "5:1:One|11:2:Two|12:3:Three" || m.OutlineTruncated {
		t.Fatalf("outline %v", got)
	}
	if m := measure("text", []byte("a\nb")); m.Lines != 2 || m.Outline != nil {
		t.Fatalf("text %+v", m)
	}
	many := strings.Repeat("# h\n", MaxOutline+5)
	if m := measure("markdown", []byte(many)); len(m.Outline) != MaxOutline || !m.OutlineTruncated {
		t.Fatalf("bound %d %v", len(m.Outline), m.OutlineTruncated)
	}
	if m := measure("image", []byte("x")); m.Pages+m.Lines+len(m.Outline) != 0 {
		t.Fatalf("image %+v", m)
	}
}
