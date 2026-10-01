package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/aleslanger/claude-code-theme-designer/internal/serialize"
)

var (
	ErrNotInstalled = errors.New("theme is not installed")
	ErrNotManaged   = errors.New("theme was not installed by claude-theme; refusing to touch it")
	ErrModified     = errors.New("theme was modified after installation")
	ErrDeclined     = errors.New("overwrite declined")
)

const claudeDirPerm = 0o700

// InstallOptions control Install.
type InstallOptions struct {
	// Force replaces an existing theme without asking.
	Force bool
	// Confirm is asked before replacing an existing theme when Force is false.
	// nil means "never replace".
	Confirm func(path string) (bool, error)
}

// InstallResult reports what Install did.
type InstallResult struct {
	Path       string
	BackupPath string // set when an existing file was replaced
	Replaced   bool
	Unchanged  bool // identical content was already installed
	CreatedDir bool // the theme directory was created; Claude Code needs one restart
	ConfigErr  error
}

// Install writes a theme into the Claude Code theme directory.
//
// The data is re-validated here (defense in depth). An existing file is never
// replaced without Force or a positive Confirm, a symlink at the destination
// is refused, and the previous content is backed up before replacement.
func (s *Store) Install(slug string, data []byte, opts InstallOptions) (InstallResult, error) {
	path, err := ThemeFile(s.Paths.ThemesDir, slug)
	if err != nil {
		return InstallResult{}, err
	}
	if _, err := serialize.Decode(data); err != nil {
		return InstallResult{}, err
	}
	res := InstallResult{Path: path}
	if _, err := EnsureDir(s.Paths.ClaudeDir, claudeDirPerm); err != nil {
		return res, err
	}
	if res.CreatedDir, err = EnsureDir(s.Paths.ThemesDir, claudeDirPerm); err != nil {
		return res, err
	}
	exists, err := checkTarget(path)
	if err != nil {
		return res, err
	}
	if !exists {
		if err := WriteFileAtomic(path, data, WriteOptions{Perm: themePerm, NoClobber: true}); err != nil && !s.tolerate(err) {
			return res, err
		}
		res.ConfigErr = s.recordManaged(slug, data)
		return res, nil
	}
	return s.replace(slug, path, data, opts, res)
}

func (s *Store) replace(slug, path string, data []byte, opts InstallOptions, res InstallResult) (InstallResult, error) {
	current, err := ReadFileLimited(path, maxBackupBytes, true)
	if err != nil {
		return res, err
	}
	if bytes.Equal(current, data) {
		// Nothing to write. An identical unmanaged file is not adopted, so a
		// later uninstall still cannot remove a file this tool did not write;
		// a managed one gets its fingerprint refreshed.
		res.Unchanged = true
		if m, managed := s.Config.Managed[slug]; managed && m.SHA256 != Fingerprint(data) {
			res.ConfigErr = s.recordManaged(slug, data)
		}
		return res, nil
	}
	if !opts.Force {
		ok, err := confirm(opts.Confirm, path)
		if err != nil {
			return res, err
		}
		if !ok {
			return res, fmt.Errorf("%s: %w (use --force to replace it)", path, ErrExists)
		}
	}
	if res.BackupPath, err = s.backupData(slug, current); err != nil {
		return res, err
	}
	if err := WriteFileAtomic(path, data, WriteOptions{Perm: themePerm}); err != nil && !s.tolerate(err) {
		return res, err
	}
	res.Replaced = true
	res.ConfigErr = s.recordManaged(slug, data)
	return res, nil
}

func confirm(fn func(string) (bool, error), path string) (bool, error) {
	if fn == nil {
		return false, nil
	}
	return fn(path)
}

func (s *Store) recordManaged(slug string, data []byte) error {
	entry := ManagedTheme{SHA256: Fingerprint(data), InstalledAt: s.now().UTC()}
	return s.UpdateConfig(func(c *Config) { c.Managed[slug] = entry })
}

func (s *Store) forgetManaged(slug string) error {
	return s.UpdateConfig(func(c *Config) { delete(c.Managed, slug) })
}

// UninstallOptions control Uninstall.
type UninstallOptions struct {
	// Force removes a managed theme even if it was edited after installation.
	// It never allows removing a theme this tool did not install.
	Force bool
}

// UninstallResult reports what Uninstall did.
type UninstallResult struct {
	Path       string
	BackupPath string
	ConfigErr  error
}

// Uninstall removes a theme installed by this tool, after backing it up. The
// bytes that are fingerprinted are the bytes that are backed up.
func (s *Store) Uninstall(slug string, opts UninstallOptions) (UninstallResult, error) {
	path, err := ThemeFile(s.Paths.ThemesDir, slug)
	if err != nil {
		return UninstallResult{}, err
	}
	res := UninstallResult{Path: path}
	exists, err := checkTarget(path)
	if err != nil {
		return res, err
	}
	m, managed := s.Config.Managed[slug]
	if !exists {
		if managed {
			// Drop the stale record so a later hand-made file with the same
			// name is never mistaken for one of ours.
			res.ConfigErr = s.forgetManaged(slug)
		}
		return res, fmt.Errorf("%s: %w", slug, ErrNotInstalled)
	}
	if !managed {
		return res, fmt.Errorf("%s: %w", path, ErrNotManaged)
	}
	data, err := ReadFileLimited(path, maxBackupBytes, true)
	if err != nil {
		return res, err
	}
	if Fingerprint(data) != m.SHA256 && !opts.Force {
		return res, fmt.Errorf("%s: %w (use --force to remove it anyway; a backup is kept)", path, ErrModified)
	}
	if res.BackupPath, err = s.backupData(slug, data); err != nil {
		return res, err
	}
	if testHookBeforeRemove != nil {
		testHookBeforeRemove(path)
	}
	// Re-read right before removing: if the file changed after it was
	// checked and backed up, the backup would not hold what gets deleted.
	if err := ensureUnchanged(path, data); err != nil {
		return res, err
	}
	if err := os.Remove(path); err != nil {
		return res, fmt.Errorf("remove %s: %w", path, err)
	}
	res.ConfigErr = s.forgetManaged(slug)
	return res, nil
}

// testHookBeforeRemove lets tests simulate a concurrent writer. Always nil
// in production.
var testHookBeforeRemove func(path string)

// ErrChangedDuringOperation means a file changed between check and action.
var ErrChangedDuringOperation = errors.New("file changed while the operation was running; nothing was removed, try again")

func ensureUnchanged(path string, want []byte) error {
	now, err := ReadFileLimited(path, maxBackupBytes, true)
	if err != nil {
		return err
	}
	if Fingerprint(now) != Fingerprint(want) {
		return fmt.Errorf("%s: %w", path, ErrChangedDuringOperation)
	}
	return nil
}
