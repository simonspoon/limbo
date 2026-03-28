package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper to create a done task, prune it into the archive, and return the storage
func setupArchiveWithTask(t *testing.T, id, name string) *storage.Storage {
	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      id,
		Name:    name,
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	prunePretty = false
	require.NoError(t, runPrune(nil, nil))

	return store
}

// --- archive list ---

func TestArchiveList_Empty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	archiveListPretty = false
	err := runArchiveList(nil, nil)
	require.NoError(t, err)
}

func TestArchiveList_WithTasks(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store := setupArchiveWithTask(t, "aaaa", "Archived Task")

	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 1)

	archiveListPretty = false
	err = runArchiveList(nil, nil)
	require.NoError(t, err)
}

func TestArchiveList_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	setupArchiveWithTask(t, "aaaa", "Archived Task")

	archiveListPretty = true
	err := runArchiveList(nil, nil)
	require.NoError(t, err)
}

func TestArchiveList_PrettyEmpty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	archiveListPretty = true
	err := runArchiveList(nil, nil)
	require.NoError(t, err)
}

// --- archive show ---

func TestArchiveShow_Found(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	setupArchiveWithTask(t, "aaaa", "Show Me")

	archiveShowPretty = false
	err := runArchiveShow(nil, []string{"aaaa"})
	require.NoError(t, err)
}

func TestArchiveShow_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	setupArchiveWithTask(t, "aaaa", "Show Me Pretty")

	archiveShowPretty = true
	err := runArchiveShow(nil, []string{"aaaa"})
	require.NoError(t, err)
}

func TestArchiveShow_NotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	archiveShowPretty = false
	err := runArchiveShow(nil, []string{"zzzz"})
	require.Error(t, err)
}

func TestArchiveShow_InvalidID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	archiveShowPretty = false
	err := runArchiveShow(nil, []string{"INVALID"})
	require.Error(t, err)
}

// --- archive restore ---

func TestArchiveRestore_Basic(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store := setupArchiveWithTask(t, "aaaa", "Restore Me")

	archiveRestorePretty = false
	err := runArchiveRestore(nil, []string{"aaaa"})
	require.NoError(t, err)

	// Task should be back in active store with status done
	task, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "Restore Me", task.Name)
	assert.Equal(t, models.StatusDone, task.Status)

	// Archive should be empty
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 0)
}

func TestArchiveRestore_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	setupArchiveWithTask(t, "aaaa", "Restore Pretty")

	archiveRestorePretty = true
	err := runArchiveRestore(nil, []string{"aaaa"})
	require.NoError(t, err)
}

func TestArchiveRestore_IDCollision(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store := setupArchiveWithTask(t, "aaaa", "Archived One")

	// Create a new task with the same ID in the active store
	now := time.Now()
	conflicting := &models.Task{
		ID:      "aaaa",
		Name:    "Active Conflicting",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(conflicting))

	archiveRestorePretty = false
	err := runArchiveRestore(nil, []string{"aaaa"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists in active store")

	// Archive should still have the task (not removed on collision)
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 1)
}

func TestArchiveRestore_OrphanedParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create a task with a parent pointer
	parentID := "pppp"
	task := &models.Task{
		ID:      "cccc",
		Name:    "Child Task",
		Status:  models.StatusDone,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Prune it to archive
	prunePretty = false
	require.NoError(t, runPrune(nil, nil))

	// Parent "pppp" does not exist in active store
	archiveRestorePretty = false
	err = runArchiveRestore(nil, []string{"cccc"})
	require.NoError(t, err)

	// Restored task should have nil Parent (orphaned)
	restored, err := store.LoadTask("cccc")
	require.NoError(t, err)
	assert.Nil(t, restored.Parent)
}

func TestArchiveRestore_PreservesParentWhenExists(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create parent in active store
	parentID := "pppp"
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Create child task
	child := &models.Task{
		ID:      "cccc",
		Name:    "Child Task",
		Status:  models.StatusDone,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	// Prune child to archive (parent is todo, so only child is pruned)
	prunePretty = false
	require.NoError(t, runPrune(nil, nil))

	// Restore
	archiveRestorePretty = false
	err = runArchiveRestore(nil, []string{"cccc"})
	require.NoError(t, err)

	// Parent pointer should be preserved
	restored, err := store.LoadTask("cccc")
	require.NoError(t, err)
	require.NotNil(t, restored.Parent)
	assert.Equal(t, parentID, *restored.Parent)
}

func TestArchiveRestore_ClearsStaleBlockedBy(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create a task that has BlockedBy references
	task := &models.Task{
		ID:        "aaaa",
		Name:      "Blocked Task",
		Status:    models.StatusDone,
		BlockedBy: []string{"xxxx", "yyyy"},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(task))

	// Prune to archive
	prunePretty = false
	require.NoError(t, runPrune(nil, nil))

	// Neither xxxx nor yyyy exist in active store
	archiveRestorePretty = false
	err = runArchiveRestore(nil, []string{"aaaa"})
	require.NoError(t, err)

	// BlockedBy should be cleared (both refs are stale)
	restored, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Len(t, restored.BlockedBy, 0)
}

func TestArchiveRestore_KeepsValidBlockedBy(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Create a valid blocker in active store
	blocker := &models.Task{
		ID:      "bbbb",
		Name:    "Blocker",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	// Create task with mixed BlockedBy (one valid, one stale)
	task := &models.Task{
		ID:        "aaaa",
		Name:      "Mixed Blocked",
		Status:    models.StatusDone,
		BlockedBy: []string{"bbbb", "zzzz"},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(task))

	// Prune to archive
	prunePretty = false
	require.NoError(t, runPrune(nil, nil))

	// Restore
	archiveRestorePretty = false
	err = runArchiveRestore(nil, []string{"aaaa"})
	require.NoError(t, err)

	// Only the valid blocker should remain
	restored, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, []string{"bbbb"}, restored.BlockedBy)
}

func TestArchiveRestore_NotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	archiveRestorePretty = false
	err := runArchiveRestore(nil, []string{"zzzz"})
	require.Error(t, err)
}

func TestArchiveRestore_InvalidID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	archiveRestorePretty = false
	err := runArchiveRestore(nil, []string{"BAD!"})
	require.Error(t, err)
}

