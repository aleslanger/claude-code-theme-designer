package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/aleslanger/claude-code-theme-designer/internal/app"
	"github.com/aleslanger/claude-code-theme-designer/internal/doctor"
	"github.com/aleslanger/claude-code-theme-designer/internal/presets"
	"github.com/aleslanger/claude-code-theme-designer/internal/render"
	"github.com/aleslanger/claude-code-theme-designer/internal/serialize"
	"github.com/aleslanger/claude-code-theme-designer/internal/store"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

const defaultPreviewWidth = 72

func (r *runner) nameArg(args []string, cmd string) (string, bool) {
	switch len(args) {
	case 0:
		return r.app.CurrentSlug(), true
	case 1:
		return args[0], true
	}
	fmt.Fprintf(r.env.Stderr, "claude-theme %s: expected at most one theme name\n", cmd)
	return "", false
}

func (r *runner) requireOne(args []string, cmd, what string) (string, bool) {
	if len(args) != 1 {
		fmt.Fprintf(r.env.Stderr, "claude-theme %s: expected exactly one %s\n", cmd, what)
		return "", false
	}
	return args[0], true
}

func (r *runner) cmdEdit(args []string) int {
	slug, ok := r.nameArg(args, "edit")
	if !ok {
		return ExitUsage
	}
	reason := r.tuiUnavailable()
	if reason == "" {
		if err := r.env.RunTUI(r.app, slug, r.profile); err != nil {
			return r.fail(fmt.Errorf("editor: %w", err))
		}
		return ExitOK
	}
	r.warnf("fullscreen editor unavailable: %s. Showing a preview instead; use the subcommands (see --help) to edit.", reason)
	return r.cmdPreview([]string{slug})
}

func (r *runner) tuiUnavailable() string {
	switch {
	case r.env.RunTUI == nil:
		return "not built in"
	case !r.env.StdinTTY || !r.env.StdoutTTY:
		return "stdin/stdout is not a terminal"
	case r.env.Getenv("TERM") == "dumb" || r.env.Getenv("TERM") == "":
		return "TERM does not support cursor control"
	}
	return ""
}

func (r *runner) cmdPreview(args []string) int {
	fs := newFlagSet("preview", r.env.Stderr)
	width := fs.Int("width", defaultPreviewWidth, "preview width in columns")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return usageExit(err)
	}
	slug, ok := r.nameArg(pos, "preview")
	if !ok {
		return ExitUsage
	}
	l, err := r.app.Resolve(slug)
	if err != nil {
		return r.fail(err)
	}
	t := l.Theme
	r.printf("%s (%s, base %s)\n\n", displayName(l), l.Source, t.EffectiveBase())
	frame := render.Preview(render.Options{Width: *width})
	fmt.Fprintln(r.env.Stdout, render.Encode(frame, r.app.Reg.Resolve(t), r.profile))
	if t.EffectiveBase().IsLight() != r.isLightGuess() {
		fmt.Fprintln(r.env.Stdout, "\nnote: the preview is drawn on your terminal's own background, as Claude Code is.")
	}
	r.printReport(r.app.Check(t))
	return ExitOK
}

// isLightGuess uses the configured terminal background, defaulting to dark.
func (r *runner) isLightGuess() bool {
	c, err := theme.ParseColor(r.app.Store.Config.TerminalBackground)
	if err != nil {
		return false
	}
	return theme.ContrastRatio(c.RGB(), theme.RGB{}) > theme.ContrastRatio(c.RGB(), theme.RGB{R: 255, G: 255, B: 255})
}

func displayName(l app.Loaded) string {
	if l.Theme.Name != "" {
		return fmt.Sprintf("%s [%s]", l.Theme.Name, l.Slug)
	}
	return l.Slug
}

func (r *runner) cmdList(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(r.env.Stderr, "claude-theme list: takes no arguments")
		return ExitUsage
	}
	r.printf("Presets (built in, not official Claude Code themes):\n")
	all := presets.All()
	for _, cat := range presets.Categories {
		r.printf("\n  %s\n", cat)
		for _, p := range all {
			if p.Category == cat {
				r.printf("    %-24s %s\n", p.Slug, p.Description)
			}
		}
	}
	drafts, err := r.app.Store.ListDrafts()
	if err != nil {
		return r.fail(err)
	}
	r.printf("\nDrafts (%s):\n", r.tilde(r.app.Store.Paths.DraftsDir))
	if len(drafts) == 0 {
		r.printf("  (none)\n")
	}
	for _, d := range drafts {
		marker := ""
		if d == r.app.Store.Config.Current {
			marker = "  (current)"
		}
		r.printf("  %s%s\n", d, marker)
	}
	return r.listInstalled()
}

