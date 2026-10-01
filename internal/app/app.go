// Package app holds the use cases shared by the CLI and the TUI: resolving a
// theme by name, validating, importing, exporting and installing. It knows
// nothing about terminals.
package app

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/aleslanger/claude-code-theme-designer/internal/presets"
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/serialize"
	"github.com/aleslanger/claude-code-theme-designer/internal/store"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
	"github.com/aleslanger/claude-code-theme-designer/internal/validate"
)

// Source says where a resolved theme came from.
type Source string

const (
	SourceDraft     Source = "draft"
	SourcePreset    Source = "preset"
	SourceInstalled Source = "installed"
)

// ErrNotFound is returned when no draft, preset or installed theme matches.
var ErrNotFound = errors.New("theme not found")

// Loaded is a resolved theme.
type Loaded struct {
	Slug   string
	Source Source
	Theme  theme.Theme
}

// App wires the registry and the store together.
type App struct {
	Reg   *registry.Registry
	Store *store.Store
}

// New creates an App.
func New(reg *registry.Registry, st *store.Store) *App { return &App{Reg: reg, Store: st} }

// Resolve finds a theme by slug: drafts first (your work), then presets, then
// themes already installed in Claude Code.
func (a *App) Resolve(slug string) (Loaded, error) {
	if err := theme.ValidateSlug(slug); err != nil {
		return Loaded{}, err
	}
	if data, err := a.Store.ReadDraft(slug); err == nil {
		return decodeLoaded(slug, SourceDraft, data)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Loaded{}, fmt.Errorf("draft %s: %w", slug, err)
	}
	if p, ok := presets.Find(slug); ok {
		return Loaded{Slug: slug, Source: SourcePreset, Theme: p.Theme}, nil
	}
	if data, err := a.Store.ReadInstalled(slug); err == nil {
		return decodeLoaded(slug, SourceInstalled, data)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Loaded{}, fmt.Errorf("installed theme %s: %w", slug, err)
	}
	return Loaded{}, fmt.Errorf("%w: %s (see `claude-theme list`)", ErrNotFound, slug)
}

func decodeLoaded(slug string, src Source, data []byte) (Loaded, error) {
	t, err := serialize.Decode(data)
	if err != nil {
		return Loaded{}, fmt.Errorf("%s %s: %w", src, slug, err)
	}
	return Loaded{Slug: slug, Source: src, Theme: t}, nil
}

// CurrentSlug returns the slug the user last worked on, or the default preset.
func (a *App) CurrentSlug() string {
	if a.Store.Config.Current != "" {
		return a.Store.Config.Current
	}
	return presets.Default().Slug
}

// ValidateOptions derives validation options from the designer config.
func (a *App) ValidateOptions() validate.Options {
	var opts validate.Options
	if c, err := theme.ParseColor(a.Store.Config.TerminalBackground); err == nil {
		opts.TerminalBackground = &c
	}
	return opts
}

// Check validates a theme semantically.
func (a *App) Check(t theme.Theme) validate.Report {
	return validate.Check(t, a.Reg, a.ValidateOptions())
}

// SaveDraft encodes and stores t as a draft and makes it current. A different
// existing draft under the same name is backed up first (backup path returned).
func (a *App) SaveDraft(slug string, t theme.Theme) (path, backup string, err error) {
	data, err := serialize.Encode(t)
	if err != nil {
		return "", "", err
	}
	if path, backup, err = a.Store.SaveDraft(slug, data); err != nil {
		return "", "", err
	}
	if err := a.Store.UpdateConfig(func(c *store.Config) { c.Current = slug }); err != nil {
		return path, backup, fmt.Errorf("draft saved, but config not updated: %w", err)
	}
	return path, backup, nil
}

// ImportResult describes an import.
type ImportResult struct {
	Slug, Path, BackupPath string
	Report                 validate.Report
}

// Import strictly validates a file and stores it as a draft. Nothing in the
// file is ever executed; only name, base and color overrides are accepted.
func (a *App) Import(file, as string, force bool) (ImportResult, error) {
	data, err := store.ReadFileLimited(file, serialize.MaxFileSize, false)
	if err != nil {
		return ImportResult{}, err
	}
	t, err := serialize.Decode(data)
	if err != nil {
		return ImportResult{}, err
	}
	slug := as
	if slug == "" {
		slug = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	}
	if err := theme.ValidateSlug(slug); err != nil {
		return ImportResult{}, fmt.Errorf("%w (choose one with --as)", err)
	}
	canonical, err := serialize.Encode(t)
	if err != nil {
		return ImportResult{}, err
	}
	path, backup, err := a.Store.ImportDraft(slug, canonical, force)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Slug: slug, Path: path, BackupPath: backup, Report: a.Check(t)}, nil
}

// InstallOutcome combines the install result with the validation report.
type InstallOutcome struct {
	store.InstallResult
	Report validate.Report
}

// ErrValidation means the theme has errors and install was not forced.
var ErrValidation = errors.New("theme has validation errors")

// Install validates and installs t under slug. allowErrors is deliberately
// separate from opts.Force: replacing a file and accepting an unreadable
// theme are different decisions.
func (a *App) Install(slug string, t theme.Theme, opts store.InstallOptions, allowErrors bool) (InstallOutcome, error) {
	report := a.Check(t)
	out := InstallOutcome{Report: report}
	if report.HasErrors() && !allowErrors {
		return out, fmt.Errorf("%w (fix them, or use --allow-errors)", ErrValidation)
	}
	data, err := serialize.Encode(t)
	if err != nil {
		return out, err
	}
	out.InstallResult, err = a.Store.Install(slug, data, opts)
	return out, err
}
