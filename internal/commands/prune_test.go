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
		Status:  models.StatusCaptured,
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

	// Archive should be empty
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 0)
}

func TestPruneCommand_ArchivesCompletedTasks(t *testing.T) {
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
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(todoTask))

	prunePretty = false

	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Only todo task should remain in active store
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "Todo Task", tasks[0].Name)

	// Done task should be in archive
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 1)
	assert.Equal(t, "aaaa", archived[0].ID)
	assert.Equal(t, "Done Task", archived[0].Name)
	assert.Equal(t, models.StatusDone, archived[0].Status)
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
		Status:  models.StatusCaptured,
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

	// Archive should be empty
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 0)
}

func TestPruneCommand_ArchivesTasksWithDoneChildren(t *testing.T) {
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

	// Both should be pruned from active store
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 0)

	// Both should be in archive
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 2)
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

	// Verify task is archived, not just deleted
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 1)
	assert.Equal(t, "aaaa", archived[0].ID)
}

func TestPruneCommand_AccumulatesInArchive(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// First done task
	task1 := &models.Task{
		ID:      "aaaa",
		Name:    "First Done",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task1))

	prunePretty = false
	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Second done task
	task2 := &models.Task{
		ID:      "bbbb",
		Name:    "Second Done",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task2))

	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Archive should contain both tasks from separate prune operations
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 2)
}

func TestPruneCommand_CleansUpContextDirs(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create two done tasks with content
	task1 := &models.Task{
		ID:          "aaaa",
		Name:        "Done With Content 1",
		Description: "Description one",
		Approach:    "Action one",
		Status:      models.StatusDone,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task1))

	task2 := &models.Task{
		ID:       "bbbb",
		Name:     "Done With Content 2",
		Approach: "Action two",
		Verify:   "Verify two",
		Status:   models.StatusDone,
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task2))

	// Verify context dirs exist
	_, err = os.Stat(store.ContextDir("aaaa"))
	require.NoError(t, err, "context dir for aaaa should exist")
	_, err = os.Stat(store.ContextDir("bbbb"))
	require.NoError(t, err, "context dir for bbbb should exist")

	prunePretty = false
	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Verify context dirs are cleaned up
	_, err = os.Stat(store.ContextDir("aaaa"))
	assert.True(t, os.IsNotExist(err), "context dir for aaaa should be removed after prune")
	_, err = os.Stat(store.ContextDir("bbbb"))
	assert.True(t, os.IsNotExist(err), "context dir for bbbb should be removed after prune")
}

func TestPruneCommand_ArchivesWithContent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create a done task with content fields
	task := &models.Task{
		ID:          "aaaa",
		Name:        "Done With Content",
		Description: "A detailed description",
		Approach:    "Do the thing",
		Verify:      "Check the thing",
		Result:      "Expected result",
		Status:      models.StatusDone,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	prunePretty = false
	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Load from archive and verify content fields are preserved
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	require.Len(t, archived, 1)
	assert.Equal(t, "aaaa", archived[0].ID)
	assert.Equal(t, "A detailed description", archived[0].Description)
	assert.Equal(t, "Do the thing", archived[0].Approach)
	assert.Equal(t, "Check the thing", archived[0].Verify)
	assert.Equal(t, "Expected result", archived[0].Result)
}
