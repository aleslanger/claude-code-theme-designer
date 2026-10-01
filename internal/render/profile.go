package render

import "strings"

// DetectProfile infers color support from the environment, following the
// common conventions (NO_COLOR, COLORTERM, TERM). It never queries the terminal.
func DetectProfile(getenv func(string) string, isTTY bool) Profile {
	if getenv("NO_COLOR") != "" || !isTTY {
		return NoColor
	}
	term := getenv("TERM")
	if term == "dumb" {
		return NoColor
	}
	switch strings.ToLower(getenv("COLORTERM")) {
	case "truecolor", "24bit":
		return TrueColor
	}
	if strings.Contains(term, "256color") {
		return ANSI256
	}
	if term == "" {
		return NoColor
	}
	return ANSI16
}

// ParseProfile parses a --color flag value. "auto" returns ok=false.
func ParseProfile(s string) (p Profile, explicit bool, ok bool) {
	switch s {
	case "", "auto":
		return NoColor, false, true
	case "truecolor", "24bit":
		return TrueColor, true, true
	case "256":
		return ANSI256, true, true
	case "16":
		return ANSI16, true, true
	case "none", "never":
		return NoColor, true, true
	}
	return NoColor, false, false
}
