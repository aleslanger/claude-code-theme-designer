package validate

import (
	"fmt"

	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

// terminalKey marks a pair whose background is the terminal's own background.
const terminalKey = ""

// contrastPair is a foreground token drawn on a background token.
type contrastPair struct {
	fg, bg string
	label  string
}

// contrastPairs are the combinations that matter for reading a transcript.
// The user-message pairs come first because they are the tool's main purpose.
var contrastPairs = []contrastPair{
	{"text", "userMessageBackground", "user message text on its background"},
	{"text", "userMessageBackgroundHover", "user message text on the hover background"},
	{"briefLabelYou", "userMessageBackground", `"You" label on the user message background`},
	{"briefLabelYou", terminalKey, `"You" label on the terminal background`},
	{"briefLabelClaude", terminalKey, `"Claude" label on the terminal background`},
	{"text", terminalKey, "text on the terminal background"},
	{"text", "bashMessageBackgroundColor", "text on the ! shell entry background"},
	{"text", "memoryBackgroundColor", "text on the # memory entry background"},
	{"text", "diffAdded", "text on added diff lines"},
	{"text", "diffRemoved", "text on removed diff lines"},
	{"error", terminalKey, "error text on the terminal background"},
	{"warning", terminalKey, "warning text on the terminal background"},
	{"success", terminalKey, "success text on the terminal background"},
}

// ContrastResult is one evaluated pair, exposed for the TUI status panel.
type ContrastResult struct {
	Label       string
	FG, BG      string
	Ratio       float64
	Approximate bool
	AssumedBG   bool
}

// Contrasts evaluates every pair on the resolved theme, whether or not the
// user changed it.
func Contrasts(t theme.Theme, reg *registry.Registry, opts Options) []ContrastResult {
	resolved := reg.Resolve(t)
	termBG, assumed := TerminalBackground(t.EffectiveBase(), opts)
	var out []ContrastResult
	for _, p := range contrastPairs {
		fg, ok := resolved[p.fg]
		if !ok {
			continue
		}
		bg, isTerminal := termBG, p.bg == terminalKey
		if !isTerminal {
			if bg, ok = resolved[p.bg]; !ok {
				continue
			}
		}
		out = append(out, ContrastResult{
			Label: p.label, FG: p.fg, BG: p.bg,
			Ratio:       theme.ContrastRatio(fg.RGB(), bg.RGB()),
			Approximate: fg.IsApproximate() || bg.IsApproximate(),
			AssumedBG:   isTerminal && assumed,
		})
	}
	return out
}

// contrastIssues reports low contrast only for pairs the theme actually
// touches (an override of either side, or a non-default base for terminal
// pairs). Defaults of Claude Code itself are not the user's problem.
func contrastIssues(t theme.Theme, reg *registry.Registry, opts Options) []Issue {
	var out []Issue
	for _, r := range Contrasts(t, reg, opts) {
		if !touches(t, r) || r.Ratio >= theme.ContrastAA {
			continue
		}
		sev, code := Warning, CodeContrastLow
		if r.Ratio < theme.ContrastUnreadable && !r.Approximate && !r.AssumedBG {
			sev, code = Error, CodeContrastUnreadable
		}
		out = append(out, Issue{sev, code, r.FG, describe(r)})
	}
	return out
}

func touches(t theme.Theme, r ContrastResult) bool {
	if _, ok := t.Override(r.FG); ok {
		return true
	}
	if r.BG == terminalKey {
		return false
	}
	_, ok := t.Override(r.BG)
	return ok
}

func describe(r ContrastResult) string {
	msg := fmt.Sprintf("contrast %.2f:1 for %s (WCAG AA needs %.1f:1)", r.Ratio, r.Label, theme.ContrastAA)
	if r.Ratio < theme.ContrastUnreadable {
		msg += "; practically unreadable"
	}
	if r.Approximate {
		msg += "; approximate, ANSI named colors depend on your terminal palette"
	}
	if r.AssumedBG {
		msg += "; terminal background assumed, set it with `claude-theme config terminal-background <color>`"
	}
	return msg
}
