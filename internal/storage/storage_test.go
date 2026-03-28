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

func TestIsBlocked(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()

	// Create blocker task (todo)
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	// Create blocked task
	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Blocked",
		Status:    models.StatusTodo,
		BlockedBy: []string{"aaaa"},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked))

	// Should be blocked (blocker is todo)
	isBlocked, err := store.IsBlocked(blocked)
	require.NoError(t, err)
	assert.True(t, isBlocked)

	// Mark blocker as done
	blocker.Status = models.StatusDone
	require.NoError(t, store.SaveTask(blocker))

	// Should no longer be blocked
	isBlocked, err = store.IsBlocked(blocked)
	require.NoError(t, err)
	assert.False(t, isBlocked)

	// Task with no blockers is not blocked
	noDeps := &models.Task{
		ID:      "aaac",
		Name:    "No deps",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	isBlocked, err = store.IsBlocked(noDeps)
	require.NoError(t, err)
	assert.False(t, isBlocked)

	// Task blocked by nonexistent task is not blocked (blocker gone = unblocked)
	ghostBlocked := &models.Task{
		ID:        "aaad",
		Name:      "Ghost blocked",
		Status:    models.StatusTodo,
		BlockedBy: []string{"zzzz"},
		Created:   now,
		Updated:   now,
	}
	isBlocked, err = store.IsBlocked(ghostBlocked)
	require.NoError(t, err)
	assert.False(t, isBlocked)
}

func TestWouldCreateCycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()

	// A blocks B, B blocks C
	a := &models.Task{ID: "aaaa", Name: "A", Status: models.StatusTodo, Created: now, Updated: now}
	b := &models.Task{ID: "aaab", Name: "B", Status: models.StatusTodo, BlockedBy: []string{"aaaa"}, Created: now, Updated: now}
	c := &models.Task{ID: "aaac", Name: "C", Status: models.StatusTodo, BlockedBy: []string{"aaab"}, Created: now, Updated: now}

	require.NoError(t, store.SaveTask(a))
	require.NoError(t, store.SaveTask(b))
	require.NoError(t, store.SaveTask(c))

	// Adding C blocks A would create cycle: A→B→C→A
	wouldCycle, err := store.WouldCreateCycle("aaac", "aaaa")
	require.NoError(t, err)
	assert.True(t, wouldCycle)

	// Adding A blocks C is the existing direction — no cycle from adding D blocks A
	d := &models.Task{ID: "aaad", Name: "D", Status: models.StatusTodo, Created: now, Updated: now}
	require.NoError(t, store.SaveTask(d))

	wouldCycle, err = store.WouldCreateCycle("aaad", "aaaa")
	require.NoError(t, err)
	assert.False(t, wouldCycle)

	// Self-cycle: A blocks A
	wouldCycle, err = store.WouldCreateCycle("aaaa", "aaaa")
	require.NoError(t, err)
	assert.False(t, wouldCycle) // BFS from A doesn't find A in A's BlockedBy (A has no blockers)

	// Direct cycle: if B blocks A (A already blocks B)
	wouldCycle, err = store.WouldCreateCycle("aaab", "aaaa")
	require.NoError(t, err)
	assert.True(t, wouldCycle)
}

