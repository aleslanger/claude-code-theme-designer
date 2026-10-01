package validate

import (
	"testing"

	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

func reg(t *testing.T) *registry.Registry {
	t.Helper()
	r, err := registry.Default()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func with(pairs ...string) theme.Theme {
	th := theme.Theme{Base: theme.BaseDark, Overrides: map[string]theme.Color{}}
	for i := 0; i+1 < len(pairs); i += 2 {
		th = th.WithOverride(pairs[i], theme.MustParseColor(pairs[i+1]))
	}
	return th
}

func codes(r Report) map[string]Severity {
	out := map[string]Severity{}
	for _, i := range r.Issues {
		out[i.Code+":"+i.Token] = i.Severity
	}
	return out
}

func TestCleanThemeHasNoWarnings(t *testing.T) {
	r := Check(with("userMessageBackground", "#332b4f", "briefLabelYou", "#c4a7ff"), reg(t), Options{})
	if r.Warnings() != 0 || r.HasErrors() {
		t.Fatalf("unexpected issues: %+v", r.Issues)
	}
}

func TestUnknownAndUnverifiedTokensWarnButDoNotBlock(t *testing.T) {
	r := Check(with("tokenFromTheFuture", "#123456", "clawd_body", "#ff0000"), reg(t), Options{})
	c := codes(r)
	if c[CodeUnknownToken+":tokenFromTheFuture"] != Warning {
		t.Fatalf("missing unknown-token warning: %+v", r.Issues)
	}
	if c[CodeUnverifiedToken+":clawd_body"] != Warning {
		t.Fatalf("missing unverified-token warning: %+v", r.Issues)
	}
	if r.HasErrors() {
		t.Fatal("unknown tokens must not block")
	}
}

func TestFullscreenOnlyTokenIsNoted(t *testing.T) {
	r := Check(with("userMessageBackgroundHover", "#403660"), reg(t), Options{})
	if codes(r)[CodeFullscreenOnly+":userMessageBackgroundHover"] != Info {
		t.Fatalf("missing fullscreen note: %+v", r.Issues)
	}
}

func TestLowContrastWarns(t *testing.T) {
	r := Check(with("userMessageBackground", "#777777"), reg(t), Options{})
	if codes(r)[CodeContrastLow+":text"] != Warning {
		t.Fatalf("expected low contrast warning, got %+v", r.Issues)
	}
	if r.HasErrors() {
		t.Fatal("merely low contrast must not block")
	}
}

func TestUnreadableContrastIsAnError(t *testing.T) {
	r := Check(with("userMessageBackground", "#fefefe"), reg(t), Options{})
	if codes(r)[CodeContrastUnreadable+":text"] != Error {
		t.Fatalf("white text on white must be an error, got %+v", r.Issues)
	}
}

func TestApproximateColorsNeverBlock(t *testing.T) {
	th := with("userMessageBackground", "ansi:white").WithOverride("text", theme.MustParseColor("ansi:whiteBright"))
	if Check(th, reg(t), Options{}).HasErrors() {
		t.Fatal("contrast of ANSI named colors is a guess and must not block")
	}
}

func TestTerminalBackgroundOptionIsUsed(t *testing.T) {
	bg := theme.MustParseColor("#000000")
	th := with("error", "#050505")
	r := Check(th, reg(t), Options{TerminalBackground: &bg})
	if codes(r)[CodeContrastUnreadable+":error"] != Error {
		t.Fatalf("configured terminal background must be used: %+v", r.Issues)
	}
	assumed := Check(th, reg(t), Options{})
	if assumed.HasErrors() {
		t.Fatal("an assumed terminal background must not block")
	}
}

func TestUntouchedDefaultsAreNotReported(t *testing.T) {
	r := Check(theme.Theme{Base: theme.BaseLightANSI}, reg(t), Options{})
	if len(r.Issues) != 0 {
		t.Fatalf("Claude Code's own defaults must not be reported: %+v", r.Issues)
	}
}
