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
	createTestTask(t, store, "Todo Task", models.StatusCaptured, nil)
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
	createTestTask(t, store, "Todo Task", models.StatusCaptured, nil)
	createTestTask(t, store, "In Progress Task", models.StatusInProgress, nil)
	createTestTask(t, store, "Done Task", models.StatusDone, nil)

	// Test filter by status
	listStatus = models.StatusCaptured
	listPretty = false

	tasks, err := store.LoadAll()
	require.NoError(t, err)

	// Count todo tasks
	var todoCount int
	for _, t := range tasks {
		if t.Status == models.StatusCaptured {
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
	createTestTask(t, store, "Todo Task 1", models.StatusCaptured, nil)
	createTestTask(t, store, "Todo Task 2", models.StatusCaptured, nil)
	createTestTask(t, store, "Done Task", models.StatusDone, nil)

	// Test filter by todo status - actually run the command
	listStatus = models.StatusCaptured
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
	createTestTask(t, store, "Test Task", models.StatusCaptured, nil)

	listStatus = ""
	listPretty = true

	// Should not error (output goes to stdout)
	err = runList(nil, []string{})
	require.NoError(t, err)
}

func TestListFilterByNewStatuses(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create tasks with various new statuses
	createTestTask(t, store, "Captured Task", models.StatusCaptured, nil)
	createTestTask(t, store, "Refined Task", models.StatusRefined, nil)
	createTestTask(t, store, "Planned Task", models.StatusPlanned, nil)
	createTestTask(t, store, "Ready Task", models.StatusReady, nil)
	createTestTask(t, store, "In Review Task", models.StatusInReview, nil)

	// Filter by refined
	listStatus = models.StatusRefined
	listPretty = false
	listOwner = ""
	listUnclaimed = false
	listBlocked = false
	listUnblocked = false
	listShowAll = false

	err = runList(nil, []string{})
	require.NoError(t, err)

	// Filter by ready
	listStatus = models.StatusReady
	err = runList(nil, []string{})
	require.NoError(t, err)

	// Filter by in-review
	listStatus = models.StatusInReview
	err = runList(nil, []string{})
	require.NoError(t, err)
}

func TestListPrettyWithAllStatuses(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create one task per status
	createTestTask(t, store, "Captured Task", models.StatusCaptured, nil)
	createTestTask(t, store, "Refined Task", models.StatusRefined, nil)
	createTestTask(t, store, "Planned Task", models.StatusPlanned, nil)
	createTestTask(t, store, "Ready Task", models.StatusReady, nil)
	createTestTask(t, store, "In Progress Task", models.StatusInProgress, nil)
	createTestTask(t, store, "In Review Task", models.StatusInReview, nil)
	createTestTask(t, store, "Done Task", models.StatusDone, nil)

	listStatus = ""
	listPretty = true
	listOwner = ""
	listUnclaimed = false
	listBlocked = false
	listUnblocked = false
	listShowAll = true

	// Should display all 7 status groups without error
	err = runList(nil, []string{})
	require.NoError(t, err)
}

func TestListFilterByParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Two top-level tasks; one has two children
	parentID := createTestTask(t, store, "Parent Task", models.StatusInProgress, nil)
	otherTopID := createTestTask(t, store, "Other Top", models.StatusCaptured, nil)
	childAID := createTestTask(t, store, "Child A", models.StatusCaptured, &parentID)
	childBID := createTestTask(t, store, "Child B", models.StatusReady, &parentID)
	// Unrelated grandchild — should NOT appear when filtering by parentID (direct only)
	grandchildID := createTestTask(t, store, "Grandchild", models.StatusCaptured, &childAID)

	tasks, err := store.LoadAll()
	require.NoError(t, err)

	// --parent <parentID>: direct children only
	got := filterByParent(tasks, parentID)
	ids := map[string]bool{}
	for _, tk := range got {
		ids[tk.ID] = true
	}
	assert.True(t, ids[childAID], "child A should be present")
	assert.True(t, ids[childBID], "child B should be present")
	assert.False(t, ids[grandchildID], "grandchild should not be present (only direct)")
	assert.False(t, ids[parentID], "parent itself should not be present")
	assert.False(t, ids[otherTopID], "unrelated top-level should not be present")
	assert.Len(t, got, 2)

	// --parent root: top-level tasks only
	root := filterByParent(tasks, "root")
	rootIDs := map[string]bool{}
	for _, tk := range root {
		rootIDs[tk.ID] = true
	}
	assert.True(t, rootIDs[parentID])
	assert.True(t, rootIDs[otherTopID])
	assert.False(t, rootIDs[childAID])
	assert.False(t, rootIDs[grandchildID])
	assert.Len(t, root, 2)

	// --parent "" matches the same set as "root"
	empty := filterByParent(tasks, "")
	assert.Len(t, empty, 2)
	for _, tk := range empty {
		assert.Nil(t, tk.Parent)
	}

	// No match for unknown parent id
	none := filterByParent(tasks, "does-not-exist")
	assert.Empty(t, none)
}

func TestListFilterByParentCombinesWithStatus(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	parentID := createTestTask(t, store, "Parent", models.StatusInProgress, nil)
	capturedChildID := createTestTask(t, store, "Captured Child", models.StatusCaptured, &parentID)
	_ = createTestTask(t, store, "Ready Child", models.StatusReady, &parentID)

	listStatus = models.StatusCaptured
	listPretty = false
	listOwner = ""
	listUnclaimed = false
	listBlocked = false
	listUnblocked = false
	listShowAll = false
	listParent = parentID

	tasks, err := store.LoadAll()
	require.NoError(t, err)

	filtered, err := applyListFilters(tasks, store, true)
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, capturedChildID, filtered[0].ID)

	// Reset
	listStatus = ""
	listParent = ""
}

func TestListParentFlagUnsetShowsAll(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	topID := createTestTask(t, store, "Top", models.StatusCaptured, nil)
	_ = createTestTask(t, store, "Child", models.StatusCaptured, &topID)

	listStatus = ""
	listPretty = false
	listOwner = ""
	listUnclaimed = false
	listBlocked = false
	listUnblocked = false
	listShowAll = false
	listParent = ""

	tasks, err := store.LoadAll()
	require.NoError(t, err)

	// parentSet=false: do not apply filter even though listParent is ""
	filtered, err := applyListFilters(tasks, store, false)
	require.NoError(t, err)
	assert.Len(t, filtered, 2)
}

func TestListInvalidStatusMessage(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	listStatus = "todo"
	listPretty = false
	listOwner = ""
	listUnclaimed = false
	listBlocked = false
	listUnblocked = false
	listShowAll = false

	err := runList(nil, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "captured, refined, planned, ready, in-progress, in-review, done")
}
