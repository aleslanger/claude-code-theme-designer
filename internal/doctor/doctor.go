// Package doctor gathers diagnostics. It only reads: it never writes files,
// never contacts the network and runs at most `claude --version`.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aleslanger/claude-code-theme-designer/internal/app"
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/render"
	"github.com/aleslanger/claude-code-theme-designer/internal/serialize"
	"github.com/aleslanger/claude-code-theme-designer/internal/store"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

// Level classifies a diagnostic line.
type Level int

const (
	OK Level = iota
	Note
	Warn
)

// Item is one diagnostic line.
type Item struct {
	Level        Level
	Label, Value string
}

// Env abstracts the environment so diagnostics are testable.
type Env struct {
	Getenv   func(string) string
	LookPath func(string) (string, error)
	// Version returns the output of `<path> --version`.
	Version func(ctx context.Context, path string) (string, error)
	IsTTY   bool
}

// VersionTimeout bounds `claude --version`.
const VersionTimeout = 3 * time.Second

var reSemver = regexp.MustCompile(`\b(\d+)\.(\d+)\.(\d+)\b`)

// Run produces the diagnostics report.
func Run(ctx context.Context, a *app.App, env Env) []Item {
	var items []Item
	items = append(items, claudeItems(ctx, a.Reg, env)...)
	items = append(items, themeDirItems(a)...)
	items = append(items, registryItems(a.Reg)...)
	items = append(items, terminalItems(a, env)...)
	items = append(items, configItems(a)...)
	return items
}

func claudeItems(ctx context.Context, reg *registry.Registry, env Env) []Item {
	path, err := env.LookPath("claude")
	if err != nil {
		return []Item{{Warn, "Claude Code", "not detected on PATH (themes are still written; install Claude Code to use them)"}}
	}
	items := []Item{{OK, "Claude Code", "detected: " + theme.Sanitize(path)}}
	version := versionFromInstallPath(path)
	if version == "" && env.Version != nil {
		ctx, cancel := context.WithTimeout(ctx, VersionTimeout)
		defer cancel()
		if out, err := env.Version(ctx, path); err == nil {
			version = firstSemver(out)
		}
	}
	if version == "" {
		return append(items, Item{Note, "Claude Code version", "unknown"})
	}
	items = append(items, Item{OK, "Claude Code version", version})
	return append(items, compatibilityItem(version, firstSemver(reg.VerifiedAgainst)))
}

// versionFromInstallPath reads the version from the native installer layout
// (…/claude/versions/<semver>) without executing anything.
func versionFromInstallPath(path string) string {
	target, err := filepath.EvalSymlinks(path)
	if err != nil || filepath.Base(filepath.Dir(target)) != "versions" {
		return ""
	}
	base := filepath.Base(target)
	if reSemver.FindString(base) == base {
		return base
	}
	return ""
}

func firstSemver(s string) string { return reSemver.FindString(s) }

func compatibilityItem(detected, verified string) Item {
	if verified == "" {
		return Item{Note, "Compatibility", "registry has no verified version"}
	}
	switch c := compareSemver(detected, verified); {
	case c > 0:
		return Item{Warn, "Compatibility", fmt.Sprintf(
			"Claude Code %s is newer than the registry (verified against %s); new tokens may exist. Unknown tokens in themes are preserved, not dropped", detected, verified)}
	case c < 0:
		return Item{Warn, "Compatibility", fmt.Sprintf(
			"Claude Code %s is older than the registry (verified against %s); some tokens may be ignored (e.g. effortUltra needs 2.1.239+)", detected, verified)}
	}
	return Item{OK, "Compatibility", "matches the registry's verified version " + verified}
}

func compareSemver(a, b string) int {
	pa, pb := reSemver.FindStringSubmatch(a), reSemver.FindStringSubmatch(b)
	if pa == nil || pb == nil {
		return 0
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x - y
		}
	}
	return 0
}

