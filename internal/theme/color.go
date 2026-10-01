// Package theme is the pure domain model of a Claude Code custom theme:
// colors, names, the theme value itself and contrast math. It performs no I/O.
package theme

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ColorKind identifies which of the documented Claude Code color syntaxes a value uses.
type ColorKind int

const (
	KindHex ColorKind = iota
	KindRGB
	KindANSI256
	KindANSINamed
)

// MaxColorLen bounds the length of a color string. The longest valid form is
// "rgb(255, 255, 255)" (18 bytes); anything much longer is not a color.
const MaxColorLen = 32

// ansiNamedPrefix is the documented prefix for the 16 standard ANSI colors.
const ansiNamedPrefix = "ansi:"

// ANSINames lists the 16 color names Claude Code accepts after "ansi:", in SGR order
// (index 0-7 normal, 8-15 bright).
var ANSINames = []string{
	"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
	"blackBright", "redBright", "greenBright", "yellowBright",
	"blueBright", "magentaBright", "cyanBright", "whiteBright",
}

// The accepted grammar is a strict subset of what Claude Code accepts
// (docs: #rrggbb, #rgb, rgb(r,g,b), ansi256(n), ansi:<name>). Being a subset
// guarantees every value this tool writes is honoured by Claude Code.
// Whitespace inside rgb() is limited to a single ASCII space and components
// must be <= 255 (Claude Code's own regex would accept 999).
var (
	reHex6    = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	reHex3    = regexp.MustCompile(`^#[0-9a-fA-F]{3}$`)
	reRGB     = regexp.MustCompile(`^rgb\( ?(\d{1,3}), ?(\d{1,3}), ?(\d{1,3}) ?\)$`)
	reANSI256 = regexp.MustCompile(`^ansi256\((\d{1,3})\)$`)
)

// ErrInvalidColor is wrapped by every color parse failure.
var ErrInvalidColor = errors.New("invalid color")

// RGB is an 8-bit-per-channel color.
type RGB struct{ R, G, B uint8 }

// Hex formats the color as lowercase #rrggbb.
func (c RGB) Hex() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// Color is a validated Claude Code color value. The zero value is not valid;
// obtain colors through ParseColor or the constructors.
type Color struct {
	kind  ColorKind
	raw   string // exact accepted spelling, preserved for round-trips
	rgb   RGB    // exact for hex/rgb/ansi256, approximate for named ANSI
	index uint8  // ansi256 index, or 0-15 for named ANSI
}

// ParseColor validates s against the supported color grammar.
func ParseColor(s string) (Color, error) {
	if len(s) == 0 || len(s) > MaxColorLen {
		return Color{}, fmt.Errorf("%w: length must be 1-%d bytes", ErrInvalidColor, MaxColorLen)
	}
	switch {
	case reHex6.MatchString(s):
		return Color{kind: KindHex, raw: s, rgb: hexRGB(s[1:])}, nil
	case reHex3.MatchString(s):
		expanded := string([]byte{s[1], s[1], s[2], s[2], s[3], s[3]})
		return Color{kind: KindHex, raw: s, rgb: hexRGB(expanded)}, nil
	case strings.HasPrefix(s, "rgb("):
		return parseRGBFunc(s)
	case strings.HasPrefix(s, "ansi256("):
		return parseANSI256(s)
	case strings.HasPrefix(s, ansiNamedPrefix):
		return parseANSINamed(s)
	}
	return Color{}, fmt.Errorf("%w: expected #rrggbb, #rgb, rgb(r,g,b), ansi256(n) or ansi:<name>", ErrInvalidColor)
}

// MustParseColor is ParseColor for compile-time constants; it panics on invalid input.
func MustParseColor(s string) Color {
	c, err := ParseColor(s)
	if err != nil {
		panic(err)
	}
	return c
}

// FromRGB builds a #rrggbb color.
func FromRGB(c RGB) Color {
	return Color{kind: KindHex, raw: c.Hex(), rgb: c}
}

// FromANSI256 builds an ansi256(n) color.
func FromANSI256(n uint8) Color {
	return Color{kind: KindANSI256, raw: fmt.Sprintf("ansi256(%d)", n), rgb: XtermRGB(n), index: n}
}

func parseRGBFunc(s string) (Color, error) {
	m := reRGB.FindStringSubmatch(s)
	if m == nil {
		return Color{}, fmt.Errorf("%w: malformed rgb()", ErrInvalidColor)
	}
	var ch [3]uint8
	for i := range ch {
		v, err := byteComponent(m[i+1])
		if err != nil {
			return Color{}, err
		}
		ch[i] = v
	}
	return Color{kind: KindRGB, raw: s, rgb: RGB{ch[0], ch[1], ch[2]}}, nil
}

func parseANSI256(s string) (Color, error) {
	m := reANSI256.FindStringSubmatch(s)
	if m == nil {
		return Color{}, fmt.Errorf("%w: malformed ansi256()", ErrInvalidColor)
	}
	n, err := byteComponent(m[1])
	if err != nil {
		return Color{}, err
	}
	return Color{kind: KindANSI256, raw: s, rgb: XtermRGB(n), index: n}, nil
}

func parseANSINamed(s string) (Color, error) {
	name := s[len(ansiNamedPrefix):]
	for i, n := range ANSINames {
		if n == name {
			return Color{kind: KindANSINamed, raw: s, rgb: XtermRGB(uint8(i)), index: uint8(i)}, nil
		}
	}
	return Color{}, fmt.Errorf("%w: unknown ANSI color name (expected one of %s)", ErrInvalidColor, strings.Join(ANSINames, ", "))
}

func byteComponent(digits string) (uint8, error) {
	v, err := strconv.Atoi(digits)
	if err != nil || v > 255 {
		return 0, fmt.Errorf("%w: component out of range 0-255", ErrInvalidColor)
	}
	return uint8(v), nil
}

func hexRGB(h string) RGB {
	v, _ := strconv.ParseUint(h, 16, 32) // input already matched [0-9a-fA-F]{6}
	return RGB{uint8(v >> 16), uint8(v >> 8), uint8(v)}
}

// IsZero reports whether c is the zero (unset) Color.
func (c Color) IsZero() bool { return c.raw == "" }

// Kind returns the syntax family of the color.
func (c Color) Kind() ColorKind { return c.kind }

// String returns the exact accepted spelling, suitable for writing to a theme file.
func (c Color) String() string { return c.raw }

// RGB returns the color as RGB. For named ANSI colors this is the xterm default
// and only an approximation, because the real value depends on the terminal palette.
func (c Color) RGB() RGB { return c.rgb }

// Index returns the palette index for ansi256 (0-255) and named ANSI (0-15) colors.
func (c Color) Index() uint8 { return c.index }

// IsApproximate reports whether RGB() is a guess rather than the exact color.
func (c Color) IsApproximate() bool { return c.kind == KindANSINamed }

// Equal compares the exact spelling, so "#fff" and "#ffffff" differ.
// That is intentional: round-trips must not rewrite the user's values.
func (c Color) Equal(o Color) bool { return c.raw == o.raw }
