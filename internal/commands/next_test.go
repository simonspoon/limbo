package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextCommand_NoTasks(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	nextPretty = false

	err := runNext(nil, nil)
	require.NoError(t, err)
}

func TestNextCommand_SingleTask(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Only Task",
		Status:  models.StatusReady,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	nextPretty = false

	err = runNext(nil, nil)
	require.NoError(t, err)
}

func TestNextCommand_ReturnsFIFO(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create older task
	older := time.Now()
	task1 := &models.Task{
		ID:      "aaaa",
		Name:    "Older Task",
		Status:  models.StatusReady,
		Created: older,
		Updated: older,
	}
	require.NoError(t, store.SaveTask(task1))

	// Create newer task
	newer := older.Add(5 * time.Millisecond)
	task2 := &models.Task{
		ID:      "aaab",
		Name:    "Newer Task",
		Status:  models.StatusReady,
		Created: newer,
		Updated: newer,
	}
	require.NoError(t, store.SaveTask(task2))

	// No in-progress tasks - returns candidates (older task first)
	next, err := store.GetNextTask()
	require.NoError(t, err)
	require.NotNil(t, next)
	require.NotEmpty(t, next.Candidates)
	assert.Equal(t, "Older Task", next.Candidates[0].Name)
}

func TestNextCommand_SkipsNonTodoTasks(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create done task (oldest)
	doneTask := &models.Task{
		ID:      "aaaa",
		Name:    "Done Task",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(doneTask))

	// Create in-progress task
	inProgress := &models.Task{
		ID:      "aaab",
		Name:    "In Progress Task",
		Status:  models.StatusInProgress,
		Created: now.Add(time.Millisecond),
		Updated: now.Add(time.Millisecond),
	}
	require.NoError(t, store.SaveTask(inProgress))

	// Create ready task (newest)
	readyTask := &models.Task{
		ID:      "aaac",
		Name:    "Ready Task",
		Status:  models.StatusReady,
		Created: now.Add(2 * time.Millisecond),
		Updated: now.Add(2 * time.Millisecond),
	}
	require.NoError(t, store.SaveTask(readyTask))

	// Should return the ready task (sibling of in-progress task)
	next, err := store.GetNextTask()
	require.NoError(t, err)
	require.NotNil(t, next)
	require.NotNil(t, next.Task)
	assert.Equal(t, "Ready Task", next.Task.Name)
}

func TestNextCommand_ReportsBlockedCount(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create a blocker task (in-progress)
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker",
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	// Create 3 blocked ready tasks
	for i, id := range []string{"aaab", "aaac", "aaad"} {
		task := &models.Task{
			ID:        id,
			Name:      "Blocked Task",
			Status:    models.StatusReady,
			BlockedBy: []string{"aaaa"},
			Created:   now.Add(time.Duration(i+1) * time.Millisecond),
			Updated:   now.Add(time.Duration(i+1) * time.Millisecond),
		}
		require.NoError(t, store.SaveTask(task))
	}

	// next should return empty result with blocked count
	result, err := store.GetNextTask()
	require.NoError(t, err)
	assert.Nil(t, result.Task)
	assert.Empty(t, result.Candidates)
	assert.Equal(t, 3, result.BlockedCount)

	// Also test via runNext (pretty)
	nextPretty = true
	nextUnclaimed = false
	err = runNext(nil, nil)
	require.NoError(t, err)
}

func TestNextCommand_PrettyStructuredFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:          "aaaa",
		Name:        "Structured Task",
		Description: "A description",
		Approach:    "run migrations",
		Verify:      "check table exists",
		Result:      "migration output",
		Status:      models.StatusReady,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	nextPretty = true
	nextUnclaimed = false

	// Should display without error
	err = runNext(nil, nil)
	require.NoError(t, err)
}

func TestNextCommand_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:          "aaaa",
		Name:        "Test Task",
		Description: "A description",
		Status:      models.StatusReady,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	nextPretty = true

	err = runNext(nil, nil)
	require.NoError(t, err)
}
