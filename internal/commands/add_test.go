package commands

import (
	"os"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/store/taskstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestEnv creates an isolated limbo project for a command test. It points
// LIMBO_HOME at a fresh temp dir (so the central storage root lands there),
// disables parent-directory climbing, chdir's into a project directory that
// holds a .limbo-id anchor, and runs `limbo init` to seed the store. The
// returned string is the project working directory; the cleanup restores cwd
// and removes the temp tree.
func setupTestEnv(t *testing.T) (string, func()) {
	t.Helper()

	homeDir, err := os.MkdirTemp("", "limbo-home-*")
	require.NoError(t, err)
	projDir, err := os.MkdirTemp("", "limbo-proj-*")
	require.NoError(t, err)

	// Anchor the project with a stable .limbo-id so resolution is git-free.
	require.NoError(t, os.WriteFile(projDir+"/.limbo-id", []byte("test-project\n"), 0o644))

	t.Setenv("LIMBO_HOME", homeDir)
	t.Setenv(noClimbEnv, "1")

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projDir))

	// Seed the central store via init.
	initPretty = false
	require.NoError(t, runInit(initCmd, nil))

	cleanup := func() {
		os.Chdir(origDir)
		os.RemoveAll(projDir)
		os.RemoveAll(homeDir)
	}

	return projDir, cleanup
}

// testStore resolves the central-path taskstore facade for the current test
// project. It replaces the legacy storage.NewStorage() call sites.
func testStore(t *testing.T) (*taskstore.Store, error) {
	t.Helper()
	return getStorage()
}

func resetAddFlags() {
	addName = ""
	addDescription = ""
	addParent = ""
	addPretty = false
	addApproach = ""
	addAction = ""
	addVerify = ""
	addResult = ""
	addAcceptanceCriteria = ""
	addScopeOut = ""
	addAffectedAreas = ""
	addTestStrategy = ""
	addRisks = ""
	addReport = ""
}

func TestAddCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Reset flags
	resetAddFlags()
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test basic add
	err := runAdd(nil, []string{"Test Task"})
	require.NoError(t, err)

	// Verify task was created
	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	task := tasks[0]
	assert.Equal(t, "Test Task", task.Name)
	assert.Equal(t, models.StatusCaptured, task.Status)
	assert.Empty(t, task.Description)
	assert.Nil(t, task.Parent)
}

func TestAddCommandWithDescription(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addDescription = "Test description"
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test add with description
	err := runAdd(nil, []string{"Task with description"})
	require.NoError(t, err)

	// Verify task
	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	task := tasks[0]
	assert.Equal(t, "Task with description", task.Name)
	assert.Equal(t, "Test description", task.Description)
}

func TestAddCommandWithParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create parent task
	resetAddFlags()
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	err := runAdd(nil, []string{"Parent Task"})
	require.NoError(t, err)

	// Get parent ID
	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	parentID := tasks[0].ID

	// Create child task (with slight delay to ensure unique ID)
	time.Sleep(2 * time.Millisecond)
	addParent = parentID
	err = runAdd(nil, []string{"Child Task"})
	require.NoError(t, err)

	// Verify child has parent
	tasks, err = store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	// Find child task
	var child *models.Task
	for _, task := range tasks {
		if task.Name == "Child Task" {
			child = &task
			break
		}
	}

	require.NotNil(t, child)
	require.NotNil(t, child.Parent)
	assert.Equal(t, parentID, *child.Parent)
}

func TestAddCommandNonExistentParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addParent = "zzzz"
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test add should fail
	err := runAdd(nil, []string{"Test Task"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parent task")
}

func TestAddCommandNotInProject(t *testing.T) {
	// Create temp directory without initializing
	tmpDir, err := os.MkdirTemp("", "limbo-cmd-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(origDir)

	require.NoError(t, os.Chdir(tmpDir))

	resetAddFlags()
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test add should fail
	err = runAdd(nil, []string{"Test Task"})
	assert.Error(t, err)
}

func TestAddCommandPrettyOutput(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addPretty = true
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	err := runAdd(nil, []string{"Test Task"})
	require.NoError(t, err)
}

func TestAddCommandToDoneParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := testStore(t)
	require.NoError(t, err)

	// Create a done parent task
	now := time.Now()
	parent := &models.Task{
		ID:      "aaaa",
		Name:    "Done Parent",
		Status:  models.StatusDone,
		Created: now,
		Updated: now,
	}
	require.NoError(t, store.SaveTask(parent))

	// Try to add child to done parent
	resetAddFlags()
	addParent = parent.ID
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	err = runAdd(nil, []string{"Child Task"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot add child to done task")
}

func TestAddCommandInvalidParentID(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addParent = "invalid"
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	// Test add should fail
	err := runAdd(nil, []string{"Test Task"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parent task ID")
}

func TestAddCommand_WithoutStructuredFlags(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()

	// Adding a task without structured flags should succeed
	err := runAdd(nil, []string{"Quick Task"})
	require.NoError(t, err)

	// Verify task was created with empty structured fields
	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	task := tasks[0]
	assert.Equal(t, "Quick Task", task.Name)
	assert.Equal(t, models.StatusCaptured, task.Status)
	assert.Empty(t, task.Approach)
	assert.Empty(t, task.Verify)
	assert.Empty(t, task.Result)
	assert.False(t, task.HasStructuredFields())
}

func TestAddCommand_ApproachFlag(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addApproach = "implement the feature"

	err := runAdd(nil, []string{"Approach Task"})
	require.NoError(t, err)

	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "implement the feature", tasks[0].Approach)
}

func TestAddCommand_ActionAliasForApproach(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addAction = "do the thing via action"

	err := runAdd(nil, []string{"Action Alias Task"})
	require.NoError(t, err)

	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "do the thing via action", tasks[0].Approach)
}

func TestAddCommand_ApproachWinsOverAction(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addApproach = "approach wins"
	addAction = "action loses"

	err := runAdd(nil, []string{"Approach Wins Task"})
	require.NoError(t, err)

	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "approach wins", tasks[0].Approach)
}

func TestAddCommand_NameFlag(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addName = "Flag-Only Task"

	// No positional arg; name comes from --name
	err := runAdd(nil, []string{})
	require.NoError(t, err)

	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "Flag-Only Task", tasks[0].Name)
}

func TestAddCommand_PositionalWinsOverNameFlag(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addName = "flag loses"

	// Both positional and --name given; positional wins
	err := runAdd(nil, []string{"positional wins"})
	require.NoError(t, err)

	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "positional wins", tasks[0].Name)
}

func TestAddCommand_NoNameError(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()

	// Neither positional nor --name; should error
	err := runAdd(nil, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task name required")
}

func TestAddCommand_NewMetadataFields(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetAddFlags()
	addAcceptanceCriteria = "must pass all tests"
	addScopeOut = "not handling edge case X"
	addAffectedAreas = "internal/commands"
	addTestStrategy = "unit tests + integration"
	addRisks = "might break backward compat"
	addReport = "changes summary"

	err := runAdd(nil, []string{"Full Metadata Task"})
	require.NoError(t, err)

	store, err := testStore(t)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	task := tasks[0]
	assert.Equal(t, "must pass all tests", task.AcceptanceCriteria)
	assert.Equal(t, "not handling edge case X", task.ScopeOut)
	assert.Equal(t, "internal/commands", task.AffectedAreas)
	assert.Equal(t, "unit tests + integration", task.TestStrategy)
	assert.Equal(t, "might break backward compat", task.Risks)
	assert.Equal(t, "changes summary", task.Report)
}