func TestRemoveFromAllBlockedBy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()

	// A blocks B and C
	a := &models.Task{ID: "aaaa", Name: "A", Status: models.StatusDone, Created: now, Updated: now}
	b := &models.Task{ID: "aaab", Name: "B", Status: models.StatusTodo, BlockedBy: []string{"aaaa", "aaac"}, Created: now, Updated: now}
	c := &models.Task{ID: "aaac", Name: "C", Status: models.StatusTodo, BlockedBy: []string{"aaaa"}, Created: now, Updated: now}
	d := &models.Task{ID: "aaad", Name: "D", Status: models.StatusTodo, Created: now, Updated: now}

	require.NoError(t, store.SaveTask(a))
	require.NoError(t, store.SaveTask(b))
	require.NoError(t, store.SaveTask(c))
	require.NoError(t, store.SaveTask(d))

	// Remove A from all BlockedBy lists
	err = store.RemoveFromAllBlockedBy("aaaa")
	require.NoError(t, err)

	// B should still be blocked by C but not A
	loadedB, err := store.LoadTask("aaab")
	require.NoError(t, err)
	assert.Equal(t, []string{"aaac"}, loadedB.BlockedBy)

	// C should have empty BlockedBy
	loadedC, err := store.LoadTask("aaac")
	require.NoError(t, err)
	assert.Empty(t, loadedC.BlockedBy)

	// D should be unchanged (was never blocked)
	loadedD, err := store.LoadTask("aaad")
	require.NoError(t, err)
	assert.Empty(t, loadedD.BlockedBy)

	// Removing a task that blocks nothing should be a no-op
	err = store.RemoveFromAllBlockedBy("zzzz")
	require.NoError(t, err)
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

// --- Archive storage-level tests ---

func TestArchiveTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()

	tasks := []models.Task{
		{ID: "aaaa", Name: "Task A", Status: models.StatusDone, Created: now, Updated: now},
		{ID: "aaab", Name: "Task B", Status: models.StatusDone, Created: now, Updated: now},
	}

	// Archive tasks
	err = store.ArchiveTasks(tasks)
	require.NoError(t, err)

	// Verify they're in the archive
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 2)
	assert.Equal(t, "aaaa", archived[0].ID)
	assert.Equal(t, "aaab", archived[1].ID)
}

func TestArchiveTasks_Append(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()

	// First batch
	batch1 := []models.Task{
		{ID: "aaaa", Name: "First", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(batch1))

	// Second batch
	batch2 := []models.Task{
		{ID: "aaab", Name: "Second", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(batch2))

	// Both batches should be in archive
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 2)
	assert.Equal(t, "First", archived[0].Name)
	assert.Equal(t, "Second", archived[1].Name)
}

func TestLoadArchive_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// No archive file exists yet — should return empty slice, not error
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 0)
}

func TestLoadArchive_WithTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "Archived A", Status: models.StatusDone, Created: now, Updated: now},
		{ID: "aaab", Name: "Archived B", Status: models.StatusDone, Created: now, Updated: now},
		{ID: "aaac", Name: "Archived C", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(tasks))

	// Load and verify all fields
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 3)

	for i, task := range archived {
		assert.Equal(t, tasks[i].ID, task.ID)
		assert.Equal(t, tasks[i].Name, task.Name)
		assert.Equal(t, models.StatusDone, task.Status)
	}
}

func TestLoadArchivedTask_Found(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "First", Status: models.StatusDone, Created: now, Updated: now},
		{ID: "aaab", Name: "Second", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(tasks))

	// Find the second task
	task, err := store.LoadArchivedTask("aaab")
	require.NoError(t, err)
	assert.Equal(t, "aaab", task.ID)
	assert.Equal(t, "Second", task.Name)
}

func TestLoadArchivedTask_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// No tasks in archive
	_, err = store.LoadArchivedTask("zzzz")
	assert.Equal(t, ErrTaskNotFound, err)
}

func TestLoadArchivedTask_NotFoundWithTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "Exists", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(tasks))

	// Look for a task that is not in the archive
	_, err = store.LoadArchivedTask("zzzz")
	assert.Equal(t, ErrTaskNotFound, err)
}

func TestUnarchiveTask_Found(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "Stay", Status: models.StatusDone, Created: now, Updated: now},
		{ID: "aaab", Name: "Remove Me", Status: models.StatusDone, Created: now, Updated: now},
		{ID: "aaac", Name: "Also Stay", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(tasks))

	// Unarchive the middle task
	task, err := store.UnarchiveTask("aaab")
	require.NoError(t, err)
	assert.Equal(t, "aaab", task.ID)
	assert.Equal(t, "Remove Me", task.Name)

	// Verify only two remain in archive
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 2)
	assert.Equal(t, "aaaa", archived[0].ID)
	assert.Equal(t, "aaac", archived[1].ID)
}

