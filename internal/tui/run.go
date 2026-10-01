package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aleslanger/claude-code-theme-designer/internal/app"
	"github.com/aleslanger/claude-code-theme-designer/internal/render"
)

// Run opens the fullscreen editor on slug (draft, preset or installed theme).
func Run(a *app.App, slug string, profile render.Profile) error {
	l, err := a.Resolve(slug)
	if err != nil {
		return err
	}
	p := tea.NewProgram(New(a, l, profile), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("terminal UI: %w", err)
	}
	return nil
}
