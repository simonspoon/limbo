//go:build windows

package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// acquireLock is the Windows implementation of AcquireLock. It opens
// dir/run.lock (creating if missing) and takes a non-blocking exclusive lock
// via LockFileEx with LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY.
//
// On ERROR_LOCK_VIOLATION or ERROR_IO_PENDING the sentinel ErrAlreadyRunning
// is returned wrapped with %w. On any other error the file descriptor is
// closed before returning.
//
// The returned unlock closure releases the lock and closes the file. It
// does NOT remove the lock file — see AcquireLock godoc for why.
func acquireLock(dir string) (func(), error) {
	path := filepath.Join(dir, "run.lock")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	handle := windows.Handle(f.Fd())
	var overlapped windows.Overlapped
	err = windows.LockFileEx(
		handle,
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,    // reserved
		1, 0, // lock 1 byte (bytesLow, bytesHigh)
		&overlapped,
	)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return nil, fmt.Errorf("acquire lock: %w", ErrAlreadyRunning)
		}
		return nil, fmt.Errorf("LockFileEx: %w", err)
	}

	unlock := func() {
		// Best-effort: unlock then close. Any errors here are not actionable
		// — kernel will clean up on process exit regardless.
		var ov windows.Overlapped
		_ = windows.UnlockFileEx(handle, 0, 1, 0, &ov)
		_ = f.Close()
		// Deliberately do NOT os.Remove(path) — inode race footgun.
	}
	return unlock, nil
}
