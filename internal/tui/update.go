package tui

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aleslanger/claude-code-theme-designer/internal/presets"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

const (
	pageStep   = 10
	adjustStep = 0.05
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m.interrupt()
		}
		if msg.Paste && m.mode == modeInput {
			return m.appendInput(string(msg.Runes)).liveInput(), nil
		}
		return m.handleKey(msg.String())
	}
	return m, nil
}

// interrupt treats ctrl+c like q, but a second ctrl+c at the prompt quits.
func (m Model) interrupt() (tea.Model, tea.Cmd) {
	if m.mode == modeConfirm || !m.dirty() {
		return m, tea.Quit
	}
	return m.ask("Quit without saving changes? (ctrl+c again to quit)", func(m Model) (Model, tea.Cmd) { return m, tea.Quit }), nil
}

func (m Model) handleKey(key string) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modePicker:
		return m.pickerKey(key), nil
	case modeInput:
		return m.inputKey(key), nil
	case modeConfirm:
		return m.confirmKey(key)
	case modePresets:
		return m.presetKey(key), nil
	case modeHelp:
		m.mode = modeBrowse
		return m, nil
	}
	return m.browseKey(key)
}

func (m Model) browseKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "esc":
		if m.dirty() {
			return m.ask("Quit without saving changes?", func(m Model) (Model, tea.Cmd) { return m, tea.Quit }), nil
		}
		return m, tea.Quit
	case "up", "k":
		return m.move(-1), nil
	case "down", "j":
		return m.move(1), nil
	case "pgup":
		return m.move(-pageStep), nil
	case "pgdown":
		return m.move(pageStep), nil
	case "home", "g":
		return m.moveTo(0, 1), nil
	case "end", "G":
		return m.moveTo(len(m.rows)-1, -1), nil
	case "?":
		m.mode = modeHelp
		return m, nil
	}
	return m.actionKey(key), nil
}

func (m Model) actionKey(key string) Model {
	r := m.currentRow()
	switch key {
	case "enter", " ":
		return m.activate(r)
	case "#":
		if r.kind == rowToken {
			return m.startColorInput(r.key)
		}
	case "i":
		if r.kind == rowToken {
			return m.setTheme(m.theme.WithoutOverride(r.key)).setStatus(r.key+": inherits from base", false)
		}
	case "r":
		if r.kind == rowToken {
			return m.resetKeys([]string{r.key}).setStatus(r.key+": reset", false)
		}
	case "R":
		return m.resetSection(r)
	case "X":
		return m.ask("Reset the whole theme to its starting point?", func(m Model) (Model, tea.Cmd) {
			return m.setTheme(m.baseline).setStatus("theme reset", false), nil
		})
	case "+", "=":
		return m.adjust(r, adjustStep)
	case "-":
		return m.adjust(r, -adjustStep)
	case "b":
		return m.cycleBase(1)
	case "p":
		m.mode = modePresets
		return m
	case "s":
		return m.save()
	case "I":
		return m.install()
	}
	return m
}

func (m Model) activate(r row) Model {
	switch r.kind {
	case rowToken:
		return m.openPicker(r.key)
	case rowBase:
		return m.cycleBase(1)
	case rowName:
		return m.startTextInput(inputName, m.theme.Name)
	case rowSlug:
		return m.startTextInput(inputSlug, m.slug)
	case rowUnsupported:
		return m.setStatus("not available through the Claude Code theme API: "+r.reason, true)
	}
	return m
}

func (m Model) move(delta int) Model {
	dir := 1
	if delta < 0 {
		dir = -1
	}
	return m.moveTo(m.cursor+delta, dir)
}

// moveTo puts the cursor on target, sliding in dir past header rows.
func (m Model) moveTo(target, dir int) Model {
	target = min(max(target, 0), len(m.rows)-1)
	for target >= 0 && target < len(m.rows) && !m.rows[target].selectable() {
		target += dir
	}
	if target < 0 || target >= len(m.rows) {
		return m
	}
	m.cursor = target
	return m
}

func (m Model) resetKeys(keys []string) Model {
	t := m.theme
	for _, k := range keys {
		if c, ok := m.baseline.Override(k); ok {
			t = t.WithOverride(k, c)
		} else {
			t = t.WithoutOverride(k)
		}
	}
	return m.setTheme(t)
}

func (m Model) resetSection(r row) Model {
	if r.section < 0 {
		t := m.theme.WithName(m.baseline.Name).WithBase(m.baseline.Base)
		return m.setTheme(t).setStatus("name and base reset", false)
	}
	s := m.sections[r.section]
	return m.resetKeys(s.Keys).setStatus("section reset: "+s.Title, false)
}

func (m Model) adjust(r row, step float64) Model {
	if r.kind != rowToken {
		return m
	}
	c, _ := m.resolvedColor(r.key)
	if c.IsApproximate() {
		return m.setStatus("named ANSI colors depend on the terminal palette; pick a palette or hex color first", true)
	}
	next := theme.FromRGB(theme.Adjust(c.RGB(), step))
	return m.setTheme(m.theme.WithOverride(r.key, next)).setStatus(r.key+" = "+next.String(), false)
}

func (m Model) cycleBase(dir int) Model {
	i := slices.Index(theme.Bases, m.theme.EffectiveBase())
	next := theme.Bases[(i+dir+len(theme.Bases))%len(theme.Bases)]
	return m.setTheme(m.theme.WithBase(next)).setStatus("base: "+string(next), false)
}

func (m Model) ask(question string, onYes func(Model) (Model, tea.Cmd)) Model {
	m.mode = modeConfirm
	m.confirm = confirmState{question: question, onYes: onYes}
	return m
}

func (m Model) confirmKey(key string) (tea.Model, tea.Cmd) {
	m.mode = modeBrowse
	if key != "y" && key != "Y" {
		return m.setStatus("cancelled", false), nil
	}
	return m.confirm.onYes(m)
}

func (m Model) presetKey(key string) Model {
	all := presets.All()
	switch key {
	case "up", "k":
		m.presets.cursor = max(m.presets.cursor-1, 0)
	case "down", "j":
		m.presets.cursor = min(m.presets.cursor+1, len(all)-1)
	case "esc", "q":
		m.mode = modeBrowse
	case "enter":
		p := all[m.presets.cursor]
		m.mode = modeBrowse
		m.baseline = p.Theme
		return m.setTheme(p.Theme).setStatus("loaded preset "+p.Slug+" (reset now returns here)", false)
	}
	return m
}
