package tui

import (
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
)

type rowKind int

const (
	rowHeader rowKind = iota
	rowName
	rowSlug
	rowBase
	rowToken
	rowUnsupported
)

// row is one line of the editor list.
type row struct {
	kind    rowKind
	label   string
	key     string // token key for rowToken
	section int    // index into sections; -1 for the Theme block
	reason  string // for rowUnsupported
}

func (r row) selectable() bool { return r.kind != rowHeader }

// buildRows lays out the editor: theme metadata, then registry sections.
// Only verified tokens appear; the registry decides which those are.
func buildRows(reg *registry.Registry) ([]row, []registry.Section) {
	sections := reg.Sections()
	rows := []row{
		{kind: rowHeader, label: "Theme", section: -1},
		{kind: rowName, label: "Name", section: -1},
		{kind: rowSlug, label: "File name", section: -1},
		{kind: rowBase, label: "Base", section: -1},
	}
	for i, s := range sections {
		rows = append(rows, row{kind: rowHeader, label: s.Title, section: i})
		for _, k := range s.Keys {
			rows = append(rows, row{kind: rowToken, label: s.Label(k, reg), key: k, section: i})
		}
		for _, u := range s.Unsupported {
			rows = append(rows, row{kind: rowUnsupported, label: u.Label, reason: u.Reason, section: i})
		}
	}
	return rows, sections
}
