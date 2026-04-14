//go:build !windows

package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// acquireLock is the Unix implementation of AcquireLock. It opens
// dir/run.lock (creating if missing) and takes a non-blocking exclusive
// advisory lock via flock(2).
//
// On EWOULDBLOCK the sentinel ErrAlreadyRunning is returned wrapped with %w.
// On any other error the file descriptor is closed before returning.
//
// The returned unlock closure releases the flock and closes the fd. It does
// NOT remove the lock file — see AcquireLock godoc for why.
func acquireLock(dir string) (func(), error) {
	path := filepath.Join(dir, "run.lock")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, fmt.Errorf("acquire lock: %w", ErrAlreadyRunning)
		}
		return nil, fmt.Errorf("flock: %w", err)
	}

	unlock := func() {
		// Best-effort: unlock then close. Any errors here are not actionable
		// — kernel will clean up on process exit regardless.
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
		// Deliberately do NOT os.Remove(path) — inode race footgun.
	}
	return unlock, nil
}
