package project

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// RequiredFrontmatter are the keys every wiki page carries.
var RequiredFrontmatter = []string{"title", "type", "status", "created", "updated", "tags"}

// PageTypes is the `type:` vocabulary. LYT adds note and moc.
var PageTypes = []string{"source", "entity", "concept", "question", "session", "comparison", "overview", "meta", "fold", "note", "moc"}

// SplitFrontmatter separates a leading YAML block from the body. ok is false when there is
// no block; an unterminated block is reported through err.
func SplitFrontmatter(text string) (front, body string, ok bool, err error) {
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return "", text, false, nil
	}
	rest := text[strings.Index(text, "\n")+1:]
	lines := strings.SplitAfter(rest, "\n")
	for i, line := range lines {
		if strings.TrimRight(line, "\r\n") == "---" {
			front = strings.Join(lines[:i], "")
			body = strings.Join(lines[i+1:], "")
			return front, body, true, nil
		}
	}
	return "", text, true, fmt.Errorf("frontmatter is not terminated by ---")
}

// Frontmatter parses a page's YAML block into a map. A page without a block yields nil.
func Frontmatter(text string) (map[string]any, string, error) {
	front, body, ok, err := SplitFrontmatter(text)
	if err != nil {
		return nil, body, err
	}
	if !ok {
		return nil, body, nil
	}
	fields := map[string]any{}
	if strings.TrimSpace(front) == "" {
		return fields, body, nil
	}
	if err := yaml.Unmarshal([]byte(front), &fields); err != nil {
		return nil, body, fmt.Errorf("frontmatter is not valid YAML: %w", err)
	}
	return fields, body, nil
}

// MissingFrontmatter lists required keys absent from fields.
func MissingFrontmatter(fields map[string]any) []string {
	var missing []string
	for _, key := range RequiredFrontmatter {
		if _, ok := fields[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

// StringField returns a frontmatter value as text when it is a scalar.
func StringField(fields map[string]any, key string) string {
	switch v := fields[key].(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// StringList returns a frontmatter value as a list of strings; a scalar becomes one item.
func StringList(fields map[string]any, key string) []string {
	switch v := fields[key].(type) {
	case []any:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	}
	return nil
}
