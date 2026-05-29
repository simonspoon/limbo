package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoClimbFlagRegistered verifies the persistent --no-climb flag is
// registered on rootCmd and defaults to false.
func TestNoClimbFlagRegistered(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("no-climb")
	require.NotNil(t, flag, "expected --no-climb persistent flag to be registered")
	assert.Equal(t, "false", flag.DefValue)
}

// TestIfRevisionFlagRegistered verifies the persistent --if-revision guard flag
// is registered globally.
func TestIfRevisionFlagRegistered(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("if-revision")
	require.NotNil(t, flag, "expected --if-revision persistent flag to be registered")
}

// TestWantNoClimbHonorsFlag verifies the --no-climb flag forces no-climb
// regardless of the environment.
func TestWantNoClimbHonorsFlag(t *testing.T) {
	t.Cleanup(func() { noClimbFlag = false })
	t.Setenv(noClimbEnv, "")

	noClimbFlag = true
	assert.True(t, wantNoClimb())

	noClimbFlag = false
	assert.False(t, wantNoClimb())
}

// TestWantNoClimbHonorsEnv verifies a truthy LIMBO_NO_CLIMB enables no-climb.
func TestWantNoClimbHonorsEnv(t *testing.T) {
	t.Cleanup(func() { noClimbFlag = false })
	noClimbFlag = false

	for _, v := range []string{"1", "true", "yes", "on", "TRUE"} {
		t.Setenv(noClimbEnv, v)
		assert.Truef(t, wantNoClimb(), "expected truthy for %q", v)
	}
	for _, v := range []string{"", "0", "no", "off"} {
		t.Setenv(noClimbEnv, v)
		assert.Falsef(t, wantNoClimb(), "expected falsy for %q", v)
	}
}

// TestGetStorageResolvesCentralRoot verifies getStorage resolves a project
// anchored by .limbo-id to a central storage root under LIMBO_HOME.
func TestGetStorageResolvesCentralRoot(t *testing.T) {
	homeDir, err := os.MkdirTemp("", "limbo-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(homeDir)
	projDir, err := os.MkdirTemp("", "limbo-proj-*")
	require.NoError(t, err)
	defer os.RemoveAll(projDir)
	require.NoError(t, os.WriteFile(filepath.Join(projDir, ".limbo-id"), []byte("root-test-id\n"), 0o644))

	t.Setenv("LIMBO_HOME", homeDir)
	t.Setenv(noClimbEnv, "1")

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projDir))
	defer os.Chdir(origDir)

	store, err := getStorage()
	require.NoError(t, err)
	expected := filepath.Join(homeDir, "projects", "root-test-id")
	assert.Equal(t, expected, store.GetRootDir())
}

// TestGetStorageErrorsOutsideProject verifies getStorage fails with a
// run-init hint when there is no project anchor and climbing is disabled.
func TestGetStorageErrorsOutsideProject(t *testing.T) {
	homeDir, err := os.MkdirTemp("", "limbo-home-*")
	require.NoError(t, err)
	defer os.RemoveAll(homeDir)
	bareDir, err := os.MkdirTemp("", "limbo-bare-*")
	require.NoError(t, err)
	defer os.RemoveAll(bareDir)

	t.Setenv("LIMBO_HOME", homeDir)
	t.Setenv(noClimbEnv, "1")

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(bareDir))
	defer os.Chdir(origDir)

	_, err = getStorage()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "init")
}
