package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaimCommand(t *testing.T) {
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

	claimPretty = false
	claimForce = false
	err = runClaim(nil, []string{task.ID, "agent-1"})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.Owner)
	assert.Equal(t, "agent-1", *updated.Owner)
}

func TestClaimCommand_AlreadyOwned(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	owner := "agent-1"
	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusCaptured,
		Owner:   &owner,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	claimPretty = false
	claimForce = false
	err = runClaim(nil, []string{task.ID, "agent-2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already owned")
}

func TestClaimCommand_ForceOverride(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	owner := "agent-1"
	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusCaptured,
		Owner:   &owner,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	claimPretty = false
	claimForce = true
	err = runClaim(nil, []string{task.ID, "agent-2"})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.Owner)
	assert.Equal(t, "agent-2", *updated.Owner)
}

func TestClaimCommand_SameOwner(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	owner := "agent-1"
	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusCaptured,
		Owner:   &owner,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	claimPretty = false
	claimForce = false
	// Same owner can re-claim
	err = runClaim(nil, []string{task.ID, "agent-1"})
	require.NoError(t, err)
}

func TestClaimCommand_TaskNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	claimPretty = false
	claimForce = false
	err := runClaim(nil, []string{"zzzz", "agent-1"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUnclaimCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	owner := "agent-1"
	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusCaptured,
		Owner:   &owner,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	unclaimPretty = false
	err = runUnclaim(nil, []string{task.ID})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.Owner)
}

func TestUnclaimCommand_NoOwner(t *testing.T) {
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

	unclaimPretty = false
	err = runUnclaim(nil, []string{task.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no owner")
}

func TestListCommand_OwnerFilter(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	owner1 := "agent-1"
	owner2 := "agent-2"
	now := time.Now()

	task1 := &models.Task{
		ID:      "aaaa",
		Name:    "Task 1",
		Status:  models.StatusCaptured,
		Owner:   &owner1,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task1))

	task2 := &models.Task{
		ID:      "aaab",
		Name:    "Task 2",
		Status:  models.StatusCaptured,
		Owner:   &owner2,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task2))

	task3 := &models.Task{
		ID:      "aaac",
		Name:    "Task 3",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task3))

	// Test --owner filter
	listPretty = false
	listStatus = ""
	listOwner = "agent-1"
	listUnclaimed = false
	listBlocked = false
	listUnblocked = false

	err = runList(nil, nil)
	require.NoError(t, err)
}

func TestListCommand_UnclaimedFilter(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	owner := "agent-1"
	now := time.Now()

	task1 := &models.Task{
		ID:      "aaaa",
		Name:    "Owned Task",
		Status:  models.StatusCaptured,
		Owner:   &owner,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task1))

	task2 := &models.Task{
		ID:      "aaab",
		Name:    "Unclaimed Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task2))

	listPretty = false
	listStatus = ""
	listOwner = ""
	listUnclaimed = true
	listBlocked = false
	listUnblocked = false

	err = runList(nil, nil)
	require.NoError(t, err)
}
