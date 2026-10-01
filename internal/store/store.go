package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aleslanger/claude-code-theme-designer/internal/serialize"
)

const (
	themePerm      = 0o644 // Claude Code must read it; contains no secrets
	backupPerm     = 0o600
	maxBackupBytes = 16 << 20
	backupStamp    = "20060102T150405.000000000Z"
)

// Store bundles the paths with the loaded designer config.
type Store struct {
	Paths        Paths
	Config       Config
	ConfigStatus ConfigStatus
	now          func() time.Time
	warnings     []error
}

// tolerate records a non-fatal ErrNotDurable as a warning and reports whether
// err may be treated as success (the file is in place).
func (s *Store) tolerate(err error) bool {
	if errors.Is(err, ErrNotDurable) {
		s.warnings = append(s.warnings, err)
		return true
	}
	return false
}

// TakeWarnings returns and clears non-fatal problems from recent operations.
func (s *Store) TakeWarnings() []error {
	w := s.warnings
	s.warnings = nil
	return w
}

// Open loads the designer config. A corrupt config is not fatal.
func Open(p Paths) (*Store, error) {
	cfg, status, err := loadConfig(p.ConfigFile)
	if err != nil {
		return nil, err
	}
	return &Store{Paths: p, Config: cfg, ConfigStatus: status, now: time.Now}, nil
}

// SetClock replaces the clock (tests).
func (s *Store) SetClock(now func() time.Time) { s.now = now }

func (s *Store) stamp() string { return s.now().UTC().Format(backupStamp) }

// Fingerprint is the SHA-256 used to recognise files this tool wrote.
func Fingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ---- drafts ----

// SaveDraft stores a work-in-progress theme in the designer's own directory.
// If a draft with different content already exists it is backed up first, so
// saving under another draft's name never loses data.
func (s *Store) SaveDraft(slug string, data []byte) (path, backup string, err error) {
	if path, err = ThemeFile(s.Paths.DraftsDir, slug); err != nil {
		return "", "", err
	}
	if _, err := serialize.Decode(data); err != nil {
		return "", "", err
	}
	if _, err := EnsureDir(s.Paths.DraftsDir, dirPerm); err != nil {
		return "", "", err
	}
	exists, err := checkTarget(path)
	if err != nil {
		return "", "", err
	}
	if exists {
		current, err := ReadFileLimited(path, maxBackupBytes, true)
		if err != nil {
			return "", "", err
		}
		if !bytes.Equal(current, data) {
			if backup, err = s.backupData(slug, current); err != nil {
				return "", "", err
			}
		}
	}
	if err := WriteFileAtomic(path, data, WriteOptions{Perm: configPerm}); err != nil && !s.tolerate(err) {
		return "", backup, err
	}
	return path, backup, nil
}

// DraftExists reports whether a draft named slug exists.
func (s *Store) DraftExists(slug string) bool {
	path, err := ThemeFile(s.Paths.DraftsDir, slug)
	if err != nil {
		return false
	}
	exists, _ := checkTarget(path)
	return exists
}

// ImportDraft stores an imported theme as a new draft. An existing draft is
// kept unless force is set, in which case it is backed up first.
func (s *Store) ImportDraft(slug string, data []byte, force bool) (path, backup string, err error) {
	if path, err = ThemeFile(s.Paths.DraftsDir, slug); err != nil {
		return "", "", err
	}
	if _, err := serialize.Decode(data); err != nil {
		return "", "", err
	}
	if _, err := EnsureDir(s.Paths.DraftsDir, dirPerm); err != nil {
		return "", "", err
	}
	exists, err := checkTarget(path)
	if err != nil {
		return "", "", err
	}
	if exists {
		if !force {
			return "", "", fmt.Errorf("draft %s: %w (use --force or --as <name>)", slug, ErrExists)
		}
		if backup, err = s.backup(slug, path); err != nil {
			return "", "", err
		}
	}
	if err := WriteFileAtomic(path, data, WriteOptions{Perm: configPerm, NoClobber: !exists}); err != nil && !s.tolerate(err) {
		return "", backup, err
	}
	return path, backup, nil
}

