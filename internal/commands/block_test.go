package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	blocked := &models.Task{
		ID:      "aaab",
		Name:    "Blocked Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocked))

	blockPretty = false
	err = runBlock(nil, []string{blocker.ID, blocked.ID})
	require.NoError(t, err)

	// Verify blocked task has blocker in BlockedBy
	updated, err := store.LoadTask(blocked.ID)
	require.NoError(t, err)
	assert.Contains(t, updated.BlockedBy, blocker.ID)
}

func TestBlockCommand_SelfBlock(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	blockPretty = false
	err = runBlock(nil, []string{task.ID, task.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot block itself")
}

func TestBlockCommand_CycleDetection(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	taskA := &models.Task{
		ID:      "aaaa",
		Name:    "Task A",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(taskA))

	taskB := &models.Task{
		ID:      "aaab",
		Name:    "Task B",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(taskB))

	blockPretty = false

	// A blocks B (B is blocked by A)
	err = runBlock(nil, []string{taskA.ID, taskB.ID})
	require.NoError(t, err)

	// B blocks A should fail (would create cycle)
	err = runBlock(nil, []string{taskB.ID, taskA.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestBlockCommand_CannotBlockOnDone(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	doneTask := &models.Task{
		ID:      "aaaa",
		Name:    "Done Task",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(doneTask))

	todoTask := &models.Task{
		ID:      "aaab",
		Name:    "Todo Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(todoTask))

	blockPretty = false
	err = runBlock(nil, []string{doneTask.ID, todoTask.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "completed task")
}

func TestBlockCommand_AlreadyBlocked(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Blocked",
		Status:    models.StatusCaptured,
		BlockedBy: []string{blocker.ID},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked))

	blockPretty = false
	err = runBlock(nil, []string{blocker.ID, blocked.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already blocked")
}

func TestUnblockCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Blocked",
		Status:    models.StatusCaptured,
		BlockedBy: []string{blocker.ID},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked))

	unblockPretty = false
	err = runUnblock(nil, []string{blocker.ID, blocked.ID})
	require.NoError(t, err)

	updated, err := store.LoadTask(blocked.ID)
	require.NoError(t, err)
	assert.NotContains(t, updated.BlockedBy, blocker.ID)
}

func TestUnblockCommand_NotBlocked(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task1 := &models.Task{
		ID:      "aaaa",
		Name:    "Task 1",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task1))

	task2 := &models.Task{
		ID:      "aaab",
		Name:    "Task 2",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task2))

	unblockPretty = false
	err = runUnblock(nil, []string{task1.ID, task2.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not blocked")
}

func TestNextCommand_SkipsBlockedTasks(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Blocked Task",
		Status:    models.StatusCaptured,
		BlockedBy: []string{blocker.ID},
		Created:   now.Add(time.Millisecond),
		Updated:   now.Add(time.Millisecond),
	}
	require.NoError(t, store.SaveTask(blocked))

	// next should return blocker, not blocked
	result, err := store.GetNextTask()
	require.NoError(t, err)
	require.NotEmpty(t, result.Candidates)
	assert.Equal(t, "Blocker Task", result.Candidates[0].Name)
	assert.Len(t, result.Candidates, 1) // blocked task should be filtered out
}

func TestManualBlock_Success(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Test Task",
		Status:  models.StatusInProgress,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	blockPretty = false
	blockReason = "waiting for external review"
	blockBy = "alice"

	err = runBlock(nil, []string{task.ID})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "waiting for external review", updated.ManualBlockReason)
	assert.Equal(t, models.StatusInProgress, updated.BlockedFromStage)
	require.NotEmpty(t, updated.History)
	last := updated.History[len(updated.History)-1]
	assert.Equal(t, models.StatusInProgress, last.From)
	assert.Equal(t, "blocked", last.To)
	assert.Equal(t, "alice", last.By)
	assert.Equal(t, "waiting for external review", last.Reason)
}

func TestManualBlock_MissingReason(t *testing.T) {
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

	blockPretty = false
	blockReason = ""
	blockBy = ""

	err = runBlock(nil, []string{task.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--reason is required")
}

func TestManualBlock_AlreadyBlocked(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:                "aaaa",
		Name:              "Test Task",
		Status:            models.StatusInProgress,
		ManualBlockReason: "existing block",
		BlockedFromStage:  models.StatusInProgress,
		Created:           now,
		Updated:           now,
	}
	require.NoError(t, store.SaveTask(task))

	blockPretty = false
	blockReason = "new reason"
	blockBy = ""

	err = runBlock(nil, []string{task.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already manually blocked")
}

func TestManualUnblock_Success(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:                "aaaa",
		Name:              "Test Task",
		Status:            models.StatusInProgress,
		ManualBlockReason: "waiting on vendor",
		BlockedFromStage:  models.StatusInProgress,
		Created:           now,
		Updated:           now,
	}
	require.NoError(t, store.SaveTask(task))

	unblockPretty = false
	unblockBy = "bob"

	err = runUnblock(nil, []string{task.ID})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusInProgress, updated.Status)
	assert.Empty(t, updated.ManualBlockReason)
	assert.Empty(t, updated.BlockedFromStage)
	require.NotEmpty(t, updated.History)
	last := updated.History[len(updated.History)-1]
	assert.Equal(t, "blocked", last.From)
	assert.Equal(t, models.StatusInProgress, last.To)
	assert.Equal(t, "bob", last.By)
}

func TestManualUnblock_NotManuallyBlocked(t *testing.T) {
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

	unblockPretty = false
	unblockBy = ""

	err = runUnblock(nil, []string{task.ID})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not manually blocked")
}

func TestManualBlock_StatusTransitionBlocked(t *testing.T) {
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

	// Manually block the task
	blockPretty = false
	blockReason = "needs architect approval"
	blockBy = ""

	err = runBlock(nil, []string{task.ID})
	require.NoError(t, err)

	// Attempt status transition — should fail because task is manually blocked
	statusPretty = false
	statusOutcome = ""
	statusReason = ""
	statusBy = ""

	err = runStatus(nil, []string{task.ID, models.StatusRefined})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manually blocked")
}

func TestStatusCommand_AutoRemovesBlockedBy(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker",
		Status:  models.StatusInReview,
		Report:  "done",
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Blocked",
		Status:    models.StatusCaptured,
		BlockedBy: []string{blocker.ID},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked))

	// Mark blocker as done
	statusPretty = false
	statusOutcome = "completed"
	err = runStatus(nil, []string{blocker.ID, models.StatusDone})
	require.NoError(t, err)

	// Blocked task should no longer be blocked
	updated, err := store.LoadTask(blocked.ID)
	require.NoError(t, err)
	assert.Empty(t, updated.BlockedBy)
}
