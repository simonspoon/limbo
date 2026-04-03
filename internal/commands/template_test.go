package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateListCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	templatePretty = false
	err := runTemplateList(nil, nil)
	require.NoError(t, err)
}

func TestTemplateListCommand_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	templatePretty = true
	err := runTemplateList(nil, nil)
	require.NoError(t, err)
}

func TestTemplateShowCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	templatePretty = false
	err := runTemplateShow(nil, []string{"bug-fix"})
	require.NoError(t, err)
}

func TestTemplateShowCommand_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	templatePretty = true
	err := runTemplateShow(nil, []string{"bug-fix"})
	require.NoError(t, err)
}

func TestTemplateShowCommand_NotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	templatePretty = false
	err := runTemplateShow(nil, []string{"nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTemplateApplyCommand(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	templatePretty = false
	templateParent = ""
	err := runTemplateApply(nil, []string{"feature"})
	require.NoError(t, err)

	// Verify tasks were created
	store, err := storage.NewStorage()
	require.NoError(t, err)
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 4) // feature: Design, Implement, Test, Review
}

func TestTemplateApplyCommand_Pretty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	templatePretty = true
	templateParent = ""
	err := runTemplateApply(nil, []string{"bug-fix"})
	require.NoError(t, err)
}

func TestTemplateApplyCommand_WithParent(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a parent task directly
	store := storage.NewStorageAt(tmpDir)
	addDescription = ""
	addParent = ""
	addPretty = false
	addApproach = "do something"
	addVerify = "check something"
	addResult = "report something"

	err := runAdd(nil, []string{"Parent Task"})
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	parentID := tasks[0].ID

	// Apply template under parent
	templatePretty = false
	templateParent = parentID
	err = runTemplateApply(nil, []string{"feature"})
	require.NoError(t, err)

	// Verify: 1 parent + 4 template tasks = 5
	tasks, err = store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 5)
}

func TestTemplateApplyCommand_NotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	templatePretty = false
	templateParent = ""
	err := runTemplateApply(nil, []string{"nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTemplateApplyCommand_InvalidParent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	templatePretty = false
	templateParent = "invalid-too-long"
	err := runTemplateApply(nil, []string{"feature"})
	assert.Error(t, err)
}

func TestTemplateApplyCommand_WithProjectTemplate(t *testing.T) {
	tmpDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a project-local template
	tmplDir := filepath.Join(tmpDir, ".limbo", "templates")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	yaml := []byte(`name: custom
description: Custom template
tasks:
  - name: "Step A"
    action: "Do A"
    verify: "Check A"
    result: "A done"
  - name: "Step B"
    action: "Do B"
    verify: "Check B"
    result: "B done"
    blocked_by: ["Step A"]
`)
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "custom.yaml"), yaml, 0644))

	templatePretty = false
	templateParent = ""
	err := runTemplateApply(nil, []string{"custom"})
	require.NoError(t, err)

	store, err := storage.NewStorage()
	require.NoError(t, err)
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
}
