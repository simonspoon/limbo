package storage

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/simonspoon/limbo/internal/models"
)

// Storage directory and file names.
const (
	LimboDir    = ".limbo"
	TasksFile   = "tasks.json"
	ArchiveFile = "archive.json"
)

// Storage errors.
var (
	ErrNotInProject = errors.New("not in a limbo project. Run 'limbo init' first")
	ErrTaskNotFound = errors.New("task not found")
)

// TaskStore is the root structure for the tasks.json file
type TaskStore struct {
	Version string        `json:"version"`
	Tasks   []models.Task `json:"tasks"`
}

// Storage handles all file operations for limbo
type Storage struct {
	rootDir string
}

// NewStorage creates a new storage instance
func NewStorage() (*Storage, error) {
	rootDir, err := findProjectRoot()
	if err != nil {
		return nil, err
	}
	return &Storage{rootDir: rootDir}, nil
}

// NewStorageAt creates a storage instance at a specific directory
func NewStorageAt(dir string) *Storage {
	return &Storage{rootDir: dir}
}

// NoClimbEnv is the environment variable that disables parent-directory
// traversal when resolving the .limbo project root. When set to a truthy
// value (1/true/yes, case-insensitive), findProjectRoot only checks the
// current working directory.
const NoClimbEnv = "LIMBO_NO_CLIMB"

// isNoClimb reports whether LIMBO_NO_CLIMB is set to a truthy value.
func isNoClimb() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(NoClimbEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// findProjectRoot searches for the .limbo directory in current or parent directories.
// If LIMBO_NO_CLIMB is truthy, only the current working directory is checked.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	noClimb := isNoClimb()

	for {
		limboPath := filepath.Join(dir, LimboDir)
		if info, err := os.Stat(limboPath); err == nil && info.IsDir() {
			return dir, nil
		}

		if noClimb {
			return "", ErrNotInProject
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInProject
		}
		dir = parent
	}
}

// Init initializes a new limbo project
func (s *Storage) Init() error {
	limboPath := filepath.Join(s.rootDir, LimboDir)

	// Check if already exists
	if _, err := os.Stat(limboPath); err == nil {
		return fmt.Errorf(".limbo directory already exists")
	}

	// Create .limbo directory
	if err := os.Mkdir(limboPath, 0755); err != nil {
		return fmt.Errorf("failed to create .limbo directory: %w", err)
	}

	// Create context directory for per-task content files
	contextPath := filepath.Join(limboPath, ContextDirName)
	if err := os.Mkdir(contextPath, 0755); err != nil {
		return fmt.Errorf("failed to create context directory: %w", err)
	}

	// Create empty task store
	store := &TaskStore{
		Version: "6.0.0",
		Tasks:   []models.Task{},
	}
	return s.saveStore(store)
}

// LoadAll loads all tasks from the store, merging content from context files.
func (s *Storage) LoadAll() ([]models.Task, error) {
	store, err := s.loadStore()
	if err != nil {
		return nil, err
	}
	for i := range store.Tasks {
		if err := s.mergeContext(&store.Tasks[i]); err != nil {
			return nil, err
		}
	}
	return store.Tasks, nil
}

// LoadAllIndex loads all tasks from the JSON index without loading context files.
// Use this when you only need metadata (status, parent, blockedBy, owner, timestamps).
func (s *Storage) LoadAllIndex() ([]models.Task, error) {
	store, err := s.loadStore()
	if err != nil {
		return nil, err
	}
	return store.Tasks, nil
}

// LoadTask loads a task by ID, merging content from its context file.
func (s *Storage) LoadTask(id string) (*models.Task, error) {
	store, err := s.loadStore()
	if err != nil {
		return nil, err
	}

	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			task := &store.Tasks[i]
			if err := s.mergeContext(task); err != nil {
				return nil, err
			}
			return task, nil
		}
	}
	return nil, ErrTaskNotFound
}

