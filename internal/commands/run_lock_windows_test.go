//go:build windows

package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain intercepts the lock helper invocation before any normal test
// machinery runs. See run_lock_unix_test.go for the same pattern.
func TestMain(m *testing.M) {
	if os.Getenv(lockHelperEnv) == "1" {
		dir := os.Getenv(lockHelperDirEnv)
		unlock, err := AcquireLock(dir)
		if err != nil {
			if errors.Is(err, ErrAlreadyRunning) {
				os.Exit(4)
			}
			fmt.Fprintln(os.Stderr, "helper:", err)
			os.Exit(1)
		}
		unlock()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestLockAcquire_FirstCallSucceeds(t *testing.T) {
	dir := t.TempDir()

	unlock, err := AcquireLock(dir)
	require.NoError(t, err)
	require.NotNil(t, unlock)
	defer unlock()

	_, err = os.Stat(filepath.Join(dir, "run.lock"))
	require.NoError(t, err)
}

func TestLockContention_CrossProcess(t *testing.T) {
	dir := t.TempDir()

	unlock, err := AcquireLock(dir)
	require.NoError(t, err)
	defer unlock()

	cmd := exec.Command(os.Args[0], "-test.run=TestLockContention_CrossProcess")
	cmd.Env = append(os.Environ(),
		lockHelperEnv+"=1",
		lockHelperDirEnv+"="+dir,
	)
	out, err := cmd.CombinedOutput()

	require.Error(t, err, "child should fail due to lock contention; output: %s", string(out))
	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr), "expected ExitError, got %T: %v", err, err)
	assert.Equal(t, 4, exitErr.ExitCode(), "child exit code mismatch; output: %s", string(out))
}

func TestLockRelease_UnlockAllowsReacquire(t *testing.T) {
	dir := t.TempDir()

	unlock, err := AcquireLock(dir)
	require.NoError(t, err)
	unlock()

	// After unlock the same dir must be lockable again.
	unlock2, err := AcquireLock(dir)
	require.NoError(t, err)
	require.NotNil(t, unlock2)
	unlock2()
}

func TestLockRelease_DoesNotRemoveFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.lock")

	unlock, err := AcquireLock(dir)
	require.NoError(t, err)
	unlock()

	info, err := os.Stat(path)
	require.NoError(t, err, "lock file must still exist after unlock")
	assert.False(t, info.IsDir())
}

func TestLockStaleFile_FromPriorRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run.lock")

	// Pre-create a stale lock file with arbitrary bytes (simulating a file
	// left behind by a prior run). The advisory lock should not care.
	require.NoError(t, os.WriteFile(path, []byte("stale pid 12345\n"), 0644))

	unlock, err := AcquireLock(dir)
	require.NoError(t, err)
	require.NotNil(t, unlock)
	unlock()
}
