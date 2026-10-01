package theme

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateSlugAcceptsSafeNames(t *testing.T) {
	for _, s := range []string{"prompt-contrast", "A", "my_theme_2", strings.Repeat("a", MaxSlugLen)} {
		if err := ValidateSlug(s); err != nil {
			t.Errorf("ValidateSlug(%q) = %v", s, err)
		}
	}
}

func TestValidateSlugRejectsTraversalAndOddNames(t *testing.T) {
	bad := []string{
		"", "../../something", "..", ".", "a/b", `a\b`, "a.json", ".hidden",
		"with space", "tab\t", "nul\x00", "esc\x1b[31m", "ünicode", "a:b",
		strings.Repeat("a", MaxSlugLen+1), "/etc/passwd", "~root",
	}
	for _, s := range bad {
		if err := ValidateSlug(s); !errors.Is(err, ErrInvalidSlug) {
			t.Errorf("ValidateSlug(%q) = %v, want ErrInvalidSlug", s, err)
		}
	}
}

func TestValidateSlugErrorDoesNotEchoControlCharacters(t *testing.T) {
	err := ValidateSlug("x\x1b]0;pwned\x07")
	if err == nil || strings.ContainsAny(err.Error(), "\x1b\x07") {
		t.Fatalf("error must be sanitized, got %q", err)
	}
}

func TestValidateDisplayName(t *testing.T) {
	for _, ok := range []string{"Prompt Contrast", "Nord-like ❄", "Tmavé téma"} {
		if err := ValidateDisplayName(ok); err != nil {
			t.Errorf("ValidateDisplayName(%q) = %v", ok, err)
		}
	}
	bad := []string{
		"evil\x1b[2J", "bell\x07", "newline\n", "del\x7f", "c1\u009b31m",
		"bidi‮gnp.exe", "iso⁦x", "sep ", "bad\xff",
		strings.Repeat("x", MaxDisplayNameLen+1),
	}
	for _, s := range bad {
		if err := ValidateDisplayName(s); !errors.Is(err, ErrInvalidName) {
			t.Errorf("ValidateDisplayName(%q) = %v, want ErrInvalidName", s, err)
		}
	}
}

func TestSanitizeNeutralizesTerminalControl(t *testing.T) {
	got := Sanitize("a\x1b[31mb\u009bc‮d\xffe")
	if strings.ContainsAny(got, "\x1b\u009b‮") {
		t.Fatalf("Sanitize left control characters: %q", got)
	}
	if !strings.HasPrefix(got, "a") || !strings.HasSuffix(got, "e") {
		t.Fatalf("Sanitize dropped printable text: %q", got)
	}
}
