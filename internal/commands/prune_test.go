package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneCommand_NoTasks(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	prunePretty = false

	err := runPrune(nil, nil)
	require.NoError(t, err)
}

func TestPruneCommand_NoCompletedTasks(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Todo Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	prunePretty = false

	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Task should still exist
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
}

func TestPruneCommand_DeletesCompletedTasks(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create done task
	doneTask := &models.Task{
		ID:      "aaaa",
		Name:    "Done Task",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(doneTask))

	// Create todo task
	todoTask := &models.Task{
		ID:      "aaab",
		Name:    "Todo Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(todoTask))

	prunePretty = false

	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Only todo task should remain
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "Todo Task", tasks[0].Name)
}

func TestPruneCommand_SkipsTasksWithUndoneChildren(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create done parent
	parentID := "aaaa"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Done Parent",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create undone child
	child := &models.Task{
		ID:      "aaab",
		Name:    "Todo Child",
		Status:  models.StatusTodo,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	prunePretty = false

	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Both tasks should still exist
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
}

func TestPruneCommand_DeletesTasksWithDoneChildren(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create done parent
	parentID := "aaaa"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Done Parent",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create done child
	child := &models.Task{
		ID:      "aaab",
		Name:    "Done Child",
		Status:  models.StatusDone,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	prunePretty = false

	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Both should be pruned
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 0)
}

func TestPruneCommand_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Done Task",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	prunePretty = true

	err = runPrune(nil, nil)
	require.NoError(t, err)
}
