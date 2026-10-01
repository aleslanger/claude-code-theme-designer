// Command screenshot renders the real editor view to an SVG for the README.
// It is a development tool:
//
//	go run ./tools/screenshot -o docs/screenshot.svg
package main

import (
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/aleslanger/claude-code-theme-designer/internal/app"
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/render"
	"github.com/aleslanger/claude-code-theme-designer/internal/store"
	"github.com/aleslanger/claude-code-theme-designer/internal/tui"
)

const (
	cols, rows   = 132, 40
	cellW, cellH = 8.4, 18.0
	pad          = 16.0
	defaultFG    = "#d4d4d4"
	terminalBG   = "#1e1e1e"
	fontFamily   = "'JetBrains Mono','DejaVu Sans Mono',Menlo,Consolas,monospace"
)

func main() {
	out := flag.String("o", "docs/screenshot.svg", "output SVG")
	slug := flag.String("theme", "prompt-contrast", "theme to show")
	focus := flag.Int("down", 4, "cursor moves before capturing")
	gallery := flag.Bool("gallery", false, "render every preset as a card grid instead")
	flag.Parse()
	svg, err := build(*gallery, *slug, *focus)
	if err == nil {
		err = os.WriteFile(*out, []byte(svg), 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "screenshot:", err)
		os.Exit(1)
	}
}

func build(gallery bool, slug string, downs int) (string, error) {
	if gallery {
		return gallerySVG()
	}
	view, err := capture(slug, downs)
	if err != nil {
		return "", err
	}
	return toSVG(view), nil
}

// capture builds the editor in an empty temporary HOME, so the image never
// contains the developer's own themes or paths.
func capture(slug string, downs int) (string, error) {
	home, err := os.MkdirTemp("", "claude-theme-shot-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(home)
	reg, err := registry.Default()
	if err != nil {
		return "", err
	}
	paths, err := store.ResolvePaths(filepath.Clean(home), func(string) string { return "" })
	if err != nil {
		return "", err
	}
	st, err := store.Open(paths)
	if err != nil {
		return "", err
	}
	a := app.New(reg, st)
	l, err := a.Resolve(slug)
	if err != nil {
		return "", err
	}
	var m tea.Model = tui.New(a, l, render.TrueColor)
	m, _ = m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	for range downs {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	return m.View(), nil
}

var reSGR = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

type style struct {
	fg, bg string
	bold   bool
	italic bool
}

func svgOpen(b *strings.Builder, w, h float64) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="%s" font-size="14">`+"\n", w, h, w, h, fontFamily)
}

func toSVG(view string) string {
	w, h := pad*2+cols*cellW, pad*2+rows*cellH
	var b strings.Builder
	svgOpen(&b, w, h)
	fmt.Fprintf(&b, `<rect width="100%%" height="100%%" rx="8" fill="%s"/>`+"\n", terminalBG)
	c := canvas{b: &b, ox: pad, oy: pad, fg: defaultFG}
	for y, line := range strings.Split(view, "\n") {
		c.line(line, y)
	}
	b.WriteString("</svg>\n")
	return b.String()
}

// canvas draws ANSI-colored text lines at an origin.
type canvas struct {
	b      *strings.Builder
	ox, oy float64
	fg     string // default foreground
}

func (c canvas) line(line string, y int) {
	st := style{}
	x := 0
	pos := 0
	full := line + "\x1b[0m"
	for _, m := range reSGR.FindAllStringSubmatchIndex(full, -1) {
		end := min(m[0], len(line))
		x = c.text(line[pos:end], x, y, st)
		st = apply(st, full[m[2]:m[3]])
		pos = min(m[1], len(line))
	}
}

func (c canvas) text(text string, x, y int, st style) int {
	b := c.b
	if text == "" {
		return x
	}
	width := runewidth.StringWidth(text)
	px, py := c.ox+float64(x)*cellW, c.oy+float64(y)*cellH
	if st.bg != "" {
		fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`+"\n", px, py, float64(width)*cellW, cellH, st.bg)
	}
	if strings.TrimSpace(text) != "" {
		fg := st.fg
		if fg == "" {
			fg = c.fg
		}
		attrs := ""
		if st.bold {
			attrs += ` font-weight="bold"`
		}
		if st.italic {
			attrs += ` font-style="italic"`
		}
		fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="%s" xml:space="preserve" textLength="%.1f"%s>%s</text>`+"\n",
			px, py+cellH*0.75, fg, float64(width)*cellW, attrs, html.EscapeString(text))
	}
	return x + width
}

// apply interprets the SGR subset that render.Encode emits.
func apply(st style, params string) style {
	p := strings.Split(params, ";")
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case "", "0":
			st = style{}
		case "1":
			st.bold = true
		case "3":
			st.italic = true
		case "38", "48":
			color, n := extendedColor(p[i+1:])
			if p[i] == "38" {
				st.fg = color
			} else {
				st.bg = color
			}
			i += n
		}
	}
	return st
}

func extendedColor(p []string) (string, int) {
	if len(p) >= 4 && p[0] == "2" {
		r, _ := strconv.Atoi(p[1])
		g, _ := strconv.Atoi(p[2])
		bl, _ := strconv.Atoi(p[3])
		return fmt.Sprintf("#%02x%02x%02x", r, g, bl), 4
	}
	if len(p) >= 2 && p[0] == "5" {
		n, _ := strconv.Atoi(p[1])
		c := xterm(n)
		return c, 2
	}
	return "", len(p)
}

func xterm(n int) string {
	levels := []int{0, 95, 135, 175, 215, 255}
	switch {
	case n >= 16 && n < 232:
		i := n - 16
		return fmt.Sprintf("#%02x%02x%02x", levels[i/36], levels[(i/6)%6], levels[i%6])
	case n >= 232:
		v := 8 + 10*(n-232)
		return fmt.Sprintf("#%02x%02x%02x", v, v, v)
	}
	return defaultFG
}
