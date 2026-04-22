package commands

import (
	"bytes"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTreeCommand_Empty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flag
	treePretty = true

	// Should not error on empty project
	err := runTree(nil, []string{})
	require.NoError(t, err)
}

func TestTreeCommand_SingleTask(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create a single task
	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Single Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Reset flag
	treePretty = true

	// Should display without error
	err = runTree(nil, []string{})
	require.NoError(t, err)
}

func TestTreeCommand_SimpleHierarchy(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create parent
	now := time.Now()
	parentID := "aaaa"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create child
	child := &models.Task{
		ID:      "aaab",
		Name:    "Child Task",
		Parent:  &parentID,
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Reset flag
	treePretty = true

	// Should display hierarchy
	err = runTree(nil, []string{})
	require.NoError(t, err)
}

func TestTreeCommand_ComplexHierarchy(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	root1ID := "aaaa"
	root1 := &models.Task{
		ID:      root1ID,
		Name:    "Root 1",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(root1))

	child1 := &models.Task{
		ID:      "aaab",
		Name:    "Child 1",
		Parent:  &root1ID,
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child1))

	child2ID := "aaac"
	child2 := &models.Task{
		ID:      child2ID,
		Name:    "Child 2",
		Parent:  &root1ID,
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child2))

	grandchild1 := &models.Task{
		ID:      "aaad",
		Name:    "Grandchild 1",
		Parent:  &child2ID,
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(grandchild1))

	// Reset flag
	treePretty = true

	// Should display full hierarchy
	err = runTree(nil, []string{})
	require.NoError(t, err)
}

func TestTreeCommand_MultipleRoots(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create multiple root tasks
	now := time.Now()
	ids := []string{"aaaa", "aaab", "aaac"}
	for _, id := range ids {
		task := &models.Task{
			ID:      id,
			Name:    "Root Task",
			Status:  models.StatusCaptured,
			Created: now,
			Updated: now,
		}
		require.NoError(t, store.SaveTask(task))
	}

	// Reset flag
	treePretty = true

	// Should display all roots
	err = runTree(nil, []string{})
	require.NoError(t, err)
}

func TestTreeCommand_EmptyJSON(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Test empty with JSON output
	treePretty = false

	err := runTree(nil, []string{})
	require.NoError(t, err)
}

func TestTreeCommand_JSONOutput(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Create a task
	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Test JSON output
	treePretty = false

	err = runTree(nil, []string{})
	require.NoError(t, err)
}

func TestFormatStatus_AllStatuses(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{models.StatusCaptured, "CAPTURED"},
		{models.StatusRefined, "REFINED"},
		{models.StatusPlanned, "PLANNED"},
		{models.StatusReady, "READY"},
		{models.StatusInProgress, "IN-PROG"},
		{models.StatusInReview, "REVIEW"},
		{models.StatusDone, "DONE"},
		{"unknown", "UNKNOWN"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatStatus(tt.status), "formatStatus(%q)", tt.status)
	}
}

func TestGetStatusColor_AllStatuses(t *testing.T) {
	// Verify all 7 statuses return non-nil color and unknown also works
	statuses := []string{
		models.StatusCaptured,
		models.StatusRefined,
		models.StatusPlanned,
		models.StatusReady,
		models.StatusInProgress,
		models.StatusInReview,
		models.StatusDone,
		"unknown",
	}
	for _, s := range statuses {
		c := getStatusColor(s)
		assert.NotNil(t, c, "getStatusColor(%q) should return non-nil", s)
	}
}

