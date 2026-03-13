package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a test task
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
	deletePretty = false

	// Delete the task
	err = runDelete(nil, []string{task.ID})
	require.NoError(t, err)

	// Verify task was deleted
	_, err = store.LoadTask(task.ID)
	assert.Error(t, err)
	assert.Equal(t, storage.ErrTaskNotFound, err)
}

func TestDeleteCommand_TaskNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flag
	deletePretty = false

	err := runDelete(nil, []string{"zzzz"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteCommand_InvalidID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flag
	deletePretty = false

	err := runDelete(nil, []string{"not-valid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid task ID")
}

func TestDeleteCommand_BlockedByUndoneChildren(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create parent task
	parentID := "aaaa"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create child task
	child := &models.Task{
		ID:      "aaab",
		Name:    "Child Task",
		Status:  models.StatusTodo,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Reset flag
	deletePretty = false

	// Try to delete parent - should fail
	err = runDelete(nil, []string{parent.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "undone children")

	// Verify parent still exists
	_, err = store.LoadTask(parent.ID)
	require.NoError(t, err)
}

func TestDeleteCommand_AllowedWithDoneChildren(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create parent task
	parentID := "aaaa"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent Task",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create child task that is done
	child := &models.Task{
		ID:      "aaab",
		Name:    "Child Task",
		Status:  models.StatusDone,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Reset flag
	deletePretty = false

	// Delete parent - should succeed since children are done
	err = runDelete(nil, []string{parent.ID})
	require.NoError(t, err)

	// Verify parent was deleted
	_, err = store.LoadTask(parent.ID)
	assert.Error(t, err)

	// Child should still exist and be orphaned
	orphanedChild, err := store.LoadTask(child.ID)
	require.NoError(t, err)
	assert.Nil(t, orphanedChild.Parent)
}

func TestDeleteCommand_BlockedByUndoneGrandchildren(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create grandparent task
	grandparentID := "aaaa"
	grandparent := &models.Task{
		ID:      grandparentID,
		Name:    "Grandparent Task",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(grandparent))

	// Create parent task (done)
	parentID := "aaab"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent Task",
		Status:  models.StatusDone,
		Parent:  &grandparentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create child task (undone)
	child := &models.Task{
		ID:      "aaac",
		Name:    "Child Task",
		Status:  models.StatusTodo,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Reset flag
	deletePretty = false

	// Try to delete grandparent - should fail due to undone grandchild
	err = runDelete(nil, []string{grandparent.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "undone children")

	// Verify grandparent still exists
	_, err = store.LoadTask(grandparent.ID)
	require.NoError(t, err)
}

func TestDeleteCommand_CleansUpBlockedBy(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create blocker task
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	// Create blocked task
	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Blocked Task",
		Status:    models.StatusTodo,
		BlockedBy: []string{"aaaa"},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked))

	// Reset flag
	deletePretty = false

	// Delete the blocker
	err = runDelete(nil, []string{blocker.ID})
	require.NoError(t, err)

	// Verify blocked task's BlockedBy is cleaned up
	updated, err := store.LoadTask(blocked.ID)
	require.NoError(t, err)
	assert.Empty(t, updated.BlockedBy)
}

func TestDeleteCommand_PrettyOutput(t *testing.T) {
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

	// Set pretty flag
	deletePretty = true

	// Delete with pretty output
	err = runDelete(nil, []string{task.ID})
	require.NoError(t, err)
}
