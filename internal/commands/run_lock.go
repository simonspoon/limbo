package commands

import "errors"

// ErrAlreadyRunning is returned by AcquireLock when another limbo process
// already holds the run lock for the given directory. Callers should use
// errors.Is to match this sentinel rather than comparing directly:
//
//	if errors.Is(err, commands.ErrAlreadyRunning) {
//	    os.Exit(4)
//	}
//
// The sentinel is always returned wrapped via fmt.Errorf("...: %w", ...), so
// direct equality (err == ErrAlreadyRunning) will not match — use errors.Is.
var ErrAlreadyRunning = errors.New("limbo run: another instance is already running for this tree")

// AcquireLock takes an exclusive, non-blocking advisory lock on "run.lock"
// inside dir and returns an unlock closure plus any error.
//
// On success, the returned unlock closure releases the lock and closes the
// underlying file descriptor. The lock file itself is intentionally left on
// disk after unlock; callers must not remove it. Removing the lock file while
// another process may have it open is an inode-race footgun — two processes
// could end up with file locks on different inodes at the same path and both
// believe they hold the lock.
//
// The lock is advisory and enforced by the kernel via flock(2) on Unix and
// LockFileEx on Windows. On Unix, per-open-file-description semantics apply:
// the lock is held on the opened file, not on the inode, and is released
// automatically when the process dies (normal exit, crash, SIGKILL) — there
// is no stale-lock-file failure mode. On Windows, LockFileEx locks are also
// released on handle close / process termination.
//
// The caller MUST ensure dir already exists. AcquireLock does not call
// MkdirAll; if dir is missing, the underlying os.OpenFile call will fail.
//
// Exit code contract for callers wiring this into "limbo run":
//
//	0 - run completed normally
//	1 - generic error
//	2 - user stop (e.g. SIGINT)
//	3 - blocked / no voice input available
//	4 - lock contention (errors.Is(err, ErrAlreadyRunning))
//
// Use ExitCodeFor to translate an error from AcquireLock into the documented
// exit code.
func AcquireLock(dir string) (func(), error) {
	return acquireLock(dir)
}

// ExitCodeFor maps an error from AcquireLock (or any wrapped caller) to the
// documented "limbo run" exit code. nil → 0, ErrAlreadyRunning → 4, anything
// else → 1. Codes 2 (user stop) and 3 (blocked) are not produced by this
// package and must be returned directly by their respective wiring.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrAlreadyRunning) {
		return 4
	}
	return 1
}
