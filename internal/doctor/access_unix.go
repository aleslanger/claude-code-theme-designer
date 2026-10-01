//go:build unix

package doctor

import "golang.org/x/sys/unix"

// writability checks write permission without writing anything.
func writability(dir string) string {
	if unix.Access(dir, unix.W_OK) == nil {
		return "writable"
	}
	return "NOT writable"
}
