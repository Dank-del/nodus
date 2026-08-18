//go:build windows

package cpm

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// Windows locks the first byte of the lock file exclusively. LockFileEx blocks
// until a concurrent CPM process releases it, matching the Unix flock behavior.
func lockMirror(mirror string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(mirror), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(mirror+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	overlapped := windows.Overlapped{}
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock shared Git mirror: %w", err)
	}
	return func() {
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped)
		_ = f.Close()
	}, nil
}
