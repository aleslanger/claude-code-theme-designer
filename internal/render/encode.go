package render

import (
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

// Profile is the color capability used to encode a frame.
type Profile int

const (
	NoColor Profile = iota
	ANSI16
	ANSI256
	TrueColor
)

func (p Profile) String() string {
	switch p {
	case TrueColor:
		return "truecolor (24-bit)"
	case ANSI256:
		return "256 colors"
	case ANSI16:
		return "16 colors"
	default:
		return "no color"
	}
}

const (
	esc         = "\x1b["
	sgrReset    = esc + "0m"
	focusMarker = "◂"
	gutterWidth = 2
	ansiNormal  = 8
)

// Encode renders a frame to text with SGR color sequences for the profile.
// Escape sequences are built only from numbers derived from validated colors;
// every piece of text is sanitized, so no theme value can inject terminal
// control sequences.
func Encode(f Frame, colors map[string]theme.Color, p Profile) string {
	contentWidth := f.Width - gutterWidth
	var b strings.Builder
	for i, l := range f.Lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		encodeLine(&b, l, contentWidth, colors, p)
	}
	return b.String()
}

func encodeLine(b *strings.Builder, l Line, width int, colors map[string]theme.Color, p Profile) {
	used := 0
	for _, s := range l.Spans {
		text := runewidth.Truncate(theme.Sanitize(s.Text), width-used, "")
		if text == "" {
			continue
		}
		st := s.Style
		if st.BG == "" {
			st.BG = l.Fill
		}
		b.WriteString(sgr(st, colors, p))
		b.WriteString(text)
		used += runewidth.StringWidth(text)
	}
	if l.Fill != "" && used < width {
		b.WriteString(sgr(Style{BG: l.Fill}, colors, p))
		b.WriteString(strings.Repeat(" ", width-used))
		used = width
	}
	if p != NoColor {
		b.WriteString(sgrReset)
	}
	if l.Focus {
		b.WriteString(strings.Repeat(" ", max(width-used, 0)+1))
		b.WriteString(focusMarker)
	}
}

func sgr(st Style, colors map[string]theme.Color, p Profile) string {
	if p == NoColor {
		return ""
	}
	params := []string{"0"}
	if st.Bold {
		params = append(params, "1")
	}
	if st.Italic {
		params = append(params, "3")
	}
	if c, ok := colors[st.FG]; ok && st.FG != "" {
		params = append(params, colorParams(c, p, false)...)
	}
	if c, ok := colors[st.BG]; ok && st.BG != "" {
		params = append(params, colorParams(c, p, true)...)
	}
	return esc + strings.Join(params, ";") + "m"
}

// colorParams maps a color to SGR parameters. Named ANSI colors are emitted
// as the terminal's own palette entries, exactly as Claude Code does, so the
// preview matches the user's terminal. RGB is downsampled to the xterm cube
// on terminals without truecolor, like Claude Code.
func colorParams(c theme.Color, p Profile, background bool) []string {
	switch c.Kind() {
	case theme.KindANSINamed:
		return []string{basicColor(c.Index(), background)}
	case theme.KindANSI256:
		if p == ANSI16 {
			return []string{basicColor(theme.NearestANSI16(c.RGB()), background)}
		}
		return extended(background, "5", strconv.Itoa(int(c.Index())))
	default:
		rgb := c.RGB()
		switch p {
		case TrueColor:
			return extended(background, "2", strconv.Itoa(int(rgb.R)), strconv.Itoa(int(rgb.G)), strconv.Itoa(int(rgb.B)))
		case ANSI256:
			return extended(background, "5", strconv.Itoa(int(theme.NearestXterm256(rgb))))
		default:
			return []string{basicColor(theme.NearestANSI16(rgb), background)}
		}
	}
}

func extended(background bool, mode string, values ...string) []string {
	lead := "38"
	if background {
		lead = "48"
	}
	return append([]string{lead, mode}, values...)
}

// basicColor returns the SGR code for system color 0-15.
func basicColor(i uint8, background bool) string {
	base := 30
	if i >= ansiNormal {
		base, i = 90, i-ansiNormal
	}
	if background {
		base += 10
	}
	return strconv.Itoa(base + int(i))
}

// Paint wraps text in the foreground (and optional background) color using
// the same encoding as the preview. Text is sanitized.
func Paint(text string, fgColor, bgColor *theme.Color, p Profile) string {
	text = theme.Sanitize(text)
	if p == NoColor {
		return text
	}
	params := []string{"0"}
	if fgColor != nil {
		params = append(params, colorParams(*fgColor, p, false)...)
	}
	if bgColor != nil {
		params = append(params, colorParams(*bgColor, p, true)...)
	}
	return esc + strings.Join(params, ";") + "m" + text + sgrReset
}

// Plain renders the frame as text without any escape sequences (snapshots,
// NO_COLOR, dumb terminals).
func Plain(f Frame) string { return Encode(f, nil, NoColor) }

func pad(s string, w int) string {
	s = runewidth.Truncate(s, w, "")
	return s + strings.Repeat(" ", max(w-runewidth.StringWidth(s), 0))
}
