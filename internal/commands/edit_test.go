package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
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
	editAction = ""
	editVerify = ""
	editResult = ""
	editAcceptanceCriteria = ""
	editScopeOut = ""
	editAffectedAreas = ""
	editTestStrategy = ""
	editRisks = ""
	editReport = ""
	editPretty = false
	editForce = false

	// Create a fresh command with clean flag state
	cmd := &cobra.Command{
		Use:  "edit <id>",
		Args: cobra.ExactArgs(1),
		RunE: runEdit,
	}
	cmd.Flags().StringVar(&editName, "name", "", "Task name")
	cmd.Flags().StringVarP(&editDescription, "description", "d", "", "Task description")
	cmd.Flags().StringVar(&editApproach, "approach", "", "What concrete work to perform")
	cmd.Flags().StringVar(&editAction, "action", "", "What concrete work to perform (alias for --approach)")
	cmd.Flags().StringVar(&editVerify, "verify", "", "How to confirm the action succeeded")
	cmd.Flags().StringVar(&editResult, "result", "", "Template for what to report back")
	cmd.Flags().StringVar(&editAcceptanceCriteria, "acceptance-criteria", "", "Criteria for acceptance")
	cmd.Flags().StringVar(&editScopeOut, "scope-out", "", "What is explicitly out of scope")
	cmd.Flags().StringVar(&editAffectedAreas, "affected-areas", "", "Areas of the codebase affected")
	cmd.Flags().StringVar(&editTestStrategy, "test-strategy", "", "How to test the changes")
	cmd.Flags().StringVar(&editRisks, "risks", "", "Known risks and mitigations")
	cmd.Flags().StringVar(&editReport, "report", "", "Completion report")
	cmd.Flags().BoolVar(&editPretty, "pretty", false, "Pretty print output")
	cmd.Flags().BoolVar(&editForce, "force", false, "Overwrite write-once fields that are already set")
	_ = cmd.Flags().MarkHidden("action")
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestEditCommand_Name(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := testStore(t)
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

	store, err := testStore(t)
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

	store, err := testStore(t)
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

	err = executeEdit("aaaa", "--action", "New action", "--force")
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

	store, err := testStore(t)
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

	err = executeEdit("aaaa", "--name", "Updated Task", "--action", "New action", "--verify", "New verify", "--force")
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

	store, err := testStore(t)
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

	store, err := testStore(t)
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

	store, err := testStore(t)
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

	store, err := testStore(t)
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

	store, err := testStore(t)
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

func TestEditCommand_ApproachFlag(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := testStore(t)
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:       "aaaa",
		Name:     "Test Task",
		Approach: "Old approach",
		Status:   models.StatusCaptured,
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa", "--approach", "New approach", "--force")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "New approach", updated.Approach)
}

func TestEditCommand_ActionAliasForApproach(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := testStore(t)
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:       "aaaa",
		Name:     "Test Task",
		Approach: "Old approach",
		Status:   models.StatusCaptured,
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa", "--action", "Via action alias", "--force")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "Via action alias", updated.Approach)
}

func TestEditCommand_NewMetadataFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := testStore(t)
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

	err = executeEdit("aaaa",
		"--acceptance-criteria", "all tests pass",
		"--scope-out", "not edge case X",
		"--affected-areas", "internal/commands",
		"--test-strategy", "unit + integration",
		"--risks", "backward compat risk",
		"--report", "summary of changes",
	)
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "all tests pass", updated.AcceptanceCriteria)
	assert.Equal(t, "not edge case X", updated.ScopeOut)
	assert.Equal(t, "internal/commands", updated.AffectedAreas)
	assert.Equal(t, "unit + integration", updated.TestStrategy)
	assert.Equal(t, "backward compat risk", updated.Risks)
	assert.Equal(t, "summary of changes", updated.Report)
}

func TestEditCommand_WriteOnceBlocksOverwrite(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := testStore(t)
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:                 "aaaa",
		Name:               "Test Task",
		Approach:           "Original approach",
		AcceptanceCriteria: "Original criteria",
		TestStrategy:       "Original strategy",
		Risks:              "Original risks",
		Report:             "Original report",
		Status:             models.StatusCaptured,
		Created:            now,
		Updated:            now,
	}
	require.NoError(t, store.SaveTask(task))

	cases := []struct {
		flag, value string
	}{
		{"--approach", "New approach"},
		{"--action", "New action"},
		{"--acceptance-criteria", "New criteria"},
		{"--test-strategy", "New strategy"},
		{"--risks", "New risks"},
		{"--report", "New report"},
	}
	for _, c := range cases {
		err := executeEdit("aaaa", c.flag, c.value)
		assert.Error(t, err, "%s should be blocked when already set", c.flag)
		assert.Contains(t, err.Error(), "write-once")
	}

	// Store must be untouched by the rejected edits.
	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "Original approach", updated.Approach)
	assert.Equal(t, "Original criteria", updated.AcceptanceCriteria)
	assert.Equal(t, "Original strategy", updated.TestStrategy)
	assert.Equal(t, "Original risks", updated.Risks)
	assert.Equal(t, "Original report", updated.Report)
}

func TestEditCommand_WriteOnceAllowsFirstSet(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := testStore(t)
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

	// First write of an empty write-once field needs no --force.
	err = executeEdit("aaaa", "--approach", "First approach")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "First approach", updated.Approach)
}

func TestEditCommand_WriteOnceForceOverwrites(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := testStore(t)
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:       "aaaa",
		Name:     "Test Task",
		Approach: "Original approach",
		Status:   models.StatusCaptured,
		Created:  now,
		Updated:  now,
	}
	require.NoError(t, store.SaveTask(task))

	err = executeEdit("aaaa", "--approach", "Forced approach", "--force")
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "Forced approach", updated.Approach)
}

func TestEditCommand_NonProtectedFieldsOverwriteFreely(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := testStore(t)
	require.NoError(t, err)

	now := time.Now()
	task := &models.Task{
		ID:            "aaaa",
		Name:          "Old Name",
		Description:   "Old desc",
		Verify:        "Old verify",
		Result:        "Old result",
		ScopeOut:      "Old scope",
		AffectedAreas: "Old areas",
		Status:        models.StatusCaptured,
		Created:       now,
		Updated:       now,
	}
	require.NoError(t, store.SaveTask(task))

	// None of these are write-once; overwriting must succeed without --force.
	err = executeEdit("aaaa",
		"--name", "New Name",
		"--verify", "New verify",
		"--result", "New result",
		"--scope-out", "New scope",
		"--affected-areas", "New areas",
	)
	require.NoError(t, err)

	updated, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, "New verify", updated.Verify)
	assert.Equal(t, "New result", updated.Result)
	assert.Equal(t, "New scope", updated.ScopeOut)
	assert.Equal(t, "New areas", updated.AffectedAreas)
}
