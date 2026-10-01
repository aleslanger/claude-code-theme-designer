//go:build unix

package store

import (
	"errors"
	"os"
	"syscall"
)

// lockFile takes an exclusive advisory lock, blocking until it is available.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, configPerm)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return func() {
		// Closing the descriptor releases the lock; nothing else to report.
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
