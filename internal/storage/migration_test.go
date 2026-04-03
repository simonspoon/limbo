package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupV4Store creates a temp dir with .limbo/tasks.json containing v4 data.
// Does NOT create the context directory (v4 stores don't have one).
func setupV4Store(t *testing.T, v4JSON string) *Storage {
	t.Helper()
	dir := t.TempDir()
	limboDir := filepath.Join(dir, LimboDir)
	require.NoError(t, os.Mkdir(limboDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(limboDir, TasksFile), []byte(v4JSON), 0644))
	return NewStorageAt(dir)
}

func TestMigrateFromV4_BasicMigration(t *testing.T) {
	v4Data := `{
  "version": "4.0.0",
  "tasks": [
    {
      "id": "abcd",
      "name": "Full Task",
      "description": "A description",
      "action": "Do something",
      "verify": "Check it",
      "result": "Report back",
      "outcome": "It worked",
      "parent": null,
      "status": "todo",
      "created": "2026-02-20T10:00:00Z",
      "updated": "2026-02-20T10:00:00Z"
    },
    {
      "id": "efgh",
      "name": "Partial Task",
      "description": "Only a description",
      "parent": null,
      "status": "in-progress",
      "created": "2026-02-20T11:00:00Z",
      "updated": "2026-02-20T11:00:00Z"
    },
    {
      "id": "ijkl",
      "name": "Metadata Only",
      "parent": null,
      "status": "done",
      "created": "2026-02-20T12:00:00Z",
      "updated": "2026-02-20T12:00:00Z"
    }
  ]
}`

	store := setupV4Store(t, v4Data)

	// Trigger migration via LoadAll
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, tasks, 3)

	// Verify backup file exists
	backupPath := filepath.Join(store.rootDir, LimboDir, TasksFile+".v4.bak")
	backupData, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Contains(t, string(backupData), `"version": "4.0.0"`)

	// Verify tasks.json now has version 5.0.0 with stripped content
	storePath := filepath.Join(store.rootDir, LimboDir, TasksFile)
	rawData, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var rawStore TaskStore
	require.NoError(t, json.Unmarshal(rawData, &rawStore))
	assert.Equal(t, "5.0.0", rawStore.Version)

	// All content fields should be empty in the JSON index
	for _, task := range rawStore.Tasks {
		assert.Equal(t, "", task.Description, "task %s description should be empty in index", task.ID)
		assert.Equal(t, "", task.Action, "task %s action should be empty in index", task.ID)
		assert.Equal(t, "", task.Verify, "task %s verify should be empty in index", task.ID)
		assert.Equal(t, "", task.Result, "task %s result should be empty in index", task.ID)
		assert.Equal(t, "", task.Outcome, "task %s outcome should be empty in index", task.ID)
		assert.Nil(t, task.Notes, "task %s notes should be nil in index", task.ID)
	}

	// Verify full task (abcd) has context file with correct content
	fullTask := findTaskInSlice(tasks, "abcd")
	require.NotNil(t, fullTask)
	assert.Equal(t, "A description", fullTask.Description)
	assert.Equal(t, "Do something", fullTask.Action)
	assert.Equal(t, "Check it", fullTask.Verify)
	assert.Equal(t, "Report back", fullTask.Result)
	assert.Equal(t, "It worked", fullTask.Outcome)

	// Verify partial task (efgh) has context file with only description
	partialTask := findTaskInSlice(tasks, "efgh")
	require.NotNil(t, partialTask)
	assert.Equal(t, "Only a description", partialTask.Description)
	assert.Equal(t, "", partialTask.Action)

	// Verify metadata-only task (ijkl) has no content
	metaTask := findTaskInSlice(tasks, "ijkl")
	require.NotNil(t, metaTask)
	assert.Equal(t, "", metaTask.Description)
	assert.Equal(t, "", metaTask.Action)
}

