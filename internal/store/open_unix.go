//go:build unix

package store

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// openForRead opens path read-only. O_NONBLOCK prevents hanging on a FIFO;
// O_NOFOLLOW makes the kernel refuse a symlink atomically (no TOCTOU window).
func openForRead(path string, noFollow bool) (*os.File, error) {
	flags := os.O_RDONLY | syscall.O_NONBLOCK
	if noFollow {
		flags |= syscall.O_NOFOLLOW
	}
	f, err := os.OpenFile(path, flags, 0)
	if err != nil {
		if noFollow && errors.Is(err, syscall.ELOOP) {
			return nil, fmt.Errorf("%s: %w", path, ErrSymlink)
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return f, nil
}
