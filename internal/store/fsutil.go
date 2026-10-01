package store

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

var (
	ErrExists     = errors.New("file already exists")
	ErrSymlink    = errors.New("refusing to follow a symbolic link")
	ErrNotRegular = errors.New("not a regular file")
	ErrTooLarge   = errors.New("file too large")
	ErrNotDir     = errors.New("not a directory")
	// ErrNotDurable means the file was published (readers see the new content)
	// but the directory could not be fsynced, so a crash might roll it back.
	ErrNotDurable = errors.New("written, but durability could not be confirmed")
)

const tempPattern = ".claude-theme-*.tmp" // not *.json, so Claude Code's watcher ignores it

// Seams for tests that simulate filesystems without hard links or without
// directory fsync. Production code never reassigns them.
var (
	linkFile    = os.Link
	syncDirFunc = syncDir
)

// WriteOptions control WriteFileAtomic.
type WriteOptions struct {
	Perm os.FileMode
	// NoClobber fails with ErrExists instead of replacing an existing file.
	// It is enforced atomically with link(2) where the filesystem supports it.
	NoClobber bool
}

// WriteFileAtomic writes data so that readers see either the old or the new
// content, never a partial file: temp file in the same directory, fsync,
// rename (or link for NoClobber), then fsync the directory.
//
// rename(2) replaces the directory entry itself, so a symlink planted at path
// is replaced, never followed; callers still refuse symlinks up front.
func WriteFileAtomic(path string, data []byte, opts WriteOptions) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if rmErr := os.Remove(tmpName); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) && err == nil {
			err = fmt.Errorf("remove temp file %s: %w", tmpName, rmErr)
		}
	}()
	if err := writeAndSync(tmp, data, opts.Perm); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if opts.NoClobber {
		err = linkNoClobber(tmpName, path)
	} else {
		err = os.Rename(tmpName, path)
	}
	if err != nil {
		return err
	}
	if err := syncDirFunc(dir); err != nil {
		return fmt.Errorf("%w: %w", ErrNotDurable, err)
	}
	return nil
}

func writeAndSync(f *os.File, data []byte, perm os.FileMode) error {
	_, werr := f.Write(data)
	serr := ignoreUnsupported(f.Sync())
	cerr := f.Chmod(perm)
	closeErr := f.Close()
	return errors.Join(werr, serr, cerr, closeErr)
}

// linkNoClobber publishes tmp at path only if path does not exist yet.
func linkNoClobber(tmp, path string) error {
	err := linkFile(tmp, path)
	if err == nil {
		return nil
	}
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s: %w", path, ErrExists)
	}
	if !linkUnsupported(err) {
		return fmt.Errorf("publish %s: %w", path, err)
	}
	// Filesystems without hard links (some FUSE/network mounts): reserve the
	// name atomically with O_EXCL, then rename the finished file over it.
	reserved, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s: %w", path, ErrExists)
	}
	if err != nil {
		return fmt.Errorf("publish %s: %w", path, err)
	}
	if err := reserved.Close(); err != nil {
		return fmt.Errorf("publish %s: %w", path, err)
	}
	return os.Rename(tmp, path)
}

func linkUnsupported(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ENOTSUP) ||
		errors.Is(err, syscall.EXDEV) || errors.Is(err, syscall.EMLINK) || errors.Is(err, errors.ErrUnsupported)
}

// syncDir makes the rename durable. Windows cannot fsync directories.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open directory %s for sync: %w", dir, err)
	}
	serr := ignoreUnsupported(d.Sync())
	return errors.Join(serr, d.Close())
}

// ignoreUnsupported drops fsync errors from filesystems that do not implement it.
func ignoreUnsupported(err error) error {
	if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTSUP) {
		return nil
	}
	return err
}

// ReadFileLimited reads a regular file of at most max bytes. With noFollow,
// a symlink at path is rejected (ErrSymlink). FIFOs and devices are rejected
// without blocking.
func ReadFileLimited(path string, max int64, noFollow bool) ([]byte, error) {
	f, err := openForRead(path, noFollow)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %w", path, ErrNotRegular)
	}
	if st.Size() > max {
		return nil, fmt.Errorf("%s: %w (%d bytes, limit %d)", path, ErrTooLarge, st.Size(), max)
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(len(data)) > max { // grew after stat
		return nil, fmt.Errorf("%s: %w (limit %d)", path, ErrTooLarge, max)
	}
	return data, nil
}

// EnsureDir creates dir (and parents) with perm if missing and reports whether
// it was created. An existing symlink to a directory is accepted, because
// dotfile managers commonly symlink ~/.claude.
func EnsureDir(dir string, perm os.FileMode) (bool, error) {
	st, err := os.Stat(dir)
	if err == nil {
		if !st.IsDir() {
			return false, fmt.Errorf("%s: %w", dir, ErrNotDir)
		}
		return false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("stat %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, perm); err != nil {
		return false, fmt.Errorf("create %s: %w", dir, err)
	}
	return true, nil
}

// checkTarget inspects an existing destination: it must be absent or a
// regular, non-symlink file. It reports whether the file exists.
func checkTarget(path string) (bool, error) {
	st, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", path, err)
	}
	if st.Mode()&fs.ModeSymlink != 0 {
		return true, fmt.Errorf("%s: %w", path, ErrSymlink)
	}
	if !st.Mode().IsRegular() {
		return true, fmt.Errorf("%s: %w", path, ErrNotRegular)
	}
	return true, nil
}
