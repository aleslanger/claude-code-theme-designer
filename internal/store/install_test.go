package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func yes(string) (bool, error) { return true, nil }
func no(string) (bool, error)  { return false, nil }

func TestInstallCreatesMissingClaudeAndThemesDirs(t *testing.T) {
	s := newStore(t)
	res, err := s.Install("prompt-contrast", []byte(themeA), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.CreatedDir || res.Replaced || res.BackupPath != "" {
		t.Fatalf("unexpected result %+v", res)
	}
	if read(t, filepath.Join(s.Paths.ThemesDir, "prompt-contrast.json")) != themeA {
		t.Fatal("theme not written")
	}
	if _, ok := s.Config.Managed["prompt-contrast"]; !ok {
		t.Fatal("install must be recorded as managed")
	}
	reopened, err := Open(s.Paths)
	if err != nil || reopened.Config.Managed["prompt-contrast"].SHA256 != Fingerprint([]byte(themeA)) {
		t.Fatalf("managed record not persisted: %v", err)
	}
}

func TestInstallRejectsInvalidThemeAndName(t *testing.T) {
	s := newStore(t)
	if _, err := s.Install("x", []byte(`{"hooks":"rm -rf ~"}`), InstallOptions{Force: true}); err == nil {
		t.Fatal("invalid theme must be rejected even with --force")
	}
	if _, err := s.Install("../../something", []byte(themeA), InstallOptions{Force: true}); err == nil {
		t.Fatal("traversal name must be rejected")
	}
	if _, err := os.Stat(filepath.Join(s.Paths.ClaudeDir, "..", "something.json")); err == nil {
		t.Fatal("file written outside the theme directory")
	}
}

func TestInstallCollisionNeverOverwritesWithoutConsent(t *testing.T) {
	s := newStore(t)
	if _, err := s.Install("t", []byte(themeA), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	for name, opts := range map[string]InstallOptions{"no confirm func": {}, "declined": {Confirm: no}} {
		if _, err := s.Install("t", []byte(themeB), opts); !errors.Is(err, ErrExists) {
			t.Fatalf("%s: want ErrExists, got %v", name, err)
		}
	}
	if read(t, filepath.Join(s.Paths.ThemesDir, "t.json")) != themeA {
		t.Fatal("existing theme was overwritten without consent")
	}
}

func TestInstallOverwriteCreatesBackupOutsideThemesDir(t *testing.T) {
	s := newStore(t)
	if _, err := s.Install("t", []byte(themeA), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Install("t", []byte(themeB), InstallOptions{Confirm: yes})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Replaced || res.BackupPath == "" {
		t.Fatalf("unexpected result %+v", res)
	}
	if read(t, res.BackupPath) != themeA || read(t, res.Path) != themeB {
		t.Fatal("backup or new content wrong")
	}
	if strings.HasPrefix(res.BackupPath, s.Paths.ThemesDir) {
		t.Fatal("backup inside the theme dir would be loaded by Claude Code as a theme")
	}
	if _, err := s.Install("t", []byte(themeA), InstallOptions{Force: true}); err != nil {
		t.Fatalf("--force must replace: %v", err)
	}
}

func TestInstallOverwritesForeignThemeOnlyWithConsentAndBacksItUp(t *testing.T) {
	s := newStore(t)
	if err := os.MkdirAll(s.Paths.ThemesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(s.Paths.ThemesDir, "mine.json")
	if err := os.WriteFile(foreign, []byte(`{"name":"hand made"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Install("mine", []byte(themeA), InstallOptions{}); !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
	res, err := s.Install("mine", []byte(themeA), InstallOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if read(t, res.BackupPath) != `{"name":"hand made"}` {
		t.Fatal("foreign theme not backed up")
	}
}

func TestInstallIdenticalContentIsNoOp(t *testing.T) {
	s := newStore(t)
	if _, err := s.Install("t", []byte(themeA), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Install("t", []byte(themeA), InstallOptions{})
	if err != nil || !res.Unchanged || res.BackupPath != "" {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestInstallRefusesSymlinkDestination(t *testing.T) {
	s := newStore(t)
	if err := os.MkdirAll(s.Paths.ThemesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(t.TempDir(), "victim.txt")
	if err := os.WriteFile(victim, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(s.Paths.ThemesDir, "t.json")); err != nil {
		t.Skip("symlinks unsupported")
	}
	if _, err := s.Install("t", []byte(themeA), InstallOptions{Force: true}); !errors.Is(err, ErrSymlink) {
		t.Fatalf("want ErrSymlink, got %v", err)
	}
	if read(t, victim) != "precious" {
		t.Fatal("symlink target was modified")
	}
}

func TestInstallAcceptsSymlinkedClaudeDir(t *testing.T) {
	s := newStore(t)
	real := t.TempDir()
	if err := os.Symlink(real, s.Paths.ClaudeDir); err != nil {
		t.Skip("symlinks unsupported")
	}
	if _, err := s.Install("t", []byte(themeA), InstallOptions{}); err != nil {
		t.Fatalf("dotfile-managed ~/.claude symlink must work: %v", err)
	}
	if read(t, filepath.Join(real, "themes", "t.json")) != themeA {
		t.Fatal("not written through the directory symlink")
	}
}

func TestInstallReportsReadOnlyThemesDir(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod cannot make a directory read-only here (Windows ACLs / root)")
	}
	s := newStore(t)
	if err := os.MkdirAll(s.Paths.ThemesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(s.Paths.ThemesDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(s.Paths.ThemesDir, 0o700) })
	if _, err := s.Install("t", []byte(themeA), InstallOptions{}); err == nil {
		t.Fatal("write into read-only dir must fail")
	}
}

func TestUninstallRemovesManagedThemeWithBackup(t *testing.T) {
	s := newStore(t)
	if _, err := s.Install("t", []byte(themeA), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := s.Uninstall("t", UninstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(res.Path); !os.IsNotExist(err) {
		t.Fatal("theme not removed")
	}
	if read(t, res.BackupPath) != themeA {
		t.Fatal("no backup before removal")
	}
	if _, ok := s.Config.Managed["t"]; ok {
		t.Fatal("managed record not cleared")
	}
}

func TestUninstallRefusesForeignTheme(t *testing.T) {
	s := newStore(t)
	if err := os.MkdirAll(s.Paths.ThemesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(s.Paths.ThemesDir, "theirs.json")
	if err := os.WriteFile(foreign, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Uninstall("theirs", UninstallOptions{Force: true}); !errors.Is(err, ErrNotManaged) {
		t.Fatalf("want ErrNotManaged even with --force, got %v", err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("foreign theme was removed")
	}
}

func TestUninstallRefusesExternallyModifiedThemeUnlessForced(t *testing.T) {
	s := newStore(t)
	res, err := s.Install("t", []byte(themeA), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(res.Path, []byte(themeB), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Uninstall("t", UninstallOptions{}); !errors.Is(err, ErrModified) {
		t.Fatalf("want ErrModified, got %v", err)
	}
	un, err := s.Uninstall("t", UninstallOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if read(t, un.BackupPath) != themeB {
		t.Fatal("forced uninstall must back up the edited content")
	}
}

func TestUninstallMissingTheme(t *testing.T) {
	if _, err := newStore(t).Uninstall("nope", UninstallOptions{}); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("want ErrNotInstalled, got %v", err)
	}
}

func TestUninstallClearsStaleManagedEntry(t *testing.T) {
	s := newStore(t)
	res, err := s.Install("t", []byte(themeA), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(res.Path); err != nil { // user deletes it by hand
		t.Fatal(err)
	}
	if _, err := s.Uninstall("t", UninstallOptions{}); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("want ErrNotInstalled, got %v", err)
	}
	if _, ok := s.Config.Managed["t"]; ok {
		t.Fatal("stale managed entry kept")
	}
	if err := os.WriteFile(res.Path, []byte(`{"name":"hand"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Uninstall("t", UninstallOptions{Force: true}); !errors.Is(err, ErrNotManaged) {
		t.Fatalf("a later hand-made file must never be treated as ours, got %v", err)
	}
}

func TestIdenticalReinstallRefreshesFingerprint(t *testing.T) {
	s := newStore(t)
	res, err := s.Install("t", []byte(themeA), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(res.Path, []byte(themeB), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Install("t", []byte(themeB), InstallOptions{}); err != nil {
		t.Fatal(err)
	}
	if s.Config.Managed["t"].SHA256 != Fingerprint([]byte(themeB)) {
		t.Fatal("fingerprint not refreshed")
	}
	if _, err := s.Uninstall("t", UninstallOptions{}); err != nil {
		t.Fatalf("uninstall must not need --force now: %v", err)
	}
}