// SaveTask saves a task (creates or updates), splitting content into a context file.
func (s *Storage) SaveTask(task *models.Task) error {
	store, err := s.loadStore()
	if err != nil {
		return err
	}

	// Write content fields to context file, or delete context if all are empty
	sections := extractContext(task)
	if len(sections) > 0 {
		if err := s.WriteContext(task.ID, sections); err != nil {
			return err
		}
	} else {
		if err := s.DeleteContext(task.ID); err != nil {
			return err
		}
	}

	// Copy task and strip content fields for the JSON index
	indexTask := *task
	indexTask.Description = ""
	indexTask.Approach = ""
	indexTask.Verify = ""
	indexTask.Result = ""
	indexTask.Outcome = ""
	indexTask.AcceptanceCriteria = ""
	indexTask.ScopeOut = ""
	indexTask.AffectedAreas = ""
	indexTask.TestStrategy = ""
	indexTask.Risks = ""
	indexTask.Report = ""
	indexTask.Notes = nil

	// Check if task exists (update) or is new (create)
	found := false
	for i := range store.Tasks {
		if store.Tasks[i].ID == task.ID {
			store.Tasks[i] = indexTask
			found = true
			break
		}
	}

	if !found {
		store.Tasks = append(store.Tasks, indexTask)
	}

	return s.saveStore(store)
}

// DeleteTask deletes a task by ID
func (s *Storage) DeleteTask(id string) error {
	store, err := s.loadStore()
	if err != nil {
		return err
	}

	newTasks := make([]models.Task, 0, len(store.Tasks))
	found := false
	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			found = true
			continue
		}
		newTasks = append(newTasks, store.Tasks[i])
	}

	if !found {
		return ErrTaskNotFound
	}

	store.Tasks = newTasks
	if err := s.saveStore(store); err != nil {
		return err
	}
	_ = s.DeleteContext(id)
	return nil
}

// DeleteTasks deletes multiple tasks by ID
func (s *Storage) DeleteTasks(ids []string) error {
	store, err := s.loadStore()
	if err != nil {
		return err
	}

	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	newTasks := make([]models.Task, 0, len(store.Tasks))
	for i := range store.Tasks {
		if !idSet[store.Tasks[i].ID] {
			newTasks = append(newTasks, store.Tasks[i])
		}
	}

	store.Tasks = newTasks
	if err := s.saveStore(store); err != nil {
		return err
	}
	for _, id := range ids {
		_ = s.DeleteContext(id)
	}
	return nil
}

// GetChildren returns all tasks that have the given task as their parent
func (s *Storage) GetChildren(parentID string) ([]models.Task, error) {
	store, err := s.loadStore()
	if err != nil {
		return nil, err
	}

	var children []models.Task
	for i := range store.Tasks {
		if store.Tasks[i].Parent != nil && *store.Tasks[i].Parent == parentID {
			children = append(children, store.Tasks[i])
		}
	}
	return children, nil
}

// findTask finds a task by ID
func findTask(tasks []models.Task, id string) *models.Task {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}

// HasUndoneChildren checks recursively if a task has any descendants that are not done
func (s *Storage) HasUndoneChildren(parentID string) (bool, error) {
	children, err := s.GetChildren(parentID)
	if err != nil {
		return false, err
	}

	for i := range children {
		if children[i].Status != models.StatusDone {
			return true, nil
		}
		// Check grandchildren recursively
		hasUndone, err := s.HasUndoneChildren(children[i].ID)
		if err != nil {
			return false, err
		}
		if hasUndone {
			return true, nil
		}
	}
	return false, nil
}

// OrphanChildren sets Parent to nil for all direct children of the given task
func (s *Storage) OrphanChildren(parentID string) error {
	store, err := s.loadStore()
	if err != nil {
		return err
	}

	for i := range store.Tasks {
		if store.Tasks[i].Parent != nil && *store.Tasks[i].Parent == parentID {
			store.Tasks[i].Parent = nil
		}
	}

	return s.saveStore(store)
}

// LegacyTask represents a task with int64 IDs (v2.0.0 format)
type LegacyTask struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Parent      *int64        `json:"parent"`
	Status      string        `json:"status"`
	BlockedBy   []int64       `json:"blockedBy,omitempty"`
	Owner       *string       `json:"owner,omitempty"`
	Notes       []models.Note `json:"notes,omitempty"`
	Created     string        `json:"created"`
	Updated     string        `json:"updated"`
}

// LegacyTaskStore represents the v2.0.0 format
type LegacyTaskStore struct {
	Version string       `json:"version"`
	Tasks   []LegacyTask `json:"tasks"`
}

