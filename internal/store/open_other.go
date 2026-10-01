//go:build !unix

package store

import (
	"fmt"
	"io/fs"
	"os"
)

// openForRead without O_NOFOLLOW: checks with Lstat first. There is a small
// race window on these platforms, documented in the threat model.
func openForRead(path string, noFollow bool) (*os.File, error) {
	if noFollow {
		st, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		if st.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s: %w", path, ErrSymlink)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return f, nil
}
