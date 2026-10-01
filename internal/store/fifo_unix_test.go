//go:build unix

package store

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadFileLimitedDoesNotBlockOnFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "evil.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("mkfifo unsupported")
	}
	done := make(chan error, 1)
	go func() { _, err := ReadFileLimited(fifo, 10, true); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrNotRegular) {
			t.Fatalf("want ErrNotRegular, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reading a FIFO blocked")
	}
}