// loadStore reads the tasks.json file
func (s *Storage) loadStore() (*TaskStore, error) {
	storePath := filepath.Join(s.rootDir, LimboDir, TasksFile)

	data, err := os.ReadFile(storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &TaskStore{Version: "6.0.0", Tasks: []models.Task{}}, nil
		}
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}

	// First, check the version
	var versionCheck struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &versionCheck); err != nil {
		return nil, fmt.Errorf("failed to parse tasks file: %w", err)
	}

	// If v2.0.0, migrate to v4.0.0 (skip v3)
	if versionCheck.Version == "2.0.0" {
		return s.migrateFromV2(data)
	}

	// If v3.0.0, migrate to v4.0.0
	if versionCheck.Version == "3.0.0" {
		return s.migrateFromV3(data)
	}

	// If v4.0.0, migrate to v5.0.0 (split content into context files)
	if versionCheck.Version == "4.0.0" {
		return s.migrateFromV4(data)
	}

	// If v5.0.0, migrate to v6.0.0 (7-stage lifecycle, Action→Approach)
	if versionCheck.Version == "5.0.0" {
		return s.migrateFromV5(data)
	}

	var store TaskStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse tasks file: %w", err)
	}

	return &store, nil
}

// migrateFromV2 migrates from v2.0.0 (int64 IDs) to v3.0.0 (string IDs)
func (s *Storage) migrateFromV2(data []byte) (*TaskStore, error) {
	var legacy LegacyTaskStore
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("failed to parse legacy tasks file: %w", err)
	}

	// Create backup before migration
	storePath := filepath.Join(s.rootDir, LimboDir, TasksFile)
	backupPath := storePath + ".bak"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	// Build mapping from old int64 IDs to new string IDs
	idMapping := make(map[int64]string)
	existingIDs := make(map[string]bool)

	for i := range legacy.Tasks {
		newID := generateRandomAlphaID()
		for existingIDs[newID] {
			newID = generateRandomAlphaID()
		}
		idMapping[legacy.Tasks[i].ID] = newID
		existingIDs[newID] = true
	}

	// Convert tasks
	newTasks := make([]models.Task, len(legacy.Tasks))
	for i := range legacy.Tasks {
		lt := &legacy.Tasks[i]
		var parent *string
		if lt.Parent != nil {
			newParent := idMapping[*lt.Parent]
			parent = &newParent
		}

		var blockedBy []string
		for _, oldBlocker := range lt.BlockedBy {
			if newID, ok := idMapping[oldBlocker]; ok {
				blockedBy = append(blockedBy, newID)
			}
		}

		newTasks[i] = models.Task{
			ID:          idMapping[lt.ID],
			Name:        lt.Name,
			Description: lt.Description,
			Parent:      parent,
			Status:      lt.Status,
			BlockedBy:   blockedBy,
			Owner:       lt.Owner,
			Notes:       lt.Notes,
		}

		// Parse timestamps
		if created, err := parseTimestamp(lt.Created); err == nil {
			newTasks[i].Created = created
		}
		if updated, err := parseTimestamp(lt.Updated); err == nil {
			newTasks[i].Updated = updated
		}
	}

	store := &TaskStore{
		Version: "4.0.0",
		Tasks:   newTasks,
	}

	// Save migrated store
	if err := s.saveStore(store); err != nil {
		return nil, fmt.Errorf("failed to save migrated store: %w", err)
	}

	// Chain to v4→v5→v6 migration
	v4Data, err := json.Marshal(store)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v4 store for chaining: %w", err)
	}
	return s.migrateFromV4(v4Data)
}

// migrateFromV3 migrates from v3.0.0 to v4.0.0 (adds structured fields, which default to "")
func (s *Storage) migrateFromV3(data []byte) (*TaskStore, error) {
	// Create backup before migration
	storePath := filepath.Join(s.rootDir, LimboDir, TasksFile)
	backupPath := storePath + ".v3.bak"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	// Unmarshal existing data — missing fields default to ""
	var store TaskStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse tasks file: %w", err)
	}

	// Bump version
	store.Version = "4.0.0"

	// Save migrated store
	if err := s.saveStore(&store); err != nil {
		return nil, fmt.Errorf("failed to save migrated store: %w", err)
	}

	// Chain to v4→v5→v6 migration
	v4Data, err := json.Marshal(&store)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v4 store for chaining: %w", err)
	}
	return s.migrateFromV4(v4Data)
}

