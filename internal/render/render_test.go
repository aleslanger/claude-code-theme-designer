package render

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

	"github.com/aleslanger/claude-code-theme-designer/internal/presets"
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

var update = flag.Bool("update", false, "rewrite golden files")

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file (run go test -update): %v", err)
	}
	if string(want) != got {
		t.Fatalf("%s differs from golden; run `go test ./internal/render -update` if intended\n--- got ---\n%s", name, got)
	}
}

// visible makes escape sequences reviewable in golden files.
func visible(s string) string { return strings.ReplaceAll(s, "\x1b", "⎋") }

func resolved(t *testing.T) map[string]theme.Color {
	t.Helper()
	reg, err := registry.Default()
	if err != nil {
		t.Fatal(err)
	}
	return reg.Resolve(presets.Default().Theme)
}

func TestPreviewPlainSnapshot(t *testing.T) {
	golden(t, "preview_plain.golden", Plain(Preview(Options{Width: 60})))
}

func TestPreviewTrueColorSnapshot(t *testing.T) {
	golden(t, "preview_truecolor.golden", visible(Encode(Preview(Options{Width: 60, Focus: "userMessageBackground"}), resolved(t), TrueColor)))
}

func TestPreview256Snapshot(t *testing.T) {
	golden(t, "preview_256.golden", visible(Encode(Preview(Options{Width: 60}), resolved(t), ANSI256)))
}

func TestPreviewShowsRequiredElements(t *testing.T) {
	out := Plain(Preview(Options{Width: 80}))
	for _, want := range []string{
		"Why can this code", "You", "- jobs are cancelled", "Claude", "```go",
		"Bash(go test", "⎿", "12 +", "Error: permission denied", "⚠ Context low",
		"(selected)", "Hovered", "! git status", "# Always", "╭",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("preview lacks %q", want)
		}
	}
}

func TestEveryLineFitsTheWidth(t *testing.T) {
	for _, w := range []int{MinWidth, 57, 120} {
		f := Preview(Options{Width: w, Focus: "text"})
		for i, line := range strings.Split(Plain(f), "\n") {
			if got := runewidth.StringWidth(line); got > w {
				t.Fatalf("width %d: line %d is %d cells: %q", w, i, got, line)
			}
		}
	}
}

func TestUserMessageRowsUseTheUserBackground(t *testing.T) {
	out := Encode(Preview(Options{Width: 60}), resolved(t), TrueColor)
	if !strings.Contains(out, "48;2;51;43;79") { // #332b4f
		t.Fatal("user message background not painted")
	}
}

func TestFocusMarksRowsUsingToken(t *testing.T) {
	f := Preview(Options{Width: 60, Focus: "briefLabelYou"})
	n := 0
	for _, l := range f.Lines {
		if l.Focus {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("expected exactly the You label row focused, got %d", n)
	}
}

var reSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestEncodeNeverEmitsUntrustedControlSequences(t *testing.T) {
	f := Frame{Width: 40, Lines: []Line{{Spans: []Span{span("evil\x1b]0;pwned\x07\x1b[2J\u009b31m", fg("text"))}}}}
	for _, p := range []Profile{NoColor, ANSI16, ANSI256, TrueColor} {
		out := reSGR.ReplaceAllString(Encode(f, resolved(t), p), "")
		if strings.ContainsAny(out, "\x1b\x07\u009b") {
			t.Fatalf("profile %v leaked control characters: %q", p, out)
		}
	}
}

func TestColorParamsPerProfile(t *testing.T) {
	hex := theme.MustParseColor("#ff0000")
	cases := []struct {
		c    theme.Color
		p    Profile
		bg   bool
		want string
	}{
		{hex, TrueColor, false, "38;2;255;0;0"},
		{hex, ANSI256, true, "48;5;196"},
		{hex, ANSI16, false, "91"},
		{theme.MustParseColor("ansi256(236)"), TrueColor, true, "48;5;236"},
		{theme.MustParseColor("ansi:blueBright"), TrueColor, false, "94"},
		{theme.MustParseColor("ansi:white"), ANSI256, true, "47"},
	}
	for _, tc := range cases {
		if got := strings.Join(colorParams(tc.c, tc.p, tc.bg), ";"); got != tc.want {
			t.Errorf("%s/%v/bg=%v = %s, want %s", tc.c, tc.p, tc.bg, got, tc.want)
		}
	}
}

func TestDetectProfile(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cases := []struct {
		env  map[string]string
		tty  bool
		want Profile
	}{
		{map[string]string{"COLORTERM": "truecolor", "TERM": "xterm"}, true, TrueColor},
		{map[string]string{"TERM": "xterm-256color"}, true, ANSI256},
		{map[string]string{"TERM": "xterm"}, true, ANSI16},
		{map[string]string{"TERM": "dumb", "COLORTERM": "truecolor"}, true, NoColor},
		{map[string]string{"NO_COLOR": "1", "COLORTERM": "truecolor"}, true, NoColor},
		{map[string]string{"COLORTERM": "truecolor"}, false, NoColor},
	}
	for _, tc := range cases {
		if got := DetectProfile(env(tc.env), tc.tty); got != tc.want {
			t.Errorf("%v tty=%v = %v, want %v", tc.env, tc.tty, got, tc.want)
		}
	}
}