// WriteUserFile writes an export to a path the user chose. It never follows
// a symlink at the destination and replaces an existing file only with force.
func WriteUserFile(path string, data []byte, force bool) error {
	exists, err := checkTarget(path)
	if err != nil {
		return err
	}
	if exists && !force {
		return fmt.Errorf("%s: %w (use --force to replace it)", path, ErrExists)
	}
	return WriteFileAtomic(path, data, WriteOptions{Perm: themePerm, NoClobber: !exists})
}

// ReadDraft returns the raw bytes of a draft.
func (s *Store) ReadDraft(slug string) ([]byte, error) {
	path, err := ThemeFile(s.Paths.DraftsDir, slug)
	if err != nil {
		return nil, err
	}
	return ReadFileLimited(path, serialize.MaxFileSize, true)
}

// ReadInstalled returns the raw bytes of an installed theme.
func (s *Store) ReadInstalled(slug string) ([]byte, error) {
	path, err := ThemeFile(s.Paths.ThemesDir, slug)
	if err != nil {
		return nil, err
	}
	return ReadFileLimited(path, serialize.MaxFileSize, true)
}

// ListDrafts returns draft slugs.
func (s *Store) ListDrafts() ([]string, error) { return listSlugs(s.Paths.DraftsDir) }

// ---- installed themes ----

// Entry describes a file in the Claude Code theme directory.
type Entry struct {
	Slug     string
	Path     string
	Managed  bool // installed by this tool
	Modified bool // managed, but changed since installation
	Symlink  bool
	ReadErr  error
}

// ListInstalled inspects every *.json in the theme directory without
// following symlinks.
func (s *Store) ListInstalled() ([]Entry, error) {
	slugs, err := listSlugs(s.Paths.ThemesDir)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(slugs))
	for _, slug := range slugs {
		out = append(out, s.inspect(slug))
	}
	return out, nil
}

func (s *Store) inspect(slug string) Entry {
	path := filepath.Join(s.Paths.ThemesDir, slug+themeExt)
	e := Entry{Slug: slug, Path: path}
	if st, err := os.Lstat(path); err == nil && st.Mode()&fs.ModeSymlink != 0 {
		e.Symlink = true
	}
	m, managed := s.Config.Managed[slug]
	e.Managed = managed
	data, err := ReadFileLimited(path, serialize.MaxFileSize, true)
	if err != nil {
		e.ReadErr = err
		return e
	}
	e.Modified = managed && Fingerprint(data) != m.SHA256
	return e
}

// listSlugs returns the names of *.json regular files and symlinks in dir.
// A missing directory is an empty list. Names that are not valid slugs are
// still listed (sanitized by the caller) because Claude Code would load them.
func listSlugs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, themeExt) || strings.HasPrefix(name, ".") {
			continue
		}
		out = append(out, strings.TrimSuffix(name, themeExt))
	}
	sort.Strings(out)
	return out, nil
}

// backup copies an existing file into BackupsDir and returns the copy's path.
func (s *Store) backup(slug, path string) (string, error) {
	data, err := ReadFileLimited(path, maxBackupBytes, true)
	if err != nil {
		return "", fmt.Errorf("backup %s: %w", path, err)
	}
	return s.backupData(slug, data)
}

// maxBackupAttempts bounds name-collision retries within one clock tick.
const maxBackupAttempts = 100

// backupData stores data as a new, never-overwritten backup of slug.
func (s *Store) backupData(slug string, data []byte) (string, error) {
	if _, err := EnsureDir(s.Paths.BackupsDir, dirPerm); err != nil {
		return "", err
	}
	base := slug + "." + s.stamp()
	for i := range maxBackupAttempts {
		name := base
		if i > 0 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		dst := filepath.Join(s.Paths.BackupsDir, name+themeExt)
		err := WriteFileAtomic(dst, data, WriteOptions{Perm: backupPerm, NoClobber: true})
		if errors.Is(err, ErrExists) {
			continue
		}
		if err != nil && !s.tolerate(err) {
			return "", fmt.Errorf("backup %s: %w", slug, err)
		}
		return dst, nil
	}
	return "", fmt.Errorf("backup %s: %w", slug, ErrExists)
}
