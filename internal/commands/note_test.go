package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoteCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	notePretty = false
	err = runNote(nil, []string{task.ID, "Started investigation"})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	require.Len(t, updated.Notes, 1)
	assert.Equal(t, "Started investigation", updated.Notes[0].Content)
	assert.True(t, updated.Updated.After(now))
}

func TestNoteCommand_MultipleNotes(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	notePretty = false

	err = runNote(nil, []string{task.ID, "First note"})
	require.NoError(t, err)

	time.Sleep(2 * time.Millisecond)
	err = runNote(nil, []string{task.ID, "Second note"})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	require.Len(t, updated.Notes, 2)
	assert.Equal(t, "First note", updated.Notes[0].Content)
	assert.Equal(t, "Second note", updated.Notes[1].Content)
	// Second note should be later
	assert.True(t, updated.Notes[1].Timestamp.After(updated.Notes[0].Timestamp) ||
		updated.Notes[1].Timestamp.Equal(updated.Notes[0].Timestamp))
}

func TestNoteCommand_TaskNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	notePretty = false
	err := runNote(nil, []string{"zzzz", "Note message"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestNoteCommand_InvalidID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	notePretty = false
	err := runNote(nil, []string{"not-valid", "Note message"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid task ID")
}

func TestNoteCommand_EmptyMessage(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	notePretty = false
	err = runNote(nil, []string{task.ID, ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestNoteCommand_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusTodo,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	notePretty = true
	err = runNote(nil, []string{task.ID, "Test note"})
	require.NoError(t, err)
}
