// Package render draws the theme preview. A Frame refers to theme tokens by
// key, never to concrete colors; Encode resolves them through the same
// registry.Resolve used by validation, so preview and checks cannot disagree.
package render

// Style refers to theme tokens; an empty key means the terminal default.
type Style struct {
	FG, BG string
	Bold   bool
	Italic bool
}

// Span is a run of text with one style.
type Span struct {
	Text  string
	Style Style
}

// Line is one preview row. Fill paints the whole row width with a background
// token, the way Claude Code paints message backgrounds.
type Line struct {
	Spans []Span
	Fill  string
	// Focus marks rows that use the token currently being edited.
	Focus bool
}

// Frame is a fixed-width block of lines.
type Frame struct {
	Width int
	Lines []Line
}

// usesToken reports whether a line paints key anywhere.
func (l Line) usesToken(key string) bool {
	if key == "" {
		return false
	}
	if l.Fill == key {
		return true
	}
	for _, s := range l.Spans {
		if s.Style.FG == key || s.Style.BG == key {
			return true
		}
	}
	return false
}

func span(text string, st Style) Span { return Span{Text: text, Style: st} }

func fg(key string) Style { return Style{FG: key} }

func bold(key string) Style { return Style{FG: key, Bold: true} }
