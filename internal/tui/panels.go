package tui

import (
	"fmt"
	"strings"

	"github.com/aleslanger/claude-code-theme-designer/internal/presets"
	"github.com/aleslanger/claude-code-theme-designer/internal/render"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

// paletteLines draws the xterm-256 palette as a 16x16 grid of swatches.
func (m Model) paletteLines(width int) []string {
	cur := theme.FromANSI256(uint8(m.picker.index))
	out := []string{
		fit(fmt.Sprintf("%s  ≈ %s", cur.String(), cur.RGB().Hex()), width),
		"",
	}
	for row := 0; row < paletteSize/paletteCols; row++ {
		var b strings.Builder
		for col := 0; col < paletteCols; col++ {
			n := row*paletteCols + col
			c := theme.FromANSI256(uint8(n))
			cell := "  "
			if n == m.picker.index {
				cell = "▐▌"
				if m.profile == render.NoColor {
					cell = "[]"
				}
			}
			b.WriteString(render.Paint(cell, contrastingInk(c), &c, m.profile))
		}
		out = append(out, b.String())
	}
	out = append(out, "", fit("0-15 follow your terminal palette; 16-255 are fixed.", width))
	if m.profile == render.NoColor {
		out = append(out, fit("No color support detected: press # to type a value.", width))
	}
	return out
}

// contrastingInk picks black or white for the cursor glyph on c.
func contrastingInk(c theme.Color) *theme.Color {
	black, white := theme.FromRGB(theme.RGB{}), theme.FromRGB(theme.RGB{R: 255, G: 255, B: 255})
	if theme.ContrastRatio(c.RGB(), black.RGB()) >= theme.ContrastRatio(c.RGB(), white.RGB()) {
		return &black
	}
	return &white
}

// presetLines lists presets grouped by category, scrolled to keep the
// cursor visible in short panes.
func (m Model) presetLines(width, height int) []string {
	all := presets.All()
	var lines []string
	cursorLine := 0
	for _, cat := range presets.Categories {
		lines = append(lines, fit(strings.ToUpper(string(cat)), width))
		for i, p := range all {
			if p.Category != cat {
				continue
			}
			cursor := "  "
			if i == m.presets.cursor {
				cursor, cursorLine = "▸ ", len(lines)
			}
			lines = append(lines, fit(cursor+pad(p.Theme.Name, 24)+p.Description, width))
		}
	}
	lines = append(lines, "", fit("\"-like\" presets approximate known palettes; they are", width),
		fit("not the official schemes and not official Claude Code themes.", width))
	if height <= 0 || len(lines) <= height {
		return lines
	}
	start := min(max(cursorLine-height/2, 0), len(lines)-height)
	return lines[start : start+height]
}

func helpLines() []string {
	return []string{
		"Edits stay in memory until you save (s) or install (I).",
		"",
		"Editor",
		"  ↑/↓ j/k  move        PgUp/PgDn  page      g/G  top/bottom",
		"  Enter    edit token (palette) / cycle base / rename",
		"  #        type a color: #rrggbb #rgb rgb() ansi256() ansi:<name>",
		"  i        inherit: remove the override, use the base value",
		"  r        reset token to the starting value",
		"  R        reset the current section",
		"  X        reset the whole theme (asks first)",
		"  + / -    lighten / darken the selected color",
		"  b        cycle base preset",
		"  p        load a starter preset",
		"  s        save draft  (~/.config/claude-theme-designer/themes)",
		"  I        install into ~/.claude/themes (backup before replace)",
		"",
		"After installing, choose the theme in Claude Code with /theme.",
		"Rows marked [fs] only show in fullscreen rendering mode (/tui fullscreen).",
		"✗ rows are not available through the theme API; no workaround is applied.",
	}
}
