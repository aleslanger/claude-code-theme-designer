package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"time"

	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

const (
	configVersion  = 1
	maxConfigBytes = 1 << 20
	configPerm     = 0o600
	dirPerm        = 0o700
)

var reSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Config is the designer's own state. It never goes into a theme file.
type Config struct {
	Version int `json:"version"`
	// Current is the draft the TUI opens by default.
	Current string `json:"current,omitempty"`
	// TerminalBackground improves terminal contrast checks (Claude Code does
	// not paint the terminal background, so it cannot be detected reliably).
	TerminalBackground string `json:"terminalBackground,omitempty"`
	// Managed records themes this tool installed, keyed by slug, so uninstall
	// only ever touches its own files.
	Managed map[string]ManagedTheme `json:"managed,omitempty"`
}

// ManagedTheme fingerprints an installed theme to detect external edits.
type ManagedTheme struct {
	SHA256      string    `json:"sha256"`
	InstalledAt time.Time `json:"installedAt"`
}

// ConfigStatus describes how the config was loaded.
type ConfigStatus int

const (
	ConfigOK ConfigStatus = iota
	ConfigMissing
	// ConfigCorrupt means the file exists but is unreadable as a config. The
	// defaults are used and the bad file is moved aside before the next save.
	ConfigCorrupt
	// ConfigNewer means a newer claude-theme wrote the config. It is read as
	// defaults and never overwritten by this (older) version.
	ConfigNewer
)

// ErrConfigNewer refuses to overwrite a config written by a newer version.
var ErrConfigNewer = errors.New("designer config was written by a newer claude-theme; refusing to overwrite it")

func (s ConfigStatus) String() string {
	switch s {
	case ConfigMissing:
		return "missing (defaults)"
	case ConfigCorrupt:
		return "corrupt (defaults in use; will be backed up on next save)"
	case ConfigNewer:
		return "written by a newer claude-theme (read-only for this version)"
	default:
		return "ok"
	}
}

func defaultConfig() Config {
	return Config{Version: configVersion, Managed: map[string]ManagedTheme{}}
}

// loadConfig reads and validates the config. Permission and I/O errors are
// returned; malformed content yields defaults with ConfigCorrupt.
func loadConfig(path string) (Config, ConfigStatus, error) {
	data, err := ReadFileLimited(path, maxConfigBytes, true)
	if errors.Is(err, fs.ErrNotExist) {
		return defaultConfig(), ConfigMissing, nil
	}
	if errors.Is(err, ErrTooLarge) || errors.Is(err, ErrNotRegular) || errors.Is(err, ErrSymlink) {
		return defaultConfig(), ConfigCorrupt, nil
	}
	if err != nil {
		return defaultConfig(), ConfigOK, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return defaultConfig(), ConfigCorrupt, nil
	}
	if c.Version > configVersion {
		return defaultConfig(), ConfigNewer, nil
	}
	if err := c.validate(); err != nil {
		return defaultConfig(), ConfigCorrupt, nil
	}
	if c.Managed == nil {
		c.Managed = map[string]ManagedTheme{}
	}
	return c, ConfigOK, nil
}

// validate rejects poisoned values (e.g. a "current" of "../../x").
func (c Config) validate() error {
	if c.Version != configVersion {
		return fmt.Errorf("unsupported config version %d", c.Version)
	}
	if c.Current != "" {
		if err := theme.ValidateSlug(c.Current); err != nil {
			return err
		}
	}
	if c.TerminalBackground != "" {
		if _, err := theme.ParseColor(c.TerminalBackground); err != nil {
			return err
		}
	}
	for slug, m := range c.Managed {
		if err := theme.ValidateSlug(slug); err != nil {
			return err
		}
		if !reSHA256.MatchString(m.SHA256) {
			return fmt.Errorf("managed theme %s: bad fingerprint", slug)
		}
	}
	return nil
}

func encodeConfig(c Config) ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("refusing to save invalid config: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// UpdateConfig applies fn to the current on-disk config and saves it, under an
// exclusive lock. Reloading first means a long-running TUI cannot discard
// entries another claude-theme process recorded meanwhile.
func (s *Store) UpdateConfig(fn func(*Config)) error {
	if _, err := EnsureDir(s.Paths.ConfigDir, dirPerm); err != nil {
		return err
	}
	unlock, err := lockFile(s.Paths.ConfigFile + ".lock")
	if err != nil {
		return fmt.Errorf("lock config: %w", err)
	}
	defer unlock()

	fresh, status, err := loadConfig(s.Paths.ConfigFile)
	if err != nil {
		return err
	}
	if status == ConfigNewer {
		return ErrConfigNewer
	}
	if status == ConfigCorrupt {
		aside := s.Paths.ConfigFile + ".corrupt-" + s.stamp()
		if err := os.Rename(s.Paths.ConfigFile, aside); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("move corrupt config aside: %w", err)
		}
	}
	fn(&fresh)
	data, err := encodeConfig(fresh)
	if err != nil {
		return err
	}
	if err := WriteFileAtomic(s.Paths.ConfigFile, data, WriteOptions{Perm: configPerm}); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	s.Config, s.ConfigStatus = fresh, ConfigOK
	return nil
}
