package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/aleslanger/claude-code-theme-designer/internal/render"
	"github.com/aleslanger/claude-code-theme-designer/internal/validate"
)

const (
	minWidth       = 60
	minHeight      = 20
	sideBySideFrom = 100
	leftMin        = 46
	leftMax        = 62
	labelWidth     = 30
	chromeRows     = 5 // header, 2 detail rows, status, key hints
	borderCols     = 4 // "│ " + " │"
	borderRows     = 2
)

// View implements tea.Model.
func (m Model) View() string {
	if m.width < minWidth || m.height < minHeight {
		return fmt.Sprintf("Terminal too small: need at least %dx%d, have %dx%d.\nResize, or use `claude-theme preview` for a non-interactive preview.\n",
			minWidth, minHeight, m.width, m.height)
	}
	body := m.body(m.height - chromeRows)
	lines := append([]string{m.header()}, body...)
	lines = append(lines, m.detail()...)
	lines = append(lines, m.statusLine(), m.hints())
	return strings.Join(lines, "\n")
}

func (m Model) header() string {
	dirty := ""
	if m.dirty() {
		dirty = "  ● unsaved"
	}
	return fit(fmt.Sprintf(" Claude Code Theme Designer · %s.json (%s)%s   — does not patch Claude Code", m.slug, m.source, dirty), m.width)
}

func (m Model) body(height int) []string {
	if m.width >= sideBySideFrom {
		leftW := min(max(m.width*45/100, leftMin), leftMax)
		rightW := m.width - leftW
		left := box(m.leftTitle(), m.leftContent(height-borderRows, leftW-borderCols), leftW, height)
		right := box("Preview", m.previewContent(height-borderRows, rightW-borderCols), rightW, height)
		out := make([]string, height)
		for i := range out {
			out[i] = left[i] + right[i]
		}
		return out
	}
	topH := height / 2
	top := box(m.leftTitle(), m.leftContent(topH-borderRows, m.width-borderCols), m.width, topH)
	bottom := box("Preview", m.previewContent(height-topH-borderRows, m.width-borderCols), m.width, height-topH)
	return append(top, bottom...)
}

func (m Model) leftTitle() string {
	switch m.mode {
	case modePicker:
		return "256-color palette · " + m.picker.key
	case modePresets:
		return "Presets"
	case modeHelp:
		return "Help"
	}
	return "Editor"
}

func (m Model) leftContent(height, width int) []string {
	switch m.mode {
	case modePicker:
		return m.paletteLines(width)
	case modePresets:
		return m.presetLines(width, height)
	case modeHelp:
		return helpLines()
	}
	return m.editorLines(height, width)
}

// editorLines renders the visible window of rows, keeping the cursor in view.
func (m Model) editorLines(height, width int) []string {
	start := m.scrollFor(height)
	var out []string
	for i := start; i < len(m.rows) && len(out) < height; i++ {
		out = append(out, m.rowLine(i, width))
	}
	return out
}

func (m Model) scrollFor(height int) int {
	if height <= 0 {
		return 0
	}
	start := 0
	if m.cursor >= height {
		start = m.cursor - height + 1
	}
	// Show the section header above the cursor when possible.
	if start > 0 && m.cursor-start < 1 {
		start--
	}
	return start
}

func (m Model) rowLine(i, width int) string {
	r := m.rows[i]
	cursor := "  "
	if i == m.cursor {
		cursor = "▸ "
	}
	switch r.kind {
	case rowHeader:
		return fit(strings.ToUpper(r.label), width)
	case rowName:
		v := m.theme.Name
		if v == "" {
			v = "(uses the file name)"
		}
		return fit(cursor+pad(r.label, labelWidth)+v, width)
	case rowSlug:
		return fit(cursor+pad(r.label, labelWidth)+m.slug+".json", width)
	case rowBase:
		v := string(m.theme.EffectiveBase())
		if m.theme.Base == "" {
			v += " (default)"
		}
		return fit(cursor+pad(r.label, labelWidth)+v, width)
	case rowUnsupported:
		return fit(cursor+"✗ "+pad(r.label, labelWidth-2)+"unsupported", width)
	}
	return fit(cursor+m.tokenCell(r), width)
}

func (m Model) tokenCell(r row) string {
	c, overridden := m.resolvedColor(r.key)
	swatch := render.Paint("██", &c, nil, m.profile)
	value := c.String()
	if !overridden {
		value = "· inherit"
	}
	var flags []string
	if tok, ok := m.app.Reg.Lookup(r.key); ok && tok.FullscreenOnly {
		flags = append(flags, "fs")
	}
	if worst := worstSeverity(m.report.ForToken(r.key)); worst >= validate.Warning {
		flags = append(flags, "!"+worst.String())
	}
	suffix := ""
	if len(flags) > 0 {
		suffix = "  [" + strings.Join(flags, ",") + "]"
	}
	return pad(r.label, labelWidth) + swatch + " " + value + suffix
}