// v4Task is used during v4→v5 migration to capture the Action field
// which no longer exists on models.Task (replaced by Approach in v6).
type v4Task struct {
	models.Task
	Action string `json:"action,omitempty"`
}

// v4Store is used during v4→v5 migration.
type v4Store struct {
	Version string   `json:"version"`
	Tasks   []v4Task `json:"tasks"`
}

// migrateFromV4 migrates from v4.0.0 to v5.0.0 (split content into per-task context files)
func (s *Storage) migrateFromV4(data []byte) (*TaskStore, error) {
	// Create backup before migration
	storePath := filepath.Join(s.rootDir, LimboDir, TasksFile)
	backupPath := storePath + ".v4.bak"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	// Unmarshal v4 data using v4-specific struct to capture Action field
	var v4 v4Store
	if err := json.Unmarshal(data, &v4); err != nil {
		return nil, fmt.Errorf("failed to parse tasks file: %w", err)
	}

	// Create context directory if it doesn't exist
	contextPath := filepath.Join(s.rootDir, LimboDir, ContextDirName)
	if err := os.MkdirAll(contextPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create context directory: %w", err)
	}

	// Convert v4 tasks to current Task type, mapping Action → Approach
	store := &TaskStore{
		Version: "5.0.0",
		Tasks:   make([]models.Task, len(v4.Tasks)),
	}
	for i := range v4.Tasks {
		store.Tasks[i] = v4.Tasks[i].Task
		if v4.Tasks[i].Action != "" && store.Tasks[i].Approach == "" {
			store.Tasks[i].Approach = v4.Tasks[i].Action
		}
	}

	// For each task, extract content to context files and strip from index
	for i := range store.Tasks {
		sections := extractContext(&store.Tasks[i])
		if len(sections) > 0 {
			if err := s.WriteContext(store.Tasks[i].ID, sections); err != nil {
				return nil, fmt.Errorf("failed to write context for task %s: %w", store.Tasks[i].ID, err)
			}
		}

		// Strip content fields from the index
		store.Tasks[i].Description = ""
		store.Tasks[i].Approach = ""
		store.Tasks[i].Verify = ""
		store.Tasks[i].Result = ""
		store.Tasks[i].Outcome = ""
		store.Tasks[i].Notes = nil
	}

	// Save the stripped store
	if err := s.saveStore(store); err != nil {
		return nil, fmt.Errorf("failed to save migrated store: %w", err)
	}

	// Chain to v5→v6 migration
	v5Data, err := json.Marshal(store)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v5 store for chaining: %w", err)
	}
	return s.migrateFromV5(v5Data)
}

// migrateFromV5 migrates from v5.0.0 to v6.0.0 (7-stage lifecycle, Action→Approach in context files)
func (s *Storage) migrateFromV5(data []byte) (*TaskStore, error) {
	// Create backup before migration
	storePath := filepath.Join(s.rootDir, LimboDir, TasksFile)
	backupPath := storePath + ".v5.bak"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	var store TaskStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse tasks file: %w", err)
	}

	// For each task, update context files: rename "Action" section to "Approach"
	contextPath := filepath.Join(s.rootDir, LimboDir, ContextDirName)
	if _, err := os.Stat(contextPath); err == nil {
		for i := range store.Tasks {
			sections, err := s.ReadContext(store.Tasks[i].ID)
			if err != nil {
				continue
			}
			if len(sections) == 0 {
				continue
			}
			if actionContent, ok := sections["Action"]; ok {
				if _, hasApproach := sections["Approach"]; !hasApproach {
					sections["Approach"] = actionContent
				}
				delete(sections, "Action")
				if err := s.WriteContext(store.Tasks[i].ID, sections); err != nil {
					return nil, fmt.Errorf("failed to update context for task %s: %w", store.Tasks[i].ID, err)
				}
			}
		}
	}

	// Map old statuses to new lifecycle stages
	for i := range store.Tasks {
		if store.Tasks[i].Status == "todo" {
			store.Tasks[i].Status = "captured"
		}
		// "in-progress" and "done" stay as-is
	}

	// Bump version
	store.Version = "6.0.0"

	// Save migrated store
	if err := s.saveStore(&store); err != nil {
		return nil, fmt.Errorf("failed to save migrated store: %w", err)
	}

	return &store, nil
}

