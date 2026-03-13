package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)

	// Test initialization
	err = store.Init()
	require.NoError(t, err)

	// Verify .limbo directory exists
	limboPath := filepath.Join(tmpDir, LimboDir)
	info, err := os.Stat(limboPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify tasks.json exists
	tasksPath := filepath.Join(limboPath, TasksFile)
	_, err = os.Stat(tasksPath)
	require.NoError(t, err)

	// Test duplicate init fails
	err = store.Init()
	assert.Error(t, err)
}

func TestSaveAndLoadTask(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create a test task
	now := time.Now()
	task := &models.Task{
		ID:          "aaaa",
		Name:        "Test Task",
		Description: "Test Description",
		Status:      models.StatusTodo,
		Created:     now,
		Updated:     now,
	}

	// Save the task
	err = store.SaveTask(task)
	require.NoError(t, err)

	// Load the task
	loaded, err := store.LoadTask(task.ID)
	require.NoError(t, err)

	// Verify task fields
	assert.Equal(t, task.ID, loaded.ID)
	assert.Equal(t, task.Name, loaded.Name)
	assert.Equal(t, task.Description, loaded.Description)
	assert.Equal(t, task.Status, loaded.Status)
}

func TestLoadAll(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create multiple tasks
	now := time.Now()
	ids := []string{"aaaa", "aaab", "aaac"}
	for _, id := range ids {
		task := &models.Task{
			ID:      id,
			Name:    "Test Task",
			Status:  models.StatusTodo,
			Created: now,
			Updated: now,
		}
		require.NoError(t, store.SaveTask(task))
	}

	// Load all tasks
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 3)
}

func TestDeleteTask(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create and save a test task
	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Task to Delete",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Delete the task
	err = store.DeleteTask(task.ID)
	require.NoError(t, err)

	// Verify task is gone
	_, err = store.LoadTask(task.ID)
	assert.Equal(t, ErrTaskNotFound, err)
}

func TestDeleteTasks(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create multiple tasks
	now := time.Now()
	ids := []string{"aaaa", "aaab", "aaac"}
	for _, id := range ids {
		task := &models.Task{
			ID:      id,
			Name:    "Task",
			Status:  models.StatusTodo,
			Created: now,
			Updated: now,
		}
		require.NoError(t, store.SaveTask(task))
	}

	// Delete first two tasks
	err = store.DeleteTasks(ids[:2])
	require.NoError(t, err)

	// Verify only one task remains
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, ids[2], tasks[0].ID)
}

