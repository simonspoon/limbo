package commands

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

// Note: these tests intentionally do NOT call t.Parallel(). Signals are
// process-wide and delivering SIGINT/SIGTERM to os.Getpid() while another
// subtest also has a signal.NotifyContext handler installed would race for
// the delivery. Every test defers cancel() so the handler is deregistered
// before the next test runs.

func TestSetupRunSignals_SIGINT(t *testing.T) {
	ctx, cancel := SetupRunSignals(context.Background())
	defer cancel()

	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("failed to send SIGINT: %v", err)
	}

	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != context.Canceled {
			t.Fatalf("ctx.Err() = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ctx to cancel after SIGINT")
	}
}

func TestSetupRunSignals_SIGTERM(t *testing.T) {
	ctx, cancel := SetupRunSignals(context.Background())
	defer cancel()

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != context.Canceled {
			t.Fatalf("ctx.Err() = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ctx to cancel after SIGTERM")
	}
}

func TestSetupRunSignals_DeferCancelSafe(t *testing.T) {
	ctx, cancel := SetupRunSignals(context.Background())

	cancel()

	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != context.Canceled {
			t.Fatalf("ctx.Err() = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ctx.Done() after explicit cancel")
	}

	// Second cancel must be a no-op (stdlib guarantees idempotence). This
	// asserts we don't wrap CancelFunc in a way that breaks that contract.
	cancel()
}

func TestSetupRunSignals_ParentCancelPropagates(t *testing.T) {
	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	ctx, cancel := SetupRunSignals(parentCtx)
	defer cancel()

	parentCancel()

	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != context.Canceled {
			t.Fatalf("ctx.Err() = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for child ctx to cancel after parent cancel")
	}
}

func TestSetupRunSignals_NoSignalNoCancel(t *testing.T) {
	ctx, cancel := SetupRunSignals(context.Background())
	defer cancel()

	time.Sleep(50 * time.Millisecond)

	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err() = %v, want nil (no signal delivered)", err)
	}
}