func worstSeverity(issues []validate.Issue) validate.Severity {
	worst := validate.Info
	for _, i := range issues {
		worst = max(worst, i.Severity)
	}
	return worst
}

func (m Model) previewContent(height, width int) []string {
	focus := ""
	if r := m.currentRow(); r.kind == rowToken {
		focus = r.key
	}
	frame := render.Preview(render.Options{Width: width, Focus: focus})
	lines := strings.Split(render.Encode(frame, m.app.Reg.Resolve(m.theme), m.profile), "\n")
	start := 0
	for i, l := range frame.Lines {
		if l.Focus {
			start = i
			break
		}
	}
	if start < height-1 {
		start = 0
	} else {
		start = min(start-height/3, len(lines)-height)
	}
	return lines[max(start, 0):]
}

func (m Model) detail() []string {
	if m.mode == modeInput {
		label := map[inputPurpose]string{inputColor: "Color for " + m.picker.key, inputName: "Display name", inputSlug: "File name (A-Z a-z 0-9 _ -)"}[m.input.purpose]
		help := "Enter apply · Esc cancel"
		if m.input.purpose == inputColor {
			help = "#rrggbb  #rgb  rgb(r,g,b)  ansi256(n)  ansi:<name> · Enter apply · Esc cancel"
		}
		second := help
		if m.input.err != "" {
			second = "✗ " + m.input.err
		}
		return []string{fit(" "+label+": "+m.input.value+"█", m.width), fit(" "+second, m.width)}
	}
	r := m.currentRow()
	switch r.kind {
	case rowToken:
		tok, _ := m.app.Reg.Lookup(r.key)
		second := "source: " + tok.Source
		if _, overridden := m.theme.Override(r.key); !overridden {
			c, _ := m.resolvedColor(r.key)
			second = fmt.Sprintf("inherits %s from base %s (captured default, approximate) · %s", c, m.theme.EffectiveBase(), second)
		}
		if issues := m.report.ForToken(r.key); len(issues) > 0 {
			second = issues[0].Severity.String() + ": " + issues[0].Message
		}
		return []string{fit(" "+r.key+" — "+tok.Description, m.width), fit(" "+second, m.width)}
	case rowUnsupported:
		return []string{fit(" "+r.label+": not available in the Claude Code theme API", m.width), fit(" "+r.reason, m.width)}
	case rowBase:
		return []string{fit(" Built-in preset the theme starts from; tokens you do not override inherit from it.", m.width), fit(" Enter or b cycles dark / light / daltonized / ansi variants.", m.width)}
	}
	return []string{fit(" Enter to edit.", m.width), ""}
}

func (m Model) statusLine() string {
	if m.mode == modeConfirm {
		return fit(" ? "+m.confirm.question+"  [y/N]", m.width)
	}
	if m.status != "" {
		mark := "✓"
		if m.statusBad {
			mark = "✗"
		}
		return fit(" "+mark+" "+m.status, m.width)
	}
	e, w := m.report.Errors(), m.report.Warnings()
	if e+w == 0 {
		return fit(" ✓ no contrast or compatibility warnings", m.width)
	}
	return fit(fmt.Sprintf(" ! %d error(s), %d warning(s) — rows marked [!…]", e, w), m.width)
}

func (m Model) hints() string {
	h := map[mode]string{
		modeBrowse:  " ↑↓ move  ⏎ edit  # hex  i inherit  r/R/X reset  +/- adjust  b base  p presets  s save  I install  ? help  q quit",
		modePicker:  " ←↑↓→ choose (live) · enter apply · # hex · r reset · i inherit · esc cancel",
		modeInput:   " type a value · enter apply · esc cancel",
		modeConfirm: " y confirm · any other key cancels",
		modePresets: " ↑↓ choose · enter load · esc back",
		modeHelp:    " any key to return",
	}[m.mode]
	return fit(h, m.width)
}

// box draws a titled frame of exactly width x height.
func box(title string, content []string, width, height int) []string {
	inner := width - borderCols
	top := "┌ " + title + " " + strings.Repeat("─", max(width-ansi.StringWidth(title)-4, 0)) + "┐"
	out := []string{fit(top, width)}
	for i := 0; i < height-borderRows; i++ {
		line := ""
		if i < len(content) {
			line = content[i]
		}
		out = append(out, "│ "+fit(line, inner)+" │")
	}
	return append(out, "└"+strings.Repeat("─", max(width-2, 0))+"┘")
}

// fit truncates or pads an ANSI string to exactly w cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "")
	return s + strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
}

func pad(s string, w int) string { return fit(s, w) }
