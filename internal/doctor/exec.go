package doctor

import (
	"context"
	"errors"
	"io"
	"os/exec"
)

// maxVersionOutput bounds what is read from `claude --version`.
const maxVersionOutput = 4 << 10

// ExecVersion runs `<path> --version` directly (no shell), bounded by ctx,
// reading at most maxVersionOutput bytes. It is the only subprocess the tool
// ever starts, and only `doctor` uses it.
func ExecVersion(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, path, "--version")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	data, readErr := io.ReadAll(io.LimitReader(out, maxVersionOutput))
	// Wait closes the pipe; a child still writing past the cap is killed by ctx.
	waitErr := cmd.Wait()
	return string(data), errors.Join(readErr, waitErr)
}
