package theme

import (
	"errors"
	"fmt"
	"regexp"
	"unicode"
	"unicode/utf8"
)

// MaxSlugLen and MaxDisplayNameLen bound user-chosen identifiers.
const (
	MaxSlugLen        = 64
	MaxDisplayNameLen = 64
)

// reSlug is the only accepted shape of a theme slug (the file name without .json).
// It contains no path separators, dots or whitespace, so it cannot traverse paths.
var reSlug = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

var (
	ErrInvalidSlug = errors.New("invalid theme name")
	ErrInvalidName = errors.New("invalid display name")
)

// ValidateSlug checks a theme slug. Allowed: [A-Za-z0-9_-], 1-64 characters.
func ValidateSlug(s string) error {
	if !reSlug.MatchString(s) {
		return fmt.Errorf("%w %q: use 1-%d characters from A-Z a-z 0-9 _ -", ErrInvalidSlug, Sanitize(s), MaxSlugLen)
	}
	return nil
}

// ValidateDisplayName checks the optional human-readable "name" field.
// Claude Code shows it in /theme, so control characters (including ESC, which
// would allow ANSI injection), bidi overrides and invalid UTF-8 are rejected.
func ValidateDisplayName(s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%w: not valid UTF-8", ErrInvalidName)
	}
	if utf8.RuneCountInString(s) > MaxDisplayNameLen {
		return fmt.Errorf("%w: longer than %d characters", ErrInvalidName, MaxDisplayNameLen)
	}
	for _, r := range s {
		if isUnsafeRune(r) {
			return fmt.Errorf("%w: contains control or invisible character U+%04X", ErrInvalidName, r)
		}
	}
	return nil
}

// isUnsafeRune reports runes that must never reach a terminal verbatim:
// C0/C1 controls and DEL (Cc), invisible format characters (Cf: bidi marks,
// overrides and isolates, zero-width characters, BOM, tag characters),
// private-use (Co) and surrogate (Cs) code points, and line/paragraph
// separators. These enable escape injection or display spoofing.
func isUnsafeRune(r rune) bool {
	switch {
	case r == utf8.RuneError:
		return true
	case unicode.IsControl(r), unicode.In(r, unicode.Cf, unicode.Co, unicode.Cs):
		return true
	case r == 0x2028 || r == 0x2029:
		return true
	}
	return false
}

// Sanitize makes an arbitrary string safe to print to a terminal by replacing
// every unsafe rune (see isUnsafeRune) and invalid UTF-8 byte with U+FFFD.
// All untrusted text shown by this tool passes through here.
func Sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s { // ranging over invalid UTF-8 yields RuneError
		if isUnsafeRune(r) {
			out = append(out, '�')
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
