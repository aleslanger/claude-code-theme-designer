// Package cli implements the claude-theme command line. All I/O goes through
// Env so every command is testable against a temporary HOME.
package cli

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/aleslanger/claude-code-theme-designer/internal/app"
	"github.com/aleslanger/claude-code-theme-designer/internal/registry"
	"github.com/aleslanger/claude-code-theme-designer/internal/render"
	"github.com/aleslanger/claude-code-theme-designer/internal/store"
	"github.com/aleslanger/claude-code-theme-designer/internal/theme"
)

// Exit codes.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// Version is set at build time with -ldflags "-X .../cli.Version=v1.2.3".
var Version = "dev"

// TUIFunc starts the fullscreen editor for slug.
type TUIFunc func(a *app.App, slug string, profile render.Profile) error

// Env is everything the CLI needs from the outside world.
type Env struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Getenv         func(string) string
	Home           string
	StdinTTY       bool
	StdoutTTY      bool
	LookPath       func(string) (string, error)
	ClaudeVersion  func(ctx context.Context, path string) (string, error)
	RunTUI         TUIFunc
}

type runner struct {
	env     Env
	app     *app.App
	profile render.Profile
	in      *bufio.Reader
}

const usageText = `Claude Code Theme Designer — design custom themes for Claude Code.
This tool does not patch Claude Code; it writes theme files to ~/.claude/themes/.

Usage:
  claude-theme [--color MODE] [command] [arguments]

Commands:
  (none) | edit [name]     open the fullscreen editor
  preview [name]           print a preview of a theme   [--width N]
  list                     list presets, drafts and installed themes
  show <name>              print the theme JSON that install would write
  install [name]           write a theme to the Claude Code theme directory
                           [--force] [--allow-errors] [--as <slug>]
  uninstall <name>         remove a theme installed by claude-theme  [--force] [--yes]
  import <file>            validate a theme file and save it as a draft
                           [--as <slug>] [--force]
  export <name>            print or save a theme  [-o file] [--force]
  validate <file>          check a theme file  [--strict]
  config [key [value]]     show or set designer settings (terminal-background)
  doctor                   diagnose the environment
  version                  print the version

Names are looked up as drafts first, then presets, then installed themes.
--color MODE: auto (default), truecolor, 256, 16, none.
`

// Run executes the command line and returns the process exit code.
func Run(args []string, env Env) int {
	r := &runner{env: env, in: bufio.NewReader(env.Stdin)}
	global := flag.NewFlagSet("claude-theme", flag.ContinueOnError)
	global.SetOutput(env.Stderr)
	global.Usage = func() {} // printed below, to stdout only when explicitly requested
	colorFlag := global.String("color", "auto", "color mode")
	showVersion := global.Bool("version", false, "print version")
	if err := global.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(env.Stdout, usageText)
			return ExitOK
		}
		fmt.Fprint(env.Stderr, usageText)
		return ExitUsage
	}
	if *showVersion {
		fmt.Fprintln(env.Stdout, "claude-theme", currentVersion())
		return ExitOK
	}
	profile, explicit, ok := render.ParseProfile(*colorFlag)
	if !ok {
		fmt.Fprintf(env.Stderr, "claude-theme: invalid --color %q\n", theme.Sanitize(*colorFlag))
		return ExitUsage
	}
	if !explicit {
		profile = render.DetectProfile(env.Getenv, env.StdoutTTY)
	}
	r.profile = profile

	rest := global.Args()
	cmd, cmdArgs := "edit", rest
	if len(rest) > 0 {
		cmd, cmdArgs = rest[0], rest[1:]
	}
	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprint(env.Stdout, usageText)
		return ExitOK
	case "version":
		fmt.Fprintln(env.Stdout, "claude-theme", currentVersion())
		return ExitOK
	}
	if err := r.open(); err != nil {
		return r.fail(err)
	}
	handler, ok := r.commands()[cmd]
	if !ok {
		fmt.Fprintf(env.Stderr, "claude-theme: unknown command %q\n\n%s", theme.Sanitize(cmd), usageText)
		return ExitUsage
	}
	code := handler(cmdArgs)
	for _, w := range r.app.Store.TakeWarnings() {
		r.warnf("%v", w)
	}
	return code
}

func (r *runner) commands() map[string]func([]string) int {
	return map[string]func([]string) int{
		"edit": r.cmdEdit, "preview": r.cmdPreview, "list": r.cmdList, "show": r.cmdShow,
		"install": r.cmdInstall, "uninstall": r.cmdUninstall, "import": r.cmdImport,
		"export": r.cmdExport, "validate": r.cmdValidate, "config": r.cmdConfig, "doctor": r.cmdDoctor,
	}
}

func (r *runner) open() error {
	reg, err := registry.Default()
	if err != nil {
		return err
	}
	paths, err := store.ResolvePaths(r.env.Home, r.env.Getenv)
	if err != nil {
		return err
	}
	st, err := store.Open(paths)
	if err != nil {
		return err
	}
	switch st.ConfigStatus {
	case store.ConfigCorrupt:
		r.warnf("designer config %s is corrupt; using defaults (it will be backed up before the next save)", paths.ConfigFile)
	case store.ConfigNewer:
		r.warnf("designer config %s was written by a newer claude-theme; it will not be modified", paths.ConfigFile)
	}
	r.app = app.New(reg, st)
	return nil
}

func usageExit(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return ExitOK
	}
	return ExitUsage
}

// fail prints an error and returns ExitError. Messages are sanitized because
// they may quote untrusted file content or names.
func (r *runner) fail(err error) int {
	fmt.Fprintln(r.env.Stderr, "claude-theme: "+theme.Sanitize(err.Error()))
	return ExitError
}

func (r *runner) warnf(format string, args ...any) {
	fmt.Fprintf(r.env.Stderr, "warning: "+format+"\n", sanitizeArgs(args)...)
}

// printf formats to stdout. The format string is trusted program text; every
// argument may carry untrusted data and is sanitized before formatting.
func (r *runner) printf(format string, args ...any) {
	fmt.Fprintf(r.env.Stdout, format, sanitizeArgs(args)...)
}

func sanitizeArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case string:
			out[i] = theme.Sanitize(v)
		case error:
			out[i] = theme.Sanitize(v.Error())
		case fmt.Stringer:
			out[i] = theme.Sanitize(v.String())
		case []string:
			s := make([]string, len(v))
			for j, x := range v {
				s[j] = theme.Sanitize(x)
			}
			out[i] = s
		default:
			out[i] = a
		}
	}
	return out
}

// confirm asks a yes/no question on an interactive stdin. Non-interactive
// sessions always get "no", so scripts must pass --force / --yes explicitly.
func (r *runner) confirm(question string) (bool, error) {
	if !r.env.StdinTTY {
		return false, nil
	}
	fmt.Fprint(r.env.Stderr, theme.Sanitize(question)+" [y/N] ")
	line, err := r.in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read answer: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// tilde shortens paths under HOME for display, using the OS separator.
func (r *runner) tilde(path string) string {
	prefix := r.env.Home + string(filepath.Separator)
	if r.env.Home != "" && strings.HasPrefix(path, prefix) {
		return "~" + path[len(r.env.Home):]
	}
	return path
}

// parseFlags parses subcommand flags allowing them after positional args.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return positional, nil
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("claude-theme "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}
