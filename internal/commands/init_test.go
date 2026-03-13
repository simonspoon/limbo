package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCommand(t *testing.T) {
	// Create temp directory without initializing
	tmpDir, err := os.MkdirTemp("", "limbo-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	// Reset flag
	initPretty = false

	// Run init
	err = runInit(nil, nil)
	require.NoError(t, err)

	// Verify .limbo directory was created
	limboPath := filepath.Join(tmpDir, storage.LimboDir)
	info, err := os.Stat(limboPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify tasks.json was created
	tasksPath := filepath.Join(limboPath, storage.TasksFile)
	_, err = os.Stat(tasksPath)
	require.NoError(t, err)
}

func TestInitCommandAlreadyExists(t *testing.T) {
	// Create temp directory and initialize it
	tmpDir, err := os.MkdirTemp("", "limbo-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	// Initialize first
	store := storage.NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Reset flag
	initPretty = false

	// Run init again should fail
	err = runInit(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInitCommandPrettyOutput(t *testing.T) {
	// Create temp directory without initializing
	tmpDir, err := os.MkdirTemp("", "limbo-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	// Set pretty flag
	initPretty = true

	// Run init (should succeed, pretty output goes to stdout)
	err = runInit(nil, nil)
	require.NoError(t, err)

	// Verify .limbo directory was created
	limboPath := filepath.Join(tmpDir, storage.LimboDir)
	info, err := os.Stat(limboPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
