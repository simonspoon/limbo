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

// fullyPopulatedTask returns a task with all gate-required fields set.
func fullyPopulatedTask(id string) *models.Task {
	owner := "agent-1"
	now := time.Now()
	return &models.Task{
		ID:                 id,
		Name:               "Full Task",
		Status:             models.StatusCaptured,
		AcceptanceCriteria: "criteria here",
		ScopeOut:           "scope out here",
		Approach:           "approach here",
		AffectedAreas:      "areas here",
		TestStrategy:       "test strategy here",
		Risks:              "risks here",
		Verify:             "verify here",
		Result:             "result here",
		Report:             "report here",
		Owner:              &owner,
		Created:            now,
		Updated:            now,
	}
}

func TestStatus_ForwardTransitions_HappyPath(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	task := fullyPopulatedTask("aaaa")
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

	// Final transition to done requires --outcome
	statusOutcome = "all good"
	err = runStatus(nil, []string{"aaaa", models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, updated.Status)
	assert.Equal(t, "all good", updated.Outcome)

	// Verify history accumulated for all transitions
	assert.Len(t, updated.History, 6)
	assert.Equal(t, models.StatusCaptured, updated.History[0].From)
	assert.Equal(t, models.StatusRefined, updated.History[0].To)
	assert.Equal(t, "tester", updated.History[0].By)
	assert.Equal(t, models.StatusInReview, updated.History[5].From)
	assert.Equal(t, models.StatusDone, updated.History[5].To)
}

func TestStatusGate_CapturedToRefined_MissingFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Bare Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	err = runStatus(nil, []string{"aaaa", models.StatusRefined})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "acceptance_criteria")
	assert.Contains(t, err.Error(), "scope_out")
}

func TestStatusGate_RefinedToPlanned_MissingFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:                 "aaaa",
		Name:               "Refined Task",
		Status:             models.StatusRefined,
		AcceptanceCriteria: "done",
		ScopeOut:           "done",
		Created:            now,
		Updated:            now,
	}
	require.NoError(t, store.SaveTask(task))

	err = runStatus(nil, []string{"aaaa", models.StatusPlanned})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approach")
	assert.Contains(t, err.Error(), "affected_areas")
	assert.Contains(t, err.Error(), "test_strategy")
	assert.Contains(t, err.Error(), "risks")
}

func TestStatusGate_PlannedToReady_MissingVerify(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:            "aaaa",
		Name:          "Planned Task",
		Status:        models.StatusPlanned,
		Approach:      "done",
		AffectedAreas: "done",
		TestStrategy:  "done",
		Risks:         "done",
		Created:       now,
		Updated:       now,
	}
	require.NoError(t, store.SaveTask(task))

	err = runStatus(nil, []string{"aaaa", models.StatusReady})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "verify")
}

func TestStatusGate_ReadyToInProgress_NotClaimed(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Ready Task",
		Status:  models.StatusReady,
		Verify:  "check it",
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	err = runStatus(nil, []string{"aaaa", models.StatusInProgress})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be claimed")
}

func TestStatusGate_InProgressToInReview_MissingReport(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	owner := "agent"
	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "InProgress Task",
		Status:  models.StatusInProgress,
		Owner:   &owner,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	err = runStatus(nil, []string{"aaaa", models.StatusInReview})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "report")
}

func TestStatusGate_InReviewToDone_MissingOutcome(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:      "aaaa",
		Name:    "InReview Task",
		Status:  models.StatusInReview,
		Report:  "done",
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// No --outcome flag set
	err = runStatus(nil, []string{"aaaa", models.StatusDone})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outcome")
}

func TestStatus_BackwardWithoutReason_Fails(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:                 "aaaa",
		Name:               "Refined Task",
		Status:             models.StatusRefined,
		AcceptanceCriteria: "done",
		ScopeOut:           "done",
		Created:            now,
		Updated:            now,
	}
	require.NoError(t, store.SaveTask(task))

	err = runStatus(nil, []string{"aaaa", models.StatusCaptured})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--reason is required")
}

func TestStatus_BackwardWithReason_Succeeds(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:                 "aaaa",
		Name:               "Refined Task",
		Status:             models.StatusRefined,
		AcceptanceCriteria: "done",
		ScopeOut:           "done",
		Created:            now,
		Updated:            now,
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

func TestStatus_MultiStageForward_ValidatesAllGates(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Task missing refined->planned gate fields (no Approach etc.)
	task := &models.Task{
		ID:                 "aaaa",
		Name:               "Jump Task",
		Status:             models.StatusCaptured,
		AcceptanceCriteria: "done",
		ScopeOut:           "done",
		Created:            now,
		Updated:            now,
	}
	require.NoError(t, store.SaveTask(task))

	// Jump captured -> planned should fail at the refined->planned gate
	err = runStatus(nil, []string{"aaaa", models.StatusPlanned})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refined")
	assert.Contains(t, err.Error(), "planned")
	assert.Contains(t, err.Error(), "approach")
}

func TestStatus_MultiStageForward_FailsFirstGate(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Task missing captured->refined gate fields
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Empty Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Jump captured -> planned should fail at the captured->refined gate first
	err = runStatus(nil, []string{"aaaa", models.StatusPlanned})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "captured")
	assert.Contains(t, err.Error(), "refined")
	assert.Contains(t, err.Error(), "acceptance_criteria")
}

