package presets

import (
	"slices"
	"testing"

	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/serialize"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
	"github.com/aleslanger/claude-code-theme-designer/internal/validate"
)

func TestPresetsAreValidVerifiedAndReadable(t *testing.T) {
	reg, err := registry.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range All() {
		t.Run(p.Slug, func(t *testing.T) {
			if err := theme.ValidateSlug(p.Slug); err != nil {
				t.Fatal(err)
			}
			for k := range p.Theme.Overrides {
				if tok, ok := reg.Lookup(k); !ok || tok.Status != registry.Verified {
					t.Errorf("preset uses non-verified token %s", k)
				}
			}
			report := validate.Check(p.Theme, reg, validate.Options{})
			for _, i := range report.Issues {
				if i.Severity >= validate.Warning {
					t.Errorf("%s: %s", i.Token, i.Message)
				}
			}
			for _, c := range validate.Contrasts(p.Theme, reg, validate.Options{}) {
				if c.FG == "text" && c.BG == "userMessageBackground" && c.Ratio < theme.ContrastAA {
					t.Errorf("user text contrast %.2f below AA", c.Ratio)
				}
			}
			data, err := serialize.Encode(p.Theme)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := serialize.Decode(data); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPresetsAreIndependentCopies(t *testing.T) {
	a := All()[0]
	a.Theme.Overrides["text"] = theme.MustParseColor("#000")
	if _, ok := All()[0].Theme.Override("text"); ok {
		t.Fatal("All must return fresh values")
	}
}

func TestFindAndDefault(t *testing.T) {
	if p, ok := Find("nord-like"); !ok || p.Theme.Name != "Nord-like" {
		t.Fatal("Find must return the preset")
	}
	if _, ok := Find("missing"); ok {
		t.Fatal("unknown preset must not be found")
	}
	if Default().Slug != "prompt-contrast" {
		t.Fatal("default preset changed unexpectedly")
	}
}

func TestPresetSlugsUniqueAndCategorised(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range All() {
		if seen[p.Slug] {
			t.Errorf("duplicate slug %s", p.Slug)
		}
		seen[p.Slug] = true
		if !slices.Contains(Categories, p.Category) {
			t.Errorf("%s has unknown category %q", p.Slug, p.Category)
		}
		if p.Description == "" || p.Theme.Name == "" {
			t.Errorf("%s lacks a name or description", p.Slug)
		}
	}
	// Display order must follow Categories so the TUI cursor matches the list.
	last := 0
	for _, p := range All() {
		i := slices.Index(Categories, p.Category)
		if i < last {
			t.Fatalf("%s is out of category order", p.Slug)
		}
		last = i
	}
}
