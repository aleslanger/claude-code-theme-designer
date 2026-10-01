package main

import "testing"

// fixture mimics the structure of the docs' token reference (synthetic text).
const fixture = "intro\n<Accordion title=\"Color token reference\">\n" +
	"  #### Text\n\n  | Token | Controls |\n  | :- | :- |\n" +
	"  | `text` | Default foreground text |\n" +
	"  | `effortUltra` | The tag. Your override takes effect on Claude Code v2.1.239 or later |\n" +
	"  #### Fullscreen mode\n\n  It uses `userMessageBackgroundHover` only in [fullscreen](/x).\n\n" +
	"  | `userMessageBackgroundHover` | Background behind a message while hovered |\n" +
	"  #### Shimmer variants and subagent colors\n\n  * `claude` and `claudeShimmer`\n\n" +
	"  The token names follow the pattern `<color>_FOR_SUBAGENTS_ONLY`, where `<color>` is `red` or `blue`. More.\n" +
	"  The token names follow the pattern `rainbow_<color>` and `rainbow_<color>_shimmer`, where `<color>` is `red`.\n" +
	"</Accordion>\n| `outside` | must be ignored |\n"

func TestParseDocs(t *testing.T) {
	toks, err := ParseDocs(fixture)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]DocToken{}
	for _, tk := range toks {
		got[tk.Key] = tk
	}
	for _, k := range []string{"text", "effortUltra", "userMessageBackgroundHover", "claudeShimmer", "red_FOR_SUBAGENTS_ONLY", "blue_FOR_SUBAGENTS_ONLY", "rainbow_red", "rainbow_red_shimmer"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
	if _, ok := got["outside"]; ok {
		t.Error("tokens outside the accordion must be ignored")
	}
	if got["effortUltra"].SinceVersion != "2.1.239" {
		t.Error("since version not parsed")
	}
	if !got["userMessageBackgroundHover"].FullscreenOnly {
		t.Error("fullscreen flag not parsed")
	}
}

func TestParseDocsFailsLoudlyOnFormatChange(t *testing.T) {
	if _, err := ParseDocs("no accordion here"); err == nil {
		t.Fatal("must fail when the docs format changes")
	}
}

func TestBuildMarksUndocumentedPaletteKeysUnverified(t *testing.T) {
	pal := paletteFile{CapturedFrom: "CC 1.0.0", Palettes: map[string]map[string]string{"dark": {"text": "#fff", "secret": "#000"}}}
	reg, missing := Build([]DocToken{{Key: "text", Category: "Text", Description: "d"}, {Key: "gone", Category: "Text", Description: "d"}}, pal, "url", "2026-01-01")
	status := map[string]string{}
	for _, tk := range reg.Tokens {
		status[tk.Key] = tk.Status
	}
	if status["text"] != "verified" || status["secret"] != "unverified" {
		t.Fatalf("got %v", status)
	}
	if len(missing) != 1 || missing[0] != "gone" {
		t.Fatalf("missing = %v", missing)
	}
}

func TestDisplayName(t *testing.T) {
	cases := map[string]string{"userMessageBackground": "User message background", "rate_limit_fill": "Rate limit fill", "red_FOR_SUBAGENTS_ONLY": "Subagent red"}
	for in, want := range cases {
		if got := DisplayName(in); got != want {
			t.Errorf("%s = %q, want %q", in, got, want)
		}
	}
}
