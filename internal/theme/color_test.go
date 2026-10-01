package theme

import (
	"errors"
	"testing"
)

func TestParseColorAcceptsDocumentedFormats(t *testing.T) {
	cases := []struct {
		in   string
		kind ColorKind
		rgb  RGB
	}{
		{"#332b4f", KindHex, RGB{0x33, 0x2b, 0x4f}},
		{"#ABCDEF", KindHex, RGB{0xab, 0xcd, 0xef}},
		{"#abc", KindHex, RGB{0xaa, 0xbb, 0xcc}},
		{"rgb(1,2,3)", KindRGB, RGB{1, 2, 3}},
		{"rgb(55, 55, 55)", KindRGB, RGB{55, 55, 55}},
		{"rgb( 0, 0, 255 )", KindRGB, RGB{0, 0, 255}},
		{"ansi256(0)", KindANSI256, RGB{0, 0, 0}},
		{"ansi256(196)", KindANSI256, RGB{255, 0, 0}},
		{"ansi256(255)", KindANSI256, RGB{238, 238, 238}},
		{"ansi:red", KindANSINamed, RGB{205, 0, 0}},
		{"ansi:cyanBright", KindANSINamed, RGB{0, 255, 255}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			c, err := ParseColor(tc.in)
			if err != nil {
				t.Fatalf("ParseColor(%q) error: %v", tc.in, err)
			}
			if c.Kind() != tc.kind || c.RGB() != tc.rgb || c.String() != tc.in {
				t.Fatalf("got kind=%v rgb=%v str=%q", c.Kind(), c.RGB(), c.String())
			}
		})
	}
}

func TestParseColorRejectsInvalidValues(t *testing.T) {
	invalid := []string{
		"", "#", "#12", "#1234", "#12345", "#1234567", "#ggg", "332b4f",
		"rgb(256,0,0)", "rgb(999,0,0)", "rgb(1,2)", "rgb(1,2,3,4)", "rgb(-1,0,0)",
		"rgb(1,  2,3)", "rgb(\t1,2,3)", "rgb(1,2,3)\n", "RGB(1,2,3)",
		"ansi256(256)", "ansi256(-1)", "ansi256()", "ansi256(1000)",
		"ansi:", "ansi:Red", "ansi:orange", "ansi:red\x1b[31m",
		"\x1b[31m", "red", "inherit", "#fff ", " #fff",
		"#ffffff‮",
	}
	for _, in := range invalid {
		if _, err := ParseColor(in); !errors.Is(err, ErrInvalidColor) {
			t.Errorf("ParseColor(%q) = %v, want ErrInvalidColor", in, err)
		}
	}
}

func TestParseColorRejectsOverlongInput(t *testing.T) {
	long := "#" + string(make([]byte, MaxColorLen))
	if _, err := ParseColor(long); !errors.Is(err, ErrInvalidColor) {
		t.Fatalf("want ErrInvalidColor, got %v", err)
	}
}

func TestNamedANSIColorsAreApproximate(t *testing.T) {
	if !MustParseColor("ansi:blue").IsApproximate() {
		t.Fatal("named ANSI color must be approximate")
	}
	if MustParseColor("ansi256(21)").IsApproximate() {
		t.Fatal("ansi256 color must be exact")
	}
}

func TestColorEqualComparesExactSpelling(t *testing.T) {
	if MustParseColor("#fff").Equal(MustParseColor("#ffffff")) {
		t.Fatal("#fff and #ffffff must stay distinct to preserve user values")
	}
}

func TestXtermPalette(t *testing.T) {
	cases := map[uint8]RGB{16: {0, 0, 0}, 21: {0, 0, 255}, 231: {255, 255, 255}, 232: {8, 8, 8}, 244: {128, 128, 128}}
	for n, want := range cases {
		if got := XtermRGB(n); got != want {
			t.Errorf("XtermRGB(%d) = %v, want %v", n, got, want)
		}
	}
	if got := NearestXterm256(RGB{0x33, 0x2b, 0x4f}); got != 59 && got != 236 && got != 237 && got != 238 {
		t.Errorf("NearestXterm256 picked unexpected index %d", got)
	}
	if got := NearestXterm256(RGB{255, 0, 0}); got != 196 {
		t.Errorf("NearestXterm256(red) = %d, want 196", got)
	}
}

func TestContrastRatio(t *testing.T) {
	if r := ContrastRatio(RGB{0, 0, 0}, RGB{255, 255, 255}); r < 20.99 || r > 21.01 {
		t.Fatalf("black/white = %f, want 21", r)
	}
	if r := ContrastRatio(RGB{10, 10, 10}, RGB{10, 10, 10}); r != 1 {
		t.Fatalf("same color = %f, want 1", r)
	}
}

func TestAdjust(t *testing.T) {
	if got := Adjust(RGB{100, 100, 100}, 1); got != (RGB{255, 255, 255}) {
		t.Fatalf("full lighten = %v", got)
	}
	if got := Adjust(RGB{100, 100, 100}, -1); got != (RGB{0, 0, 0}) {
		t.Fatalf("full darken = %v", got)
	}
}
