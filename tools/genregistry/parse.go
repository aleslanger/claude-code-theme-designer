package main

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// DocToken is a token as described by the documentation.
type DocToken struct {
	Key            string
	Category       string
	Description    string
	SinceVersion   string
	FullscreenOnly bool
}

const (
	accordionMarker   = `<Accordion title="Color token reference">`
	shimmerCategory   = "Shimmer variants"
	subagentCategory  = "Subagent colors"
	rainbowCategory   = "Ultrathink rainbow"
	shimmerHeadingKey = "Shimmer variants and subagent colors"
)

var (
	reHeading    = regexp.MustCompile(`^\s*####\s+(.+?)\s*$`)
	reTableRow   = regexp.MustCompile("^\\s*\\|\\s*`([A-Za-z0-9_]+)`\\s*\\|\\s*(.+?)\\s*\\|\\s*$")
	reShimmer    = regexp.MustCompile("^\\s*\\*\\s*`([A-Za-z0-9_]+)`\\s+and\\s+`([A-Za-z0-9_]+)`")
	reSubagents  = regexp.MustCompile("`<color>_FOR_SUBAGENTS_ONLY`, where `<color>` is ([^.]+)\\.")
	reRainbow    = regexp.MustCompile("`rainbow_<color>` and `rainbow_<color>_shimmer`, where `<color>` is ([^.]+)\\.")
	reBackticked = regexp.MustCompile("`([A-Za-z0-9_]+)`")
	reSince      = regexp.MustCompile(`v(\d+\.\d+\.\d+) or later`)
	reFullscreen = regexp.MustCompile(`It uses (.+?) only in`)
	reMdLink     = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
)

// ParseDocs extracts every token from the documentation's color token reference.
func ParseDocs(md string) ([]DocToken, error) {
	start := strings.Index(md, accordionMarker)
	if start < 0 {
		return nil, errors.New("color token reference accordion not found; docs format changed")
	}
	body := md[start:]
	if end := strings.Index(body, "</Accordion>"); end >= 0 {
		body = body[:end]
	}

	var tokens []DocToken
	category := ""
	for _, line := range strings.Split(body, "\n") {
		if m := reHeading.FindStringSubmatch(line); m != nil {
			category = m[1]
			continue
		}
		if m := reTableRow.FindStringSubmatch(line); m != nil {
			tokens = append(tokens, tableToken(m[1], m[2], category))
			continue
		}
		if m := reShimmer.FindStringSubmatch(line); m != nil && category == shimmerHeadingKey {
			tokens = append(tokens, DocToken{
				Key:         m[2],
				Category:    shimmerCategory,
				Description: "Lighter partner of `" + m[1] + "` used in the spinner's animated gradient",
			})
		}
	}
	tokens = append(tokens, patternTokens(body)...)
	markFullscreen(body, tokens)
	if len(tokens) == 0 {
		return nil, errors.New("no tokens parsed; docs format changed")
	}
	return tokens, nil
}

func tableToken(key, desc, category string) DocToken {
	t := DocToken{Key: key, Category: category, Description: cleanMarkdown(desc)}
	if m := reSince.FindStringSubmatch(desc); m != nil {
		t.SinceVersion = m[1]
	}
	return t
}

func patternTokens(body string) []DocToken {
	var out []DocToken
	if m := reSubagents.FindStringSubmatch(body); m != nil {
		for _, c := range backticked(m[1]) {
			out = append(out, DocToken{
				Key:         c + "_FOR_SUBAGENTS_ONLY",
				Category:    subagentCategory,
				Description: "Color for subagents and parallel tasks assigned `" + c + "`",
			})
		}
	}
	if m := reRainbow.FindStringSubmatch(body); m != nil {
		for _, c := range backticked(m[1]) {
			out = append(out,
				DocToken{Key: "rainbow_" + c, Category: rainbowCategory, Description: "`ultrathink` keyword gradient: " + c},
				DocToken{Key: "rainbow_" + c + "_shimmer", Category: rainbowCategory, Description: "`ultrathink` keyword gradient shimmer: " + c},
			)
		}
	}
	return out
}

func markFullscreen(body string, tokens []DocToken) {
	m := reFullscreen.FindStringSubmatch(body)
	if m == nil {
		return
	}
	keys := backticked(m[1])
	for i := range tokens {
		if slices.Contains(keys, tokens[i].Key) {
			tokens[i].FullscreenOnly = true
		}
	}
}

func backticked(s string) []string {
	var out []string
	for _, m := range reBackticked.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

func cleanMarkdown(s string) string {
	return strings.TrimSpace(reMdLink.ReplaceAllString(s, "$1"))
}

// DisplayName turns a token key into a readable label.
func DisplayName(key string) string {
	if c, ok := strings.CutSuffix(key, "_FOR_SUBAGENTS_ONLY"); ok {
		return "Subagent " + c
	}
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = nil
		}
	}
	for _, r := range key {
		switch {
		case r == '_':
			flush()
		case unicode.IsUpper(r):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	if len(words) == 0 {
		return key
	}
	first := []rune(words[0])
	first[0] = unicode.ToUpper(first[0])
	words[0] = string(first)
	return strings.Join(words, " ")
}
