// Package presets provides conservative starter themes. Palettes inspired by
// well-known schemes are labelled "-like": they are approximations, not the
// official schemes and not official Claude Code themes.
package presets

import (
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

// Category groups presets in listings.
type Category string

const (
	// CategoryOriginal presets are designed for this tool.
	CategoryOriginal Category = "Original"
	// CategoryInspired presets approximate well-known palettes ("-like").
	CategoryInspired Category = "Inspired by popular palettes (unofficial)"
	// CategoryAccessibility presets prioritise legibility.
	CategoryAccessibility Category = "Accessibility"
)

// Categories lists categories in display order.
var Categories = []Category{CategoryOriginal, CategoryInspired, CategoryAccessibility}

// Preset is a named starting point.
type Preset struct {
	Slug        string
	Category    Category
	Description string
	Theme       theme.Theme
}

func build(name string, base theme.Base, overrides map[string]string) theme.Theme {
	t := theme.Theme{Name: name, Base: base, Overrides: make(map[string]theme.Color, len(overrides))}
	for k, v := range overrides {
		t.Overrides[k] = theme.MustParseColor(v)
	}
	return t
}

// All returns every preset in display order. Each call returns fresh values.
func All() []Preset {
	var out []Preset
	for _, group := range [][]Preset{originals(), inspired(), accessible()} {
		out = append(out, group...)
	}
	return out
}

// Find returns the preset with the given slug.
func Find(slug string) (Preset, bool) {
	for _, p := range All() {
		if p.Slug == slug {
			return p, true
		}
	}
	return Preset{}, false
}

// Default is the preset used for a brand-new theme.
func Default() Preset {
	p, _ := Find("prompt-contrast")
	return p
}
