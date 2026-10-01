package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/aleslanger/claude-code-theme-designer/internal/app"
	"github.com/aleslanger/claude-code-theme-designer/internal/presets"
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/render"
	"github.com/aleslanger/claude-code-theme-designer/internal/store"
)

var update = flag.Bool("update", false, "rewrite golden files")

func newModel(t *testing.T, slug string, w, h int) (Model, *app.App) {
	t.Helper()
	reg, err := registry.Default()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := store.ResolvePaths(t.TempDir(), func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(paths)
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(reg, st)
	l, err := a.Resolve(slug)
	if err != nil {
		t.Fatal(err)
	}
	m := New(a, l, render.NoColor)
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(Model), a
}

func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "up", "down", "left", "right", "enter", "esc", "backspace", "pgup", "pgdown":
			msg = tea.KeyMsg{Type: map[string]tea.KeyType{
				"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
				"enter": tea.KeyEnter, "esc": tea.KeyEsc, "backspace": tea.KeyBackspace,
				"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
			}[k]}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m = press(t, m, string(r))
	}
	return m
}

// gotoToken moves the cursor to key.
func gotoToken(t *testing.T, m Model, key string) Model {
	t.Helper()
	for i, r := range m.rows {
		if r.kind == rowToken && r.key == key {
			m.cursor = i
			return m
		}
	}
	t.Fatalf("token %s not in editor", key)
	return m
}

func value(m Model, key string) string {
	c, ok := m.Theme().Override(key)
	if !ok {
		return "inherit"
	}
	return c.String()
}

func TestCursorStartsOnFirstEditableRowAndSkipsHeaders(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	if m.currentRow().kind != rowName {
		t.Fatalf("cursor on %v", m.currentRow())
	}
	m = press(t, m, "down", "down", "down")
	if m.currentRow().kind != rowToken || m.currentRow().key != "userMessageBackground" {
		t.Fatalf("header not skipped: %+v", m.currentRow())
	}
}

func TestPickerAppliesLiveAndEscRestores(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m = gotoToken(t, m, "userMessageBackground")
	m = press(t, m, "enter", "right")
	if m.mode != modePicker || !strings.HasPrefix(value(m, "userMessageBackground"), "ansi256(") {
		t.Fatalf("picker did not apply live: %s", value(m, "userMessageBackground"))
	}
	m = press(t, m, "esc")
	if value(m, "userMessageBackground") != "#332b4f" {
		t.Fatalf("esc must restore, got %s", value(m, "userMessageBackground"))
	}
	m = press(t, m, "enter", "down", "enter")
	if m.mode != modeBrowse || !strings.HasPrefix(value(m, "userMessageBackground"), "ansi256(") {
		t.Fatal("enter must commit the palette color")
	}
}

func TestHexInputValidatesAndPreviewsLive(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m = gotoToken(t, m, "briefLabelYou")
	m = press(t, m, "#")
	for range len("#c4a7ff") {
		m = press(t, m, "backspace")
	}
	m = typeText(t, m, "#12")
	if value(m, "briefLabelYou") != "#c4a7ff" {
		t.Fatal("incomplete input must not apply")
	}
	m = typeText(t, m, "3")
	if value(m, "briefLabelYou") != "#123" {
		t.Fatalf("valid #rgb must preview live, got %s", value(m, "briefLabelYou"))
	}
	m = typeText(t, m, "x")
	m = press(t, m, "enter")
	if m.mode != modeInput || m.input.err == "" {
		t.Fatal("invalid value must keep the input open with an error")
	}
	m = press(t, m, "esc")
	if value(m, "briefLabelYou") != "#c4a7ff" {
		t.Fatal("esc must restore the original value")
	}
}

