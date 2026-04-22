package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStderr redirects os.Stderr for the duration of fn and returns
// everything written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	os.Stderr = orig
	return <-done
}

// resetClimbWarnOnce lets each test observe the one-shot warning fresh.
func resetClimbWarnOnce() {
	climbWarnOnce = sync.Once{}
}

// TestMaybeWarnHomeClimbEmitsWhenRootIsHome verifies the warning fires
// when the resolved root is an ancestor of cwd AND equals $HOME.
func TestMaybeWarnHomeClimbEmitsWhenRootIsHome(t *testing.T) {
	resetClimbWarnOnce()

	fakeHome, err := os.MkdirTemp("", "limbo-fakehome-*")
	require.NoError(t, err)
	defer os.RemoveAll(fakeHome)

	fakeHome, err = filepath.EvalSymlinks(fakeHome)
	require.NoError(t, err)

	// Put .limbo at fakeHome to simulate ~/.limbo.
	store := storage.NewStorageAt(fakeHome)
	require.NoError(t, store.Init())

	// Child dir of fakeHome — cwd below home.
	childDir := filepath.Join(fakeHome, "project")
	require.NoError(t, os.MkdirAll(childDir, 0755))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(childDir))
	defer os.Chdir(origDir)

	t.Setenv("HOME", fakeHome)

	out := captureStderr(t, func() {
		maybeWarnHomeClimb(fakeHome)
	})

	assert.Contains(t, out, "warning:")
	assert.Contains(t, out, fakeHome)
	assert.Contains(t, out, "LIMBO_NO_CLIMB")
}

// TestMaybeWarnHomeClimbSilentWhenRootIsCwd verifies no warning when
// the resolved root is cwd (no climb happened).
func TestMaybeWarnHomeClimbSilentWhenRootIsCwd(t *testing.T) {
	resetClimbWarnOnce()

	fakeHome, err := os.MkdirTemp("", "limbo-fakehome-*")
	require.NoError(t, err)
	defer os.RemoveAll(fakeHome)

	fakeHome, err = filepath.EvalSymlinks(fakeHome)
	require.NoError(t, err)

	store := storage.NewStorageAt(fakeHome)
	require.NoError(t, store.Init())

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(fakeHome))
	defer os.Chdir(origDir)

	t.Setenv("HOME", fakeHome)

	out := captureStderr(t, func() {
		maybeWarnHomeClimb(fakeHome)
	})

	assert.Empty(t, out, "expected no warning when root==cwd")
}

// TestMaybeWarnHomeClimbSilentWhenRootIsNonHomeAncestor verifies no
// warning when the resolved root is an ancestor but is not $HOME.
func TestMaybeWarnHomeClimbSilentWhenRootIsNonHomeAncestor(t *testing.T) {
	resetClimbWarnOnce()

	fakeHome, err := os.MkdirTemp("", "limbo-fakehome-*")
	require.NoError(t, err)
	defer os.RemoveAll(fakeHome)

	projectRoot, err := os.MkdirTemp("", "limbo-project-*")
	require.NoError(t, err)
	defer os.RemoveAll(projectRoot)

	projectRoot, err = filepath.EvalSymlinks(projectRoot)
	require.NoError(t, err)

	store := storage.NewStorageAt(projectRoot)
	require.NoError(t, store.Init())

	childDir := filepath.Join(projectRoot, "sub")
	require.NoError(t, os.MkdirAll(childDir, 0755))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(childDir))
	defer os.Chdir(origDir)

	t.Setenv("HOME", fakeHome)

	out := captureStderr(t, func() {
		maybeWarnHomeClimb(projectRoot)
	})

	assert.Empty(t, out, "expected no warning when ancestor is not $HOME")
}

// TestMaybeWarnHomeClimbOnlyOncePerProcess verifies the sync.Once guard.
func TestMaybeWarnHomeClimbOnlyOncePerProcess(t *testing.T) {
	resetClimbWarnOnce()

	fakeHome, err := os.MkdirTemp("", "limbo-fakehome-*")
	require.NoError(t, err)
	defer os.RemoveAll(fakeHome)

	fakeHome, err = filepath.EvalSymlinks(fakeHome)
	require.NoError(t, err)

	store := storage.NewStorageAt(fakeHome)
	require.NoError(t, store.Init())

	childDir := filepath.Join(fakeHome, "project")
	require.NoError(t, os.MkdirAll(childDir, 0755))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(childDir))
	defer os.Chdir(origDir)

	t.Setenv("HOME", fakeHome)

	out := captureStderr(t, func() {
		maybeWarnHomeClimb(fakeHome)
		maybeWarnHomeClimb(fakeHome)
		maybeWarnHomeClimb(fakeHome)
	})

	count := strings.Count(out, "warning:")
	assert.Equal(t, 1, count, "expected exactly one warning across three calls, got %d:\n%s", count, out)
}

// TestNoClimbFlagRegistered verifies the persistent --no-climb flag
// is registered on rootCmd and defaults to false.
func TestNoClimbFlagRegistered(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("no-climb")
	require.NotNil(t, flag, "expected --no-climb persistent flag to be registered")
	assert.Equal(t, "false", flag.DefValue)
}

// TestPersistentPreRunESetsEnvWhenFlagSet verifies PersistentPreRunE
// propagates the flag into LIMBO_NO_CLIMB before subcommand Run.
func TestPersistentPreRunESetsEnvWhenFlagSet(t *testing.T) {
	// Start from a clean env.
	require.NoError(t, os.Unsetenv(storage.NoClimbEnv))
	// Register cleanup to restore flag and env.
	t.Cleanup(func() {
		noClimbFlag = false
		_ = os.Unsetenv(storage.NoClimbEnv)
	})

	noClimbFlag = true
	require.NotNil(t, rootCmd.PersistentPreRunE)
	require.NoError(t, rootCmd.PersistentPreRunE(rootCmd, nil))
	assert.Equal(t, "1", os.Getenv(storage.NoClimbEnv))
}

// TestPersistentPreRunELeavesEnvAloneWhenFlagUnset verifies the flag
// does not clobber a pre-existing LIMBO_NO_CLIMB env value.
func TestPersistentPreRunELeavesEnvAloneWhenFlagUnset(t *testing.T) {
	t.Setenv(storage.NoClimbEnv, "1")
	t.Cleanup(func() { noClimbFlag = false })

	noClimbFlag = false
	require.NoError(t, rootCmd.PersistentPreRunE(rootCmd, nil))
	assert.Equal(t, "1", os.Getenv(storage.NoClimbEnv), "env should remain as set by the user")
}