// TestPrintTaskTree_Subtree_BlockedChild verifies tree rendering over a
// parent with a mix of blocked and non-blocked children. Exercises the
// same showBlocked=true + allTaskMap wiring that runTree now plumbs,
// matching limbo watch --pretty behavior.
func TestPrintTaskTree_Subtree_BlockedChild(t *testing.T) {
	prev := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = prev })

	now := time.Now()
	rootID := "aaaa"
	root := models.Task{
		ID:      rootID,
		Name:    "Root Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}

	// Manual-blocked child (reason set).
	manualID := "aaab"
	manual := models.Task{
		ID:                manualID,
		Name:              "Manual Blocked Child",
		Parent:            &rootID,
		Status:            models.StatusInProgress,
		ManualBlockReason: "awaiting design review",
		Created:           now.Add(1 * time.Millisecond),
		Updated:           now.Add(1 * time.Millisecond),
	}

	// Dep-blocked child (waits on non-done blocker).
	blockerID := "bbbb"
	blocker := models.Task{
		ID:      blockerID,
		Name:    "Upstream Job",
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	depID := "aaac"
	dep := models.Task{
		ID:        depID,
		Name:      "Dep Blocked Child",
		Parent:    &rootID,
		Status:    models.StatusCaptured,
		BlockedBy: []string{blockerID},
		Created:   now.Add(2 * time.Millisecond),
		Updated:   now.Add(2 * time.Millisecond),
	}

	// Plain (unblocked) child — sibling control.
	plainID := "aaad"
	plain := models.Task{
		ID:      plainID,
		Name:    "Plain Child",
		Parent:  &rootID,
		Status:  models.StatusCaptured,
		Created: now.Add(3 * time.Millisecond),
		Updated: now.Add(3 * time.Millisecond),
	}

	taskMap := map[string]models.Task{
		rootID:   root,
		manualID: manual,
		depID:    dep,
		plainID:  plain,
	}
	// allTaskMap includes the blocker task so its name resolves even
	// when it would otherwise be outside the rendered slice.
	allTaskMap := map[string]models.Task{
		rootID:    root,
		manualID:  manual,
		depID:     dep,
		plainID:   plain,
		blockerID: blocker,
	}

	var buf bytes.Buffer
	r := root
	printTaskTree(&buf, &r, taskMap, "", true, true, allTaskMap)
	out := buf.String()

	// Blocked children carry the 🚫 prefix + dimmed sub-lines.
	assert.Contains(t, out, "🚫 Manual Blocked Child")
	assert.Contains(t, out, "↳ awaiting design review")
	assert.Contains(t, out, "🚫 Dep Blocked Child")
	assert.Contains(t, out, "↳ blocked by: Upstream Job")

	// Root and plain sibling must not have the blocked indicator.
	assert.NotContains(t, out, "🚫 Root Task")
	assert.NotContains(t, out, "🚫 Plain Child")
}

// TestPrintTaskTree_Subtree_ShowBlockedFalse verifies the pre-fix
// behavior: when showBlocked=false the blocked indicator is suppressed
// even inside a subtree. Guards against regressions that would leak 🚫
// into callers that opt out.
func TestPrintTaskTree_Subtree_ShowBlockedFalse(t *testing.T) {
	prev := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = prev })

	now := time.Now()
	rootID := "aaaa"
	root := models.Task{ID: rootID, Name: "Root", Status: models.StatusCaptured, Created: now, Updated: now}
	childID := "aaab"
	child := models.Task{
		ID:                childID,
		Name:              "Blocked Child",
		Parent:            &rootID,
		Status:            models.StatusInProgress,
		ManualBlockReason: "halt",
		Created:           now.Add(time.Millisecond),
		Updated:           now.Add(time.Millisecond),
	}
	taskMap := map[string]models.Task{rootID: root, childID: child}

	var buf bytes.Buffer
	r := root
	printTaskTree(&buf, &r, taskMap, "", true, false, nil)
	out := buf.String()

	assert.NotContains(t, out, "🚫")
	assert.NotContains(t, out, "↳")
}

// TestTreeCommand_BlockedSubtree exercises the full runTree path with
// a blocked child in the hierarchy, ensuring the pretty-tree wiring
// (showBlocked=true + unfiltered allTaskMap) survives end-to-end.
func TestTreeCommand_BlockedSubtree(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	rootID := "aaaa"
	require.NoError(t, store.SaveTask(&models.Task{
		ID: rootID, Name: "Root", Status: models.StatusCaptured,
		Created: now, Updated: now,
	}))
	require.NoError(t, store.SaveTask(&models.Task{
		ID: "aaab", Name: "Blocked Child", Parent: &rootID,
		Status: models.StatusInProgress, ManualBlockReason: "waiting",
		Created: now.Add(time.Millisecond), Updated: now.Add(time.Millisecond),
	}))

	treePretty = true
	treeShowAll = false

	// runTree should not error with a blocked child in the hierarchy.
	require.NoError(t, runTree(nil, []string{}))
}

func TestTreeCommand_AllStatuses(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	statuses := []string{
		models.StatusCaptured,
		models.StatusRefined,
		models.StatusPlanned,
		models.StatusReady,
		models.StatusInProgress,
		models.StatusInReview,
		models.StatusDone,
	}
	ids := []string{"aaaa", "aaab", "aaac", "aaad", "aaae", "aaaf", "aaag"}
	for i, id := range ids {
		task := &models.Task{
			ID:      id,
			Name:    "Task " + statuses[i],
			Status:  statuses[i],
			Created: now.Add(time.Duration(i) * time.Millisecond),
			Updated: now.Add(time.Duration(i) * time.Millisecond),
		}
		require.NoError(t, store.SaveTask(task))
	}

	treePretty = true
	treeShowAll = true

	err = runTree(nil, []string{})
	require.NoError(t, err)
}
