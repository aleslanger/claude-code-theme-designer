package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// withLink replaces the hard-link primitive for one test.
func withLink(t *testing.T, fn func(string, string) error) {
	t.Helper()
	orig := linkFile
	linkFile = fn
	t.Cleanup(func() { linkFile = orig })
}

func withSyncDir(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := syncDirFunc
	syncDirFunc = fn
	t.Cleanup(func() { syncDirFunc = orig })
}

func linkErr(errno syscall.Errno) func(string, string) error {
	return func(a, b string) error { return &os.LinkError{Op: "link", Old: a, New: b, Err: errno} }
}

func TestNoClobberFallbackWithoutHardLinks(t *testing.T) {
	withLink(t, linkErr(syscall.ENOTSUP))
	path := filepath.Join(t.TempDir(), "x.json")
	if err := WriteFileAtomic(path, []byte("one"), WriteOptions{Perm: 0o644, NoClobber: true}); err != nil {
		t.Fatalf("fallback must publish: %v", err)
	}
	if read(t, path) != "one" {
		t.Fatal("content not published")
	}
	err := WriteFileAtomic(path, []byte("two"), WriteOptions{Perm: 0o644, NoClobber: true})
	if !errors.Is(err, ErrExists) || read(t, path) != "one" {
		t.Fatalf("O_EXCL reservation must refuse an existing file: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestLinkFailureThatIsNotUnsupportedIsReported(t *testing.T) {
	withLink(t, linkErr(syscall.EIO))
	path := filepath.Join(t.TempDir(), "x.json")
	if err := WriteFileAtomic(path, []byte("x"), WriteOptions{Perm: 0o644, NoClobber: true}); err == nil || errors.Is(err, ErrExists) {
		t.Fatalf("I/O error must surface, got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("nothing may be published after a failed link")
	}
}

func TestDirectorySyncFailureIsTypedAndFileIsPublished(t *testing.T) {
	withSyncDir(t, func(string) error { return syscall.EIO })
	path := filepath.Join(t.TempDir(), "x.json")
	err := WriteFileAtomic(path, []byte("x"), WriteOptions{Perm: 0o644})
	if !errors.Is(err, ErrNotDurable) {
		t.Fatalf("want ErrNotDurable, got %v", err)
	}
	if read(t, path) != "x" {
		t.Fatal("file must be published even though durability is unconfirmed")
	}
}

func TestInstallTreatsUnconfirmedDurabilityAsWarning(t *testing.T) {
	s := newStore(t)
	// Fail only for the themes directory so the config still saves normally.
	withSyncDir(t, func(dir string) error {
		if dir == s.Paths.ThemesDir {
			return syscall.EIO
		}
		return nil
	})
	res, err := s.Install("t", []byte(themeA), InstallOptions{})
	if err != nil {
		t.Fatalf("published theme must count as installed: %v", err)
	}
	if _, ok := s.Config.Managed["t"]; !ok || res.ConfigErr != nil {
		t.Fatal("published theme must be recorded as managed")
	}
	if w := s.TakeWarnings(); len(w) != 1 || !errors.Is(w[0], ErrNotDurable) {
		t.Fatalf("expected one durability warning, got %v", w)
	}
	if len(s.TakeWarnings()) != 0 {
		t.Fatal("TakeWarnings must clear the list")
	}
}

func TestUninstallAbortsWhenFileChangesBeforeRemoval(t *testing.T) {
	s := newStore(t)
	res, err := s.Install("t", []byte(themeA), InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	testHookBeforeRemove = func(path string) {
		if err := os.WriteFile(path, []byte(themeB), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { testHookBeforeRemove = nil })

	un, err := s.Uninstall("t", UninstallOptions{})
	if !errors.Is(err, ErrChangedDuringOperation) {
		t.Fatalf("want ErrChangedDuringOperation, got %v", err)
	}
	if read(t, res.Path) != themeB {
		t.Fatal("concurrently written content must not be deleted")
	}
	if read(t, un.BackupPath) != themeA {
		t.Fatal("backup must hold exactly the fingerprinted bytes")
	}
	if _, ok := s.Config.Managed["t"]; !ok {
		t.Fatal("aborted uninstall must keep the managed record")
	}
}

func TestImportDraftNoClobberForceAndBackup(t *testing.T) {
	s := newStore(t)
	if _, _, err := s.ImportDraft("d", []byte(themeA), false); err != nil {
		t.Fatal(err)
	}
	if !s.DraftExists("d") || s.DraftExists("missing") || s.DraftExists("../x") {
		t.Fatal("DraftExists wrong")
	}
	if _, _, err := s.ImportDraft("d", []byte(themeB), false); !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
	path, backup, err := s.ImportDraft("d", []byte(themeB), true)
	if err != nil || read(t, path) != themeB || read(t, backup) != themeA {
		t.Fatalf("forced import must back up and replace: %v", err)
	}
	if _, _, err := s.ImportDraft("x", []byte(`{"hooks":1}`), true); err == nil {
		t.Fatal("invalid theme must be rejected")
	}
	if _, _, err := s.ImportDraft("../x", []byte(themeA), true); err == nil {
		t.Fatal("unsafe name must be rejected")
	}
}

func TestWriteUserFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := WriteUserFile(path, []byte("1"), false); err != nil {
		t.Fatal(err)
	}
	if err := WriteUserFile(path, []byte("2"), false); !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
	if err := WriteUserFile(path, []byte("2"), true); err != nil || read(t, path) != "2" {
		t.Fatalf("force must replace: %v", err)
	}
	victim := filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(victim, link); err != nil {
		t.Skip("symlinks unsupported")
	}
	if err := WriteUserFile(link, []byte("x"), true); !errors.Is(err, ErrSymlink) || read(t, victim) != "keep" {
		t.Fatalf("symlink destination must be refused: %v", err)
	}
}

func TestReadInstalledAndEnsureDirErrors(t *testing.T) {
	s := newStore(t)
	if _, err := s.ReadInstalled("missing"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want not-exist, got %v", err)
	}
	if _, err := s.ReadInstalled("../x"); err == nil {
		t.Fatal("unsafe name must fail")
	}
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureDir(file, 0o700); !errors.Is(err, ErrNotDir) {
		t.Fatalf("want ErrNotDir, got %v", err)
	}
	if created, err := EnsureDir(filepath.Join(file+"-dir", "a"), 0o700); err != nil || !created {
		t.Fatalf("nested create: %v %v", created, err)
	}
}

func TestConfigStatusStringsAndIgnoreUnsupported(t *testing.T) {
	for _, st := range []ConfigStatus{ConfigOK, ConfigMissing, ConfigCorrupt, ConfigNewer} {
		if st.String() == "" {
			t.Errorf("status %d has no description", st)
		}
	}
	if ignoreUnsupported(syscall.EINVAL) != nil || ignoreUnsupported(syscall.EIO) == nil {
		t.Fatal("ignoreUnsupported must drop only unsupported-fsync errors")
	}
}

func TestOpenFailsOnUnreadableConfig(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod cannot revoke read access here (Windows ACLs / root)")
	}
	s := newStore(t)
	writeConfig(t, s, `{"version":1}`)
	if err := os.Chmod(s.Paths.ConfigFile, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(s.Paths); err == nil {
		t.Fatal("permission errors must be reported, not treated as corrupt")
	}
}
