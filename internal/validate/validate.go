// Package validate performs semantic checks on a structurally valid theme:
// token provenance, version notes and color contrast. It never rejects a theme
// for being merely unusual; only practically unreadable text is an error.
package validate

import (
	"fmt"
	"slices"

	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

// Severity orders issues; only Error blocks an install (unless forced).
type Severity int

const (
	Info Severity = iota
	Warning
	Error
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	default:
		return "info"
	}
}

// Issue codes are stable identifiers usable in tests and scripts.
const (
	CodeUnverifiedToken    = "unverified-token"
	CodeUnknownToken       = "unknown-token"
	CodeFullscreenOnly     = "fullscreen-only"
	CodeSinceVersion       = "since-version"
	CodeContrastLow        = "contrast-low"
	CodeContrastUnreadable = "contrast-unreadable"
)

// Issue is one finding.
type Issue struct {
	Severity Severity
	Code     string
	Token    string
	Message  string
}

// Report is the result of Check.
type Report struct{ Issues []Issue }

// HasErrors reports whether any issue is an Error.
func (r Report) HasErrors() bool { return r.count(Error) > 0 }

// Errors and Warnings return the issue counts per severity.
func (r Report) Errors() int   { return r.count(Error) }
func (r Report) Warnings() int { return r.count(Warning) }

func (r Report) count(s Severity) int {
	n := 0
	for _, i := range r.Issues {
		if i.Severity == s {
			n++
		}
	}
	return n
}

// ForToken returns issues attached to key.
func (r Report) ForToken(key string) []Issue {
	var out []Issue
	for _, i := range r.Issues {
		if i.Token == key {
			out = append(out, i)
		}
	}
	return out
}

// Options tunes environment-dependent checks.
type Options struct {
	// TerminalBackground, when set, replaces the assumed terminal background
	// (Claude Code does not paint it, so the tool cannot know it).
	TerminalBackground *theme.Color
}

// Assumed terminal backgrounds when the user has not configured one.
var (
	assumedDarkTerminal  = theme.MustParseColor("#1e1e1e")
	assumedLightTerminal = theme.MustParseColor("#ffffff")
)

// TerminalBackground returns the background used for terminal contrast checks
// and whether it is an assumption.
func TerminalBackground(b theme.Base, opts Options) (theme.Color, bool) {
	if opts.TerminalBackground != nil {
		return *opts.TerminalBackground, false
	}
	if b.IsLight() {
		return assumedLightTerminal, true
	}
	return assumedDarkTerminal, true
}

// Check runs every semantic check. Issues are ordered by severity, then token.
func Check(t theme.Theme, reg *registry.Registry, opts Options) Report {
	var issues []Issue
	issues = append(issues, tokenIssues(t, reg)...)
	issues = append(issues, contrastIssues(t, reg, opts)...)
	slices.SortStableFunc(issues, func(a, b Issue) int {
		if a.Severity != b.Severity {
			return int(b.Severity) - int(a.Severity)
		}
		switch {
		case a.Token < b.Token:
			return -1
		case a.Token > b.Token:
			return 1
		}
		return 0
	})
	return Report{Issues: issues}
}

func tokenIssues(t theme.Theme, reg *registry.Registry) []Issue {
	var out []Issue
	for _, k := range t.SortedKeys() {
		tok, ok := reg.Lookup(k)
		switch {
		case !ok:
			out = append(out, Issue{Warning, CodeUnknownToken, k, fmt.Sprintf(
				"unknown token; kept as passthrough. Claude Code ignores tokens it does not know (registry verified against %s; it may be newer)",
				reg.VerifiedAgainst)})
		case tok.Status == registry.Unverified:
			out = append(out, Issue{Warning, CodeUnverifiedToken, k,
				"undocumented token; kept as passthrough but not editable, may stop working in any Claude Code release"})
		default:
			out = append(out, versionNotes(tok)...)
		}
	}
	return out
}

func versionNotes(tok registry.Token) []Issue {
	var out []Issue
	if tok.FullscreenOnly {
		out = append(out, Issue{Info, CodeFullscreenOnly, tok.Key,
			"only visible in fullscreen rendering mode (/tui fullscreen)"})
	}
	if tok.SinceVersion != "" {
		out = append(out, Issue{Info, CodeSinceVersion, tok.Key,
			"takes effect on Claude Code v" + tok.SinceVersion + " or later"})
	}
	return out
}