// --- archive purge ---

func TestArchivePurge_Empty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	archivePurgePretty = false
	err := runArchivePurge(nil, nil)
	require.NoError(t, err)
}

func TestArchivePurge_WithTasks(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store := setupArchiveWithTask(t, "aaaa", "To Purge")

	archivePurgePretty = false
	err := runArchivePurge(nil, nil)
	require.NoError(t, err)

	// Archive should be empty after purge
	archived, err := store.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 0)
}

func TestArchivePurge_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	setupArchiveWithTask(t, "aaaa", "To Purge Pretty")

	archivePurgePretty = true
	err := runArchivePurge(nil, nil)
	require.NoError(t, err)
}

func TestArchivePurge_PrettyEmpty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	archivePurgePretty = true
	err := runArchivePurge(nil, nil)
	require.NoError(t, err)
}

// --- full lifecycle ---

func TestArchive_FullLifecycle(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Step 1: Create and complete a task
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Lifecycle Task",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Step 2: Prune moves it to archive
	prunePretty = false
	require.NoError(t, runPrune(nil, nil))

	tasks, _ := store.LoadAll()
	assert.Len(t, tasks, 0)
	archived, _ := store.LoadArchive()
	assert.Len(t, archived, 1)

	// Step 3: Restore from archive
	archiveRestorePretty = false
	require.NoError(t, runArchiveRestore(nil, []string{"aaaa"}))

	tasks, _ = store.LoadAll()
	assert.Len(t, tasks, 1)
	archived, _ = store.LoadArchive()
	assert.Len(t, archived, 0)

	// Step 4: Prune again
	require.NoError(t, runPrune(nil, nil))

	tasks, _ = store.LoadAll()
	assert.Len(t, tasks, 0)
	archived, _ = store.LoadArchive()
	assert.Len(t, archived, 1)

	// Step 5: Purge the archive
	archivePurgePretty = false
	require.NoError(t, runArchivePurge(nil, nil))

	archived, _ = store.LoadArchive()
	assert.Len(t, archived, 0)
}

// --- global flag integration test ---

func TestPruneGlobal_ArchivesToGlobalPath(t *testing.T) {
	// Create a temp dir to act as the global LIMBO_ROOT
	globalRoot, err := os.MkdirTemp("", "limbo-global-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(globalRoot)

	// Initialize a global .limbo store
	globalStore := storage.NewStorageAt(globalRoot)
	require.NoError(t, globalStore.Init())

	// Add a done task to the global store
	now := time.Now()
	doneTask := &models.Task{
		ID:      "gggg",
		Name:    "Global Done Task",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, globalStore.SaveTask(doneTask))

	// Set LIMBO_ROOT to point to our temp global dir
	origLimboRoot := os.Getenv("LIMBO_ROOT")
	os.Setenv("LIMBO_ROOT", globalRoot)
	defer os.Setenv("LIMBO_ROOT", origLimboRoot)

	// Run prune (getStorage will use LIMBO_ROOT)
	prunePretty = false
	err = runPrune(nil, nil)
	require.NoError(t, err)

	// Verify archive.json exists at the global path
	archivePath := filepath.Join(globalRoot, storage.LimboDir, storage.ArchiveFile)
	_, err = os.Stat(archivePath)
	require.NoError(t, err, "archive.json should exist at global path %s", archivePath)

	// Verify the archived task is in the global archive
	archived, err := globalStore.LoadArchive()
	require.NoError(t, err)
	assert.Len(t, archived, 1)
	assert.Equal(t, "gggg", archived[0].ID)
	assert.Equal(t, "Global Done Task", archived[0].Name)

	// Verify the active store no longer has the task
	tasks, err := globalStore.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 0)
}
