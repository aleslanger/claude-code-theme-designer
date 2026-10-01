package theme

// xterm16 holds the conventional xterm defaults for the 16 system colors.
// Real terminals let users change these, so they are only approximations.
var xterm16 = [16]RGB{
	{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
	{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
	{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

// cubeLevels are the six channel intensities of the xterm 6x6x6 color cube
// (indices 16-231). Claude Code uses the same levels when downsampling RGB.
var cubeLevels = [6]uint8{0, 95, 135, 175, 215, 255}

const (
	cubeStart     = 16
	cubeSide      = 6
	grayStart     = 232
	grayBase      = 8
	grayStep      = 10
	systemColors  = 16
	paletteColors = 256
)

// XtermRGB returns the RGB value of xterm-256 palette index n.
func XtermRGB(n uint8) RGB {
	switch {
	case n < systemColors:
		return xterm16[n]
	case n < grayStart:
		i := int(n) - cubeStart
		return RGB{
			cubeLevels[i/(cubeSide*cubeSide)],
			cubeLevels[(i/cubeSide)%cubeSide],
			cubeLevels[i%cubeSide],
		}
	default:
		v := uint8(grayBase + grayStep*(int(n)-grayStart))
		return RGB{v, v, v}
	}
}

// NearestXterm256 maps c to the closest palette index in 16-255, skipping the
// user-configurable system colors so the result is terminal-independent.
func NearestXterm256(c RGB) uint8 {
	best, bestDist := uint8(cubeStart), -1
	for n := cubeStart; n < paletteColors; n++ {
		d := distSq(c, XtermRGB(uint8(n)))
		if bestDist < 0 || d < bestDist {
			best, bestDist = uint8(n), d
		}
	}
	return best
}

// NearestANSI16 maps c to the closest of the 16 system colors (approximate).
func NearestANSI16(c RGB) uint8 {
	best, bestDist := uint8(0), -1
	for n := 0; n < systemColors; n++ {
		d := distSq(c, xterm16[n])
		if bestDist < 0 || d < bestDist {
			best, bestDist = uint8(n), d
		}
	}
	return best
}

func distSq(a, b RGB) int {
	dr, dg, db := int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B)
	return dr*dr + dg*dg + db*db
}