func TestInheritResetAndSectionReset(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m = gotoToken(t, m, "userMessageBackground")
	m = press(t, m, "i")
	if value(m, "userMessageBackground") != "inherit" {
		t.Fatal("i must remove the override")
	}
	m = press(t, m, "r")
	if value(m, "userMessageBackground") != "#332b4f" {
		t.Fatal("r must restore the starting value")
	}
	m = gotoToken(t, m, "briefLabelYou")
	m = press(t, m, "i")
	m = gotoToken(t, m, "userMessageBackgroundHover")
	m = press(t, m, "i", "R")
	if value(m, "briefLabelYou") != "#c4a7ff" || value(m, "userMessageBackgroundHover") != "#403660" {
		t.Fatal("R must reset the whole section")
	}
	m = press(t, m, "i", "X")
	if m.mode != modeConfirm {
		t.Fatal("X must ask first")
	}
	m = press(t, m, "y")
	if !m.Theme().Equal(m.baseline) {
		t.Fatal("X must reset the whole theme")
	}
}

func TestAdjustLightensAndDarkens(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m = gotoToken(t, m, "userMessageBackground")
	m = press(t, m, "+")
	lighter := value(m, "userMessageBackground")
	m = press(t, m, "-", "-")
	if lighter == "#332b4f" || value(m, "userMessageBackground") == lighter {
		t.Fatal("+/- must change the color")
	}
}

func TestBaseCyclesAndPresetLoads(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m = press(t, m, "b")
	if m.Theme().EffectiveBase() != "light" {
		t.Fatalf("b must cycle base, got %s", m.Theme().EffectiveBase())
	}
	m = press(t, m, "p", "down", "enter")
	if m.Theme().Name != presets.All()[1].Theme.Name || !m.baseline.Equal(m.Theme()) {
		t.Fatal("preset must load and become the reset point")
	}
}

func TestEditsDoNotTouchDiskUntilSave(t *testing.T) {
	m, a := newModel(t, "prompt-contrast", 120, 40)
	m = gotoToken(t, m, "userMessageBackground")
	m = press(t, m, "+", "+", "enter", "right", "enter")
	if _, err := os.Stat(a.Store.Paths.ConfigDir); !os.IsNotExist(err) {
		t.Fatal("editing must not write anything")
	}
	if !m.dirty() {
		t.Fatal("changes must mark the model dirty")
	}
	m = press(t, m, "s")
	if m.dirty() || m.statusBad {
		t.Fatalf("save failed: %s", m.status)
	}
	data, err := a.Store.ReadDraft("prompt-contrast")
	if err != nil || !strings.Contains(string(data), "ansi256(") {
		t.Fatalf("draft not saved: %v", err)
	}
}

func TestInstallFromTUIAsksBeforeReplacing(t *testing.T) {
	m, a := newModel(t, "prompt-contrast", 120, 40)
	m = press(t, m, "I")
	path := filepath.Join(a.Store.Paths.ThemesDir, "prompt-contrast.json")
	if _, err := os.Stat(path); err != nil || m.statusBad {
		t.Fatalf("install failed: %s", m.status)
	}
	m = gotoToken(t, m, "briefLabelYou")
	m = press(t, m, "i", "I")
	if m.mode != modeConfirm {
		t.Fatal("replacing a different installed file must ask")
	}
	m = press(t, m, "n")
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "briefLabelYou") {
		t.Fatal("declined install must not change the file")
	}
	m = press(t, m, "I", "y")
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "briefLabelYou") {
		t.Fatal("confirmed install must replace the file")
	}
}

func TestQuitAsksWhenDirty(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd == nil {
		t.Fatal("clean quit must exit immediately")
	}
	m = gotoToken(t, m, "text")
	m = press(t, m, "+")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd != nil || next.(Model).mode != modeConfirm {
		t.Fatal("dirty quit must ask first")
	}
	if _, cmd := next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}); cmd == nil {
		t.Fatal("confirmed quit must exit")
	}
}

func TestRenameRejectsUnsafeValues(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m.cursor = 2 // File name row
	m = press(t, m, "enter")
	for range len("prompt-contrast") {
		m = press(t, m, "backspace")
	}
	m = typeText(t, m, "../x")
	m = press(t, m, "enter")
	if m.mode != modeInput || m.input.err == "" || m.slug != "prompt-contrast" {
		t.Fatal("unsafe file name must be rejected")
	}
}