func TestTaskWithParent(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create parent task
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

	// Create child task
	child := &models.Task{
		ID:      "aaab",
		Name:    "Child Task",
		Parent:  &parentID,
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Load and verify child has parent
	loaded, err := store.LoadTask(child.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.Parent)
	assert.Equal(t, parentID, *loaded.Parent)
}

func TestGetChildren(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create parent task
	now := time.Now()
	parentID := "aaaa"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create child tasks
	childIDs := []string{"aaab", "aaac", "aaad"}
	for _, id := range childIDs {
		child := &models.Task{
			ID:      id,
			Name:    "Child",
			Parent:  &parentID,
			Status:  models.StatusTodo,
			Created: now,
			Updated: now,
		}
		require.NoError(t, store.SaveTask(child))
	}

	// Get children
	children, err := store.GetChildren(parentID)
	require.NoError(t, err)
	assert.Len(t, children, 3)
}

func TestGetNextTask(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// No tasks - should return empty result
	next, err := store.GetNextTask()
	require.NoError(t, err)
	require.NotNil(t, next)
	assert.Nil(t, next.Task)
	assert.Empty(t, next.Candidates)

	// Create tasks with different creation times
	baseTime := time.Now()
	task1 := &models.Task{
		ID:      "aaaa",
		Name:    "First Task",
		Status:  models.StatusTodo,
		Created: baseTime,
		Updated: baseTime,
	}
	require.NoError(t, store.SaveTask(task1))

	time.Sleep(10 * time.Millisecond)
	task2 := &models.Task{
		ID:      "aaab",
		Name:    "Second Task",
		Status:  models.StatusTodo,
		Created: time.Now(),
		Updated: time.Now(),
	}
	require.NoError(t, store.SaveTask(task2))

	// No in-progress tasks - should return candidates (oldest first)
	next, err = store.GetNextTask()
	require.NoError(t, err)
	require.NotNil(t, next)
	require.Len(t, next.Candidates, 2)
	assert.Equal(t, task1.ID, next.Candidates[0].ID)

	// Mark first task as in-progress
	task1.Status = models.StatusInProgress
	require.NoError(t, store.SaveTask(task1))

	// Should get second task as sibling of in-progress task
	next, err = store.GetNextTask()
	require.NoError(t, err)
	require.NotNil(t, next)
	require.NotNil(t, next.Task)
	assert.Equal(t, task2.ID, next.Task.ID)
}

func TestGetNextTask_DepthFirst(t *testing.T) {
	// Test progressive decomposition scenario:
	// - Feature A (in-progress) has children A1 (in-progress), A2 (todo), A3 (todo)
	// - A1 has children A1a (done), A1b (todo), A1c (todo)
	// Expected: next should return A1b (sibling of deepest in-progress)

	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	baseTime := time.Now()

	// Create Feature A (in-progress, root level)
	featureAID := "aaaa"
	featureA := &models.Task{
		ID:      featureAID,
		Name:    "Feature A",
		Status:  models.StatusInProgress,
		Created: baseTime,
		Updated: baseTime,
	}
	require.NoError(t, store.SaveTask(featureA))

	// Create A1 (in-progress, child of A)
	a1ID := "aaab"
	a1 := &models.Task{
		ID:      a1ID,
		Name:    "A1",
		Parent:  &featureAID,
		Status:  models.StatusInProgress,
		Created: baseTime.Add(time.Millisecond),
		Updated: baseTime.Add(time.Millisecond),
	}
	require.NoError(t, store.SaveTask(a1))

	// Create A2 (todo, child of A)
	a2 := &models.Task{
		ID:      "aaac",
		Name:    "A2",
		Parent:  &featureAID,
		Status:  models.StatusTodo,
		Created: baseTime.Add(2 * time.Millisecond),
		Updated: baseTime.Add(2 * time.Millisecond),
	}
	require.NoError(t, store.SaveTask(a2))

	// Create A1a (done, child of A1)
	a1a := &models.Task{
		ID:      "aaba",
		Name:    "A1a",
		Parent:  &a1ID,
		Status:  models.StatusDone,
		Created: baseTime.Add(10 * time.Millisecond),
		Updated: baseTime.Add(10 * time.Millisecond),
	}
	require.NoError(t, store.SaveTask(a1a))

	// Create A1b (todo, child of A1) - this should be returned
	a1b := &models.Task{
		ID:      "aabb",
		Name:    "A1b",
		Parent:  &a1ID,
		Status:  models.StatusTodo,
		Created: baseTime.Add(11 * time.Millisecond),
		Updated: baseTime.Add(11 * time.Millisecond),
	}
	require.NoError(t, store.SaveTask(a1b))

	// Create A1c (todo, child of A1)
	a1c := &models.Task{
		ID:      "aabc",
		Name:    "A1c",
		Parent:  &a1ID,
		Status:  models.StatusTodo,
		Created: baseTime.Add(12 * time.Millisecond),
		Updated: baseTime.Add(12 * time.Millisecond),
	}
	require.NoError(t, store.SaveTask(a1c))

	// Should return A1b (sibling of deepest in-progress task A1)
	next, err := store.GetNextTask()
	require.NoError(t, err)
	require.NotNil(t, next)
	require.NotNil(t, next.Task)
	assert.Equal(t, "A1b", next.Task.Name)

	// Mark A1b as done, should return A1c
	a1b.Status = models.StatusDone
	require.NoError(t, store.SaveTask(a1b))

	next, err = store.GetNextTask()
	require.NoError(t, err)
	require.NotNil(t, next.Task)
	assert.Equal(t, "A1c", next.Task.Name)

	// Mark A1c as done, A1 has no more todo children
	// Should move up and return A2 (sibling of A1)
	a1c.Status = models.StatusDone
	require.NoError(t, store.SaveTask(a1c))

	next, err = store.GetNextTask()
	require.NoError(t, err)
	require.NotNil(t, next.Task)
	assert.Equal(t, "A2", next.Task.Name)
}

func TestGetNextTask_InProgressRootNoTodos(t *testing.T) {
	// Edge case: in-progress root task with no todo children or siblings
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()

	// Single in-progress root task, no children
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Lone Task",
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Should return empty result (no todos anywhere)
	next, err := store.GetNextTask()
	require.NoError(t, err)
	assert.Nil(t, next.Task)
	assert.Empty(t, next.Candidates)
}

func TestGetNextTask_WalksUpToRoot(t *testing.T) {
	// Test that we walk up and find root-level siblings
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()

	// Root task A (in-progress)
	taskAID := "aaaa"
	taskA := &models.Task{
		ID:      taskAID,
		Name:    "Task A",
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(taskA))

	// Root task B (todo) - sibling of A at root level
	taskB := &models.Task{
		ID:      "aaab",
		Name:    "Task B",
		Status:  models.StatusTodo,
		Created: now.Add(time.Millisecond),
		Updated: now.Add(time.Millisecond),
	}
	require.NoError(t, store.SaveTask(taskB))

	// Child of A (in-progress, deepest)
	childA1 := &models.Task{
		ID:      "aaba",
		Name:    "A1",
		Parent:  &taskAID,
		Status:  models.StatusInProgress,
		Created: now.Add(10 * time.Millisecond),
		Updated: now.Add(10 * time.Millisecond),
	}
	require.NoError(t, store.SaveTask(childA1))

	// A1 has no todo children, A has no todo children
	// Should walk up and find B (root-level sibling of A)
	next, err := store.GetNextTask()
	require.NoError(t, err)
	require.NotNil(t, next.Task)
	assert.Equal(t, "Task B", next.Task.Name)
}

func TestHasUndoneChildren(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create parent task
	now := time.Now()
	parentID := "aaaa"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// No children - should return false
	hasUndone, err := store.HasUndoneChildren(parentID)
	require.NoError(t, err)
	assert.False(t, hasUndone)

	// Add undone child
	child := &models.Task{
		ID:      "aaab",
		Name:    "Child",
		Parent:  &parentID,
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Should return true
	hasUndone, err = store.HasUndoneChildren(parentID)
	require.NoError(t, err)
	assert.True(t, hasUndone)

	// Mark child as done
	child.Status = models.StatusDone
	require.NoError(t, store.SaveTask(child))

	// Should return false
	hasUndone, err = store.HasUndoneChildren(parentID)
	require.NoError(t, err)
	assert.False(t, hasUndone)
}

func TestHasUndoneChildrenRecursive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()
	grandparentID := "aaaa"

	// Create grandparent
	grandparent := &models.Task{
		ID:      grandparentID,
		Name:    "Grandparent",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(grandparent))

	// Create parent (done)
	parentID := "aaab"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent",
		Status:  models.StatusDone,
		Parent:  &grandparentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create child (undone)
	child := &models.Task{
		ID:      "aaac",
		Name:    "Child",
		Status:  models.StatusTodo,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Grandparent should have undone descendants
	hasUndone, err := store.HasUndoneChildren(grandparentID)
	require.NoError(t, err)
	assert.True(t, hasUndone)
}

func TestOrphanChildren(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()
	parentID := "aaaa"

	// Create parent
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create two children
	child1 := &models.Task{
		ID:      "aaab",
		Name:    "Child 1",
		Status:  models.StatusDone,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child1))

	child2 := &models.Task{
		ID:      "aaac",
		Name:    "Child 2",
		Status:  models.StatusDone,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child2))

	// Verify children have parent
	children, err := store.GetChildren(parentID)
	require.NoError(t, err)
	assert.Len(t, children, 2)

	// Orphan children
	err = store.OrphanChildren(parentID)
	require.NoError(t, err)

	// Verify children are orphaned
	children, err = store.GetChildren(parentID)
	require.NoError(t, err)
	assert.Len(t, children, 0)

	// Verify children still exist but have no parent
	loadedChild1, err := store.LoadTask(child1.ID)
	require.NoError(t, err)
	assert.Nil(t, loadedChild1.Parent)

	loadedChild2, err := store.LoadTask(child2.ID)
	require.NoError(t, err)
	assert.Nil(t, loadedChild2.Parent)
}

