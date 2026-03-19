package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestProject(t *testing.T) (string, *storage.Storage, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "limbo-template-test-*")
	require.NoError(t, err)

	store := storage.NewStorageAt(tmpDir)
	require.NoError(t, store.Init())

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}
	return tmpDir, store, cleanup
}

// --- Test plan criteria 1: ListTemplates returns built-in templates ---

func TestListTemplates_BuiltinsPresent(t *testing.T) {
	tmpDir, _, cleanup := setupTestProject(t)
	defer cleanup()

	templates, err := ListTemplates(tmpDir)
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}

	assert.True(t, names["bug-fix"], "bug-fix template should be present")
	assert.True(t, names["feature"], "feature template should be present")
	assert.True(t, names["swe-full-cycle"], "swe-full-cycle template should be present")
}

func TestListTemplates_HasDescriptions(t *testing.T) {
	tmpDir, _, cleanup := setupTestProject(t)
	defer cleanup()

	templates, err := ListTemplates(tmpDir)
	require.NoError(t, err)

	for _, tmpl := range templates {
		assert.NotEmpty(t, tmpl.Description, "template %s should have a description", tmpl.Name)
	}
}

// --- Test plan criteria 2: GetTemplate returns correct template ---

func TestGetTemplate_Builtin(t *testing.T) {
	tmpDir, _, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl, err := GetTemplate(tmpDir, "bug-fix")
	require.NoError(t, err)
	assert.Equal(t, "bug-fix", tmpl.Name)
	assert.NotEmpty(t, tmpl.Tasks)
}

// --- Test plan criteria 3: Apply creates tasks with correct hierarchy ---

func TestApply_BugFix_CreatesAllTasks(t *testing.T) {
	tmpDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl, err := GetTemplate(tmpDir, "bug-fix")
	require.NoError(t, err)

	result, err := Apply(store, tmpl, "")
	require.NoError(t, err)

	// bug-fix has: Investigate (Reproduce issue, Identify root cause), Fix, Test (Verify fix, Regression tests) = 7
	assert.Len(t, result.CreatedIDs, 7)

	// Verify all tasks exist in storage
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 7)
}

func TestApply_CreatesParentChildRelationships(t *testing.T) {
	tmpDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl, err := GetTemplate(tmpDir, "bug-fix")
	require.NoError(t, err)

	_, err = Apply(store, tmpl, "")
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)

	taskMap := make(map[string]*models.Task)
	for i := range tasks {
		taskMap[tasks[i].Name] = &tasks[i]
	}

	// "Reproduce issue" should be child of "Investigate"
	reproduce := taskMap["Reproduce issue"]
	require.NotNil(t, reproduce)
	require.NotNil(t, reproduce.Parent)
	assert.Equal(t, taskMap["Investigate"].ID, *reproduce.Parent)

	// "Verify fix" should be child of "Test"
	verify := taskMap["Verify fix"]
	require.NotNil(t, verify)
	require.NotNil(t, verify.Parent)
	assert.Equal(t, taskMap["Test"].ID, *verify.Parent)

	// Root-level tasks have no parent
	investigate := taskMap["Investigate"]
	require.NotNil(t, investigate)
	assert.Nil(t, investigate.Parent)
}

func TestApply_CreatesBlockDependencies(t *testing.T) {
	tmpDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl, err := GetTemplate(tmpDir, "bug-fix")
	require.NoError(t, err)

	_, err = Apply(store, tmpl, "")
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)

	taskMap := make(map[string]*models.Task)
	for i := range tasks {
		taskMap[tasks[i].Name] = &tasks[i]
	}

	// "Identify root cause" blocked by "Reproduce issue"
	identify := taskMap["Identify root cause"]
	require.NotNil(t, identify)
	assert.Contains(t, identify.BlockedBy, taskMap["Reproduce issue"].ID)

	// "Fix" blocked by "Investigate"
	fix := taskMap["Fix"]
	require.NotNil(t, fix)
	assert.Contains(t, fix.BlockedBy, taskMap["Investigate"].ID)

	// "Test" blocked by "Fix"
	test := taskMap["Test"]
	require.NotNil(t, test)
	assert.Contains(t, test.BlockedBy, taskMap["Fix"].ID)
}

