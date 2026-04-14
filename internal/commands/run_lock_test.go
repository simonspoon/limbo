package commands

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// lockHelperEnv is the environment variable used by the cross-process
// contention test to signal "act as a lock helper, not as a normal test run".
const lockHelperEnv = "LIMBO_LOCK_HELPER"

// lockHelperDirEnv carries the directory path the helper should lock.
const lockHelperDirEnv = "LIMBO_LOCK_HELPER_DIR"

func TestLockErrorsIsSentinel(t *testing.T) {
	// errors.Is against itself.
	assert.True(t, errors.Is(ErrAlreadyRunning, ErrAlreadyRunning))

	// fmt.Errorf with %w preserves the match — this is the contract callers
	// rely on (e.g. `if errors.Is(err, ErrAlreadyRunning) { exit(4) }`).
	wrapped := fmt.Errorf("acquire lock: %w", ErrAlreadyRunning)
	assert.True(t, errors.Is(wrapped, ErrAlreadyRunning))

	// Double-wrap also matches.
	doubled := fmt.Errorf("outer: %w", wrapped)
	assert.True(t, errors.Is(doubled, ErrAlreadyRunning))

	// A plain wrapped error should not match.
	other := fmt.Errorf("unrelated: %w", errors.New("boom"))
	assert.False(t, errors.Is(other, ErrAlreadyRunning))
}
