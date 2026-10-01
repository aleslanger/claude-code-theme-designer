package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/store"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

func newApp(t *testing.T) *App {
	t.Helper()
	reg, err := registry.Default()
	if err != nil {
		t.Fatal(err)
	}
	p, err := store.ResolvePaths(t.TempDir(), func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	return New(reg, st)
}

func TestResolveOrderDraftPresetInstalled(t *testing.T) {
	a := newApp(t)
	if l, err := a.Resolve("nord-like"); err != nil || l.Source != SourcePreset {
		t.Fatalf("preset: %+v %v", l, err)
	}
	if _, _, err := a.SaveDraft("nord-like", theme.Theme{Name: "My Nord"}); err != nil {
		t.Fatal(err)
	}
	if l, _ := a.Resolve("nord-like"); l.Source != SourceDraft || l.Theme.Name != "My Nord" {
		t.Fatalf("draft must shadow preset: %+v", l)
	}
	if err := os.MkdirAll(a.Store.Paths.ThemesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Store.Paths.ThemesDir, "hand.json"), []byte(`{"name":"Hand"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if l, _ := a.Resolve("hand"); l.Source != SourceInstalled {
		t.Fatalf("installed: %+v", l)
	}
	if _, err := a.Resolve("nothing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if _, err := a.Resolve("../x"); err == nil {
		t.Fatal("unsafe name must fail")
	}
}

func TestSaveDraftMakesItCurrent(t *testing.T) {
	a := newApp(t)
	if _, _, err := a.SaveDraft("mine", theme.Theme{}); err != nil {
		t.Fatal(err)
	}
	if a.CurrentSlug() != "mine" {
		t.Fatalf("current = %s", a.CurrentSlug())
	}
}

func TestInstallBlocksUnreadableUnlessForced(t *testing.T) {
	a := newApp(t)
	bad := theme.Theme{Overrides: map[string]theme.Color{"userMessageBackground": theme.MustParseColor("#ffffff")}}
	if _, err := a.Install("bad", bad, store.InstallOptions{}, false); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
	if _, err := a.Install("bad", bad, store.InstallOptions{Force: true}, false); !errors.Is(err, ErrValidation) {
		t.Fatalf("--force (replace) must not imply accepting errors, got %v", err)
	}
	if _, err := a.Install("bad", bad, store.InstallOptions{}, true); err != nil {
		t.Fatalf("allowErrors must allow it: %v", err)
	}
}

func TestTerminalBackgroundFromConfig(t *testing.T) {
	a := newApp(t)
	a.Store.Config.TerminalBackground = "#000000"
	if opts := a.ValidateOptions(); opts.TerminalBackground == nil || opts.TerminalBackground.String() != "#000000" {
		t.Fatal("configured terminal background ignored")
	}
}

func TestImportValidatesAndStoresDraft(t *testing.T) {
	a := newApp(t)
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"overrides":{"futureToken":"#123456"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := a.Import(good, "", false)
	if err != nil || res.Slug != "good" || len(res.Report.Issues) == 0 {
		t.Fatalf("import: %+v %v", res, err)
	}
	if l, err := a.Resolve("good"); err != nil || l.Source != SourceDraft {
		t.Fatalf("imported draft not resolvable: %v", err)
	}
	evil := filepath.Join(dir, "evil.json")
	if err := os.WriteFile(evil, []byte(`{"hooks":{"x":"rm -rf ~"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Import(evil, "", false); err == nil {
		t.Fatal("malicious file must be rejected")
	}
	if _, err := a.Import(good, "bad name", false); err == nil {
		t.Fatal("unsafe --as must be rejected")
	}
	if _, err := a.Import(filepath.Join(dir, "missing.json"), "", false); err == nil {
		t.Fatal("missing file must fail")
	}
}

func TestResolveReportsCorruptDraft(t *testing.T) {
	a := newApp(t)
	if err := os.MkdirAll(a.Store.Paths.DraftsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Store.Paths.DraftsDir, "broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Resolve("broken"); err == nil {
		t.Fatal("corrupt draft must be reported")
	}
}

func TestCurrentSlugDefaultsToPreset(t *testing.T) {
	if got := newApp(t).CurrentSlug(); got != "prompt-contrast" {
		t.Fatalf("got %s", got)
	}
}

func TestSaveDraftRejectsInvalidTheme(t *testing.T) {
	a := newApp(t)
	if _, _, err := a.SaveDraft("x", theme.Theme{Name: "bad\x1b"}); err == nil {
		t.Fatal("invalid theme must not be saved")
	}
	if _, _, err := a.SaveDraft("../x", theme.Theme{}); err == nil {
		t.Fatal("unsafe name must not be saved")
	}
}