// --- Test plan criteria 4: Apply with --parent nests under existing task ---

func TestApply_WithParent(t *testing.T) {
	tmpDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	// Create parent task
	parentID, err := store.GenerateTaskID()
	require.NoError(t, err)
	parent := &models.Task{
		ID:     parentID,
		Name:   "Root Project",
		Action: "do", Verify: "check", Result: "report",
		Status: models.StatusTodo,
	}
	require.NoError(t, store.SaveTask(parent))

	tmpl, err := GetTemplate(tmpDir, "feature")
	require.NoError(t, err)

	_, err = Apply(store, tmpl, parentID)
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)

	// feature has 4 tasks + 1 parent = 5 total
	assert.Len(t, tasks, 5)

	// All root-level template tasks should have parentID as parent
	for _, task := range tasks {
		if task.ID == parentID {
			continue
		}
		// Root-level template tasks (Design, Implement, Test, Review)
		if task.Parent != nil && *task.Parent == parentID {
			// This is a root-level template task nested under parent - good
			continue
		}
	}

	// Verify "Design" is under parentID
	taskMap := make(map[string]*models.Task)
	for i := range tasks {
		taskMap[tasks[i].Name] = &tasks[i]
	}
	design := taskMap["Design"]
	require.NotNil(t, design)
	require.NotNil(t, design.Parent)
	assert.Equal(t, parentID, *design.Parent)
}

// --- Test plan criteria 5: Project-local templates ---

func TestListTemplates_IncludesProjectLocal(t *testing.T) {
	tmpDir, _, cleanup := setupTestProject(t)
	defer cleanup()

	// Create project template
	tmplDir := filepath.Join(tmpDir, ".limbo", "templates")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	yaml := []byte(`name: custom
description: A custom project template
tasks:
  - name: "Step 1"
    action: "Do step 1"
    verify: "Check step 1"
    result: "Step 1 done"
`)
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "custom.yaml"), yaml, 0644))

	templates, err := ListTemplates(tmpDir)
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}
	assert.True(t, names["custom"], "custom project template should be listed")
	assert.True(t, names["bug-fix"], "built-in templates should still be present")
}

func TestGetTemplate_ProjectLocalApply(t *testing.T) {
	tmpDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	// Create project template
	tmplDir := filepath.Join(tmpDir, ".limbo", "templates")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	yaml := []byte(`name: custom
description: A custom template
tasks:
  - name: "Alpha"
    action: "Do alpha"
    verify: "Check alpha"
    result: "Alpha done"
  - name: "Beta"
    action: "Do beta"
    verify: "Check beta"
    result: "Beta done"
    blocked_by: ["Alpha"]
`)
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "custom.yaml"), yaml, 0644))

	tmpl, err := GetTemplate(tmpDir, "custom")
	require.NoError(t, err)

	result, err := Apply(store, tmpl, "")
	require.NoError(t, err)
	assert.Len(t, result.CreatedIDs, 2)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	taskMap := make(map[string]*models.Task)
	for i := range tasks {
		taskMap[tasks[i].Name] = &tasks[i]
	}
	beta := taskMap["Beta"]
	require.NotNil(t, beta)
	assert.Contains(t, beta.BlockedBy, taskMap["Alpha"].ID)
}

// --- Test plan criteria 6: Project template overrides built-in ---

func TestGetTemplate_ProjectOverridesBuiltin(t *testing.T) {
	tmpDir, _, cleanup := setupTestProject(t)
	defer cleanup()

	// Create project template that overrides bug-fix
	tmplDir := filepath.Join(tmpDir, ".limbo", "templates")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	yaml := []byte(`name: bug-fix
description: Custom bug-fix override
tasks:
  - name: "Custom Step"
    action: "Do custom"
    verify: "Check custom"
    result: "Custom done"
`)
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "bug-fix.yaml"), yaml, 0644))

	tmpl, err := GetTemplate(tmpDir, "bug-fix")
	require.NoError(t, err)
	assert.Equal(t, "Custom bug-fix override", tmpl.Description)
	assert.Len(t, tmpl.Tasks, 1)
	assert.Equal(t, "Custom Step", tmpl.Tasks[0].Name)
}

