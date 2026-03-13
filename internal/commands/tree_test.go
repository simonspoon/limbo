package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
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
		Status:  models.StatusTodo,
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
		Status:  models.StatusTodo,
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
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(root1))

	child1 := &models.Task{
		ID:      "aaab",
		Name:    "Child 1",
		Parent:  &root1ID,
		Status:  models.StatusTodo,
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
		Status:  models.StatusTodo,
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
			Status:  models.StatusTodo,
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
