package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIntegrationTest(t *testing.T) *Storage {
	t.Helper()
	dir := t.TempDir()
	store := NewStorageAt(dir)
	require.NoError(t, store.Init())
	return store
}

func TestSaveTask_SplitsContentToContextFile(t *testing.T) {
	store := setupIntegrationTest(t)

	now := time.Now()
	task := &models.Task{
		ID:          "abcd",
		Name:        "Test Task",
		Description: "A test description",
		Action:      "Do the thing",
		Verify:      "Check it worked",
		Result:      "Report back",
		Outcome:     "It worked",
		Status:      models.StatusTodo,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	// Read raw tasks.json and verify content fields are absent
	storePath := filepath.Join(store.rootDir, LimboDir, TasksFile)
	data, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var rawStore TaskStore
	require.NoError(t, json.Unmarshal(data, &rawStore))
	require.Len(t, rawStore.Tasks, 1)

	indexTask := rawStore.Tasks[0]
	assert.Equal(t, "abcd", indexTask.ID)
	assert.Equal(t, "Test Task", indexTask.Name)
	assert.Equal(t, models.StatusTodo, indexTask.Status)
	assert.Equal(t, "", indexTask.Description)
	assert.Equal(t, "", indexTask.Action)
	assert.Equal(t, "", indexTask.Verify)
	assert.Equal(t, "", indexTask.Result)
	assert.Equal(t, "", indexTask.Outcome)
	assert.Nil(t, indexTask.Notes)

	// Read context.md and verify content is there
	sections, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Equal(t, "A test description", sections["Description"])
	assert.Equal(t, "Do the thing", sections["Action"])
	assert.Equal(t, "Check it worked", sections["Verify"])
	assert.Equal(t, "Report back", sections["Result"])
	assert.Equal(t, "It worked", sections["Outcome"])
}

func TestLoadTask_MergesContextFile(t *testing.T) {
	store := setupIntegrationTest(t)

	now := time.Now()
	task := &models.Task{
		ID:          "abcd",
		Name:        "Test Task",
		Description: "A test description",
		Action:      "Do the thing",
		Verify:      "Check it worked",
		Result:      "Report back",
		Status:      models.StatusTodo,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	loaded, err := store.LoadTask("abcd")
	require.NoError(t, err)

	assert.Equal(t, "abcd", loaded.ID)
	assert.Equal(t, "Test Task", loaded.Name)
	assert.Equal(t, "A test description", loaded.Description)
	assert.Equal(t, "Do the thing", loaded.Action)
	assert.Equal(t, "Check it worked", loaded.Verify)
	assert.Equal(t, "Report back", loaded.Result)
}

func TestLoadAllIndex_ReturnsMetadataOnly(t *testing.T) {
	store := setupIntegrationTest(t)

	now := time.Now()
	task := &models.Task{
		ID:          "abcd",
		Name:        "Test Task",
		Description: "A test description",
		Action:      "Do the thing",
		Verify:      "Check it worked",
		Status:      models.StatusTodo,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	tasks, err := store.LoadAllIndex()
	require.NoError(t, err)
	require.Len(t, tasks, 1)

	// Index tasks should have metadata but no content
	assert.Equal(t, "abcd", tasks[0].ID)
	assert.Equal(t, "Test Task", tasks[0].Name)
	assert.Equal(t, models.StatusTodo, tasks[0].Status)
	assert.Equal(t, "", tasks[0].Description)
	assert.Equal(t, "", tasks[0].Action)
	assert.Equal(t, "", tasks[0].Verify)
	assert.Nil(t, tasks[0].Notes)
}

func TestRoundTrip_AllFields(t *testing.T) {
	store := setupIntegrationTest(t)

	now := time.Now().Truncate(time.Second)
	parentID := "zzzz"
	owner := "agent-1"
	task := &models.Task{
		ID:          "abcd",
		Name:        "Full Task",
		Description: "A thorough description\nwith multiple lines",
		Action:      "Step 1: Do X\nStep 2: Do Y",
		Verify:      "Run `go test ./...`",
		Result:      "All tests pass",
		Outcome:     "Feature shipped",
		Parent:      &parentID,
		Status:      models.StatusInProgress,
		BlockedBy:   []string{"aaaa", "bbbb"},
		Owner:       &owner,
		Notes: []models.Note{
			{
				Content:   "Started working on this",
				Timestamp: now.Add(-time.Hour),
			},
			{
				Content:   "Made good progress",
				Timestamp: now,
			},
		},
		Created: now.Add(-2 * time.Hour),
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	loaded, err := store.LoadTask("abcd")
	require.NoError(t, err)

	// Compare all fields
	assert.Equal(t, task.ID, loaded.ID)
	assert.Equal(t, task.Name, loaded.Name)
	assert.Equal(t, task.Description, loaded.Description)
	assert.Equal(t, task.Action, loaded.Action)
	assert.Equal(t, task.Verify, loaded.Verify)
	assert.Equal(t, task.Result, loaded.Result)
	assert.Equal(t, task.Outcome, loaded.Outcome)
	assert.Equal(t, task.Status, loaded.Status)
	assert.Equal(t, task.BlockedBy, loaded.BlockedBy)
	require.NotNil(t, loaded.Parent)
	assert.Equal(t, *task.Parent, *loaded.Parent)
	require.NotNil(t, loaded.Owner)
	assert.Equal(t, *task.Owner, *loaded.Owner)

	// Compare notes
	require.Len(t, loaded.Notes, 2)
	assert.Equal(t, task.Notes[0].Content, loaded.Notes[0].Content)
	assert.Equal(t, task.Notes[0].Timestamp.UTC(), loaded.Notes[0].Timestamp.UTC())
	assert.Equal(t, task.Notes[1].Content, loaded.Notes[1].Content)
	assert.Equal(t, task.Notes[1].Timestamp.UTC(), loaded.Notes[1].Timestamp.UTC())
}

func TestSaveTask_EmptyContent_NoContextFile(t *testing.T) {
	store := setupIntegrationTest(t)

	now := time.Now()
	task := &models.Task{
		ID:      "abcd",
		Name:    "Metadata Only",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// No context directory should be created
	_, err := os.Stat(store.ContextDir("abcd"))
	assert.True(t, os.IsNotExist(err))
}

func TestSaveTask_ClearContent_DeletesContextFile(t *testing.T) {
	store := setupIntegrationTest(t)

	now := time.Now()
	task := &models.Task{
		ID:          "abcd",
		Name:        "Test Task",
		Description: "Has a description",
		Action:      "Do something",
		Status:      models.StatusTodo,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	// Verify context file exists
	_, err := os.Stat(store.ContextDir("abcd"))
	require.NoError(t, err)

	// Clear all content fields and save again
	task.Description = ""
	task.Action = ""
	require.NoError(t, store.SaveTask(task))

	// Context directory should be removed
	_, err = os.Stat(store.ContextDir("abcd"))
	assert.True(t, os.IsNotExist(err))

	// LoadTask should still work with no context file
	loaded, err := store.LoadTask("abcd")
	require.NoError(t, err)
	assert.Equal(t, "", loaded.Description)
	assert.Equal(t, "", loaded.Action)
}

func TestSaveTask_DoesNotMutateOriginal(t *testing.T) {
	store := setupIntegrationTest(t)

	now := time.Now()
	task := &models.Task{
		ID:          "abcd",
		Name:        "Test Task",
		Description: "A description",
		Action:      "Do something",
		Notes: []models.Note{
			{Content: "A note", Timestamp: now},
		},
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Verify the original task pointer still has its content fields
	assert.Equal(t, "A description", task.Description)
	assert.Equal(t, "Do something", task.Action)
	require.Len(t, task.Notes, 1)
	assert.Equal(t, "A note", task.Notes[0].Content)
}

func TestLoadAll_MergesAllContextFiles(t *testing.T) {
	store := setupIntegrationTest(t)

	now := time.Now()
	for _, id := range []string{"aaaa", "bbbb", "cccc"} {
		task := &models.Task{
			ID:          id,
			Name:        "Task " + id,
			Description: "Description for " + id,
			Action:      "Action for " + id,
			Status:      models.StatusTodo,
			Created:     now,
			Updated:     now,
		}
		require.NoError(t, store.SaveTask(task))
	}

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	require.Len(t, tasks, 3)

	for _, task := range tasks {
		assert.Equal(t, "Description for "+task.ID, task.Description)
		assert.Equal(t, "Action for "+task.ID, task.Action)
	}
}

func TestInit_CreatesContextDirectory(t *testing.T) {
	dir := t.TempDir()
	store := NewStorageAt(dir)
	require.NoError(t, store.Init())

	contextDir := filepath.Join(dir, LimboDir, ContextDirName)
	info, err := os.Stat(contextDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestInit_Version5(t *testing.T) {
	dir := t.TempDir()
	store := NewStorageAt(dir)
	require.NoError(t, store.Init())

	storePath := filepath.Join(dir, LimboDir, TasksFile)
	data, err := os.ReadFile(storePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"version": "5.0.0"`)
}
