package render

// MinWidth is the narrowest preview that still shows every element.
const MinWidth = 40

// Options configure Preview.
type Options struct {
	Width int
	// Focus is the token being edited; rows using it get a gutter marker.
	Focus string
}

// Token keys used by the preview. All are verified tokens.
const (
	tText       = "text"
	tInactive   = "inactive"
	tSubtle     = "subtle"
	tSuccess    = "success"
	tError      = "error"
	tWarning    = "warning"
	tSuggestion = "suggestion"
	tPlanMode   = "planMode"
	tBorder     = "promptBorder"
	tBashBorder = "bashBorder"
	tRemember   = "remember"
	tUserBG     = "userMessageBackground"
	tUserHover  = "userMessageBackgroundHover"
	tBashBG     = "bashMessageBackgroundColor"
	tMemoryBG   = "memoryBackgroundColor"
	tLabelYou   = "briefLabelYou"
	tLabelBot   = "briefLabelClaude"
	tDiffAdd    = "diffAdded"
	tDiffDel    = "diffRemoved"
	tDiffAddW   = "diffAddedWord"
	tDiffDelW   = "diffRemovedWord"
)

const (
	dot     = "⏺ "
	elbow   = "  ⎿  "
	pointer = "❯ "
)

// Preview builds a simulated Claude Code transcript. It approximates the real
// layout; it does not run or embed Claude Code.
func Preview(opts Options) Frame {
	w := max(opts.Width, MinWidth)
	var lines []Line
	add := func(ls ...Line) { lines = append(lines, ls...) }
	blank := Line{}

	add(userPrompt(tUserBG, "> ", "Why can this code cause a race condition?"))
	add(blank)
	add(reply(dot, "Two goroutines write `count` without a lock, so")...)
	add(Line{Spans: []Span{span("  updates can interleave and be lost.", fg(tText))}})
	add(blank)
	add(Line{Spans: []Span{span("You", bold(tLabelYou))}})
	add(userPrompt(tUserBG, " ", "Refactor the worker pool so that:"))
	add(userPrompt(tUserBG, " ", "- jobs are cancelled on shutdown"))
	add(userPrompt(tUserBG, " ", "- errors are collected and returned"))
	add(blank)
	add(Line{Spans: []Span{span("Claude", bold(tLabelBot))}})
	add(Line{Spans: []Span{span(" Here is the updated Stop method:", fg(tText))}})
	add(codeBlock()...)
	add(blank)
	add(toolCall(tSuccess, "Bash", "go test ./..."), toolOutput("ok   example.com/pool  0.412s", tInactive))
	add(toolCall(tSuccess, "Update", "pool.go"), toolOutput("Updated pool.go with 1 addition and 1 removal", tInactive))
	add(diffLine(tDiffDel, tDiffDelW, "  12 - ", "  p.wg.", "Wait()"))
	add(diffLine(tDiffAdd, tDiffAddW, "  12 + ", "  return p.", "waitCtx(ctx)"))
	add(toolCall(tError, "Bash", "rm -rf /build"), toolOutput("Error: permission denied", tError))
	add(blank)
	add(Line{Spans: []Span{span("⚠ Context low · 12% remaining before auto-compact", fg(tWarning))}})
	add(blank)
	add(selectedPrompt())
	add(hoverPrompt())
	add(Line{Fill: tBashBG, Spans: []Span{span("! ", bold(tBashBorder)), span("git status", fg(tText))}})
	add(Line{Fill: tMemoryBG, Spans: []Span{span("# ", bold(tRemember)), span("Always run gofmt before committing", fg(tText))}})
	add(blank)
	add(inputBox(w - gutterWidth)...)
	add(Line{Spans: []Span{span("  ? for shortcuts", fg(tInactive)), span("   ⏸ plan mode on", fg(tPlanMode))}})

	for i := range lines {
		lines[i].Focus = lines[i].usesToken(opts.Focus)
	}
	return Frame{Width: w, Lines: lines}
}

func userPrompt(bg, prefix, text string) Line {
	return Line{Fill: bg, Spans: []Span{span(prefix, fg(tSubtle)), span(text, fg(tText))}}
}

func reply(prefix, text string) []Line {
	return []Line{{Spans: []Span{span(prefix, fg(tText)), span(text, fg(tText))}}}
}

func codeBlock() []Line {
	code := []string{
		"func (p *Pool) Stop(ctx context.Context) error {",
		"    close(p.jobs)",
		"    return p.waitCtx(ctx)",
		"}",
	}
	out := []Line{{Spans: []Span{span("  ```go", fg(tInactive))}}}
	for _, c := range code {
		out = append(out, Line{Spans: []Span{span("  "+c, fg(tText))}})
	}
	return append(out, Line{Spans: []Span{span("  ```", fg(tInactive))}})
}

func toolCall(status, tool, arg string) Line {
	return Line{Spans: []Span{span(dot, fg(status)), span(tool, bold(tText)), span("("+arg+")", fg(tText))}}
}

func toolOutput(text, color string) Line {
	return Line{Spans: []Span{span(elbow, fg(tInactive)), span(text, fg(color))}}
}

func diffLine(bg, wordBG, gutter, same, changed string) Line {
	return Line{Fill: bg, Spans: []Span{
		span(gutter, fg(tText)),
		span(same, fg(tText)),
		span(changed, Style{FG: tText, BG: wordBG}),
	}}
}

// selectedPrompt mirrors what Claude Code 2.1.287 does for a selected message
// (pointer and text in the suggestion color). This mapping is observed, not
// documented, hence the "approx." caption.
func selectedPrompt() Line {
	return Line{Fill: tUserBG, Spans: []Span{
		span(pointer, fg(tSuggestion)),
		span("Earlier prompt (selected)", fg(tSuggestion)),
		span("  approx.", Style{FG: tInactive, Italic: true}),
	}}
}

func hoverPrompt() Line {
	return Line{Fill: tUserHover, Spans: []Span{
		span("> ", fg(tSubtle)),
		span("Hovered / expanded prompt", fg(tText)),
		span("  fullscreen only", Style{FG: tInactive, Italic: true}),
	}}
}

func inputBox(w int) []Line {
	inner := max(w-4, 1)
	top := "╭" + repeat("─", inner+2) + "╮"
	bottom := "╰" + repeat("─", inner+2) + "╯"
	placeholder := `> Try "refactor pool.go"`
	return []Line{
		{Spans: []Span{span(top, fg(tBorder))}},
		{Spans: []Span{span("│ ", fg(tBorder)), span(pad(placeholder, inner), fg(tInactive)), span(" │", fg(tBorder))}},
		{Spans: []Span{span(bottom, fg(tBorder))}},
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for range n {
		out = append(out, s...)
	}
	return string(out)
}
