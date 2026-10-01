// Package serialize converts between theme.Theme and the Claude Code theme
// JSON format. Decoding is strict: it is the trust boundary for imported files.
package serialize

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

const (
	// MaxFileSize matches the limit above which Claude Code skips a theme file.
	MaxFileSize = 256 << 10
	// MaxOverrides bounds the number of tokens; Claude Code knows ~70.
	MaxOverrides = 512
	// MaxProblems caps the problem list so hostile input cannot flood output.
	MaxProblems = 50

	fieldName      = "name"
	fieldBase      = "base"
	fieldOverrides = "overrides"
	utf8BOM        = "\xef\xbb\xbf"
)

// reTokenKey restricts override keys to identifier-like names. Every real
// token matches; anything else (paths, escapes, whitespace) is rejected.
var reTokenKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

// Problem is one reason a document was rejected.
type Problem struct {
	Path    string // JSON path such as "overrides.text"
	Message string
}

func (p Problem) String() string {
	if p.Path == "" {
		return p.Message
	}
	return p.Path + ": " + p.Message
}

// DecodeError lists every problem found in a rejected document.
type DecodeError struct{ Problems []Problem }

func (e *DecodeError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.String()
	}
	return "invalid theme: " + strings.Join(parts, "; ")
}

type collector struct{ problems []Problem }

func (c *collector) add(path, format string, args ...any) {
	if len(c.problems) >= MaxProblems {
		return
	}
	c.problems = append(c.problems, Problem{Path: theme.Sanitize(path), Message: theme.Sanitize(fmt.Sprintf(format, args...))})
}

// Decode parses and strictly validates a theme document.
//
// Rejected: files over MaxFileSize, invalid UTF-8, a BOM, non-object roots,
// duplicate keys, unknown top-level keys, wrong types, unknown bases, invalid
// display names, malformed token keys, invalid colors and trailing data.
// Unknown but well-formed override keys are accepted and preserved
// (passthrough); semantic warnings about them come from package validate.
func Decode(data []byte) (theme.Theme, error) {
	c := &collector{}
	if len(data) > MaxFileSize {
		c.add("", "file larger than %d bytes", MaxFileSize)
		return theme.Theme{}, &DecodeError{c.problems}
	}
	if !utf8.Valid(data) {
		c.add("", "not valid UTF-8")
		return theme.Theme{}, &DecodeError{c.problems}
	}
	if bytes.HasPrefix(data, []byte(utf8BOM)) {
		c.add("", "byte order mark is not allowed (Claude Code's JSON parser rejects it)")
		return theme.Theme{}, &DecodeError{c.problems}
	}
	members, err := objectMembers(data)
	if err != nil {
		c.add("", "%v", err)
		return theme.Theme{}, &DecodeError{c.problems}
	}
	t := theme.Theme{Overrides: map[string]theme.Color{}}
	for _, m := range members {
		switch m.key {
		case fieldName:
			t.Name = decodeName(c, m.raw)
		case fieldBase:
			t.Base = decodeBase(c, m.raw)
		case fieldOverrides:
			t.Overrides = decodeOverrides(c, m.raw)
		default:
			c.add(m.key, "unknown top-level key (allowed: name, base, overrides)")
		}
	}
	if len(c.problems) > 0 {
		return theme.Theme{}, &DecodeError{c.problems}
	}
	return t, nil
}

type member struct {
	key string
	raw json.RawMessage
}

// objectMembers splits a JSON object into members, rejecting duplicate keys
// (which encoding/json would silently resolve to the last one) and trailing data.
func objectMembers(data []byte) ([]member, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("malformed JSON: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errors.New("top level must be a JSON object")
	}
	seen := map[string]bool{}
	var out []member
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("malformed JSON: %w", err)
		}
		key, _ := kt.(string) // object keys are always strings
		if seen[key] {
			return nil, fmt.Errorf("duplicate key %q", theme.Sanitize(key))
		}
		seen[key] = true
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("malformed JSON: %w", err)
		}
		out = append(out, member{key: key, raw: raw})
	}
	if _, err := dec.Token(); err != nil { // closing brace
		return nil, fmt.Errorf("malformed JSON: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the JSON object")
	}
	return out, nil
}

func decodeString(c *collector, path string, raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		c.add(path, "must be a string")
		return "", false
	}
	return s, true
}

func decodeName(c *collector, raw json.RawMessage) string {
	s, ok := decodeString(c, fieldName, raw)
	if !ok {
		return ""
	}
	if s == "" {
		c.add(fieldName, "must not be empty (omit the field to use the file name)")
		return ""
	}
	if err := theme.ValidateDisplayName(s); err != nil {
		c.add(fieldName, "%v", err)
		return ""
	}
	return s
}

func decodeBase(c *collector, raw json.RawMessage) theme.Base {
	s, ok := decodeString(c, fieldBase, raw)
	if !ok {
		return ""
	}
	b := theme.Base(s)
	if !theme.IsValidBase(b) {
		c.add(fieldBase, "unknown base %q (allowed: %s)", s, baseList())
		return ""
	}
	return b
}

func decodeOverrides(c *collector, raw json.RawMessage) map[string]theme.Color {
	out := map[string]theme.Color{}
	members, err := objectMembers(raw)
	if err != nil {
		c.add(fieldOverrides, "%v", err)
		return out
	}
	if len(members) > MaxOverrides {
		c.add(fieldOverrides, "more than %d tokens", MaxOverrides)
		return out
	}
	for _, m := range members {
		path := fieldOverrides + "." + m.key
		if !reTokenKey.MatchString(m.key) {
			c.add(path, "invalid token name (expected letters, digits and _)")
			continue
		}
		s, ok := decodeString(c, path, m.raw)
		if !ok {
			continue
		}
		col, err := theme.ParseColor(s)
		if err != nil {
			c.add(path, "%v", err)
			continue
		}
		out[m.key] = col
	}
	return out
}

func baseList() string {
	names := make([]string, len(theme.Bases))
	for i, b := range theme.Bases {
		names[i] = string(b)
	}
	return strings.Join(names, ", ")
}

type document struct {
	Name      string            `json:"name,omitempty"`
	Base      theme.Base        `json:"base,omitempty"`
	Overrides map[string]string `json:"overrides"`
}

// Encode produces deterministic JSON: fields in name/base/overrides order,
// override keys sorted, two-space indent and a trailing newline, the same shape
// Claude Code writes itself. Absent name/base stay absent. It refuses to emit a
// document that Decode would reject.
func Encode(t theme.Theme) ([]byte, error) {
	if t.Name != "" {
		if err := theme.ValidateDisplayName(t.Name); err != nil {
			return nil, err
		}
	}
	if t.Base != "" && !theme.IsValidBase(t.Base) {
		return nil, fmt.Errorf("unknown base %q", theme.Sanitize(string(t.Base)))
	}
	if len(t.Overrides) > MaxOverrides {
		return nil, fmt.Errorf("more than %d tokens", MaxOverrides)
	}
	doc := document{Name: t.Name, Base: t.Base, Overrides: make(map[string]string, len(t.Overrides))}
	for k, col := range t.Overrides {
		if !reTokenKey.MatchString(k) {
			return nil, fmt.Errorf("invalid token name %q", theme.Sanitize(k))
		}
		if col.IsZero() {
			return nil, fmt.Errorf("token %q has no color", k)
		}
		doc.Overrides[k] = col.String()
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil { // encoding/json sorts map keys
		return nil, err
	}
	if buf.Len() > MaxFileSize {
		return nil, fmt.Errorf("encoded theme larger than %d bytes", MaxFileSize)
	}
	return buf.Bytes(), nil
}
