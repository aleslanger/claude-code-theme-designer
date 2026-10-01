package tui

import (
	"bytes"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aleslanger/claude-code-theme-designer/internal/app"
	"github.com/aleslanger/claude-code-theme-designer/internal/serialize"
	"github.com/aleslanger/claude-code-theme-designer/internal/store"
)

// save writes the current theme as a draft (explicit user action only). It
// asks before replacing a draft that is not the one being edited.
func (m Model) save() Model {
	if !m.ownsDraft && m.app.Store.DraftExists(m.slug) {
		return m.ask("A different draft named "+m.slug+" exists. Replace it? (a backup is kept)",
			func(m Model) (Model, tea.Cmd) { return m.doSave(), nil })
	}
	return m.doSave()
}

func (m Model) doSave() Model {
	path, backup, err := m.app.SaveDraft(m.slug, m.theme)
	if err != nil {
		return m.setStatus("save failed: "+err.Error(), true)
	}
	m.saved, m.source, m.unsaved, m.ownsDraft = m.theme, app.SourceDraft, false, true
	msg := "saved draft " + path
	if backup != "" {
		msg += " (previous draft backed up to " + backup + ")"
	}
	return m.withStoreWarnings(msg)
}

// withStoreWarnings appends non-fatal store problems to a success message.
func (m Model) withStoreWarnings(msg string) Model {
	warnings := m.app.Store.TakeWarnings()
	for _, w := range warnings {
		msg += "; warning: " + w.Error()
	}
	return m.setStatus(msg, len(warnings) > 0)
}

// install asks for confirmation when the theme has errors or would replace a
// different file, then installs with a backup.
func (m Model) install() Model {
	var reasons []string
	if n := m.report.Errors(); n > 0 {
		reasons = append(reasons, fmt.Sprintf("it has %d validation error(s)", n))
	}
	if m.wouldReplace() {
		reasons = append(reasons, m.slug+".json already exists (a backup will be kept)")
	}
	if len(reasons) == 0 {
		return m.doInstall(false)
	}
	return m.ask("Install anyway? "+strings.Join(reasons, "; ")+".", func(m Model) (Model, tea.Cmd) { return m.doInstall(true), nil })
}

func (m Model) wouldReplace() bool {
	data, err := serialize.Encode(m.theme)
	if err != nil {
		return false
	}
	existing, err := m.app.Store.ReadInstalled(m.slug)
	return err == nil && !bytes.Equal(existing, data)
}

func (m Model) doInstall(force bool) Model {
	// The confirmation already listed every reason, so one answer covers both
	// replacing the file and accepting validation errors.
	out, err := m.app.Install(m.slug, m.theme, store.InstallOptions{Force: force}, force)
	if err != nil {
		return m.setStatus("install failed: "+err.Error(), true)
	}
	msg := "installed " + out.Path + " — select it in Claude Code with /theme"
	if out.Unchanged {
		msg = "already installed (identical): " + out.Path
	}
	if out.CreatedDir {
		msg += " (theme folder was created: restart Claude Code once)"
	}
	if out.ConfigErr != nil {
		msg += "; warning: " + out.ConfigErr.Error()
	}
	m = m.withStoreWarnings(msg)
	m.statusBad = m.statusBad || out.ConfigErr != nil
	return m
}
