package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusCommand(t *testing.T) {
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

	// Reset flags
	statusPretty = false
	statusOutcome = ""

	// Test updating status
	err = runStatus(nil, []string{task.ID, models.StatusInProgress})
	require.NoError(t, err)

	// Verify status was updated
	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusInProgress, updated.Status)
	assert.True(t, updated.Updated.After(now))
}

func TestStatusCommand_InvalidStatus(t *testing.T) {
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

	// Reset flags
	statusPretty = false
	statusOutcome = ""

	// Test invalid status
	err = runStatus(nil, []string{task.ID, "invalid-status"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestStatusCommand_TaskNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags
	statusPretty = false
	statusOutcome = ""

	// Test non-existent task
	err := runStatus(nil, []string{"zzzz", models.StatusDone})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStatusCommand_InvalidID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags
	statusPretty = false
	statusOutcome = ""

	// Test invalid ID format
	err := runStatus(nil, []string{"not-valid", models.StatusDone})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid task ID")
}

func TestStatusCommand_AllStatuses(t *testing.T) {
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

	// Reset flags
	statusPretty = false
	statusOutcome = ""

	// Test each valid status
	statuses := []string{models.StatusTodo, models.StatusInProgress, models.StatusDone}
	for _, status := range statuses {
		err = runStatus(nil, []string{task.ID, status})
		require.NoError(t, err)

		updated, err := store.LoadTask(task.ID)
		require.NoError(t, err)
		assert.Equal(t, status, updated.Status)
	}
}

func TestStatusCommand_CannotMarkDoneWithUndoneChildren(t *testing.T) {
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

	// Reset flags
	statusPretty = false
	statusOutcome = ""

	// Try to mark parent as done - should fail
	err = runStatus(nil, []string{parent.ID, models.StatusDone})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "undone children")

	// Mark child as done
	err = runStatus(nil, []string{child.ID, models.StatusDone})
	require.NoError(t, err)

	// Now parent can be marked done
	err = runStatus(nil, []string{parent.ID, models.StatusDone})
	require.NoError(t, err)
}

func TestStatusCommand_CannotStartBlockedTask(t *testing.T) {
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

	// Reset flags
	statusPretty = false
	statusOutcome = ""

	// Try to start the blocked task - should fail
	err = runStatus(nil, []string{blocked.ID, models.StatusInProgress})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot start task")
	assert.Contains(t, err.Error(), "blocked by")
	assert.Contains(t, err.Error(), "aaaa")

	// Verify task is still todo
	updated, err := store.LoadTask(blocked.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusTodo, updated.Status)
}

func TestStatusCommand_CanStartAfterUnblock(t *testing.T) {
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

	// Reset flags
	statusPretty = false
	statusOutcome = ""

	// Mark blocker as done (auto-removes from BlockedBy lists)
	err = runStatus(nil, []string{blocker.ID, models.StatusDone})
	require.NoError(t, err)

	// Now should be able to start the previously blocked task
	err = runStatus(nil, []string{blocked.ID, models.StatusInProgress})
	require.NoError(t, err)

	// Verify task is in-progress
	updated, err := store.LoadTask(blocked.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusInProgress, updated.Status)
}

func TestStatusCommand_PrettyOutput(t *testing.T) {
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
	statusPretty = true
	statusOutcome = ""

	err = runStatus(nil, []string{task.ID, models.StatusInProgress})
	require.NoError(t, err)
}

func TestStatusCommand_RequiresOutcomeForStructuredTask(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Structured Task",
		Status:  models.StatusTodo,
		Action:  "do X",
		Verify:  "check Y",
		Result:  "report Z",
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	statusPretty = false
	statusOutcome = ""

	// Try to mark done without outcome - should fail
	err = runStatus(nil, []string{task.ID, models.StatusDone})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "requires --outcome")
}

func TestStatusCommand_AcceptsOutcomeForStructuredTask(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Structured Task",
		Status:  models.StatusTodo,
		Action:  "do X",
		Verify:  "check Y",
		Result:  "report Z",
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	statusPretty = false
	statusOutcome = "done, Y confirmed"

	err = runStatus(nil, []string{task.ID, models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, updated.Status)
	assert.Equal(t, "done, Y confirmed", updated.Outcome)
}

func TestStatusCommand_LegacyTaskDoneWithoutOutcome(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Legacy Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	statusPretty = false
	statusOutcome = ""

	// Legacy task can be marked done without outcome
	err = runStatus(nil, []string{task.ID, models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, updated.Status)
	assert.Empty(t, updated.Outcome)
}
