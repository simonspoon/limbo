package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterCompletedTasks_HidesTopLevelDone(t *testing.T) {
	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "Todo task", Status: models.StatusCaptured, Created: now, Updated: now},
		{ID: "aaab", Name: "In-progress task", Status: models.StatusInProgress, Created: now, Updated: now},
		{ID: "aaac", Name: "Done task", Status: models.StatusDone, Created: now, Updated: now},
	}

	result := filterCompletedTasks(tasks)
	assert.Len(t, result, 2)
	for _, task := range result {
		assert.NotEqual(t, models.StatusDone, task.Status)
	}
}

func TestFilterCompletedTasks_HidesDoneWithDoneParent(t *testing.T) {
	now := time.Now()
	parentID := "aaaa"
	tasks := []models.Task{
		{ID: "aaaa", Name: "Parent task", Status: models.StatusDone, Created: now, Updated: now},
		{ID: "aaab", Name: "Child task", Status: models.StatusDone, Parent: &parentID, Created: now, Updated: now},
	}

	result := filterCompletedTasks(tasks)
	assert.Len(t, result, 0)
}

func TestFilterCompletedTasks_ShowsDoneWithActiveParent(t *testing.T) {
	now := time.Now()
	parentID := "aaaa"
	tasks := []models.Task{
		{ID: "aaaa", Name: "Parent task", Status: models.StatusInProgress, Created: now, Updated: now},
		{ID: "aaab", Name: "Child task", Status: models.StatusDone, Parent: &parentID, Created: now, Updated: now},
	}

	result := filterCompletedTasks(tasks)
	assert.Len(t, result, 2)
}

func TestFilterCompletedTasks_KeepsAllNonDone(t *testing.T) {
	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "Todo 1", Status: models.StatusCaptured, Created: now, Updated: now},
		{ID: "aaab", Name: "Todo 2", Status: models.StatusCaptured, Created: now, Updated: now},
		{ID: "aaac", Name: "In-progress", Status: models.StatusInProgress, Created: now, Updated: now},
	}

	result := filterCompletedTasks(tasks)
	assert.Len(t, result, 3)
}

func TestFilterCompletedTasks_EmptyInput(t *testing.T) {
	result := filterCompletedTasks([]models.Task{})
	assert.Empty(t, result)
}

func TestFilterCompletedTasks_MixedHierarchy(t *testing.T) {
	now := time.Now()
	rootAID := "aaaa"
	rootBID := "bbbb"
	tasks := []models.Task{
		{ID: "aaaa", Name: "Root A", Status: models.StatusInProgress, Created: now, Updated: now},
		{ID: "aaab", Name: "Child A1", Status: models.StatusDone, Parent: &rootAID, Created: now, Updated: now},
		{ID: "aaac", Name: "Child A2", Status: models.StatusCaptured, Parent: &rootAID, Created: now, Updated: now},
		{ID: "bbbb", Name: "Root B", Status: models.StatusDone, Created: now, Updated: now},
		{ID: "bbbc", Name: "Child B1", Status: models.StatusDone, Parent: &rootBID, Created: now, Updated: now},
	}

	result := filterCompletedTasks(tasks)
	assert.Len(t, result, 3)

	resultIDs := make(map[string]bool)
	for _, task := range result {
		resultIDs[task.ID] = true
	}
	assert.True(t, resultIDs["aaaa"], "Root A should be kept")
	assert.True(t, resultIDs["aaab"], "Child A1 (done with active parent) should be kept")
	assert.True(t, resultIDs["aaac"], "Child A2 should be kept")
	assert.False(t, resultIDs["bbbb"], "Root B (done, no parent) should be hidden")
	assert.False(t, resultIDs["bbbc"], "Child B1 (done with done parent) should be hidden")
}

func TestListHidesDoneByDefault(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createTestTask(t, store, "Todo task", models.StatusCaptured, nil)
	createTestTask(t, store, "Done task", models.StatusDone, nil)

	// Reset flags
	listStatus = ""
	listPretty = false
	listShowAll = false
	listOwner = ""
	listUnclaimed = false
	listBlocked = false
	listUnblocked = false

	err = runList(nil, []string{})
	assert.NoError(t, err)
}

func TestListShowAll(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createTestTask(t, store, "Todo task", models.StatusCaptured, nil)
	createTestTask(t, store, "Done task", models.StatusDone, nil)

	// Reset flags
	listStatus = ""
	listPretty = false
	listShowAll = true
	listOwner = ""
	listUnclaimed = false
	listBlocked = false
	listUnblocked = false

	err = runList(nil, []string{})
	assert.NoError(t, err)
}

func TestTreeHidesDoneByDefault(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createTestTask(t, store, "Todo task", models.StatusCaptured, nil)
	createTestTask(t, store, "Done task", models.StatusDone, nil)

	// Reset flags
	treePretty = true
	treeShowAll = false

	err = runTree(nil, []string{})
	assert.NoError(t, err)
}

func TestTreeShowAll(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createTestTask(t, store, "Todo task", models.StatusCaptured, nil)
	createTestTask(t, store, "Done task", models.StatusDone, nil)

	// Reset flags
	treePretty = true
	treeShowAll = true

	err = runTree(nil, []string{})
	assert.NoError(t, err)
}
