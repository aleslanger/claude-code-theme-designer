package theme

import (
	"maps"
	"slices"
)

// Base is one of Claude Code's built-in presets a custom theme starts from.
type Base string

const (
	BaseDark            Base = "dark"
	BaseLight           Base = "light"
	BaseDarkDaltonized  Base = "dark-daltonized"
	BaseLightDaltonized Base = "light-daltonized"
	BaseDarkANSI        Base = "dark-ansi"
	BaseLightANSI       Base = "light-ansi"
)

// Bases lists every documented base preset, in the order /theme shows them.
var Bases = []Base{BaseDark, BaseLight, BaseDarkDaltonized, BaseLightDaltonized, BaseDarkANSI, BaseLightANSI}

// DefaultBase is what Claude Code uses when "base" is absent.
const DefaultBase = BaseDark

// IsValidBase reports whether b is a documented base preset.
func IsValidBase(b Base) bool { return slices.Contains(Bases, b) }

// IsLight reports whether the base is one of the light presets.
func (b Base) IsLight() bool {
	return b == BaseLight || b == BaseLightDaltonized || b == BaseLightANSI
}

// Theme is the in-memory form of a Claude Code theme file.
//
// Absent fields are preserved as absent: Name == "" means the file has no
// "name", Base == "" means no "base", and a token missing from Overrides is
// inherited from the base preset. Overrides may contain tokens this tool does
// not know (passthrough); they are kept untouched.
//
// Theme is treated as immutable: the With* methods return modified copies.
type Theme struct {
	Name      string
	Base      Base
	Overrides map[string]Color
}

// EffectiveBase returns the base Claude Code will actually use.
func (t Theme) EffectiveBase() Base {
	if t.Base == "" {
		return DefaultBase
	}
	return t.Base
}

// Override returns the override for key and whether one is set.
func (t Theme) Override(key string) (Color, bool) {
	c, ok := t.Overrides[key]
	return c, ok
}

// SortedKeys returns override keys in stable order.
func (t Theme) SortedKeys() []string {
	return slices.Sorted(maps.Keys(t.Overrides))
}

// WithOverride returns a copy of t with key set to c.
func (t Theme) WithOverride(key string, c Color) Theme {
	next := t.clone()
	next.Overrides[key] = c
	return next
}

// WithoutOverride returns a copy of t where key inherits from the base.
func (t Theme) WithoutOverride(key string) Theme {
	next := t.clone()
	delete(next.Overrides, key)
	return next
}

// WithBase returns a copy of t with a different base preset.
func (t Theme) WithBase(b Base) Theme {
	next := t.clone()
	next.Base = b
	return next
}

// WithName returns a copy of t with a different display name.
func (t Theme) WithName(name string) Theme {
	next := t.clone()
	next.Name = name
	return next
}

// Equal reports whether two themes serialize identically.
func (t Theme) Equal(o Theme) bool {
	if t.Name != o.Name || t.Base != o.Base || len(t.Overrides) != len(o.Overrides) {
		return false
	}
	for k, c := range t.Overrides {
		if oc, ok := o.Overrides[k]; !ok || !oc.Equal(c) {
			return false
		}
	}
	return true
}

func (t Theme) clone() Theme {
	next := t
	next.Overrides = make(map[string]Color, len(t.Overrides)+1)
	maps.Copy(next.Overrides, t.Overrides)
	return next
}
