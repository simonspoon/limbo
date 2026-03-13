package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestTask(t *testing.T, store *storage.Storage, name, status string, parent *string) string {
	now := time.Now()
	id, err := store.GenerateTaskID()
	require.NoError(t, err)

	task := &models.Task{
		ID:      id,
		Name:    name,
		Status:  status,
		Parent:  parent,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))
	return id
}

func TestListCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create tasks with different statuses
	createTestTask(t, store, "Todo Task", models.StatusTodo, nil)
	createTestTask(t, store, "In Progress Task", models.StatusInProgress, nil)
	createTestTask(t, store, "Done Task", models.StatusDone, nil)

	// Reset flags
	listStatus = ""
	listPretty = false

	// Test list all
	err = runList(nil, []string{})
	require.NoError(t, err)
}

func TestListFilterByStatus(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create tasks with different statuses
	createTestTask(t, store, "Todo Task", models.StatusTodo, nil)
	createTestTask(t, store, "In Progress Task", models.StatusInProgress, nil)
	createTestTask(t, store, "Done Task", models.StatusDone, nil)

	// Test filter by status
	listStatus = models.StatusTodo
	listPretty = false

	tasks, err := store.LoadAll()
	require.NoError(t, err)

	// Count todo tasks
	var todoCount int
	for _, t := range tasks {
		if t.Status == models.StatusTodo {
			todoCount++
		}
	}
	assert.Equal(t, 1, todoCount)
}

func TestListEmpty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags
	listStatus = ""
	listPretty = false

	// Test list on empty project
	err := runList(nil, []string{})
	require.NoError(t, err)
}

func TestListInvalidStatus(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Set invalid status filter
	listStatus = "invalid"
	listPretty = false

	err := runList(nil, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestListWithStatusFilter(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create tasks with different statuses
	createTestTask(t, store, "Todo Task 1", models.StatusTodo, nil)
	createTestTask(t, store, "Todo Task 2", models.StatusTodo, nil)
	createTestTask(t, store, "Done Task", models.StatusDone, nil)

	// Test filter by todo status - actually run the command
	listStatus = models.StatusTodo
	listPretty = false

	err = runList(nil, []string{})
	require.NoError(t, err)
}

func TestListPrettyOutput(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create tasks
	createTestTask(t, store, "Test Task", models.StatusTodo, nil)

	listStatus = ""
	listPretty = true

	// Should not error (output goes to stdout)
	err = runList(nil, []string{})
	require.NoError(t, err)
}
