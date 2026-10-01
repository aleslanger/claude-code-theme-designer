package main

import (
	"fmt"
	"html"
	"strings"

	"github.com/aleslanger/claude-code-theme-designer/internal/presets"
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/render"
)

const (
	cardCols    = 3
	cardChars   = 42 // frame width incl. the 2-column gutter
	cardLines   = 9
	cardGap     = 14.0
	cardPadding = 12.0
	titleHeight = 22.0
	lightCanvas = "#ffffff"
	lightInk    = "#1f2328"
)

// gallerySVG renders one card per preset, each on a canvas matching its base
// (light themes on white), using the same resolver and encoder as the app.
func gallerySVG() (string, error) {
	reg, err := registry.Default()
	if err != nil {
		return "", err
	}
	all := presets.All()
	cardW := cardPadding*2 + float64(cardChars-2)*cellW
	cardH := cardPadding*2 + titleHeight + cardLines*cellH
	rowsN := (len(all) + cardCols - 1) / cardCols
	w := pad*2 + cardCols*cardW + (cardCols-1)*cardGap
	h := pad*2 + float64(rowsN)*cardH + float64(rowsN-1)*cardGap

	var b strings.Builder
	svgOpen(&b, w, h)
	for i, p := range all {
		x := pad + float64(i%cardCols)*(cardW+cardGap)
		y := pad + float64(i/cardCols)*(cardH+cardGap)
		bg, ink := terminalBG, defaultFG
		if p.Theme.EffectiveBase().IsLight() {
			bg, ink = lightCanvas, lightInk
		}
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="8" fill="%s" stroke="#8884"/>`+"\n", x, y, cardW, cardH, bg)
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="%s" font-weight="bold">%s</text>`+"\n",
			x+cardPadding, y+cardPadding+14, ink, html.EscapeString(p.Theme.Name))
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="%s" font-size="11" text-anchor="end" opacity="0.7">%s</text>`+"\n",
			x+cardW-cardPadding, y+cardPadding+14, ink, html.EscapeString(p.Slug))
		encoded := render.Encode(cardFrame(), reg.Resolve(p.Theme), render.TrueColor)
		c := canvas{b: &b, ox: x + cardPadding, oy: y + cardPadding + titleHeight, fg: ink}
		for ly, line := range strings.Split(encoded, "\n") {
			c.line(line, ly)
		}
	}
	b.WriteString("</svg>\n")
	return b.String(), nil
}

func cardFrame() render.Frame {
	fg := func(k string) render.Style { return render.Style{FG: k} }
	bold := func(k string) render.Style { return render.Style{FG: k, Bold: true} }
	span := func(t string, s render.Style) render.Span { return render.Span{Text: t, Style: s} }
	return render.Frame{Width: cardChars, Lines: []render.Line{
		{Fill: "userMessageBackground", Spans: []render.Span{span("> ", fg("subtle")), span("Why can this race?", fg("text"))}},
		{Spans: []render.Span{span("You", bold("briefLabelYou"))}},
		{Fill: "userMessageBackground", Spans: []render.Span{span(" Guard count with a mutex", fg("text"))}},
		{Spans: []render.Span{span("Claude", bold("briefLabelClaude"))}},
		{Spans: []render.Span{span(" Done: count is now guarded.", fg("text"))}},
		{Spans: []render.Span{span("⏺ ", fg("success")), span("Bash", bold("text")), span("(go test ./...)", fg("text"))}},
		{Spans: []render.Span{span("  ⎿  ", fg("inactive")), span("Error: build failed", fg("error"))}},
		{Spans: []render.Span{span("⚠ Context low", fg("warning"))}},
		{Fill: "userMessageBackgroundHover", Spans: []render.Span{span("> hovered prompt", fg("text"))}},
	}}
}
