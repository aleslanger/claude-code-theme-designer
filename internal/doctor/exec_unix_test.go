//go:build unix

package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeClaude(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExecVersionReadsOutput(t *testing.T) {
	out, err := ExecVersion(context.Background(), fakeClaude(t, `echo "2.1.287 (Claude Code) $1"`))
	if err != nil || !strings.Contains(out, "2.1.287") || !strings.Contains(out, "--version") {
		t.Fatalf("got %q %v", out, err)
	}
}

func TestExecVersionCapsOutputAndTimesOut(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	out, err := ExecVersion(ctx, fakeClaude(t, `while :; do echo xxxxxxxxxxxxxxxx; done`))
	if err == nil {
		t.Fatal("a runaway child must be reported")
	}
	if len(out) > maxVersionOutput {
		t.Fatalf("output not capped: %d bytes", len(out))
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("timeout not enforced")
	}
}

func TestExecVersionMissingBinary(t *testing.T) {
	if _, err := ExecVersion(context.Background(), "/nonexistent/claude"); err == nil {
		t.Fatal("missing binary must fail")
	}
}