func (r *runner) listInstalled() int {
	entries, err := r.app.Store.ListInstalled()
	if err != nil {
		return r.fail(err)
	}
	r.printf("\nInstalled in Claude Code (%s):\n", r.tilde(r.app.Store.Paths.ThemesDir))
	if len(entries) == 0 {
		r.printf("  (none)\n")
	}
	for _, e := range entries {
		var flags []string
		if e.Managed {
			flags = append(flags, "managed")
		} else {
			flags = append(flags, "not managed by claude-theme")
		}
		if e.Modified {
			flags = append(flags, "edited since install")
		}
		if e.Symlink {
			flags = append(flags, "symlink")
		}
		if e.ReadErr != nil {
			flags = append(flags, "unreadable: "+e.ReadErr.Error())
		}
		r.printf("  %-16s %v\n", e.Slug, flags)
	}
	r.printf("\nSelect an installed theme in Claude Code with /theme.\n")
	return ExitOK
}

func (r *runner) cmdShow(args []string) int {
	slug, ok := r.requireOne(args, "show", "theme name")
	if !ok {
		return ExitUsage
	}
	l, err := r.app.Resolve(slug)
	if err != nil {
		return r.fail(err)
	}
	data, err := serialize.Encode(l.Theme)
	if err != nil {
		return r.fail(err)
	}
	_, _ = r.env.Stdout.Write(data) // JSON encoder escapes control characters
	return ExitOK
}

func (r *runner) cmdInstall(args []string) int {
	fs := newFlagSet("install", r.env.Stderr)
	force := fs.Bool("force", false, "replace an existing theme (a backup is kept)")
	allowErrors := fs.Bool("allow-errors", false, "install even if validation reports errors (e.g. unreadable contrast)")
	as := fs.String("as", "", "install under a different name")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return usageExit(err)
	}
	slug, ok := r.nameArg(pos, "install")
	if !ok {
		return ExitUsage
	}
	l, err := r.app.Resolve(slug)
	if err != nil {
		return r.fail(err)
	}
	target := l.Slug
	if *as != "" {
		target = *as
	}
	out, err := r.app.Install(target, l.Theme, store.InstallOptions{
		Force: *force,
		Confirm: func(path string) (bool, error) {
			return r.confirm(fmt.Sprintf("%s already exists. Replace it? A backup will be kept.", r.tilde(path)))
		},
	}, *allowErrors)
	r.printReport(out.Report)
	if err != nil {
		return r.fail(err)
	}
	r.printInstalled(out)
	return ExitOK
}

func (r *runner) printInstalled(out app.InstallOutcome) {
	if out.Unchanged {
		r.printf("Already installed (identical):\n%s\n", r.tilde(out.Path))
	} else {
		r.printf("Installed:\n%s\n", r.tilde(out.Path))
	}
	if out.BackupPath != "" {
		r.printf("\nPrevious version backed up to:\n%s\n", r.tilde(out.BackupPath))
	}
	r.printf("\nThen select it in Claude Code using:\n/theme\n")
	if out.CreatedDir {
		r.printf("\nNote: %s was just created. Restart Claude Code once so it starts watching it;\nafter that, theme changes apply without a restart.\n", r.tilde(r.app.Store.Paths.ThemesDir))
	}
	if out.ConfigErr != nil {
		r.warnf("theme installed, but recording it as managed failed: %v (uninstall will refuse it)", out.ConfigErr)
	}
}

func (r *runner) cmdUninstall(args []string) int {
	fs := newFlagSet("uninstall", r.env.Stderr)
	force := fs.Bool("force", false, "remove even if edited after install (a backup is kept)")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return usageExit(err)
	}
	slug, ok := r.requireOne(pos, "uninstall", "theme name")
	if !ok {
		return ExitUsage
	}
	path, err := store.ThemeFile(r.app.Store.Paths.ThemesDir, slug)
	if err != nil {
		return r.fail(err)
	}
	if !*yes {
		ok, err := r.confirm(fmt.Sprintf("Remove %s? A backup will be kept.", r.tilde(path)))
		if err != nil {
			return r.fail(err)
		}
		if !ok {
			return r.fail(errors.New("not removed (confirm interactively or pass --yes)"))
		}
	}
	res, err := r.app.Store.Uninstall(slug, store.UninstallOptions{Force: *force})
	if err != nil {
		return r.fail(err)
	}
	r.printf("Removed:\n%s\n\nBackup:\n%s\n\nIf it was selected in Claude Code, choose another theme with /theme.\n", r.tilde(res.Path), r.tilde(res.BackupPath))
	if res.ConfigErr != nil {
		r.warnf("removed, but updating the designer config failed: %v", res.ConfigErr)
	}
	return ExitOK
}