func TestGetRootDir(t *testing.T) {
	store := NewStorageAt("/some/path")
	assert.Equal(t, "/some/path", store.GetRootDir())
}

func TestNewStorage(t *testing.T) {
	// Create temp directory with .limbo initialized
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Save original directory and change to temp dir
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	// NewStorage should find the project
	newStore, err := NewStorage()
	require.NoError(t, err)
	assert.Equal(t, tmpDir, newStore.GetRootDir())
}

func TestNewStorageNotInProject(t *testing.T) {
	// Create temp directory without .limbo
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Save original directory and change to temp dir
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer os.Chdir(origDir)

	// NewStorage should fail
	_, err = NewStorage()
	assert.Equal(t, ErrNotInProject, err)
}

func TestFindProjectRootInParent(t *testing.T) {
	// Create temp directory structure: parent/.limbo and parent/child
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Resolve symlinks (macOS /var -> /private/var)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)

	// Initialize .limbo in parent
	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create nested child directory
	childDir := filepath.Join(tmpDir, "child", "grandchild")
	require.NoError(t, os.MkdirAll(childDir, 0755))

	// Save original directory and change to child dir
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(childDir))
	defer os.Chdir(origDir)

	// NewStorage should find the project in parent
	newStore, err := NewStorage()
	require.NoError(t, err)
	assert.Equal(t, tmpDir, newStore.GetRootDir())
}

func TestLoadStoreCorruptedJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Corrupt the tasks.json file
	tasksPath := filepath.Join(tmpDir, LimboDir, TasksFile)
	err = os.WriteFile(tasksPath, []byte("not valid json{"), 0644)
	require.NoError(t, err)

	// LoadAll should fail
	_, err = store.LoadAll()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestDeleteTaskNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Delete non-existent task
	err = store.DeleteTask("zzzz")
	assert.Equal(t, ErrTaskNotFound, err)
}

func TestDeleteTaskWithMultipleTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create multiple tasks
	now := time.Now()
	ids := []string{"aaaa", "aaab", "aaac", "aaad", "aaae"}
	for _, id := range ids {
		task := &models.Task{
			ID:      id,
			Name:    "Task",
			Status:  models.StatusTodo,
			Created: now,
			Updated: now,
		}
		require.NoError(t, store.SaveTask(task))
	}

	// Delete middle task
	err = store.DeleteTask("aaac")
	require.NoError(t, err)

	// Verify 4 tasks remain
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 4)
}

func TestLoadTaskNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Load non-existent task
	_, err = store.LoadTask("zzzz")
	assert.Equal(t, ErrTaskNotFound, err)
}

func TestGetChildrenEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Get children of non-existent task
	children, err := store.GetChildren("zzzz")
	require.NoError(t, err)
	assert.Len(t, children, 0)
}

func TestMigrateFromV3(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create .limbo directory manually with a v3 store
	limboPath := filepath.Join(tmpDir, LimboDir)
	require.NoError(t, os.Mkdir(limboPath, 0755))

	v3Data := []byte(`{"version":"3.0.0","tasks":[{"id":"aaaa","name":"Legacy Task","parent":null,"status":"todo","created":"2026-01-01T00:00:00Z","updated":"2026-01-01T00:00:00Z"}]}`)
	tasksPath := filepath.Join(limboPath, TasksFile)
	require.NoError(t, os.WriteFile(tasksPath, v3Data, 0644))

	store := NewStorageAt(tmpDir)

	// Loading should trigger migration
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "Legacy Task", tasks[0].Name)
	assert.Empty(t, tasks[0].Action)
	assert.Empty(t, tasks[0].Verify)
	assert.Empty(t, tasks[0].Result)

	// Verify backup was created
	backupPath := tasksPath + ".v3.bak"
	_, err = os.Stat(backupPath)
	require.NoError(t, err, "v3 backup file should exist")

	// Verify version was bumped in the file
	data, err := os.ReadFile(tasksPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"version": "4.0.0"`)
}

func TestGenerateTaskID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Generate a few IDs and check they are valid
	for i := 0; i < 10; i++ {
		id, err := store.GenerateTaskID()
		require.NoError(t, err)
		assert.True(t, models.IsValidTaskID(id), "Generated ID %q should be valid", id)
	}
}

func TestGenerateTaskID_Collision(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Generate several IDs and ensure uniqueness
	generated := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := store.GenerateTaskID()
		require.NoError(t, err)

		// Create a task with this ID to force collision checking
		task := &models.Task{
			ID:      id,
			Name:    "Task",
			Status:  models.StatusTodo,
			Created: time.Now(),
			Updated: time.Now(),
		}
		require.NoError(t, store.SaveTask(task))
		generated[id] = true
	}

	// All IDs should be unique
	assert.Len(t, generated, 100)
}
