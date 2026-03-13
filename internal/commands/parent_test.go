package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create parent and child tasks
	now := time.Now()
	parent := &models.Task{
		ID:      "aaaa",
		Name:    "Parent Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	child := &models.Task{
		ID:      "aaab",
		Name:    "Child Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Reset flag
	parentPretty = false

	// Set parent
	err = runParent(nil, []string{child.ID, parent.ID})
	require.NoError(t, err)

	// Verify parent was set
	updated, err := store.LoadTask(child.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.Parent)
	assert.Equal(t, parent.ID, *updated.Parent)
}

func TestParentCommand_InvalidChildID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flag
	parentPretty = false

	err := runParent(nil, []string{"not-valid", "abcd"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid task ID")
}

func TestParentCommand_InvalidParentID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flag
	parentPretty = false

	err := runParent(nil, []string{"abcd", "not-valid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parent ID")
}

func TestParentCommand_SelfParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Reset flag
	parentPretty = false

	// Try to set task as its own parent
	err = runParent(nil, []string{task.ID, task.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot set task as its own parent")
}

func TestParentCommand_ChildNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	parent := &models.Task{
		ID:      "aaaa",
		Name:    "Parent Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Reset flag
	parentPretty = false

	// Try to set parent for non-existent child
	err = runParent(nil, []string{"zzzz", parent.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestParentCommand_ParentNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	child := &models.Task{
		ID:      "aaaa",
		Name:    "Child Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Reset flag
	parentPretty = false

	// Try to set non-existent parent
	err = runParent(nil, []string{child.ID, "zzzz"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent task")
	assert.Contains(t, err.Error(), "not found")
}

func TestParentCommand_DoneParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create done parent
	now := time.Now()
	parent := &models.Task{
		ID:      "aaaa",
		Name:    "Done Parent",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create child
	child := &models.Task{
		ID:      "aaab",
		Name:    "Child Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Reset flag
	parentPretty = false

	// Try to set done task as parent
	err = runParent(nil, []string{child.ID, parent.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "done task")
}

func TestParentCommand_CircularDependency(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create a chain: A -> B -> C
	now := time.Now()
	taskAID := "aaaa"
	taskA := &models.Task{
		ID:      taskAID,
		Name:    "Task A",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(taskA))

	taskBID := "aaab"
	taskB := &models.Task{
		ID:      taskBID,
		Name:    "Task B",
		Parent:  &taskAID,
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(taskB))

	taskC := &models.Task{
		ID:      "aaac",
		Name:    "Task C",
		Parent:  &taskBID,
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(taskC))

	// Reset flag
	parentPretty = false

	// Try to create a cycle: A -> B -> C -> A
	err = runParent(nil, []string{taskA.ID, taskC.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestParentCommand_PrettyOutput(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	parent := &models.Task{
		ID:      "aaaa",
		Name:    "Parent Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	child := &models.Task{
		ID:      "aaab",
		Name:    "Child Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	parentPretty = true

	err = runParent(nil, []string{child.ID, parent.ID})
	require.NoError(t, err)
}
