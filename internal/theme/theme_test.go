package theme

import "testing"

func TestWithMethodsDoNotMutateOriginal(t *testing.T) {
	orig := Theme{Name: "a", Overrides: map[string]Color{"text": MustParseColor("#fff")}}

	changed := orig.WithOverride("claude", MustParseColor("#000")).WithoutOverride("text").WithBase(BaseLight).WithName("b")

	if _, ok := orig.Override("claude"); ok {
		t.Fatal("WithOverride mutated the original")
	}
	if _, ok := orig.Override("text"); !ok {
		t.Fatal("WithoutOverride mutated the original")
	}
	if orig.Base != "" || orig.Name != "a" {
		t.Fatal("WithBase/WithName mutated the original")
	}
	if _, ok := changed.Override("text"); ok || changed.Base != BaseLight || changed.Name != "b" {
		t.Fatalf("unexpected result %+v", changed)
	}
}

func TestEffectiveBaseDefaultsToDark(t *testing.T) {
	if (Theme{}).EffectiveBase() != BaseDark {
		t.Fatal("absent base must behave as dark, like Claude Code")
	}
}

func TestEqual(t *testing.T) {
	a := Theme{Base: BaseDark, Overrides: map[string]Color{"text": MustParseColor("#fff")}}
	if !a.Equal(a.WithOverride("text", MustParseColor("#fff"))) {
		t.Fatal("identical themes must be equal")
	}
	if a.Equal(a.WithOverride("text", MustParseColor("#ffffff"))) {
		t.Fatal("different spelling must not be equal")
	}
}
