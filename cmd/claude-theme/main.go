// Command claude-theme designs, previews and installs custom Claude Code
// themes. It writes theme JSON files only; it never patches Claude Code.
package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/mattn/go-isatty"

	"github.com/aleslanger/claude-code-theme-designer/internal/cli"
	"github.com/aleslanger/claude-code-theme-designer/internal/doctor"
	"github.com/aleslanger/claude-code-theme-designer/internal/tui"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "claude-theme: cannot determine home directory:", err)
		os.Exit(cli.ExitError)
	}
	os.Exit(cli.Run(os.Args[1:], cli.Env{
		Stdin:         os.Stdin,
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		Getenv:        os.Getenv,
		Home:          home,
		StdinTTY:      isTTY(os.Stdin),
		StdoutTTY:     isTTY(os.Stdout),
		LookPath:      exec.LookPath,
		ClaudeVersion: doctor.ExecVersion,
		RunTUI:        tui.Run,
	}))
}

func isTTY(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}
