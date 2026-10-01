package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, s *Store, content string) {
	t.Helper()
	if err := os.MkdirAll(s.Paths.ConfigDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Paths.ConfigFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMissingConfigUsesDefaults(t *testing.T) {
	s := newStore(t)
	if s.ConfigStatus != ConfigMissing || s.Config.Version != configVersion {
		t.Fatalf("got %v %+v", s.ConfigStatus, s.Config)
	}
}

func TestCorruptOrPoisonedConfigFallsBackAndIsPreserved(t *testing.T) {
	cases := map[string]string{
		"not json":         "{{{",
		"traversal":        `{"version":1,"current":"../../etc/passwd"}`,
		"bad color":        `{"version":1,"terminalBackground":"\u001b[31m"}`,
		"bad managed slug": `{"version":1,"managed":{"../x":{"sha256":"` + strings.Repeat("a", 64) + `"}}}`,
		"bad fingerprint":  `{"version":1,"managed":{"x":{"sha256":"zz"}}}`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			writeConfig(t, s, content)
			reopened, err := Open(s.Paths)
			if err != nil {
				t.Fatal(err)
			}
			if reopened.ConfigStatus != ConfigCorrupt || reopened.Config.Current != "" {
				t.Fatalf("got %v %+v", reopened.ConfigStatus, reopened.Config)
			}
			if err := reopened.UpdateConfig(func(*Config) {}); err != nil {
				t.Fatal(err)
			}
			matches, _ := filepath.Glob(s.Paths.ConfigFile + ".corrupt-*")
			if len(matches) != 1 || read(t, matches[0]) != content {
				t.Fatalf("corrupt config not preserved aside: %v", matches)
			}
		})
	}
}

func TestUpdateConfigRefusesInvalidValues(t *testing.T) {
	s := newStore(t)
	if err := s.UpdateConfig(func(c *Config) { c.Current = "../evil" }); err == nil {
		t.Fatal("invalid config must not be written")
	}
}

func TestNewerConfigIsNeverOverwritten(t *testing.T) {
	s := newStore(t)
	writeConfig(t, s, `{"version":99,"futureField":true}`)
	reopened, err := Open(s.Paths)
	if err != nil || reopened.ConfigStatus != ConfigNewer {
		t.Fatalf("got %v %v", reopened.ConfigStatus, err)
	}
	if err := reopened.UpdateConfig(func(*Config) {}); !errors.Is(err, ErrConfigNewer) {
		t.Fatalf("want ErrConfigNewer, got %v", err)
	}
	if read(t, s.Paths.ConfigFile) != `{"version":99,"futureField":true}` {
		t.Fatal("newer config was modified")
	}
}

func TestConcurrentStoresDoNotLoseManagedEntries(t *testing.T) {
	a := newStore(t)
	b, err := Open(a.Paths) // second process with a stale in-memory config
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Install("one", []byte(themeA), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Install("two", []byte(themeB), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	final, err := Open(a.Paths)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := final.Config.Managed["one"]; !ok {
		t.Fatal("entry recorded by the other process was lost")
	}
	if _, ok := final.Config.Managed["two"]; !ok {
		t.Fatal("own entry missing")
	}
}

func TestSaveDraftBacksUpDifferentExistingDraft(t *testing.T) {
	s := newStore(t)
	if _, _, err := s.SaveDraft("d", []byte(themeA)); err != nil {
		t.Fatal(err)
	}
	_, backup, err := s.SaveDraft("d", []byte(themeA))
	if err != nil || backup != "" {
		t.Fatalf("identical save must not back up: %q %v", backup, err)
	}
	_, backup, err = s.SaveDraft("d", []byte(themeB))
	if err != nil || backup == "" || read(t, backup) != themeA {
		t.Fatalf("overwritten draft must be backed up: %q %v", backup, err)
	}
}

func TestBackupNamesNeverCollide(t *testing.T) {
	s := newStore(t)
	frozen := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return frozen })
	first, err := s.backupData("x", []byte("1"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.backupData("x", []byte("2"))
	if err != nil || first == second || read(t, first) != "1" || read(t, second) != "2" {
		t.Fatalf("backups collided: %s %s %v", first, second, err)
	}
}

func TestDraftsRoundTrip(t *testing.T) {
	s := newStore(t)
	if _, _, err := s.SaveDraft("d", []byte(themeA)); err != nil {
		t.Fatal(err)
	}
	b, err := s.ReadDraft("d")
	if err != nil || string(b) != themeA {
		t.Fatalf("got %q %v", b, err)
	}
	slugs, err := s.ListDrafts()
	if err != nil || len(slugs) != 1 || slugs[0] != "d" {
		t.Fatalf("got %v %v", slugs, err)
	}
	if _, _, err := s.SaveDraft("d", []byte(`{"x":1}`)); err == nil {
		t.Fatal("invalid draft must be rejected")
	}
}

func TestListInstalledFlagsManagedModifiedAndSymlinks(t *testing.T) {
	s := newStore(t)
	if _, err := s.Install("ours", []byte(themeA), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Install("edited", []byte(themeA), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Paths.ThemesDir, "edited.json"), []byte(themeB), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Paths.ThemesDir, "theirs.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink("/etc/hostname", filepath.Join(s.Paths.ThemesDir, "link.json"))
	entries, err := s.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Entry{}
	for _, e := range entries {
		got[e.Slug] = e
	}
	if !got["ours"].Managed || got["ours"].Modified {
		t.Fatalf("ours: %+v", got["ours"])
	}
	if !got["edited"].Modified {
		t.Fatalf("edited: %+v", got["edited"])
	}
	if got["theirs"].Managed {
		t.Fatalf("theirs: %+v", got["theirs"])
	}
	if e, ok := got["link"]; ok && (!e.Symlink || e.ReadErr == nil) {
		t.Fatalf("link must be flagged and not followed: %+v", e)
	}
}
