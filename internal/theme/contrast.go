package theme

import "math"

// WCAG 2.x contrast thresholds.
const (
	// ContrastAA is the WCAG AA minimum for normal text.
	ContrastAA = 4.5
	// ContrastUnreadable is the ratio below which text is treated as practically
	// invisible. Only this level may block an install (overridable with --force).
	ContrastUnreadable = 1.5
)

// ContrastRatio returns the WCAG contrast ratio between two colors, in [1, 21].
func ContrastRatio(a, b RGB) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func relativeLuminance(c RGB) float64 {
	return 0.2126*linearize(c.R) + 0.7152*linearize(c.G) + 0.0722*linearize(c.B)
}

func linearize(v uint8) float64 {
	s := float64(v) / 255
	if s <= 0.03928 {
		return s / 12.92
	}
	return math.Pow((s+0.055)/1.055, 2.4)
}

// Adjust lightens (positive step) or darkens (negative step) a color by mixing
// it toward white or black. step is a fraction in [-1, 1].
func Adjust(c RGB, step float64) RGB {
	target := 0.0
	if step > 0 {
		target = 255
	}
	f := math.Min(math.Abs(step), 1)
	mix := func(v uint8) uint8 {
		return uint8(math.Round(float64(v) + (target-float64(v))*f))
	}
	return RGB{mix(c.R), mix(c.G), mix(c.B)}
}
