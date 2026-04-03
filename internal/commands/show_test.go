package commands

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create a test task
	taskID := createTestTask(t, store, "Test Task", models.StatusCaptured, nil)

	// Reset flag
	showPretty = false

	// Test show command
	err = runShow(nil, []string{taskID})
	require.NoError(t, err)
}

func TestShowCommandWithAllFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create parent task
	parentID := createTestTask(t, store, "Parent Task", models.StatusCaptured, nil)

	// Create task with parent
	taskID := createTestTask(t, store, "Child Task", models.StatusInProgress, &parentID)

	// Load task to add description
	task, err := store.LoadTask(taskID)
	require.NoError(t, err)
	task.Description = "Full description"
	require.NoError(t, store.SaveTask(task))

	// Reset flag
	showPretty = false

	// Show the task - just verify it doesn't error
	err = runShow(nil, []string{taskID})
	require.NoError(t, err)
}

func TestShowCommandTaskNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flag
	showPretty = false

	// Test show non-existent task
	err := runShow(nil, []string{"zzzz"})
	assert.Error(t, err)
}

func TestShowCommandInvalidID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flag
	showPretty = false

	// Test show with invalid ID (wrong length)
	err := runShow(nil, []string{"not-valid"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid task ID")
}

func TestShowCommand_BlockedByEnriched(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create blocker task
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Design schema",
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	// Create blocked task
	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Implement API",
		Status:    models.StatusCaptured,
		BlockedBy: []string{"aaaa"},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked))

	// Reset flag
	showPretty = false

	// Capture output by running show
	// We test the JSON structure by unmarshaling the showResult
	// Since runShow prints to stdout, we verify the logic via the types directly
	// But we can verify no error occurs
	err = runShow(nil, []string{blocked.ID})
	require.NoError(t, err)

	// Verify enrichment logic directly
	allTasks, err := store.LoadAll()
	require.NoError(t, err)

	var blockers []blockerInfo
	for _, blockerID := range blocked.BlockedBy {
		if info := findBlockerInfo(allTasks, blockerID); info != nil {
			blockers = append(blockers, *info)
		}
	}

	require.Len(t, blockers, 1)
	assert.Equal(t, "aaaa", blockers[0].ID)
	assert.Equal(t, "Design schema", blockers[0].Name)
	assert.Equal(t, models.StatusInProgress, blockers[0].Status)

	// Verify JSON output structure
	result := showResult{
		Task:     blocked,
		Blockers: blockers,
	}
	out, err := json.Marshal(result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &parsed))
	assert.Contains(t, parsed, "blockers")
	assert.Contains(t, parsed, "blockedBy")
}

func TestShowCommand_BlocksReverseLookup(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create the blocker task
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Design schema",
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	// Create two tasks blocked by the blocker
	blocked1 := &models.Task{
		ID:        "aaab",
		Name:      "Implement API",
		Status:    models.StatusCaptured,
		BlockedBy: []string{"aaaa"},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked1))

	blocked2 := &models.Task{
		ID:        "aaac",
		Name:      "Write tests",
		Status:    models.StatusCaptured,
		BlockedBy: []string{"aaaa"},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked2))

	// Reset flag
	showPretty = false

	// Show the blocker - should list what it blocks
	err = runShow(nil, []string{blocker.ID})
	require.NoError(t, err)

	// Verify reverse lookup logic
	allTasks, err := store.LoadAll()
	require.NoError(t, err)

	var blocks []blockerInfo
	for i := range allTasks {
		for _, depID := range allTasks[i].BlockedBy {
			if depID == blocker.ID {
				blocks = append(blocks, blockerInfo{
					ID:     allTasks[i].ID,
					Name:   allTasks[i].Name,
					Status: allTasks[i].Status,
				})
				break
			}
		}
	}

	require.Len(t, blocks, 2)
	// Verify both blocked tasks are found
	ids := []string{blocks[0].ID, blocks[1].ID}
	assert.Contains(t, ids, "aaab")
	assert.Contains(t, ids, "aaac")
}

func TestShowCommand_StructuredFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:       "aaaa",
		Name:     "Structured Task",
		Status:   models.StatusDone,
		Approach: "run migrations",
		Verify:   "check table exists",
		Result:   "migration output",
		Outcome:  "table created successfully",
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task))

	// Test pretty output doesn't error
	showPretty = true
	err = runShow(nil, []string{task.ID})
	require.NoError(t, err)

	// Test JSON output doesn't error
	showPretty = false
	err = runShow(nil, []string{task.ID})
	require.NoError(t, err)
}

func TestShowCommandNotInProject(t *testing.T) {
	// Create temp directory without initializing
	tmpDir, err := os.MkdirTemp("", "limbo-cmd-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)

	require.NoError(t, os.Chdir(tmpDir))

	// Reset flag
	showPretty = false

	// Test show should fail
	err = runShow(nil, []string{"aaaa"})
	assert.Error(t, err)
}