func TestStatus_ManuallyBlockedTask_CannotTransition(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:                 "aaaa",
		Name:               "Blocked Task",
		Status:             models.StatusCaptured,
		ManualBlockReason:  "waiting on external review",
		AcceptanceCriteria: "done",
		ScopeOut:           "done",
		Created:            now,
		Updated:            now,
	}
	require.NoError(t, store.SaveTask(task))

	// Forward transition should fail
	err = runStatus(nil, []string{"aaaa", models.StatusRefined})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manually blocked")

	// Even same-status should work fine (no-op transition), but backward with reason should also fail
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

	task := fullyPopulatedTask("aaaa")
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

func TestStatusCommand_CannotMarkDoneWithUndoneChildren(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	parentID := "aaaa"

	// Parent in in-review with all fields to pass gates
	parent := &models.Task{
		ID:      parentID,
		Name:    "Parent Task",
		Status:  models.StatusInReview,
		Report:  "done",
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Child task not done
	child := &models.Task{
		ID:      "aaab",
		Name:    "Child Task",
		Status:  models.StatusCaptured,
		Parent:  &parentID,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(child))

	statusOutcome = "all done"
	err = runStatus(nil, []string{parent.ID, models.StatusDone})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "undone children")
}

func TestStatusCommand_CannotStartBlockedTask(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	owner := "agent"
	now := time.Now()

	// Create blocker task
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker Task",
		Status:  models.StatusCaptured,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	// Create blocked task at ready stage with all fields
	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Blocked Task",
		Status:    models.StatusReady,
		BlockedBy: []string{"aaaa"},
		Verify:    "check",
		Owner:     &owner,
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked))

	err = runStatus(nil, []string{blocked.ID, models.StatusInProgress})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked by")
	assert.Contains(t, err.Error(), "aaaa")
}

func TestStatusCommand_CanStartAfterUnblock(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	owner := "agent"
	now := time.Now()

	// Create blocker task at in-review with outcome fields
	blocker := &models.Task{
		ID:      "aaaa",
		Name:    "Blocker Task",
		Status:  models.StatusInReview,
		Report:  "done",
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(blocker))

	// Create blocked task at ready stage
	blocked := &models.Task{
		ID:        "aaab",
		Name:      "Blocked Task",
		Status:    models.StatusReady,
		BlockedBy: []string{"aaaa"},
		Verify:    "check",
		Owner:     &owner,
		Created:   now,
		Updated:   now,
	}
	require.NoError(t, store.SaveTask(blocked))

	// Mark blocker as done
	statusOutcome = "completed"
	err = runStatus(nil, []string{blocker.ID, models.StatusDone})
	require.NoError(t, err)

	// Now should be able to start the previously blocked task
	statusOutcome = ""
	err = runStatus(nil, []string{blocked.ID, models.StatusInProgress})
	require.NoError(t, err)

	updated, err := store.LoadTask(blocked.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusInProgress, updated.Status)
}

func TestStatusCommand_PrettyOutput(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	task := fullyPopulatedTask("aaaa")
	require.NoError(t, store.SaveTask(task))

	statusPretty = true

	err = runStatus(nil, []string{task.ID, models.StatusRefined})
	require.NoError(t, err)
}

func TestStatusCommand_RequiresOutcomeForStructuredTask(t *testing.T) {
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
		Report:   "work done",
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task))

	// Try to mark done without outcome - should fail (gate catches it)
	err = runStatus(nil, []string{task.ID, models.StatusDone})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outcome")
}

func TestStatusCommand_AcceptsOutcomeForStructuredTask(t *testing.T) {
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
		Report:   "work done",
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task))

	statusOutcome = "done, Y confirmed"

	err = runStatus(nil, []string{task.ID, models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, updated.Status)
	assert.Equal(t, "done, Y confirmed", updated.Outcome)
}

func TestStatusCommand_LegacyTaskDoneWithoutOutcome(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()
	resetStatusFlags()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()

	// Legacy task: at in-review, no structured fields, but has outcome via gate
	task := &models.Task{
		ID:      "aaaa",
		Name:    "Legacy Task",
		Status:  models.StatusInReview,
		Report:  "done",
		Outcome: "shipped",
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(task))

	// Legacy task with outcome already set can be marked done
	err = runStatus(nil, []string{task.ID, models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, updated.Status)
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

	// Same status should succeed (no gates to cross)
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
		Report:  "done",
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

	statusOutcome = "completed"
	err = runStatus(nil, []string{"aaaa", models.StatusDone})
	require.NoError(t, err)

	updated, err := store.LoadTask("aaab")
	require.NoError(t, err)
	assert.Empty(t, updated.BlockedBy)
}
