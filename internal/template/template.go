// Package template provides template loading, parsing, and application for limbo.
// Templates define reusable task hierarchies that can be applied to create
// tasks with parent/child relationships and block dependencies.
package template

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"gopkg.in/yaml.v3"
)

//go:embed builtin/*.yaml
var builtinFS embed.FS

// Template represents a reusable task hierarchy definition.
type Template struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Tasks       []TaskTemplate `yaml:"tasks"`
}

// TaskTemplate represents a single task within a template.
type TaskTemplate struct {
	Name      string         `yaml:"name"`
	Approach  string         `yaml:"action"`
	Verify    string         `yaml:"verify"`
	Result    string         `yaml:"result"`
	BlockedBy []string       `yaml:"blocked_by,omitempty"`
	Children  []TaskTemplate `yaml:"children,omitempty"`
}

// ApplyResult holds the outcome of applying a template.
type ApplyResult struct {
	CreatedIDs []string `json:"createdIds"`
}

// ListTemplates returns all available templates. Project-local templates
// in .limbo/templates/ override built-in templates with the same name.
func ListTemplates(rootDir string) ([]Template, error) {
	templates := make(map[string]Template)

	// Load built-in templates first
	builtins, err := loadBuiltinTemplates()
	if err != nil {
		return nil, fmt.Errorf("loading built-in templates: %w", err)
	}
	for _, t := range builtins {
		templates[t.Name] = t
	}

	// Load project-local templates (override built-ins)
	projectDir := filepath.Join(rootDir, storage.LimboDir, "templates")
	locals, err := loadProjectTemplates(projectDir)
	if err != nil {
		return nil, fmt.Errorf("loading project templates: %w", err)
	}
	for _, t := range locals {
		templates[t.Name] = t
	}

	// Convert to sorted slice
	result := make([]Template, 0, len(templates))
	for _, t := range templates {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// GetTemplate returns a single template by name. Project-local templates
// take precedence over built-in templates.
func GetTemplate(rootDir, name string) (*Template, error) {
	// Check project-local first
	projectDir := filepath.Join(rootDir, storage.LimboDir, "templates")
	locals, err := loadProjectTemplates(projectDir)
	if err != nil {
		return nil, fmt.Errorf("loading project templates: %w", err)
	}
	for i := range locals {
		if locals[i].Name == name {
			return &locals[i], nil
		}
	}

	// Fall back to built-in
	builtins, err := loadBuiltinTemplates()
	if err != nil {
		return nil, fmt.Errorf("loading built-in templates: %w", err)
	}
	for i := range builtins {
		if builtins[i].Name == name {
			return &builtins[i], nil
		}
	}

	return nil, fmt.Errorf("template %q not found", name)
}

// Apply creates all tasks defined in the template, establishing parent/child
// relationships and block dependencies. If parentID is non-empty, all root-level
// template tasks are nested under that existing task.
func Apply(store *storage.Storage, tmpl *Template, parentID string) (*ApplyResult, error) {
	if len(tmpl.Tasks) == 0 {
		return &ApplyResult{CreatedIDs: []string{}}, nil
	}

	// Validate parent if specified
	if parentID != "" {
		normalizedParent := models.NormalizeTaskID(parentID)
		if !models.IsValidTaskID(normalizedParent) {
			return nil, fmt.Errorf("invalid parent task ID: %s", parentID)
		}
		parentTask, err := store.LoadTask(normalizedParent)
		if err != nil {
			return nil, fmt.Errorf("parent task %s not found", parentID)
		}
		if parentTask.Status == models.StatusDone {
			return nil, fmt.Errorf("cannot nest under done task %s", parentID)
		}
		parentID = normalizedParent
	}

	// Pass 1: Create all tasks, collecting name-to-ID mappings per scope.
	// Pass 2: Apply blocked_by dependencies using the name-to-ID mappings.

	var allIDs []string
	// scopeMap maps a scope key to a name->ID map.
	// Scope key is the parent task ID (or "" for root-level template tasks).
	scopeMap := make(map[string]map[string]string)

	// Pass 1: recursive creation
	if err := createTasks(store, tmpl.Tasks, parentID, &allIDs, scopeMap); err != nil {
		return nil, fmt.Errorf("creating tasks: %w", err)
	}

	// Pass 2: apply blocked_by
	if err := applyBlockedBy(store, tmpl.Tasks, parentID, scopeMap); err != nil {
		return nil, fmt.Errorf("applying dependencies: %w", err)
	}

	return &ApplyResult{CreatedIDs: allIDs}, nil
}

// RenderTree returns a human-readable tree representation of the template
// without creating any tasks.
func RenderTree(tmpl *Template) string {
	var b strings.Builder
	b.WriteString(tmpl.Name)
	b.WriteString(": ")
	b.WriteString(tmpl.Description)
	b.WriteString("\n")
	renderTaskTree(&b, tmpl.Tasks, "")
	return b.String()
}

func renderTaskTree(b *strings.Builder, tasks []TaskTemplate, prefix string) {
	for i, t := range tasks {
		isLast := i == len(tasks)-1
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		b.WriteString(prefix)
		b.WriteString(connector)
		b.WriteString(t.Name)
		if len(t.BlockedBy) > 0 {
			b.WriteString(" (after: ")
			b.WriteString(strings.Join(t.BlockedBy, ", "))
			b.WriteString(")")
		}
		b.WriteString("\n")

		if len(t.Children) > 0 {
			renderTaskTree(b, t.Children, childPrefix)
		}
	}
}

// createTasks recursively creates tasks and populates scopeMap and allIDs.
func createTasks(store *storage.Storage, tasks []TaskTemplate, parentID string, allIDs *[]string, scopeMap map[string]map[string]string) error {
	if _, ok := scopeMap[parentID]; !ok {
		scopeMap[parentID] = make(map[string]string)
	}

	for _, tt := range tasks {
		taskID, err := store.GenerateTaskID()
		if err != nil {
			return err
		}

		var parent *string
		if parentID != "" {
			p := parentID
			parent = &p
		}

		now := time.Now()
		task := &models.Task{
			ID:       taskID,
			Name:     tt.Name,
			Approach: tt.Approach,
			Verify:   tt.Verify,
			Result:   tt.Result,
			Parent:   parent,
			Status:   models.StatusCaptured,
			Created:  now,
			Updated:  now,
		}

		if err := store.SaveTask(task); err != nil {
			return fmt.Errorf("saving task %q: %w", tt.Name, err)
		}

		scopeMap[parentID][tt.Name] = taskID
		*allIDs = append(*allIDs, taskID)

		// Recurse into children
		if len(tt.Children) > 0 {
			if err := createTasks(store, tt.Children, taskID, allIDs, scopeMap); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyBlockedBy recursively resolves blocked_by names to IDs and sets dependencies.
func applyBlockedBy(store *storage.Storage, tasks []TaskTemplate, parentID string, scopeMap map[string]map[string]string) error {
	scope := scopeMap[parentID]

	for _, tt := range tasks {
		taskID := scope[tt.Name]

		for _, blockerName := range tt.BlockedBy {
			blockerID, ok := scope[blockerName]
			if !ok {
				return fmt.Errorf("blocked_by reference %q not found in scope (siblings of %q)", blockerName, tt.Name)
			}

			// Load the task, add blocked_by, save
			task, err := store.LoadTask(taskID)
			if err != nil {
				return fmt.Errorf("loading task %s: %w", taskID, err)
			}
			task.BlockedBy = append(task.BlockedBy, blockerID)
			task.Updated = time.Now()
			if err := store.SaveTask(task); err != nil {
				return fmt.Errorf("saving task %s: %w", taskID, err)
			}
		}

		// Recurse into children
		if len(tt.Children) > 0 {
			childParentID := scope[tt.Name]
			if err := applyBlockedBy(store, tt.Children, childParentID, scopeMap); err != nil {
				return err
			}
		}
	}

	return nil
}

// loadBuiltinTemplates reads all YAML files from the embedded filesystem.
func loadBuiltinTemplates() ([]Template, error) {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil, fmt.Errorf("reading embedded templates: %w", err)
	}

	var templates []Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := builtinFS.ReadFile("builtin/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		var tmpl Template
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}
		templates = append(templates, tmpl)
	}

	return templates, nil
}

// loadProjectTemplates reads YAML files from the project's .limbo/templates/ directory.
// Returns an empty slice (not an error) if the directory does not exist.
func loadProjectTemplates(dir string) ([]Template, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading templates directory: %w", err)
	}

	var templates []Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		var tmpl Template
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}
		templates = append(templates, tmpl)
	}

	return templates, nil
}
