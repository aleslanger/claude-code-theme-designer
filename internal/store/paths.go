// Package store is the filesystem layer: path resolution, safe atomic writes,
// limited reads, the designer's own config and drafts, and installing themes
// into the Claude Code theme directory.
package store

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

const (
	appDirName     = "claude-theme-designer"
	claudeDirName  = ".claude"
	themesDirName  = "themes"
	draftsDirName  = "themes"
	backupsDirName = "backups"
	configFileName = "config.json"
	themeExt       = ".json"

	envClaudeConfigDir = "CLAUDE_CONFIG_DIR"
	envXDGConfigHome   = "XDG_CONFIG_HOME"
)

// Paths are every location the tool touches. The designer's state lives in
// ConfigDir, strictly separate from Claude Code's ThemesDir.
type Paths struct {
	ClaudeDir  string // ~/.claude or $CLAUDE_CONFIG_DIR
	ThemesDir  string // <ClaudeDir>/themes, watched by Claude Code
	ConfigDir  string // ~/.config/claude-theme-designer or $XDG_CONFIG_HOME/...
	ConfigFile string // <ConfigDir>/config.json
	DraftsDir  string // <ConfigDir>/themes, work-in-progress themes
	BackupsDir string // <ConfigDir>/backups; never inside ThemesDir, where
	// Claude Code would load every .json backup as a theme
}

// ResolvePaths derives Paths from the home directory and environment, using
// the same CLAUDE_CONFIG_DIR override Claude Code uses for its config dir.
func ResolvePaths(home string, getenv func(string) string) (Paths, error) {
	if home == "" || !filepath.IsAbs(home) {
		return Paths{}, errors.New("cannot determine an absolute home directory")
	}
	claudeDir := filepath.Join(home, claudeDirName)
	if v := getenv(envClaudeConfigDir); v != "" {
		if !filepath.IsAbs(v) {
			return Paths{}, fmt.Errorf("%s must be an absolute path", envClaudeConfigDir)
		}
		claudeDir = filepath.Clean(v)
	}
	configRoot := filepath.Join(home, ".config")
	if v := getenv(envXDGConfigHome); v != "" && filepath.IsAbs(v) {
		configRoot = filepath.Clean(v)
	}
	configDir := filepath.Join(configRoot, appDirName)
	return Paths{
		ClaudeDir:  claudeDir,
		ThemesDir:  filepath.Join(claudeDir, themesDirName),
		ConfigDir:  configDir,
		ConfigFile: filepath.Join(configDir, configFileName),
		DraftsDir:  filepath.Join(configDir, draftsDirName),
		BackupsDir: filepath.Join(configDir, backupsDirName),
	}, nil
}

// ThemeFile returns <dir>/<slug>.json after validating the slug and proving the
// result stays directly inside dir. This is the only way paths are built from
// theme names, so "../../x" can never escape.
func ThemeFile(dir, slug string) (string, error) {
	if err := theme.ValidateSlug(slug); err != nil {
		return "", err
	}
	p := filepath.Join(dir, slug+themeExt)
	if filepath.Dir(p) != filepath.Clean(dir) {
		return "", fmt.Errorf("%w: resolves outside %s", theme.ErrInvalidSlug, dir)
	}
	return p, nil
}
