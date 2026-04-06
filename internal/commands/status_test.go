package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetStatusFlags() {
	statusPretty = false
	statusOutcome = ""
	statusReason = ""
	statusBy = ""
}

func TestStatus_ForwardTransitions_HappyPath(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Simple Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	stages := []string{
		models.StatusRefined,
		models.StatusPlanned,
		models.StatusReady,
		models.StatusInProgress,
		models.StatusInReview,
	}

	statusBy = "tester"

	for _, stage := range stages {
		err = runStatus(nil, []string{"aaaa", stage})
		require.NoError(t, err, "failed to transition to %s", stage)

		updated, err := store.LoadTask("aaaa")
		require.NoError(t, err)
		assert.Equal(t, stage, updated.Status)
	}

	// Final transition to done -- no --outcome required
	err = runStatus(nil, []string{"aaaa", models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, updated.Status)

	// Verify history accumulated for all transitions
	assert.Len(t, updated.History, 6)
	assert.Equal(t, models.StatusCaptured, updated.History[0].From)
	assert.Equal(t, models.StatusRefined, updated.History[0].To)
	assert.Equal(t, "tester", updated.History[0].By)
	assert.Equal(t, models.StatusInReview, updated.History[5].From)
	assert.Equal(t, models.StatusDone, updated.History[5].To)
}

func TestStatus_ForwardWithOutcome(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Task with outcome",
		Status:  models.StatusInReview,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	statusOutcome = "all good"
	err = runStatus(nil, []string{"aaaa", models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, updated.Status)
	assert.Equal(t, "all good", updated.Outcome)
}

func TestStatus_BackwardWithoutReason_Succeeds(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Refined Task",
		Status:  models.StatusRefined,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Backward transition should succeed without --reason
	err = runStatus(nil, []string{"aaaa", models.StatusCaptured})
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusCaptured, updated.Status)
}

func TestStatus_BackwardWithReason_RecordsIt(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Refined Task",
		Status:  models.StatusRefined,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	statusReason = "requirements changed"
	statusBy = "pm"
	err = runStatus(nil, []string{"aaaa", models.StatusCaptured})
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusCaptured, updated.Status)
	require.Len(t, updated.History, 1)
	assert.Equal(t, models.StatusRefined, updated.History[0].From)
	assert.Equal(t, models.StatusCaptured, updated.History[0].To)
	assert.Equal(t, "requirements changed", updated.History[0].Reason)
	assert.Equal(t, "pm", updated.History[0].By)
}

func TestStatus_ManuallyBlockedTask_CannotTransition(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:                "aaaa",
		Name:              "Blocked Task",
		Status:            models.StatusCaptured,
		ManualBlockReason: "waiting on external review",
		Created:           now,
		Updated:           now,
	}
	require.NoError(t, store.SaveTask(task))

	// Forward transition should fail
	err = runStatus(nil, []string{"aaaa", models.StatusRefined})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manually blocked")

	// Backward with reason should also fail
	statusReason = "test"
	err = runStatus(nil, []string{"aaaa", models.StatusCaptured})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manually blocked")
}

func TestStatus_HistoryRecorded(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

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

	statusBy = "agent-1"

	// First transition
	err = runStatus(nil, []string{"aaaa", models.StatusRefined})
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	require.Len(t, updated.History, 1)
	assert.Equal(t, models.StatusCaptured, updated.History[0].From)
	assert.Equal(t, models.StatusRefined, updated.History[0].To)
	assert.Equal(t, "agent-1", updated.History[0].By)
	assert.False(t, updated.History[0].At.IsZero())
	assert.Empty(t, updated.History[0].Reason)

	// Second transition
	err = runStatus(nil, []string{"aaaa", models.StatusPlanned})
	require.NoError(t, err)

	updated, err = store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Len(t, updated.History, 2)
	assert.Equal(t, models.StatusRefined, updated.History[1].From)
	assert.Equal(t, models.StatusPlanned, updated.History[1].To)
}

func TestStatusCommand_InvalidStatus(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

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

	err = runStatus(nil, []string{task.ID, "invalid-status"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestStatusCommand_TaskNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	err := runStatus(nil, []string{"zzzz", models.StatusDone})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStatusCommand_InvalidID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	err := runStatus(nil, []string{"not-valid", models.StatusDone})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid task ID")
}

func TestStatusCommand_PrettyOutput(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

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

	statusPretty = true

	err = runStatus(nil, []string{task.ID, models.StatusRefined})
	require.NoError(t, err)
}

func TestStatusCommand_SameStatusNoOp(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

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

	// Same status should succeed
	err = runStatus(nil, []string{"aaaa", models.StatusCaptured})
	require.NoError(t, err)
}

func TestStatus_AutoRemovesBlockedBy(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker",
		Status:  models.StatusInReview,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Blocked",
		Status:    models.StatusCaptured,
		BlockedBy: []string{"aaaa"},
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked))

	err = runStatus(nil, []string{"aaaa", models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask("aaab")
	require.NoError(t, err)
	assert.Empty(t, updated.BlockedBy)
}

func TestStatus_DoneWithoutOutcome_Succeeds(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:       "aaaa",
		Name:     "Structured Task",
		Status:   models.StatusInReview,
		Approach: "do X",
		Verify:   "check Y",
		Result:   "report Z",
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task))

	// Done without --outcome should succeed (no gate enforcement)
	err = runStatus(nil, []string{task.ID, models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, updated.Status)
}

func TestStatus_MultiStageJump_Succeeds(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Jump Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Jump from captured directly to in-progress -- no gates
	err = runStatus(nil, []string{"aaaa", models.StatusInProgress})
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusInProgress, updated.Status)
}
