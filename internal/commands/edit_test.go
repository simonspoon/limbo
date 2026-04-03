package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeEdit creates a fresh edit cobra command and executes it with the given args.
// This ensures flag state is isolated between test runs.
func executeEdit(args ...string) error {
	// Reset package-level vars
	editName = ""
	editDescription = ""
	editApproach = ""
	editVerify = ""
	editResult = ""
	editPretty = false

	// Create a fresh command with clean flag state
	cmd := &cobra.Command{
		Use:  "edit <id>",
		Args: cobra.ExactArgs(1),
		RunE: runEdit,
	}
	cmd.Flags().StringVar(&editName, "name", "", "Task name")
	cmd.Flags().StringVarP(&editDescription, "description", "d", "", "Task description")
	cmd.Flags().StringVar(&editApproach, "action", "", "What concrete work to perform")
	cmd.Flags().StringVar(&editVerify, "verify", "", "How to confirm the action succeeded")
	cmd.Flags().StringVar(&editResult, "result", "", "Template for what to report back")
	cmd.Flags().BoolVar(&editPretty, "pretty", false, "Pretty print output")
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestEditCommand_Name(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Original Name",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa", "--name", "New Name")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.True(t, updated.Updated.After(now))
}

func TestEditCommand_Description(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:          "aaaa",
		Name:        "Test Task",
		Description: "Old description",
		Status:      models.StatusCaptured,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa", "-d", "New description")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "New description", updated.Description)
}

func TestEditCommand_Action(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:       "aaaa",
		Name:     "Test Task",
		Approach: "Old action",
		Verify:   "Old verify",
		Result:   "Old result",
		Status:   models.StatusCaptured,
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa", "--action", "New action")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "New action", updated.Approach)
	// Unchanged fields should remain
	assert.Equal(t, "Old verify", updated.Verify)
	assert.Equal(t, "Old result", updated.Result)
	assert.Equal(t, "Test Task", updated.Name)
}

func TestEditCommand_MultipleFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:       "aaaa",
		Name:     "Test Task",
		Approach: "Old action",
		Verify:   "Old verify",
		Result:   "Old result",
		Status:   models.StatusCaptured,
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa", "--name", "Updated Task", "--action", "New action", "--verify", "New verify")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "Updated Task", updated.Name)
	assert.Equal(t, "New action", updated.Approach)
	assert.Equal(t, "New verify", updated.Verify)
	// Unchanged fields
	assert.Equal(t, "Old result", updated.Result)
}

func TestEditCommand_NoFlags(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nothing to edit")
}

func TestEditCommand_TaskNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	err := executeEdit("zzzz", "--name", "New Name")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEditCommand_InvalidID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	err := executeEdit("bad!", "--name", "New Name")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid task ID")
}

func TestEditCommand_CaseInsensitiveID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("AAAA", "--name", "Updated Name")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
}

func TestEditCommand_PrettyOutput(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa", "--name", "Updated", "--pretty")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
}

func TestEditCommand_ClearDescription(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:          "aaaa",
		Name:        "Test Task",
		Description: "Has a description",
		Status:      models.StatusCaptured,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))

	// Setting description to empty string should clear it
	err = executeEdit("aaaa", "-d", "")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "", updated.Description)
}

func TestEditCommand_PreservesImmutableFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	owner := "agent-1"
	parentID := "bbbb"
	task := &models.Task{
		ID:        "aaaa",
		Name:      "Test Task",
		Status:    models.StatusInProgress,
		Owner:     &owner,
		Parent:    &parentID,
		BlockedBy: []string{"cccc"},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa", "--name", "New Name")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	// Immutable fields preserved
	assert.Equal(t, models.StatusInProgress, updated.Status)
	require.NotNil(t, updated.Owner)
	assert.Equal(t, "agent-1", *updated.Owner)
	require.NotNil(t, updated.Parent)
	assert.Equal(t, "bbbb", *updated.Parent)
	assert.Equal(t, []string{"cccc"}, updated.BlockedBy)
	assert.Equal(t, now.Unix(), updated.Created.Unix())
}