func themeDirItems(a *app.App) []Item {
	dir := a.Store.Paths.ThemesDir
	st, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Item{{Warn, "Theme directory", theme.Sanitize(dir) + " does not exist yet; it is created on first install. Restart Claude Code once afterwards so it starts watching it"}}
	}
	if err != nil {
		return []Item{{Warn, "Theme directory", theme.Sanitize(err.Error())}}
	}
	if !st.IsDir() {
		return []Item{{Warn, "Theme directory", theme.Sanitize(dir) + " is not a directory"}}
	}
	items := []Item{{OK, "Theme directory", theme.Sanitize(dir) + " (" + writability(dir) + ")"}}
	if lst, err := os.Lstat(dir); err == nil && lst.Mode()&fs.ModeSymlink != 0 {
		items = append(items, Item{Note, "Theme directory", "is a symlink (fine for dotfile managers)"})
	}
	return append(items, installedItems(a)...)
}

func installedItems(a *app.App) []Item {
	entries, err := a.Store.ListInstalled()
	if err != nil {
		return []Item{{Warn, "Installed themes", theme.Sanitize(err.Error())}}
	}
	managed := 0
	var items []Item
	for _, e := range entries {
		name := theme.Sanitize(e.Slug)
		if e.Managed {
			managed++
		}
		switch {
		case e.ReadErr != nil:
			items = append(items, Item{Warn, "Theme " + name, theme.Sanitize(e.ReadErr.Error())})
		case e.Modified:
			items = append(items, Item{Note, "Theme " + name, "edited outside claude-theme since install"})
		default:
			if data, err := a.Store.ReadInstalled(e.Slug); err == nil {
				if _, err := serialize.Decode(data); err != nil {
					items = append(items, Item{Warn, "Theme " + name, "fails strict validation: " + theme.Sanitize(err.Error())})
				}
			}
		}
	}
	summary := Item{OK, "Installed themes", fmt.Sprintf("%d total, %d managed by claude-theme", len(entries), managed)}
	return append([]Item{summary}, items...)
}

func registryItems(reg *registry.Registry) []Item {
	return []Item{
		{OK, "Token registry", fmt.Sprintf("version %s, %d verified, %d unverified (passthrough only)",
			reg.Version, reg.Count(registry.Verified), reg.Count(registry.Unverified))},
		{Note, "Registry source", reg.DocsURL + "; verified against " + reg.VerifiedAgainst},
		{Note, "Base palettes", reg.PaletteSource + " (approximate; used only for previewing inherited tokens)"},
	}
}

func terminalItems(a *app.App, env Env) []Item {
	profile := render.DetectProfile(env.Getenv, env.IsTTY)
	items := []Item{
		{OK, "Terminal", fmt.Sprintf("TERM=%q COLORTERM=%q tty=%v", theme.Sanitize(env.Getenv("TERM")), theme.Sanitize(env.Getenv("COLORTERM")), env.IsTTY)},
		{OK, "Color support", profile.String()},
	}
	switch profile {
	case render.NoColor:
		items = append(items, Item{Warn, "Color support", "no color output; the fullscreen editor falls back to plain previews (override with --color)"})
	case render.ANSI16, render.ANSI256:
		items = append(items, Item{Note, "Color support", "no truecolor; hex colors are previewed as their nearest palette color, which is what Claude Code shows here too"})
	}
	if a.Store.Config.TerminalBackground == "" {
		items = append(items, Item{Note, "Terminal background", "assumed for contrast checks; set it with `claude-theme config terminal-background <color>`"})
	} else {
		items = append(items, Item{OK, "Terminal background", a.Store.Config.TerminalBackground})
	}
	return items
}

func configItems(a *app.App) []Item {
	lvl := OK
	if a.Store.ConfigStatus == store.ConfigCorrupt {
		lvl = Warn
	}
	drafts, err := a.Store.ListDrafts()
	draftInfo := fmt.Sprintf("%d drafts in %s", len(drafts), theme.Sanitize(a.Store.Paths.DraftsDir))
	if err != nil {
		draftInfo = theme.Sanitize(err.Error())
	}
	return []Item{
		{lvl, "Designer config", theme.Sanitize(a.Store.Paths.ConfigFile) + ": " + a.Store.ConfigStatus.String()},
		{OK, "Drafts", draftInfo},
	}
}

// Format renders the report as aligned text.
func Format(items []Item) string {
	width := 0
	for _, it := range items {
		width = max(width, len(it.Label))
	}
	var b strings.Builder
	for _, it := range items {
		mark := map[Level]string{OK: "✓", Note: "·", Warn: "!"}[it.Level]
		fmt.Fprintf(&b, "%s %-*s  %s\n", mark, width, it.Label, it.Value)
	}
	return b.String()
}