func (r *runner) cmdImport(args []string) int {
	fs := newFlagSet("import", r.env.Stderr)
	as := fs.String("as", "", "draft name (default: file name)")
	force := fs.Bool("force", false, "replace an existing draft (a backup is kept)")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return usageExit(err)
	}
	file, ok := r.requireOne(pos, "import", "file")
	if !ok {
		return ExitUsage
	}
	res, err := r.app.Import(file, *as, *force)
	if err != nil {
		return r.fail(err)
	}
	r.printf("Imported as draft %q:\n%s\n", res.Slug, r.tilde(res.Path))
	if res.BackupPath != "" {
		r.printf("Previous draft backed up to %s\n", r.tilde(res.BackupPath))
	}
	r.printReport(res.Report)
	r.printf("\nReview it with `claude-theme preview %s`, install it with `claude-theme install %s`.\n", res.Slug, res.Slug)
	return ExitOK
}

func (r *runner) cmdExport(args []string) int {
	fs := newFlagSet("export", r.env.Stderr)
	out := fs.String("o", "", "write to file instead of stdout")
	force := fs.Bool("force", false, "replace an existing output file")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return usageExit(err)
	}
	slug, ok := r.requireOne(pos, "export", "theme name")
	if !ok {
		return ExitUsage
	}
	l, err := r.app.Resolve(slug)
	if err != nil {
		return r.fail(err)
	}
	data, err := serialize.Encode(l.Theme)
	if err != nil {
		return r.fail(err)
	}
	if *out == "" {
		_, _ = r.env.Stdout.Write(data)
		return ExitOK
	}
	if err := store.WriteUserFile(*out, data, *force); err != nil {
		return r.fail(err)
	}
	r.printf("Exported %s to %s\n", slug, *out)
	return ExitOK
}

func (r *runner) cmdValidate(args []string) int {
	fs := newFlagSet("validate", r.env.Stderr)
	strict := fs.Bool("strict", false, "treat warnings as failures")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return usageExit(err)
	}
	file, ok := r.requireOne(pos, "validate", "file")
	if !ok {
		return ExitUsage
	}
	data, err := store.ReadFileLimited(file, serialize.MaxFileSize, false)
	if err != nil {
		return r.fail(err)
	}
	t, err := serialize.Decode(data)
	var de *serialize.DecodeError
	if errors.As(err, &de) {
		r.printf("INVALID: %s\n", file)
		for _, p := range de.Problems {
			r.printf("  error   %s\n", p)
		}
		return ExitError
	}
	if err != nil {
		return r.fail(err)
	}
	report := r.app.Check(t)
	r.printReport(report)
	if report.HasErrors() || (*strict && report.Warnings() > 0) {
		r.printf("FAILED: %s\n", file)
		return ExitError
	}
	r.printf("OK: %s (%d overrides, base %s)\n", file, len(t.Overrides), t.EffectiveBase())
	return ExitOK
}

const configKeyTerminalBG = "terminal-background"

func (r *runner) cmdConfig(args []string) int {
	cfg := &r.app.Store.Config // read-only view; writes go through UpdateConfig
	switch len(args) {
	case 0:
		r.printf("config file:          %s\n", r.tilde(r.app.Store.Paths.ConfigFile))
		r.printf("%s:  %s\n", configKeyTerminalBG, orDefault(cfg.TerminalBackground, "(assumed from base)"))
		r.printf("current draft:        %s\n", orDefault(cfg.Current, "(none)"))
		return ExitOK
	case 2:
	default:
		fmt.Fprintf(r.env.Stderr, "usage: claude-theme config [%s <color|unset>]\n", configKeyTerminalBG)
		return ExitUsage
	}
	if args[0] != configKeyTerminalBG {
		fmt.Fprintf(r.env.Stderr, "claude-theme config: unknown key %q\n", theme.Sanitize(args[0]))
		return ExitUsage
	}
	value := ""
	if args[1] != "unset" {
		c, err := theme.ParseColor(args[1])
		if err != nil {
			return r.fail(err)
		}
		value = c.String()
	}
	if err := r.app.Store.UpdateConfig(func(c *store.Config) { c.TerminalBackground = value }); err != nil {
		return r.fail(err)
	}
	r.printf("%s = %s\n", configKeyTerminalBG, orDefault(cfg.TerminalBackground, "(unset)"))
	return ExitOK
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (r *runner) cmdDoctor(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(r.env.Stderr, "claude-theme doctor: takes no arguments")
		return ExitUsage
	}
	items := doctor.Run(context.Background(), r.app, doctor.Env{
		Getenv: r.env.Getenv, LookPath: r.env.LookPath, Version: r.env.ClaudeVersion, IsTTY: r.env.StdoutTTY,
	})
	fmt.Fprint(r.env.Stdout, doctor.Format(items))
	return ExitOK
}
