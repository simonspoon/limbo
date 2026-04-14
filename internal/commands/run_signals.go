package commands

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// SetupRunSignals wires SIGINT and SIGTERM into a cancellable context for the
// `limbo run` tick loop. Both signals are routed through signal.NotifyContext so
// that the caller's deferred cleanup (notably AcquireLock's unlock func from
// run_lock.go) runs on normal return. An os.Exit-based hard-exit path would
// strand run.lock; graceful ctx cancellation is the ONLY path where deferred
// unlock actually fires.
//
// Caller MUST defer the returned CancelFunc — otherwise signal.NotifyContext
// leaks a signal relay goroutine and keeps SIGINT/SIGTERM trapped for the
// remainder of the process's lifetime.
//
// Exit code mapping applied by the caller AFTER SetupRunSignals returns
// normally (helper itself never calls os.Exit):
//
//	0  completed successfully
//	1  error (cobra default)
//	2  user-stop (SIGINT or SIGTERM delivered)
//	3  blocked-no-vox (task blocked, no interactive override)
//	4  lock-contention (another limbo run already active)
//
// Mirrors the signal.NotifyContext pattern used in watch.go (around line 61),
// extended with syscall.SIGTERM for supervisor compatibility (systemd, docker).
func SetupRunSignals(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
}
