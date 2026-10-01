package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

const (
	paletteCols = 16
	paletteSize = 256
	maxInputLen = 64
)

// openPicker starts the 256-color palette for key. Moving the cursor applies
// the color live; Esc restores the original.
func (m Model) openPicker(key string) Model {
	orig, had := m.theme.Override(key)
	start := 0
	if c, _ := m.resolvedColor(key); !c.IsZero() {
		start = int(paletteIndexFor(c))
	}
	m.mode = modePicker
	m.picker = pickerState{index: start, key: key, original: orig, hadOriginal: had}
	return m.previewPick()
}

func paletteIndexFor(c theme.Color) uint8 {
	switch c.Kind() {
	case theme.KindANSI256, theme.KindANSINamed:
		return c.Index()
	}
	return theme.NearestXterm256(c.RGB())
}

func (m Model) previewPick() Model {
	c := theme.FromANSI256(uint8(m.picker.index))
	return m.setTheme(m.theme.WithOverride(m.picker.key, c))
}

func (m Model) restorePick() Model {
	if m.picker.hadOriginal {
		return m.setTheme(m.theme.WithOverride(m.picker.key, m.picker.original))
	}
	return m.setTheme(m.theme.WithoutOverride(m.picker.key))
}

func (m Model) pickerKey(key string) Model {
	step := map[string]int{"left": -1, "h": -1, "right": 1, "l": 1, "up": -paletteCols, "k": -paletteCols, "down": paletteCols, "j": paletteCols}
	if d, ok := step[key]; ok {
		m.picker.index = (m.picker.index + d + paletteSize) % paletteSize
		return m.previewPick()
	}
	switch key {
	case "enter", " ":
		m.mode = modeBrowse
		return m.setStatus(m.picker.key+" = "+theme.FromANSI256(uint8(m.picker.index)).String(), false)
	case "esc", "q":
		m.mode = modeBrowse
		return m.restorePick().setStatus("cancelled", false)
	case "#":
		m = m.restorePick()
		return m.startColorInput(m.picker.key)
	case "r":
		m.mode = modeBrowse
		return m.resetKeys([]string{m.picker.key}).setStatus(m.picker.key+": reset", false)
	case "i":
		m.mode = modeBrowse
		return m.setTheme(m.theme.WithoutOverride(m.picker.key)).setStatus(m.picker.key+": inherits from base", false)
	}
	return m
}

// startColorInput opens a text field accepting any documented color syntax.
func (m Model) startColorInput(key string) Model {
	m.picker.key = key
	value := ""
	if c, ok := m.theme.Override(key); ok {
		value = c.String()
	}
	return m.startTextInput(inputColor, value)
}

func (m Model) startTextInput(p inputPurpose, value string) Model {
	m.mode = modeInput
	m.input = inputState{purpose: p, value: value, original: m.theme}
	return m
}

func (m Model) inputKey(key string) Model {
	switch key {
	case "esc":
		m.mode = modeBrowse
		return m.setTheme(m.input.original).setStatus("cancelled", false)
	case "enter":
		return m.commitInput()
	case "backspace":
		if m.input.value != "" {
			_, size := utf8.DecodeLastRuneInString(m.input.value)
			m.input.value = m.input.value[:len(m.input.value)-size]
		}
	case "space":
		m.input.value += " "
	default:
		if utf8.RuneCountInString(key) == 1 {
			m = m.appendInput(key)
		}
	}
	return m.liveInput()
}

// appendInput adds typed or pasted text, sanitized and capped in runes.
func (m Model) appendInput(text string) Model {
	for _, r := range theme.Sanitize(text) {
		if utf8.RuneCountInString(m.input.value) >= maxInputLen {
			break
		}
		m.input.value += string(r)
	}
	return m
}

// liveInput previews a color as soon as the typed text is valid.
func (m Model) liveInput() Model {
	m.input.err = ""
	if m.input.purpose != inputColor {
		return m
	}
	if c, err := theme.ParseColor(strings.TrimSpace(m.input.value)); err == nil {
		return m.setTheme(m.input.original.WithOverride(m.picker.key, c))
	}
	return m.setTheme(m.input.original)
}

func (m Model) commitInput() Model {
	v := strings.TrimSpace(m.input.value)
	switch m.input.purpose {
	case inputColor:
		c, err := theme.ParseColor(v)
		if err != nil {
			m.input.err = err.Error()
			return m
		}
		m.mode = modeBrowse
		return m.setTheme(m.input.original.WithOverride(m.picker.key, c)).setStatus(m.picker.key+" = "+c.String(), false)
	case inputName:
		if v != "" {
			if err := theme.ValidateDisplayName(v); err != nil {
				m.input.err = err.Error()
				return m
			}
		}
		m.mode = modeBrowse
		return m.setTheme(m.theme.WithName(v)).setStatus("name updated", false)
	default:
		if err := theme.ValidateSlug(v); err != nil {
			m.input.err = err.Error()
			return m
		}
		m.mode = modeBrowse
		if v != m.slug {
			m.slug, m.unsaved, m.ownsDraft = v, true, false
		}
		return m.setStatus("file name: "+v+".json", false)
	}
}
