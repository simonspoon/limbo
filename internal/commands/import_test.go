package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/require"
)

func writeExportFile(t *testing.T, dir string, tasks []models.Task) string {
	t.Helper()
	exportData := storage.TaskStore{
		Version: "4.0.0",
		Tasks:   tasks,
	}
	data, err := json.MarshalIndent(exportData, "", "  ")
	require.NoError(t, err)

	filePath := filepath.Join(dir, "export.json")
	require.NoError(t, os.WriteFile(filePath, data, 0644))
	return filePath
}

func TestImportBasic(t *testing.T) {
	dir, cleanup := setupTestEnv(t)
	defer cleanup()

	resetImportFlags()

	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "Task one", Status: models.StatusTodo, Created: now, Updated: now},
		{ID: "bbbb", Name: "Task two", Status: models.StatusInProgress, Created: now, Updated: now},
	}
	filePath := writeExportFile(t, dir, tasks)

	err := runImport(nil, []string{filePath})
	require.NoError(t, err)

	store, err := storage.NewStorage()
	require.NoError(t, err)
	loaded, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 2)
}

func TestImportMergeMode(t *testing.T) {
	dir, cleanup := setupTestEnv(t)
	defer cleanup()

	resetImportFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create an existing task
	now := time.Now()
	existingID, err := store.GenerateTaskID()
	require.NoError(t, err)
	existing := &models.Task{
		ID:      existingID,
		Name:    "Existing task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(existing))

	// Import one more task (merge mode = default)
	tasks := []models.Task{
		{ID: "aaaa", Name: "Imported task", Status: models.StatusTodo, Created: now, Updated: now},
	}
	filePath := writeExportFile(t, dir, tasks)

	err = runImport(nil, []string{filePath})
	require.NoError(t, err)

	loaded, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 2) // existing + imported
}

func TestImportReplaceMode(t *testing.T) {
	dir, cleanup := setupTestEnv(t)
	defer cleanup()

	resetImportFlags()
	importReplace = true

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create existing tasks
	now := time.Now()
	for i := 0; i < 3; i++ {
		id, err := store.GenerateTaskID()
		require.NoError(t, err)
		task := &models.Task{
			ID:      id,
			Name:    "Existing",
			Status:  models.StatusTodo,
			Created: now,
			Updated: now,
		}
		require.NoError(t, store.SaveTask(task))
	}

	// Import with replace
	tasks := []models.Task{
		{ID: "aaaa", Name: "Replaced task", Status: models.StatusTodo, Created: now, Updated: now},
	}
	filePath := writeExportFile(t, dir, tasks)

	err = runImport(nil, []string{filePath})
	require.NoError(t, err)

	loaded, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1) // only the imported one
	require.Equal(t, "Replaced task", loaded[0].Name)
}

func TestImportPreservesRelationships(t *testing.T) {
	dir, cleanup := setupTestEnv(t)
	defer cleanup()

	resetImportFlags()
	importReplace = true

	now := time.Now()
	parentID := "aaaa"
	childID := "bbbb"
	tasks := []models.Task{
		{ID: parentID, Name: "Parent", Status: models.StatusInProgress, Created: now, Updated: now},
		{ID: childID, Name: "Child", Parent: &parentID, Status: models.StatusTodo, BlockedBy: []string{parentID}, Created: now, Updated: now},
	}
	filePath := writeExportFile(t, dir, tasks)

	err := runImport(nil, []string{filePath})
	require.NoError(t, err)

	store, err := storage.NewStorage()
	require.NoError(t, err)
	loaded, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 2)

	// Find parent and child by name (IDs are remapped)
	var newParentID string
	for _, task := range loaded {
		if task.Name == "Parent" {
			newParentID = task.ID
		}
	}
	require.NotEmpty(t, newParentID)

	for _, task := range loaded {
		if task.Name == "Child" {
			require.NotNil(t, task.Parent)
			require.Equal(t, newParentID, *task.Parent)
			require.Contains(t, task.BlockedBy, newParentID)
		}
	}
}

func TestImportNewIDs(t *testing.T) {
	dir, cleanup := setupTestEnv(t)
	defer cleanup()

	resetImportFlags()

	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "Task", Status: models.StatusTodo, Created: now, Updated: now},
	}
	filePath := writeExportFile(t, dir, tasks)

	err := runImport(nil, []string{filePath})
	require.NoError(t, err)

	store, err := storage.NewStorage()
	require.NoError(t, err)
	loaded, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	// New ID should be generated, not "aaaa"
	require.NotEqual(t, "aaaa", loaded[0].ID)
	require.Equal(t, "Task", loaded[0].Name)
}

func TestImportInvalidFile(t *testing.T) {
	dir, cleanup := setupTestEnv(t)
	defer cleanup()

	resetImportFlags()

	// Write invalid JSON
	filePath := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(filePath, []byte("not json"), 0644))

	err := runImport(nil, []string{filePath})
	require.Error(t, err)
}

func TestImportMissingFile(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetImportFlags()

	err := runImport(nil, []string{"/nonexistent/file.json"})
	require.Error(t, err)
}

func TestImportEmptyFile(t *testing.T) {
	dir, cleanup := setupTestEnv(t)
	defer cleanup()

	resetImportFlags()

	tasks := []models.Task{}
	filePath := writeExportFile(t, dir, tasks)

	err := runImport(nil, []string{filePath})
	require.NoError(t, err)

	store, err := storage.NewStorage()
	require.NoError(t, err)
	loaded, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, loaded, 0)
}
