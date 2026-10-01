// Package tui is the fullscreen editor. It holds only UI state and key
// handling; all theme logic lives in app, validate, serialize and render.
// Edits stay in memory: the disk is touched only on explicit save/install.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/aleslanger/claude-code-theme-designer/internal/app"
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/render"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
	"github.com/aleslanger/claude-code-theme-designer/internal/validate"
)

type mode int

const (
	modeBrowse mode = iota
	modePicker
	modeInput
	modeConfirm
	modePresets
	modeHelp
)

type inputPurpose int

const (
	inputColor inputPurpose = iota
	inputName
	inputSlug
)

// Model is the Bubble Tea model.
type Model struct {
	app     *app.App
	profile render.Profile

	slug     string
	source   app.Source
	theme    theme.Theme // what is being edited
	baseline theme.Theme // what "reset" returns to
	saved    theme.Theme // last state written to disk, for the dirty flag
	// unsaved forces the dirty flag, e.g. after renaming the file.
	unsaved bool
	// ownsDraft is true when the draft under slug is the one being edited, so
	// saving over it needs no confirmation.
	ownsDraft bool

	rows     []row
	sections []registry.Section
	cursor   int
	scroll   int

	width, height int
	mode          mode

	picker  pickerState
	input   inputState
	confirm confirmState
	presets presetState

	status    string
	statusBad bool
	report    validate.Report
}

type pickerState struct {
	index       int
	key         string
	original    theme.Color
	hadOriginal bool
}

type inputState struct {
	purpose  inputPurpose
	value    string
	err      string
	original theme.Theme // restored on Esc
}

type confirmState struct {
	question string
	onYes    func(Model) (Model, tea.Cmd)
}

type presetState struct{ cursor int }

// New creates a model editing the given loaded theme.
func New(a *app.App, l app.Loaded, profile render.Profile) Model {
	rows, sections := buildRows(a.Reg)
	m := Model{
		app: a, profile: profile,
		slug: l.Slug, source: l.Source,
		theme: l.Theme, baseline: l.Theme, saved: l.Theme,
		rows: rows, sections: sections,
		width: minWidth, height: minHeight,
		ownsDraft: l.Source == app.SourceDraft,
	}
	m.cursor = m.firstSelectable()
	return m.revalidate()
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Theme returns the theme being edited (tests).
func (m Model) Theme() theme.Theme { return m.theme }

func (m Model) dirty() bool { return m.unsaved || !m.theme.Equal(m.saved) }

// setTheme replaces the edited theme and refreshes validation. Pure: no I/O.
func (m Model) setTheme(t theme.Theme) Model {
	m.theme = t
	return m.revalidate()
}

func (m Model) revalidate() Model {
	m.report = m.app.Check(m.theme)
	return m
}

func (m Model) setStatus(msg string, bad bool) Model {
	m.status, m.statusBad = theme.Sanitize(msg), bad
	return m
}

func (m Model) currentRow() row { return m.rows[m.cursor] }

func (m Model) firstSelectable() int {
	for i, r := range m.rows {
		if r.selectable() {
			return i
		}
	}
	return 0
}

// resolvedColor returns the effective color of key and whether it is overridden.
func (m Model) resolvedColor(key string) (theme.Color, bool) {
	if c, ok := m.theme.Override(key); ok {
		return c, true
	}
	c, _ := m.app.Reg.BaseDefault(m.theme.EffectiveBase(), key)
	return c, false
}
