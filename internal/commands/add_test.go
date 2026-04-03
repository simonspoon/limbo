package commands

import (
	"os"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnv(t *testing.T) (string, func()) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-cmd-test-*")
	require.NoError(t, err)

	// Change to temp directory
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))

	// Initialize limbo
	store := storage.NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Return cleanup function
	cleanup := func() {
		os.Chdir(origDir)
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestAddCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags
	addDescription = ""
	addParent = ""
	addPretty = false
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test basic add
	err := runAdd(nil, []string{"Test Task"})
	require.NoError(t, err)

	// Verify task was created
	store, err := storage.NewStorage()
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	task := tasks[0]
	assert.Equal(t, "Test Task", task.Name)
	assert.Equal(t, models.StatusCaptured, task.Status)
	assert.Empty(t, task.Description)
	assert.Nil(t, task.Parent)
}

func TestAddCommandWithDescription(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Set flags
	addDescription = "Test description"
	addParent = ""
	addPretty = false
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test add with description
	err := runAdd(nil, []string{"Task with description"})
	require.NoError(t, err)

	// Verify task
	store, err := storage.NewStorage()
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	task := tasks[0]
	assert.Equal(t, "Task with description", task.Name)
	assert.Equal(t, "Test description", task.Description)
}

func TestAddCommandWithParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create parent task
	addDescription = ""
	addParent = ""
	addPretty = false
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	err := runAdd(nil, []string{"Parent Task"})
	require.NoError(t, err)

	// Get parent ID
	store, err := storage.NewStorage()
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	parentID := tasks[0].ID

	// Create child task (with slight delay to ensure unique ID)
	time.Sleep(2 * time.Millisecond)
	addParent = parentID
	err = runAdd(nil, []string{"Child Task"})
	require.NoError(t, err)

	// Verify child has parent
	tasks, err = store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	// Find child task
	var child *models.Task
	for _, task := range tasks {
		if task.Name == "Child Task" {
			child = &task
			break
		}
	}

	require.NotNil(t, child)
	require.NotNil(t, child.Parent)
	assert.Equal(t, parentID, *child.Parent)
}

func TestAddCommandNonExistentParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Set non-existent parent
	addParent = "zzzz"
	addDescription = ""
	addPretty = false
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test add should fail
	err := runAdd(nil, []string{"Test Task"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent task")
}

func TestAddCommandNotInProject(t *testing.T) {
	// Create temp directory without initializing
	tmpDir, err := os.MkdirTemp("", "limbo-cmd-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)

	require.NoError(t, os.Chdir(tmpDir))

	// Reset flags
	addDescription = ""
	addParent = ""
	addPretty = false
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test add should fail
	err = runAdd(nil, []string{"Test Task"})
	assert.Error(t, err)
}

func TestAddCommandPrettyOutput(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	addDescription = ""
	addParent = ""
	addPretty = true
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	err := runAdd(nil, []string{"Test Task"})
	require.NoError(t, err)
}

func TestAddCommandToDoneParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create a done parent task
	now := time.Now()
	parent := &models.Task{
		ID:      "aaaa",
		Name:    "Done Parent",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Try to add child to done parent
	addDescription = ""
	addParent = parent.ID
	addPretty = false
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	err = runAdd(nil, []string{"Child Task"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot add child to done task")
}

func TestAddCommandInvalidParentID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Set invalid parent ID format
	addParent = "invalid"
	addDescription = ""
	addPretty = false
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test add should fail
	err := runAdd(nil, []string{"Test Task"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parent task ID")
}

func TestAddCommand_WithoutStructuredFlags(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset all flags
	addDescription = ""
	addParent = ""
	addPretty = false
	addApproach = ""
	addVerify = ""
	addResult = ""

	// Adding a task without structured flags should succeed
	err := runAdd(nil, []string{"Quick Task"})
	require.NoError(t, err)

	// Verify task was created with empty structured fields
	store, err := storage.NewStorage()
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	task := tasks[0]
	assert.Equal(t, "Quick Task", task.Name)
	assert.Equal(t, models.StatusCaptured, task.Status)
	assert.Empty(t, task.Approach)
	assert.Empty(t, task.Verify)
	assert.Empty(t, task.Result)
	assert.False(t, task.HasStructuredFields())
}