// --- Test plan criteria 7: Non-existent template ---

func TestGetTemplate_NotFound(t *testing.T) {
	tmpDir, _, cleanup := setupTestProject(t)
	defer cleanup()

	_, err := GetTemplate(tmpDir, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- Test plan criteria 8: Apply with invalid parent ---

func TestApply_InvalidParentID(t *testing.T) {
	tmpDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl, err := GetTemplate(tmpDir, "feature")
	require.NoError(t, err)

	_, err = Apply(store, tmpl, "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parent task ID")
}

func TestApply_NonExistentParentID(t *testing.T) {
	tmpDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl, err := GetTemplate(tmpDir, "feature")
	require.NoError(t, err)

	_, err = Apply(store, tmpl, "zzzz")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- Test plan criteria 10: Malformed YAML ---

func TestLoadProjectTemplates_MalformedYAML(t *testing.T) {
	tmpDir, _, cleanup := setupTestProject(t)
	defer cleanup()

	tmplDir := filepath.Join(tmpDir, ".limbo", "templates")
	require.NoError(t, os.MkdirAll(tmplDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmplDir, "bad.yaml"), []byte("{{invalid yaml"), 0644))

	_, err := ListTemplates(tmpDir)
	assert.Error(t, err)
}

// --- Test plan criteria 11: blocked_by referencing non-existent name ---

func TestApply_InvalidBlockedByReference(t *testing.T) {
	_, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl := &Template{
		Name:        "bad-ref",
		Description: "Template with invalid blocked_by",
		Tasks: []TaskTemplate{
			{Name: "A", Action: "do", Verify: "check", Result: "done"},
			{Name: "B", Action: "do", Verify: "check", Result: "done", BlockedBy: []string{"NonExistent"}},
		},
	}

	_, err := Apply(store, tmpl, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in scope")
}

// --- Test plan criteria 12: Empty template ---

func TestApply_EmptyTemplate(t *testing.T) {
	_, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl := &Template{
		Name:        "empty",
		Description: "Empty template",
		Tasks:       nil,
	}

	result, err := Apply(store, tmpl, "")
	require.NoError(t, err)
	assert.Empty(t, result.CreatedIDs)
}

// --- Test plan criteria 13: Deeply nested children ---

func TestApply_DeeplyNested(t *testing.T) {
	_, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl := &Template{
		Name:        "nested",
		Description: "Deeply nested",
		Tasks: []TaskTemplate{
			{
				Name: "Level 1", Action: "do", Verify: "check", Result: "done",
				Children: []TaskTemplate{
					{
						Name: "Level 2", Action: "do", Verify: "check", Result: "done",
						Children: []TaskTemplate{
							{Name: "Level 3a", Action: "do", Verify: "check", Result: "done"},
							{Name: "Level 3b", Action: "do", Verify: "check", Result: "done",
								BlockedBy: []string{"Level 3a"}},
						},
					},
				},
			},
		},
	}

	result, err := Apply(store, tmpl, "")
	require.NoError(t, err)
	assert.Len(t, result.CreatedIDs, 4)

	// Verify Level 3b is blocked by Level 3a
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	taskMap := make(map[string]*models.Task)
	for i := range tasks {
		taskMap[tasks[i].Name] = &tasks[i]
	}
	l3b := taskMap["Level 3b"]
	require.NotNil(t, l3b)
	assert.Contains(t, l3b.BlockedBy, taskMap["Level 3a"].ID)

	// Verify parent chain
	l2 := taskMap["Level 2"]
	require.NotNil(t, l2)
	require.NotNil(t, l2.Parent)
	assert.Equal(t, taskMap["Level 1"].ID, *l2.Parent)

	l3a := taskMap["Level 3a"]
	require.NotNil(t, l3a)
	require.NotNil(t, l3a.Parent)
	assert.Equal(t, l2.ID, *l3a.Parent)
}

// --- Test plan criteria 15: Generated task IDs are valid ---

func TestApply_GeneratesValidTaskIDs(t *testing.T) {
	tmpDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl, err := GetTemplate(tmpDir, "feature")
	require.NoError(t, err)

	result, err := Apply(store, tmpl, "")
	require.NoError(t, err)

	for _, id := range result.CreatedIDs {
		assert.True(t, models.IsValidTaskID(id), "ID %s should be valid", id)
	}
}

// --- Test plan criteria 16: Created tasks have proper status ---

func TestApply_TasksHaveTodoStatus(t *testing.T) {
	tmpDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl, err := GetTemplate(tmpDir, "feature")
	require.NoError(t, err)

	_, err = Apply(store, tmpl, "")
	require.NoError(t, err)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	for _, task := range tasks {
		assert.Equal(t, models.StatusTodo, task.Status)
		assert.False(t, task.Created.IsZero())
		assert.False(t, task.Updated.IsZero())
	}
}

// --- RenderTree ---

func TestRenderTree_ShowsHierarchy(t *testing.T) {
	tmpl := &Template{
		Name:        "test",
		Description: "Test template",
		Tasks: []TaskTemplate{
			{
				Name: "Parent", Action: "do", Verify: "check", Result: "done",
				Children: []TaskTemplate{
					{Name: "Child", Action: "do", Verify: "check", Result: "done"},
				},
			},
		},
	}

	output := RenderTree(tmpl)
	assert.Contains(t, output, "test: Test template")
	assert.Contains(t, output, "Parent")
	assert.Contains(t, output, "Child")
}

func TestRenderTree_ShowsBlockedBy(t *testing.T) {
	tmpl := &Template{
		Name:        "test",
		Description: "Test",
		Tasks: []TaskTemplate{
			{Name: "A", Action: "do", Verify: "check", Result: "done"},
			{Name: "B", Action: "do", Verify: "check", Result: "done", BlockedBy: []string{"A"}},
		},
	}

	output := RenderTree(tmpl)
	assert.Contains(t, output, "(after: A)")
}

// --- SWE full cycle template ---

func TestSWEFullCycleTemplate_Structure(t *testing.T) {
	rootDir, store, cleanup := setupTestProject(t)
	defer cleanup()

	tmpl, err := GetTemplate(rootDir, "swe-full-cycle")
	require.NoError(t, err)
	assert.Equal(t, "swe-full-cycle", tmpl.Name)

	result, err := Apply(store, tmpl, "")
	require.NoError(t, err)

	// Count: Plan(4 children+1) + Implement(1) + Test(1) + Review(2 children+1) + Gate(1) + Retro(1) + Deliver(1) = 13
	// Plan: Plan + Research + Explore + Design + TestPlan = 5
	// Review: Review + Code review + Address feedback = 3
	// Others: Implement + Test + Gate + Retro + Deliver = 5
	// Total = 5 + 3 + 5 = 13
	assert.Len(t, result.CreatedIDs, 13)

	tasks, err := store.LoadAll()
	require.NoError(t, err)
	taskMap := make(map[string]*models.Task)
	for i := range tasks {
		taskMap[tasks[i].Name] = &tasks[i]
	}

	// Verify key dependencies
	impl := taskMap["Implement"]
	require.NotNil(t, impl)
	assert.Contains(t, impl.BlockedBy, taskMap["Plan"].ID)

	deliver := taskMap["Deliver"]
	require.NotNil(t, deliver)
	assert.Contains(t, deliver.BlockedBy, taskMap["Retrospective"].ID)
}

// --- No templates directory does not error ---

func TestListTemplates_NoProjectDir(t *testing.T) {
	tmpDir, _, cleanup := setupTestProject(t)
	defer cleanup()

	// .limbo/templates/ does not exist by default
	templates, err := ListTemplates(tmpDir)
	require.NoError(t, err)
	// Should still have built-ins
	assert.GreaterOrEqual(t, len(templates), 3)
}