// saveStore writes the tasks.json file
func (s *Storage) saveStore(store *TaskStore) error {
	storePath := filepath.Join(s.rootDir, LimboDir, TasksFile)

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	if err := os.WriteFile(storePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	return nil
}

// loadArchive reads the archive.json file, returning an empty store if it doesn't exist
func (s *Storage) loadArchive() (*TaskStore, error) {
	archivePath := filepath.Join(s.rootDir, LimboDir, ArchiveFile)

	data, err := os.ReadFile(archivePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &TaskStore{Version: "6.0.0", Tasks: []models.Task{}}, nil
		}
		return nil, fmt.Errorf("failed to read archive file: %w", err)
	}

	var store TaskStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse archive file: %w", err)
	}

	return &store, nil
}

// saveArchive writes the archive.json file
func (s *Storage) saveArchive(store *TaskStore) error {
	archivePath := filepath.Join(s.rootDir, LimboDir, ArchiveFile)

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal archive: %w", err)
	}

	if err := os.WriteFile(archivePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write archive file: %w", err)
	}

	return nil
}

// ArchiveTasks appends tasks to the archive file
func (s *Storage) ArchiveTasks(tasks []models.Task) error {
	archive, err := s.loadArchive()
	if err != nil {
		return err
	}

	archive.Tasks = append(archive.Tasks, tasks...)
	return s.saveArchive(archive)
}

// LoadArchive loads all archived tasks
func (s *Storage) LoadArchive() ([]models.Task, error) {
	archive, err := s.loadArchive()
	if err != nil {
		return nil, err
	}
	return archive.Tasks, nil
}

// LoadArchivedTask loads a single archived task by ID
func (s *Storage) LoadArchivedTask(id string) (*models.Task, error) {
	archive, err := s.loadArchive()
	if err != nil {
		return nil, err
	}

	for i := range archive.Tasks {
		if archive.Tasks[i].ID == id {
			return &archive.Tasks[i], nil
		}
	}
	return nil, ErrTaskNotFound
}

// UnarchiveTask removes a task from the archive and returns it
func (s *Storage) UnarchiveTask(id string) (*models.Task, error) {
	archive, err := s.loadArchive()
	if err != nil {
		return nil, err
	}

	var found *models.Task
	newTasks := make([]models.Task, 0, len(archive.Tasks))
	for i := range archive.Tasks {
		if archive.Tasks[i].ID == id {
			task := archive.Tasks[i]
			found = &task
			continue
		}
		newTasks = append(newTasks, archive.Tasks[i])
	}

	if found == nil {
		return nil, ErrTaskNotFound
	}

	archive.Tasks = newTasks
	if err := s.saveArchive(archive); err != nil {
		return nil, err
	}

	return found, nil
}

// PurgeArchive permanently deletes all archived tasks
func (s *Storage) PurgeArchive() error {
	archive := &TaskStore{Version: "6.0.0", Tasks: []models.Task{}}
	return s.saveArchive(archive)
}

// GetRootDir returns the project root directory
func (s *Storage) GetRootDir() string {
	return s.rootDir
}

// IsBlocked returns true if any task in BlockedBy is not done
func (s *Storage) IsBlocked(task *models.Task) (bool, error) {
	if len(task.BlockedBy) == 0 {
		return false, nil
	}

	store, err := s.loadStore()
	if err != nil {
		return false, err
	}

	for _, blockerID := range task.BlockedBy {
		blocker := findTask(store.Tasks, blockerID)
		if blocker != nil && blocker.Status != models.StatusDone {
			return true, nil
		}
	}
	return false, nil
}

// WouldCreateCycle checks if adding blockerID to blockedID's BlockedBy would create a cycle
func (s *Storage) WouldCreateCycle(blockerID, blockedID string) (bool, error) {
	store, err := s.loadStore()
	if err != nil {
		return false, err
	}

	// BFS from blockerID following BlockedBy chains
	// If we reach blockedID, adding this dependency would create a cycle
	visited := make(map[string]bool)
	queue := []string{blockerID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		task := findTask(store.Tasks, current)
		if task == nil {
			continue
		}

		for _, depID := range task.BlockedBy {
			if depID == blockedID {
				return true, nil
			}
			if !visited[depID] {
				queue = append(queue, depID)
			}
		}
	}
	return false, nil
}

