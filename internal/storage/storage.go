package storage

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/simonspoon/limbo/internal/models"
)

// Storage directory and file names.
const (
	LimboDir  = ".limbo"
	TasksFile = "tasks.json"
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

// findProjectRoot searches for the .limbo directory in current or parent directories
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		limboPath := filepath.Join(dir, LimboDir)
		if info, err := os.Stat(limboPath); err == nil && info.IsDir() {
			return dir, nil
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

	// Create empty task store
	store := &TaskStore{
		Version: "4.0.0",
		Tasks:   []models.Task{},
	}
	return s.saveStore(store)
}

// LoadAll loads all tasks from the store
func (s *Storage) LoadAll() ([]models.Task, error) {
	store, err := s.loadStore()
	if err != nil {
		return nil, err
	}
	return store.Tasks, nil
}

// LoadTask loads a task by ID
func (s *Storage) LoadTask(id string) (*models.Task, error) {
	store, err := s.loadStore()
	if err != nil {
		return nil, err
	}

	for i := range store.Tasks {
		if store.Tasks[i].ID == id {
			return &store.Tasks[i], nil
		}
	}
	return nil, ErrTaskNotFound
}

// SaveTask saves a task (creates or updates)
func (s *Storage) SaveTask(task *models.Task) error {
	store, err := s.loadStore()
	if err != nil {
		return err
	}

	// Check if task exists (update) or is new (create)
	found := false
	for i := range store.Tasks {
		if store.Tasks[i].ID == task.ID {
			store.Tasks[i] = *task
			found = true
			break
		}
	}

	if !found {
		store.Tasks = append(store.Tasks, *task)
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
	return s.saveStore(store)
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
	return s.saveStore(store)
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

// NextResult represents the result of GetNextTask
type NextResult struct {
	Task         *models.Task  `json:"task,omitempty"`
	Candidates   []models.Task `json:"candidates,omitempty"`
	BlockedCount int           `json:"blockedCount,omitempty"`
}

// GetNextTask returns the next task using depth-first traversal.
// When in-progress tasks exist: returns todo children or siblings of the deepest in-progress task.
// When no in-progress tasks: returns root-level todos as candidates.
// Blocked tasks are always skipped.
func (s *Storage) GetNextTask() (*NextResult, error) {
	return s.GetNextTaskFiltered(false)
}

// GetNextTaskFiltered returns the next task with optional ownership filter.
// When unclaimedOnly is true, tasks with an owner are skipped.
func (s *Storage) GetNextTaskFiltered(unclaimedOnly bool) (*NextResult, error) {
	store, err := s.loadStore()
	if err != nil {
		return nil, err
	}

	deepest := getDeepestInProgress(store.Tasks)
	if deepest == nil {
		// No in-progress context - return root-level todos as candidates
		candidates := getRootTodos(store.Tasks, true)
		if unclaimedOnly {
			candidates = filterUnclaimed(candidates)
		}
		result := &NextResult{Candidates: candidates}
		if len(candidates) == 0 {
			result.BlockedCount = countBlockedTodos(store.Tasks)
		}
		return result, nil
	}

	// Walk up from deepest, looking for todo children first, then siblings
	current := deepest
	for {
		// First, check for todo children of current task
		children := getTodoChildren(store.Tasks, current.ID, true)
		if unclaimedOnly {
			children = filterUnclaimed(children)
		}
		if len(children) > 0 {
			return &NextResult{Task: &children[0]}, nil
		}

		// Then, check for todo siblings
		siblings := getTodoSiblings(store.Tasks, current.ID, true)
		if unclaimedOnly {
			siblings = filterUnclaimed(siblings)
		}
		if len(siblings) > 0 {
			return &NextResult{Task: &siblings[0]}, nil
		}

		// Move up to parent
		if current.Parent == nil {
			break
		}
		parent := findTask(store.Tasks, *current.Parent)
		if parent == nil {
			break
		}
		current = parent
	}
	return &NextResult{BlockedCount: countBlockedTodos(store.Tasks)}, nil
}

// filterUnclaimed removes tasks that have an owner
func filterUnclaimed(tasks []models.Task) []models.Task {
	var result []models.Task
	for i := range tasks {
		if tasks[i].Owner == nil {
			result = append(result, tasks[i])
		}
	}
	return result
}

// getDeepestInProgress finds the in-progress task that has no in-progress children
func getDeepestInProgress(tasks []models.Task) *models.Task {
	// Build map of tasks that have in-progress children
	hasInProgressChild := make(map[string]bool)
	for i := range tasks {
		if tasks[i].Status == models.StatusInProgress && tasks[i].Parent != nil {
			hasInProgressChild[*tasks[i].Parent] = true
		}
	}

	// Find in-progress task with no in-progress children (deepest)
	var deepest *models.Task
	for i := range tasks {
		if tasks[i].Status == models.StatusInProgress && !hasInProgressChild[tasks[i].ID] {
			if deepest == nil || tasks[i].Created.Before(deepest.Created) {
				deepest = &tasks[i]
			}
		}
	}
	return deepest
}

// getTodoChildren returns todo tasks that are children of the given task, sorted by created time
func getTodoChildren(tasks []models.Task, parentID string, skipBlocked bool) []models.Task {
	var children []models.Task
	for i := range tasks {
		if tasks[i].Status == models.StatusTodo && tasks[i].Parent != nil && *tasks[i].Parent == parentID {
			if skipBlocked && isTaskBlocked(&tasks[i], tasks) {
				continue
			}
			children = append(children, tasks[i])
		}
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].Created.Before(children[j].Created)
	})
	return children
}

// getTodoSiblings returns todo tasks with the same parent as the given task, sorted by created time
func getTodoSiblings(tasks []models.Task, taskID string, skipBlocked bool) []models.Task {
	// Find the task to get its parent
	var targetParent *string
	for i := range tasks {
		if tasks[i].ID == taskID {
			targetParent = tasks[i].Parent
			break
		}
	}

	// Find all todo tasks with the same parent
	var siblings []models.Task
	for i := range tasks {
		if tasks[i].Status != models.StatusTodo {
			continue
		}
		if skipBlocked && isTaskBlocked(&tasks[i], tasks) {
			continue
		}
		// Check if same parent (both nil or both point to same ID)
		sameParent := (targetParent == nil && tasks[i].Parent == nil) ||
			(targetParent != nil && tasks[i].Parent != nil && *targetParent == *tasks[i].Parent)
		if sameParent {
			siblings = append(siblings, tasks[i])
		}
	}

	// Sort by created time (oldest first)
	sort.Slice(siblings, func(i, j int) bool {
		return siblings[i].Created.Before(siblings[j].Created)
	})

	return siblings
}

// getRootTodos returns all todo tasks with no parent, sorted by created time
func getRootTodos(tasks []models.Task, skipBlocked bool) []models.Task {
	var roots []models.Task
	for i := range tasks {
		if tasks[i].Status == models.StatusTodo && tasks[i].Parent == nil {
			if skipBlocked && isTaskBlocked(&tasks[i], tasks) {
				continue
			}
			roots = append(roots, tasks[i])
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].Created.Before(roots[j].Created)
	})
	return roots
}

// countBlockedTodos counts todo tasks that are blocked by incomplete dependencies
func countBlockedTodos(tasks []models.Task) int {
	count := 0
	for i := range tasks {
		if tasks[i].Status == models.StatusTodo && isTaskBlocked(&tasks[i], tasks) {
			count++
		}
	}
	return count
}

// isTaskBlocked checks if any task in BlockedBy is not done
func isTaskBlocked(task *models.Task, allTasks []models.Task) bool {
	if len(task.BlockedBy) == 0 {
		return false
	}
	for _, blockerID := range task.BlockedBy {
		blocker := findTask(allTasks, blockerID)
		if blocker != nil && blocker.Status != models.StatusDone {
			return true
		}
	}
	return false
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
			return &TaskStore{Version: "4.0.0", Tasks: []models.Task{}}, nil
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

	return store, nil
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

// GenerateTaskID generates a unique 4-character alphabetic ID
func (s *Storage) GenerateTaskID() (string, error) {
	store, err := s.loadStore()
	if err != nil {
		return "", err
	}

	// Build set of existing IDs
	existingIDs := make(map[string]bool)
	for i := range store.Tasks {
		existingIDs[store.Tasks[i].ID] = true
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
