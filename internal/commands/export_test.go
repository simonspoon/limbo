package commands

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestExportEmptyProject(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	err := runExport(nil, nil)
	require.NoError(t, err)
}

func TestExportWithTasks(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	id, err := store.GenerateTaskID()
	require.NoError(t, err)

	task := &models.Task{
		ID:          id,
		Name:        "Test task",
		Description: "A test task",
		Status:      models.StatusCaptured,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	// Export should not error
	err = runExport(nil, nil)
	require.NoError(t, err)
}

func TestExportPreservesRelationships(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create parent task
	parentID, err := store.GenerateTaskID()
	require.NoError(t, err)
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent",
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create child task blocked by parent
	childID, err := store.GenerateTaskID()
	require.NoError(t, err)
	child := &models.Task{
		ID:        childID,
		Name:      "Child",
		Parent:    &parentID,
		Status:    models.StatusCaptured,
		BlockedBy: []string{parentID},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(child))

	// Load and verify relationships are present
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, tasks, 2)

	// Marshal to verify export format
	exportData := storage.TaskStore{
		Version: "4.0.0",
		Tasks:   tasks,
	}
	data, err := json.Marshal(exportData)
	require.NoError(t, err)

	// Parse back and verify
	var parsed storage.TaskStore
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Len(t, parsed.Tasks, 2)

	// Find child in parsed data
	for _, task := range parsed.Tasks {
		if task.Name == "Child" {
			require.NotNil(t, task.Parent)
			require.Equal(t, parentID, *task.Parent)
			require.Contains(t, task.BlockedBy, parentID)
		}
	}
}