// RemoveFromAllBlockedBy removes taskID from all tasks' BlockedBy lists
func (s *Storage) RemoveFromAllBlockedBy(taskID string) error {
	store, err := s.loadStore()
	if err != nil {
		return err
	}

	modified := false
	for i := range store.Tasks {
		newBlockedBy := make([]string, 0, len(store.Tasks[i].BlockedBy))
		for _, id := range store.Tasks[i].BlockedBy {
			if id != taskID {
				newBlockedBy = append(newBlockedBy, id)
			} else {
				modified = true
			}
		}
		store.Tasks[i].BlockedBy = newBlockedBy
	}

	if modified {
		return s.saveStore(store)
	}
	return nil
}

// mergeContext reads a task's context file and populates content fields.
func (s *Storage) mergeContext(task *models.Task) error {
	sections, err := s.ReadContext(task.ID)
	if err != nil {
		return err
	}
	if len(sections) == 0 {
		return nil
	}
	task.Description = sections["Description"]
	// Backward compat: if "Action" section exists but "Approach" does not, map it
	if sections["Approach"] != "" {
		task.Approach = sections["Approach"]
	} else if sections["Action"] != "" {
		task.Approach = sections["Action"]
	}
	task.Verify = sections["Verify"]
	task.Result = sections["Result"]
	task.Outcome = sections["Outcome"]
	task.AcceptanceCriteria = sections["AcceptanceCriteria"]
	task.ScopeOut = sections["ScopeOut"]
	task.AffectedAreas = sections["AffectedAreas"]
	task.TestStrategy = sections["TestStrategy"]
	task.Risks = sections["Risks"]
	task.Report = sections["Report"]
	if notesStr, ok := sections["Notes"]; ok && notesStr != "" {
		task.Notes = ParseNotes(notesStr)
	}
	return nil
}

// extractContext builds a section map from a task's content fields.
func extractContext(task *models.Task) map[string]string {
	sections := make(map[string]string)
	if task.Approach != "" {
		sections["Approach"] = task.Approach
	}
	if task.Verify != "" {
		sections["Verify"] = task.Verify
	}
	if task.Result != "" {
		sections["Result"] = task.Result
	}
	if task.Outcome != "" {
		sections["Outcome"] = task.Outcome
	}
	if task.AcceptanceCriteria != "" {
		sections["AcceptanceCriteria"] = task.AcceptanceCriteria
	}
	if task.ScopeOut != "" {
		sections["ScopeOut"] = task.ScopeOut
	}
	if task.AffectedAreas != "" {
		sections["AffectedAreas"] = task.AffectedAreas
	}
	if task.TestStrategy != "" {
		sections["TestStrategy"] = task.TestStrategy
	}
	if task.Risks != "" {
		sections["Risks"] = task.Risks
	}
	if task.Report != "" {
		sections["Report"] = task.Report
	}
	if task.Description != "" {
		sections["Description"] = task.Description
	}
	if len(task.Notes) > 0 {
		var b strings.Builder
		for i, note := range task.Notes {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("### ")
			b.WriteString(note.Timestamp.Format(time.RFC3339))
			b.WriteString("\n")
			b.WriteString(note.Content)
			b.WriteString("\n")
		}
		sections["Notes"] = strings.TrimSpace(b.String())
	}
	return sections
}

// GenerateTaskID generates a unique 4-character alphabetic ID
func (s *Storage) GenerateTaskID() (string, error) {
	store, err := s.loadStore()
	if err != nil {
		return "", err
	}

	archive, err := s.loadArchive()
	if err != nil {
		return "", err
	}

	// Build set of existing IDs (active + archived)
	existingIDs := make(map[string]bool)
	for i := range store.Tasks {
		existingIDs[store.Tasks[i].ID] = true
	}
	for i := range archive.Tasks {
		existingIDs[archive.Tasks[i].ID] = true
	}

	// Generate new ID with collision checking
	for attempts := 0; attempts < 100; attempts++ {
		id := generateRandomAlphaID()
		if !existingIDs[id] {
			return id, nil
		}
	}

	return "", fmt.Errorf("failed to generate unique task ID after 100 attempts")
}

// generateRandomAlphaID generates a random 4-character lowercase alphabetic string
func generateRandomAlphaID() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback to less random but still functional
		for i := range b {
			b[i] = letters[i%26]
		}
		return string(b)
	}
	for i := range b {
		b[i] = letters[int(b[i])%26]
	}
	return string(b)
}

// parseTimestamp parses a timestamp string from JSON
func parseTimestamp(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