func TestUnsupportedRowExplainsInsteadOfEditing(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	for i, r := range m.rows {
		if r.kind == rowUnsupported {
			m.cursor = i
			break
		}
	}
	before := m.Theme()
	m = press(t, m, "enter")
	if !m.Theme().Equal(before) || !m.statusBad || !strings.Contains(m.status, "not available") {
		t.Fatal("unsupported capability must be explained, not edited")
	}
}

func TestViewFitsTerminalAndSnapshots(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{{"wide", 130, 42}, {"narrow", 80, 36}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newModel(t, "prompt-contrast", tc.w, tc.h)
			m = gotoToken(t, m, "userMessageBackground")
			view := m.View()
			lines := strings.Split(view, "\n")
			if len(lines) != tc.h {
				t.Fatalf("view has %d lines, want %d", len(lines), tc.h)
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w > tc.w {
					t.Fatalf("line %d is %d cells wide (max %d): %q", i, w, tc.w, l)
				}
			}
			golden(t, "view_"+tc.name+".golden", view)
		})
	}
}

func TestPaletteViewSnapshot(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 130, 42)
	m = gotoToken(t, m, "briefLabelYou")
	m = press(t, m, "enter")
	golden(t, "view_palette.golden", m.View())
}

func TestTooSmallTerminal(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 50, 10)
	if !strings.Contains(m.View(), "Terminal too small") {
		t.Fatal("small terminals need a clear message")
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file (run go test -update): %v", err)
	}
	if string(want) != got {
		t.Fatalf("%s differs; run `go test ./internal/tui -update` if intended\n%s", name, got)
	}
}

func TestCtrlCAsksWhenDirtyAndSecondCtrlCQuits(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m = gotoToken(t, m, "text")
	m = press(t, m, "+")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil || next.(Model).mode != modeConfirm {
		t.Fatal("ctrl+c with unsaved edits must ask first")
	}
	if _, cmd := next.(Model).Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Fatal("second ctrl+c must quit")
	}
}

func TestSavingOverAnotherDraftAsksAndBacksUp(t *testing.T) {
	m, a := newModel(t, "prompt-contrast", 120, 40)
	if _, _, err := a.SaveDraft("other", m.Theme().WithName("Other")); err != nil {
		t.Fatal(err)
	}
	m.cursor = 2 // File name row
	m = press(t, m, "enter")
	for range len("prompt-contrast") {
		m = press(t, m, "backspace")
	}
	m = typeText(t, m, "other")
	m = press(t, m, "enter")
	if !m.dirty() {
		t.Fatal("renamed theme must count as unsaved")
	}
	m = press(t, m, "s")
	if m.mode != modeConfirm {
		t.Fatal("saving over a different draft must ask")
	}
	m = press(t, m, "y")
	if m.statusBad || !strings.Contains(m.status, "backed up") {
		t.Fatalf("expected backup, got %q", m.status)
	}
}

func TestPasteIntoColorInput(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m = gotoToken(t, m, "briefLabelYou")
	m = press(t, m, "#")
	for range len("#c4a7ff") {
		m = press(t, m, "backspace")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("#00ff88"), Paste: true})
	m = press(t, next.(Model), "enter")
	if value(m, "briefLabelYou") != "#00ff88" {
		t.Fatalf("paste not applied: %s", value(m, "briefLabelYou"))
	}
}

