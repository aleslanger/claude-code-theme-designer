//go:build !unix

package store

// lockFile is a no-op where flock is unavailable; UpdateConfig still reloads
// before saving, which narrows the lost-update window.
func lockFile(string) (func(), error) { return func() {}, nil }