func TestMigrateFromV4_TaskWithNotes(t *testing.T) {
	v4Data := `{
  "version": "4.0.0",
  "tasks": [
    {
      "id": "abcd",
      "name": "Task With Notes",
      "description": "Has notes",
      "parent": null,
      "status": "in-progress",
      "notes": [
        {
          "content": "First observation",
          "timestamp": "2026-02-20T10:00:00Z"
        },
        {
          "content": "Second observation",
          "timestamp": "2026-02-20T11:00:00Z"
        }
      ],
      "created": "2026-02-20T09:00:00Z",
      "updated": "2026-02-20T11:00:00Z"
    }
  ]
}`

	store := setupV4Store(t, v4Data)

	// Trigger migration
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	task := tasks[0]
	assert.Equal(t, "Has notes", task.Description)
	require.Len(t, task.Notes, 2)
	assert.Equal(t, "First observation", task.Notes[0].Content)
	assert.Equal(t, "Second observation", task.Notes[1].Content)

	// Verify context file contains notes in correct format
	sections, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Contains(t, sections["Notes"], "### 2026-02-20T10:00:00Z")
	assert.Contains(t, sections["Notes"], "First observation")
	assert.Contains(t, sections["Notes"], "### 2026-02-20T11:00:00Z")
	assert.Contains(t, sections["Notes"], "Second observation")
}

func TestMigrateFromV4_MetadataOnlyTask(t *testing.T) {
	v4Data := `{
  "version": "4.0.0",
  "tasks": [
    {
      "id": "abcd",
      "name": "Just Metadata",
      "parent": null,
      "status": "todo",
      "created": "2026-02-20T10:00:00Z",
      "updated": "2026-02-20T10:00:00Z"
    }
  ]
}`

	store := setupV4Store(t, v4Data)

	// Trigger migration
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	assert.Equal(t, "abcd", tasks[0].ID)
	assert.Equal(t, "Just Metadata", tasks[0].Name)
	assert.Equal(t, models.StatusTodo, tasks[0].Status)
	assert.Equal(t, "", tasks[0].Description)

	// No context directory should exist for this task
	_, err = os.Stat(store.ContextDir("abcd"))
	assert.True(t, os.IsNotExist(err))
}

func TestMigrateFromV4_PreservesRelationships(t *testing.T) {
	owner := "agent-1"
	v4Data := `{
  "version": "4.0.0",
  "tasks": [
    {
      "id": "aaaa",
      "name": "Parent Task",
      "description": "The parent",
      "parent": null,
      "status": "in-progress",
      "owner": "` + owner + `",
      "created": "2026-02-20T10:00:00Z",
      "updated": "2026-02-20T10:00:00Z"
    },
    {
      "id": "bbbb",
      "name": "Child Task",
      "action": "Do child work",
      "parent": "aaaa",
      "status": "todo",
      "blockedBy": ["cccc"],
      "created": "2026-02-20T11:00:00Z",
      "updated": "2026-02-20T11:00:00Z"
    },
    {
      "id": "cccc",
      "name": "Blocker Task",
      "parent": null,
      "status": "todo",
      "created": "2026-02-20T12:00:00Z",
      "updated": "2026-02-20T12:00:00Z"
    }
  ]
}`

	store := setupV4Store(t, v4Data)

	// Trigger migration via LoadAll
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, tasks, 3)

	// Verify parent task preserved relationships
	parentTask := findTaskInSlice(tasks, "aaaa")
	require.NotNil(t, parentTask)
	assert.Nil(t, parentTask.Parent)
	require.NotNil(t, parentTask.Owner)
	assert.Equal(t, owner, *parentTask.Owner)
	assert.Equal(t, models.StatusInProgress, parentTask.Status)
	assert.Equal(t, "The parent", parentTask.Description)

	// Verify child task preserved relationships
	childTask := findTaskInSlice(tasks, "bbbb")
	require.NotNil(t, childTask)
	require.NotNil(t, childTask.Parent)
	assert.Equal(t, "aaaa", *childTask.Parent)
	assert.Equal(t, []string{"cccc"}, childTask.BlockedBy)
	assert.Equal(t, "Do child work", childTask.Action)

	// Verify blocker task
	blockerTask := findTaskInSlice(tasks, "cccc")
	require.NotNil(t, blockerTask)
	assert.Nil(t, blockerTask.Parent)

	// Also verify the raw JSON preserves relationships
	storePath := filepath.Join(store.rootDir, LimboDir, TasksFile)
	rawData, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var rawStore TaskStore
	require.NoError(t, json.Unmarshal(rawData, &rawStore))

	rawChild := findTaskInSlice(rawStore.Tasks, "bbbb")
	require.NotNil(t, rawChild)
	require.NotNil(t, rawChild.Parent)
	assert.Equal(t, "aaaa", *rawChild.Parent)
	assert.Equal(t, []string{"cccc"}, rawChild.BlockedBy)
}

// findTaskInSlice is a test helper that finds a task by ID in a slice.
func findTaskInSlice(tasks []models.Task, id string) *models.Task {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}