func TestNavigationKeysAndPanels(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	if m.Init() != nil {
		t.Fatal("Init must not start commands")
	}
	m = press(t, m, "G")
	if m.cursor != len(m.rows)-1 && !m.rows[m.cursor].selectable() {
		t.Fatal("G must go to the last selectable row")
	}
	m = press(t, m, "g")
	if m.currentRow().kind != rowName {
		t.Fatal("g must go to the first row")
	}
	m = press(t, m, "pgdown", "pgup", "j", "k")
	if m.currentRow().kind != rowName {
		t.Fatalf("paging must return to the top, got %+v", m.currentRow())
	}
	m = press(t, m, "?")
	if m.mode != modeHelp || !strings.Contains(m.View(), "Edits stay in memory") {
		t.Fatal("help must render")
	}
	m = press(t, m, "x")
	m = press(t, m, "p")
	if !strings.Contains(m.View(), "Nord-like") || !strings.Contains(m.View(), "not official") {
		t.Fatal("preset panel must render")
	}
	m = press(t, m, "down", "up", "esc")
	if m.mode != modeBrowse {
		t.Fatal("esc must leave the preset panel")
	}
}

func TestMetaRowsAndDetails(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m.cursor = 3 // Base
	if !strings.Contains(m.View(), "Built-in preset the theme starts from") {
		t.Fatal("base detail missing")
	}
	m = press(t, m, "enter")
	if m.Theme().EffectiveBase() != "light" {
		t.Fatal("enter on base must cycle")
	}
	m.cursor = 1 // Name
	m = press(t, m, "enter")
	for range len("Prompt Contrast") {
		m = press(t, m, "backspace")
	}
	m = typeText(t, m, "Mine")
	if !strings.Contains(m.View(), "Display name: Mine") {
		t.Fatal("input line must show typed text")
	}
	m = press(t, m, "enter")
	if m.Theme().Name != "Mine" {
		t.Fatal("rename failed")
	}
	m.cursor = 0
	m = press(t, m, "R")
	if m.Theme().Name != "Prompt Contrast" || m.Theme().EffectiveBase() != "dark" {
		t.Fatal("R on the theme block must reset name and base")
	}
	m = gotoToken(t, m, "text")
	if !strings.Contains(m.View(), "inherits") {
		t.Fatal("inherited token detail must show the base value")
	}
}

func TestPickerKeys(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m = gotoToken(t, m, "claude") // inherited: picker starts from the base color
	m = press(t, m, "enter", "left", "up", "down", "right", "i")
	if value(m, "claude") != "inherit" || m.mode != modeBrowse {
		t.Fatal("i in the picker must inherit")
	}
	m = press(t, m, "enter", "right", "r")
	if value(m, "claude") != "inherit" {
		t.Fatal("r in the picker must reset to the starting value")
	}
	m = press(t, m, "enter", "right", "q")
	if value(m, "claude") != "inherit" {
		t.Fatal("q in the picker must cancel")
	}
	m = press(t, m, "enter", "#")
	if m.mode != modeInput {
		t.Fatal("# in the picker must open hex input")
	}
}

func TestLowContrastShowsInStatusAndRow(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 40)
	m = gotoToken(t, m, "userMessageBackground")
	m = press(t, m, "#")
	for range len("#332b4f") {
		m = press(t, m, "backspace")
	}
	m = typeText(t, m, "#888888")
	m = press(t, m, "enter")
	m.status = ""
	view := m.View()
	if !strings.Contains(view, "warning(s)") {
		t.Fatalf("status must summarise warnings:\n%s", view)
	}
	m = gotoToken(t, m, "text")
	if !strings.Contains(m.View(), "contrast") {
		t.Fatal("token detail must show the contrast issue")
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 120, 24)
	m = press(t, m, "G")
	if !strings.Contains(m.View(), "▸") {
		t.Fatal("cursor row must stay visible when scrolled")
	}
}

func TestPresetPanelScrollsInShortPanes(t *testing.T) {
	m, _ := newModel(t, "prompt-contrast", 80, 24)
	m = press(t, m, "p")
	for range len(presets.All()) {
		m = press(t, m, "down")
	}
	view := m.View()
	if !strings.Contains(view, "▸ ANSI 16") {
		t.Fatalf("last preset must be visible after scrolling:\n%s", view)
	}
	m = press(t, m, "enter")
	if m.Theme().Name != "ANSI 16" || m.Theme().EffectiveBase() != "dark-ansi" {
		t.Fatal("cursor must load the highlighted preset")
	}
}
