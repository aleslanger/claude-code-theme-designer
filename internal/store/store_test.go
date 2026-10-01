package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	themeA = "{\n  \"name\": \"A\",\n  \"overrides\": {\n    \"text\": \"#ffffff\"\n  }\n}\n"
	themeB = "{\n  \"name\": \"B\",\n  \"overrides\": {\n    \"text\": \"#eeeeee\"\n  }\n}\n"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	home := t.TempDir()
	p, err := ResolvePaths(home, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	tick := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { tick = tick.Add(time.Second); return tick })
	return s
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestResolvePathsKeepsDesignerStateSeparate(t *testing.T) {
	p, err := ResolvePaths("/home/u", func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if p.ThemesDir != "/home/u/.claude/themes" || p.ConfigFile != "/home/u/.config/claude-theme-designer/config.json" {
		t.Fatalf("unexpected paths %+v", p)
	}
	if strings.HasPrefix(p.BackupsDir, p.ThemesDir) || strings.HasPrefix(p.DraftsDir, p.ThemesDir) {
		t.Fatal("backups and drafts must never live in the Claude Code theme directory")
	}
}

func TestResolvePathsHonoursEnvironment(t *testing.T) {
	env := map[string]string{"CLAUDE_CONFIG_DIR": "/opt/claude", "XDG_CONFIG_HOME": "/xdg"}
	p, err := ResolvePaths("/home/u", func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if p.ThemesDir != "/opt/claude/themes" || p.ConfigDir != "/xdg/claude-theme-designer" {
		t.Fatalf("env not honoured: %+v", p)
	}
	if _, err := ResolvePaths("/home/u", func(k string) string { return map[string]string{"CLAUDE_CONFIG_DIR": "rel"}[k] }); err == nil {
		t.Fatal("relative CLAUDE_CONFIG_DIR must be rejected")
	}
}

func TestThemeFileRejectsTraversal(t *testing.T) {
	for _, slug := range []string{"../../something", "..", "a/b", "", "x.json", "/abs"} {
		if _, err := ThemeFile("/tmp/themes", slug); err == nil {
			t.Errorf("ThemeFile(%q) must fail", slug)
		}
	}
	p, err := ThemeFile("/tmp/themes", "ok-name")
	if err != nil || p != "/tmp/themes/ok-name.json" {
		t.Fatalf("got %q, %v", p, err)
	}
}

func TestWriteFileAtomicReplacesAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	for _, content := range []string{"one", "two"} {
		if err := WriteFileAtomic(path, []byte(content), WriteOptions{Perm: 0o644}); err != nil {
			t.Fatal(err)
		}
	}
	if read(t, path) != "two" {
		t.Fatal("content not replaced")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
	if st, _ := os.Stat(path); runtime.GOOS != "windows" && st.Mode().Perm() != 0o644 {
		t.Fatalf("perm = %v", st.Mode().Perm())
	}
}

func TestWriteFileAtomicNoClobber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.json")
	if err := WriteFileAtomic(path, []byte("one"), WriteOptions{Perm: 0o644, NoClobber: true}); err != nil {
		t.Fatal(err)
	}
	err := WriteFileAtomic(path, []byte("two"), WriteOptions{Perm: 0o644, NoClobber: true})
	if !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
	if read(t, path) != "one" {
		t.Fatal("NoClobber overwrote the file")
	}
}

func TestWriteFileAtomicFailsInReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission semantics differ")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	err := WriteFileAtomic(filepath.Join(dir, "x.json"), []byte("x"), WriteOptions{Perm: 0o644})
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("want permission error, got %v", err)
	}
}

func TestReadFileLimited(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	if err := os.WriteFile(path, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFileLimited(path, 4, true); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
	if b, err := ReadFileLimited(path, 5, true); err != nil || string(b) != "12345" {
		t.Fatalf("got %q %v", b, err)
	}
	if _, err := ReadFileLimited(dir, 10, true); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("directory: want ErrNotRegular, got %v", err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Skip("symlinks unsupported")
	}
	if _, err := ReadFileLimited(link, 10, true); !errors.Is(err, ErrSymlink) {
		t.Fatalf("symlink: want ErrSymlink, got %v", err)
	}
}