func TestUnarchiveTask_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Empty archive
	_, err = store.UnarchiveTask("zzzz")
	assert.Equal(t, ErrTaskNotFound, err)
}

func TestUnarchiveTask_NotFoundWithTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "Exists", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(tasks))

	// Look for a task that is not there
	_, err = store.UnarchiveTask("zzzz")
	assert.Equal(t, ErrTaskNotFound, err)

	// Existing task should be untouched
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 1)
}

func TestPurgeArchive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "A", Status: models.StatusDone, Created: now, Updated: now},
		{ID: "aaab", Name: "B", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(tasks))

	// Purge
	err = store.PurgeArchive()
	require.NoError(t, err)

	// Archive should be empty
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 0)
}

func TestPurgeArchive_AlreadyEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Purge with no archive file — should not error
	err = store.PurgeArchive()
	require.NoError(t, err)

	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 0)
}

func TestLoadArchive_CorruptJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Write corrupt JSON to archive file
	archivePath := filepath.Join(tmpDir, LimboDir, ArchiveFile)
	err = os.WriteFile(archivePath, []byte("{not valid json["), 0644)
	require.NoError(t, err)

	// LoadArchive should fail
	_, err = store.LoadArchive()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse archive file")
}

func TestLoadArchivedTask_CorruptJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Write corrupt JSON to archive file
	archivePath := filepath.Join(tmpDir, LimboDir, ArchiveFile)
	err = os.WriteFile(archivePath, []byte("{corrupt}"), 0644)
	require.NoError(t, err)

	_, err = store.LoadArchivedTask("aaaa")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse archive file")
}

func TestUnarchiveTask_CorruptJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Write corrupt JSON to archive file
	archivePath := filepath.Join(tmpDir, LimboDir, ArchiveFile)
	err = os.WriteFile(archivePath, []byte("{{bad}}"), 0644)
	require.NoError(t, err)

	_, err = store.UnarchiveTask("aaaa")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse archive file")
}

func TestArchiveTasks_CorruptExistingArchive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Write corrupt JSON to archive file
	archivePath := filepath.Join(tmpDir, LimboDir, ArchiveFile)
	err = os.WriteFile(archivePath, []byte("not json"), 0644)
	require.NoError(t, err)

	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "A", Status: models.StatusDone, Created: now, Updated: now},
	}

	// ArchiveTasks should fail because it tries to load existing archive first
	err = store.ArchiveTasks(tasks)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse archive file")
}

func TestArchive_ReadPermissionError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	// Create a valid archive then make it unreadable
	now := time.Now()
	tasks := []models.Task{
		{ID: "aaaa", Name: "A", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(tasks))

	archivePath := filepath.Join(tmpDir, LimboDir, ArchiveFile)
	err = os.Chmod(archivePath, 0000)
	require.NoError(t, err)
	defer os.Chmod(archivePath, 0600) //nolint:gosec // restore so cleanup can remove it

	// LoadArchive should fail with a read error
	_, err = store.LoadArchive()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read archive file")
}

func TestGenerateTaskID_ArchiveCollision(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "limbo-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	now := time.Now()

	// Archive a task with a known ID
	archivedTask := []models.Task{
		{ID: "aaaa", Name: "Archived", Status: models.StatusDone, Created: now, Updated: now},
	}
	require.NoError(t, store.ArchiveTasks(archivedTask))

	// Generate many IDs — none should collide with the archived "aaaa"
	for i := 0; i < 200; i++ {
		id, err := store.GenerateTaskID()
		require.NoError(t, err)
		assert.NotEqual(t, "aaaa", id, "GenerateTaskID returned an ID that collides with an archived task")

		// Save the task so subsequent calls also avoid it
		task := &models.Task{
			ID:      id,
			Name:    "Generated",
			Status:  models.StatusTodo,
			Created: now,
			Updated: now,
		}
		require.NoError(t, store.SaveTask(task))
	}
}
