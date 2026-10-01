package doctor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aleslanger/claude-code-theme-designer/internal/app"
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/store"
)

func newApp(t *testing.T) *app.App {
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
	return app.New(reg, st)
}

func env(version string) Env {
	return Env{
		Getenv:   func(k string) string { return map[string]string{"TERM": "xterm-256color"}[k] },
		LookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
		Version:  func(context.Context, string) (string, error) { return version + " (Claude Code)\n", nil },
		IsTTY:    true,
	}
}

func find(items []Item, label string) []Item {
	var out []Item
	for _, it := range items {
		if it.Label == label {
			out = append(out, it)
		}
	}
	return out
}

func TestDoctorReportsVersionCompatibility(t *testing.T) {
	a := newApp(t)
	cases := map[string]Level{"2.1.287": OK, "9.0.0": Warn, "2.0.1": Warn}
	for v, want := range cases {
		got := find(Run(context.Background(), a, env(v)), "Compatibility")
		if len(got) != 1 || got[0].Level != want {
			t.Errorf("version %s: got %+v, want level %v", v, got, want)
		}
	}
}

func TestDoctorWithoutClaudeCode(t *testing.T) {
	e := env("")
	e.LookPath = func(string) (string, error) { return "", errors.New("missing") }
	items := Run(context.Background(), newApp(t), e)
	if got := find(items, "Claude Code"); len(got) != 1 || got[0].Level != Warn {
		t.Fatalf("got %+v", got)
	}
}

func TestDoctorWarnsAboutMissingThemeDirAndInvalidThemes(t *testing.T) {
	a := newApp(t)
	items := Run(context.Background(), a, env("2.1.287"))
	if got := find(items, "Theme directory"); len(got) == 0 || got[0].Level != Warn || !strings.Contains(got[0].Value, "Restart") {
		t.Fatalf("missing dir: %+v", got)
	}
	if err := os.MkdirAll(a.Store.Paths.ThemesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Store.Paths.ThemesDir, "broken.json"), []byte(`{"base":"nope"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	items = Run(context.Background(), a, env("2.1.287"))
	if got := find(items, "Theme broken"); len(got) != 1 || got[0].Level != Warn {
		t.Fatalf("invalid theme not reported: %+v", items)
	}
}

func TestVersionFromInstallPathNeedsNoExec(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "claude", "versions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "2.1.300")
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "claude")
	if err := os.Symlink(bin, link); err != nil {
		t.Skip("symlinks unsupported")
	}
	if got := versionFromInstallPath(link); got != "2.1.300" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatAlignsAndSanitizes(t *testing.T) {
	out := Format([]Item{{OK, "A", "x"}, {Warn, "Longer", "y"}})
	if !strings.Contains(out, "✓ A       x") || !strings.Contains(out, "! Longer  y") {
		t.Fatalf("got %q", out)
	}
}
