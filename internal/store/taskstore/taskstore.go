// Package taskstore is the command-facing facade over the minimal
// store.Store seam. The limbo CLI historically depended on a rich storage
// surface (LoadTask, SaveTask, GenerateTaskID, IsBlocked, WouldCreateCycle,
// archive operations, ...). The T01 store.Store interface is intentionally
// minimal (Load/SaveAll/Revision/AddTask/UpdateTask/DeleteTask/AppendNote plus
// the context sidecar pair). taskstore bridges the two: it offers the rich,
// in-memory query and mutation API the commands expect, implemented as pure
// computation over Load() results persisted via SaveAll(), on top of the T02
// jsonstore backend.
//
// taskstore also owns the central-path resolution and project-identity wiring
// (via the T01 resolvers in internal/store), the on-disk archive (archive.json,
// which is outside the Store interface and handled here as direct JSON IO under
// the central storage root), and the revision-0 seed written by `limbo init`.
//
// This package lives under internal/store, so its direct file IO is exempt
// from the A1 "no direct tasks.json/context.md IO outside internal/store"
// constraint that applies to the command layer.
package taskstore

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/store"
	"github.com/simonspoon/limbo/internal/store/jsonstore"
)

// ErrTaskNotFound is returned by query and mutation methods that target a task
// ID absent from the active store. It mirrors the legacy storage.ErrTaskNotFound
// sentinel so callers can keep using errors.Is / direct comparison.
var ErrTaskNotFound = errors.New("task not found")

// archiveFileName is the on-disk archive index under the storage root. The
// archive is not part of the Store interface; taskstore manages it directly.
const archiveFileName = "archive.json"

// contextDirName is the per-task content directory created at init time so the
// seeded store has the same shape jsonstore expects.
const contextDirName = "context"

// seedEnvelope is the exact revision-0 envelope `limbo init` writes. It is kept
// here (rather than going through SaveAll, which would bump the revision to 1)
// so a freshly initialized store reports revision 0 per the schema-7.0.0
// contract.
type seedEnvelope struct {
	Version  string        `json:"version"`
	Revision int           `json:"revision"`
	Tasks    []models.Task `json:"tasks"`
}

// archiveEnvelope is the on-disk archive.json structure. It deliberately omits
// a revision counter: the archive is an append log, not a revisioned index.
type archiveEnvelope struct {
	Version string        `json:"version"`
	Tasks   []models.Task `json:"tasks"`
}

// Store is the rich, command-facing facade. It wraps a store.Store backend and
// carries the storage root so it can manage the sibling archive.json and report
// the root for run-lock placement.
type Store struct {
	backend store.Store
	root    string
}

// New constructs a facade over a jsonstore rooted at the given central storage
// path. The path need not exist yet; the backend treats a missing root as an
// empty store at revision 0.
func New(root string) *Store {
	return &Store{backend: jsonstore.New(root), root: root}
}

// Backend exposes the underlying Store seam for callers (e.g. the --if-revision
// guard) that need the minimal interface directly.
func (s *Store) Backend() store.Store { return s.backend }

// GetRootDir returns the central storage root backing this facade.
func (s *Store) GetRootDir() string { return s.root }

// ContextDir returns the on-disk directory holding a task's content sidecar.
// It mirrors the jsonstore layout (<root>/context/<id>) and exists so callers
// can assert on sidecar presence.
func (s *Store) ContextDir(id string) string {
	return filepath.Join(s.root, contextDirName, id)
}

// Revision returns the backend's current revision counter.
func (s *Store) Revision() (int, error) { return s.backend.Revision() }

// Seed writes the canonical revision-0 envelope ({"version":"7.0.0",
// "revision":0,"tasks":[]}) and creates the context directory. It is used by
// `limbo init` to materialize a fresh store without the +1 revision bump a
// SaveAll would incur.
func (s *Store) Seed() error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("create storage root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(s.root, contextDirName), 0o755); err != nil {
		return fmt.Errorf("create context dir: %w", err)
	}
	env := seedEnvelope{Version: jsonstore.SchemaVersion, Revision: 0, Tasks: []models.Task{}}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal seed envelope: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(s.root, "tasks.json"), data, 0o644); err != nil {
		return fmt.Errorf("write seed tasks.json: %w", err)
	}
	return nil
}

// ---- read queries (Load() + in-memory logic) ----

// LoadAll returns every active task with sidecar content merged in.
func (s *Store) LoadAll() ([]models.Task, error) {
	return s.backend.Load()
}

// LoadAllIndex returns every active task. The jsonstore backend merges sidecar
// content on Load; there is no metadata-only read, so this is equivalent to
// LoadAll. Callers that only consult metadata (status/parent/blockedBy) are
// unaffected.
func (s *Store) LoadAllIndex() ([]models.Task, error) {
	return s.backend.Load()
}

// LoadTask returns a single active task by ID with sidecar content merged in,
// or ErrTaskNotFound.
func (s *Store) LoadTask(id string) (*models.Task, error) {
	tasks, err := s.backend.Load()
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		if tasks[i].ID == id {
			t := tasks[i]
			return &t, nil
		}
	}
	return nil, ErrTaskNotFound
}

// GetChildren returns all active tasks whose Parent is parentID.
func (s *Store) GetChildren(parentID string) ([]models.Task, error) {
	tasks, err := s.backend.Load()
	if err != nil {
		return nil, err
	}
	var children []models.Task
	for i := range tasks {
		if tasks[i].Parent != nil && *tasks[i].Parent == parentID {
			children = append(children, tasks[i])
		}
	}
	return children, nil
}

// HasUndoneChildren reports whether any descendant of parentID is not done.
func (s *Store) HasUndoneChildren(parentID string) (bool, error) {
	tasks, err := s.backend.Load()
	if err != nil {
		return false, err
	}
	return hasUndoneChildren(tasks, parentID), nil
}

func hasUndoneChildren(tasks []models.Task, parentID string) bool {
	for i := range tasks {
		if tasks[i].Parent == nil || *tasks[i].Parent != parentID {
			continue
		}
		if tasks[i].Status != models.StatusDone {
			return true
		}
		if hasUndoneChildren(tasks, tasks[i].ID) {
			return true
		}
	}
	return false
}

// IsBlocked reports whether a task is blocked, either by a manual block reason
// or by an unfinished BlockedBy predecessor. A BlockedBy entry pointing at a
// missing task is treated as resolved.
func (s *Store) IsBlocked(task *models.Task) (bool, error) {
	if task.ManualBlockReason != "" {
		return true, nil
	}
	if len(task.BlockedBy) == 0 {
		return false, nil
	}
	tasks, err := s.backend.Load()
	if err != nil {
		return false, err
	}
	for _, blockerID := range task.BlockedBy {
		blocker := findTask(tasks, blockerID)
		if blocker != nil && blocker.Status != models.StatusDone {
			return true, nil
		}
	}
	return false, nil
}

// WouldCreateCycle reports whether adding blockerID to blockedID's BlockedBy
// would introduce a dependency cycle. BFS from blockerID following BlockedBy
// chains; reaching blockedID means a cycle.
func (s *Store) WouldCreateCycle(blockerID, blockedID string) (bool, error) {
	tasks, err := s.backend.Load()
	if err != nil {
		return false, err
	}
	visited := make(map[string]bool)
	queue := []string{blockerID}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		task := findTask(tasks, current)
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

// ---- mutations (Load + mutate slice + SaveAll) ----

// SaveTask upserts a task by ID and persists the full set. Sidecar content
// (Description, Approach, Notes, ...) is split out by the backend on SaveAll.
func (s *Store) SaveTask(task *models.Task) error {
	tasks, err := s.backend.Load()
	if err != nil {
		return err
	}
	found := false
	for i := range tasks {
		if tasks[i].ID == task.ID {
			tasks[i] = *task
			found = true
			break
		}
	}
	if !found {
		tasks = append(tasks, *task)
	}
	return s.backend.SaveAll(tasks)
}

// DeleteTask removes a single task by ID, returning ErrTaskNotFound if absent.
func (s *Store) DeleteTask(id string) error {
	tasks, err := s.backend.Load()
	if err != nil {
		return err
	}
	out := make([]models.Task, 0, len(tasks))
	found := false
	for i := range tasks {
		if tasks[i].ID == id {
			found = true
			continue
		}
		out = append(out, tasks[i])
	}
	if !found {
		return ErrTaskNotFound
	}
	if err := s.backend.SaveAll(out); err != nil {
		return err
	}
	// Remove the deleted task's content sidecar. SaveAll only writes sidecars
	// for tasks in the set; it does not prune sidecars of removed tasks.
	_ = os.RemoveAll(s.ContextDir(id))
	return nil
}

// DeleteTasks removes several tasks by ID in a single persist. IDs absent from
// the store are silently ignored (matching the legacy behavior).
func (s *Store) DeleteTasks(ids []string) error {
	tasks, err := s.backend.Load()
	if err != nil {
		return err
	}
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	out := make([]models.Task, 0, len(tasks))
	for i := range tasks {
		if idSet[tasks[i].ID] {
			continue
		}
		out = append(out, tasks[i])
	}
	if err := s.backend.SaveAll(out); err != nil {
		return err
	}
	for id := range idSet {
		_ = os.RemoveAll(s.ContextDir(id))
	}
	return nil
}

// OrphanChildren clears the Parent pointer of every direct child of parentID.
func (s *Store) OrphanChildren(parentID string) error {
	tasks, err := s.backend.Load()
	if err != nil {
		return err
	}
	for i := range tasks {
		if tasks[i].Parent != nil && *tasks[i].Parent == parentID {
			tasks[i].Parent = nil
		}
	}
	return s.backend.SaveAll(tasks)
}

// RemoveFromAllBlockedBy strips taskID from every task's BlockedBy list. It
// only persists when something actually changed.
func (s *Store) RemoveFromAllBlockedBy(taskID string) error {
	tasks, err := s.backend.Load()
	if err != nil {
		return err
	}
	modified := false
	for i := range tasks {
		filtered := make([]string, 0, len(tasks[i].BlockedBy))
		for _, id := range tasks[i].BlockedBy {
			if id == taskID {
				modified = true
				continue
			}
			filtered = append(filtered, id)
		}
		tasks[i].BlockedBy = filtered
	}
	if !modified {
		return nil
	}
	return s.backend.SaveAll(tasks)
}

// GenerateTaskID returns a fresh 4-character lowercase alphabetic ID that
// collides with neither an active nor an archived task.
func (s *Store) GenerateTaskID() (string, error) {
	tasks, err := s.backend.Load()
	if err != nil {
		return "", err
	}
	archived, err := s.LoadArchive()
	if err != nil {
		return "", err
	}
	existing := make(map[string]bool, len(tasks)+len(archived))
	for i := range tasks {
		existing[tasks[i].ID] = true
	}
	for i := range archived {
		existing[archived[i].ID] = true
	}
	for attempts := 0; attempts < 100; attempts++ {
		id := generateRandomAlphaID()
		if !existing[id] {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique task ID after 100 attempts")
}

// ---- legacy migration (direct JSON IO under internal/store) ----

// MigrateLegacy copies a legacy in-tree store rooted at legacyDir into the
// central storage root, rewriting the schema version to the current 7.0.0 and
// incrementing the revision by one. It reads <legacyDir>/tasks.json verbatim,
// builds the destination envelope ({version:7.0.0, revision: legacyRev+1, tasks:
// <legacy tasks verbatim>}), creates the central root and context dir, writes
// the destination tasks.json, and recursively copies <legacyDir>/context/ to
// <centralRoot>/context/.
//
// The transcode is deliberately verbatim: it does NOT round-trip through SaveAll
// (which would re-split sidecar content and bump the revision a second time).
// The caller is responsible for refusing to overwrite an existing central store
// and for renaming the source aside afterwards; MigrateLegacy never deletes or
// renames the source.
func MigrateLegacy(legacyDir, centralRoot string) error {
	data, err := os.ReadFile(filepath.Join(legacyDir, "tasks.json"))
	if err != nil {
		return fmt.Errorf("read legacy tasks.json: %w", err)
	}

	// Parse only the fields we need to transcode. Tasks are preserved verbatim
	// as raw JSON so no content reshaping happens during migration.
	var legacy struct {
		Revision int             `json:"revision"`
		Tasks    json.RawMessage `json:"tasks"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("parse legacy tasks.json: %w", err)
	}
	if len(legacy.Tasks) == 0 {
		legacy.Tasks = json.RawMessage("[]")
	}

	dest := struct {
		Version  string          `json:"version"`
		Revision int             `json:"revision"`
		Tasks    json.RawMessage `json:"tasks"`
	}{
		Version:  jsonstore.SchemaVersion,
		Revision: legacy.Revision + 1,
		Tasks:    legacy.Tasks,
	}

	if err := os.MkdirAll(centralRoot, 0o755); err != nil {
		return fmt.Errorf("create central root: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(centralRoot, contextDirName), 0o755); err != nil {
		return fmt.Errorf("create central context dir: %w", err)
	}

	out, err := json.MarshalIndent(dest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal destination tasks.json: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(filepath.Join(centralRoot, "tasks.json"), out, 0o644); err != nil {
		return fmt.Errorf("write destination tasks.json: %w", err)
	}

	srcContext := filepath.Join(legacyDir, contextDirName)
	if info, statErr := os.Stat(srcContext); statErr == nil && info.IsDir() {
		if err := copyTree(srcContext, filepath.Join(centralRoot, contextDirName)); err != nil {
			return fmt.Errorf("copy context subtree: %w", err)
		}
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("stat legacy context dir: %w", statErr)
	}

	return nil
}

// copyTree recursively copies the directory tree at src into dst, creating dst
// (and intermediate directories) as needed. It copies file contents and
// preserves a sensible permission mode; it does not follow symlinks specially.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
}

// ---- archive (direct JSON IO under the storage root) ----

func (s *Store) archivePath() string { return filepath.Join(s.root, archiveFileName) }

func (s *Store) loadArchiveEnvelope() (*archiveEnvelope, error) {
	data, err := os.ReadFile(s.archivePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &archiveEnvelope{Version: jsonstore.SchemaVersion, Tasks: []models.Task{}}, nil
		}
		return nil, fmt.Errorf("read archive file: %w", err)
	}
	var env archiveEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse archive file: %w", err)
	}
	if env.Tasks == nil {
		env.Tasks = []models.Task{}
	}
	return &env, nil
}

func (s *Store) saveArchiveEnvelope(env *archiveEnvelope) error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("create storage root: %w", err)
	}
	if env.Version == "" {
		env.Version = jsonstore.SchemaVersion
	}
	if env.Tasks == nil {
		env.Tasks = []models.Task{}
	}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal archive: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(s.archivePath(), data, 0o644); err != nil {
		return fmt.Errorf("write archive file: %w", err)
	}
	return nil
}

// ArchiveTasks appends tasks to archive.json.
func (s *Store) ArchiveTasks(tasks []models.Task) error {
	env, err := s.loadArchiveEnvelope()
	if err != nil {
		return err
	}
	env.Tasks = append(env.Tasks, tasks...)
	return s.saveArchiveEnvelope(env)
}

// LoadArchive returns all archived tasks.
func (s *Store) LoadArchive() ([]models.Task, error) {
	env, err := s.loadArchiveEnvelope()
	if err != nil {
		return nil, err
	}
	return env.Tasks, nil
}

// LoadArchivedTask returns a single archived task by ID, or ErrTaskNotFound.
func (s *Store) LoadArchivedTask(id string) (*models.Task, error) {
	env, err := s.loadArchiveEnvelope()
	if err != nil {
		return nil, err
	}
	for i := range env.Tasks {
		if env.Tasks[i].ID == id {
			t := env.Tasks[i]
			return &t, nil
		}
	}
	return nil, ErrTaskNotFound
}

// UnarchiveTask removes a task from the archive and returns it, or
// ErrTaskNotFound.
func (s *Store) UnarchiveTask(id string) (*models.Task, error) {
	env, err := s.loadArchiveEnvelope()
	if err != nil {
		return nil, err
	}
	var found *models.Task
	out := make([]models.Task, 0, len(env.Tasks))
	for i := range env.Tasks {
		if env.Tasks[i].ID == id {
			t := env.Tasks[i]
			found = &t
			continue
		}
		out = append(out, env.Tasks[i])
	}
	if found == nil {
		return nil, ErrTaskNotFound
	}
	env.Tasks = out
	if err := s.saveArchiveEnvelope(env); err != nil {
		return nil, err
	}
	return found, nil
}

// PurgeArchive permanently empties the archive.
func (s *Store) PurgeArchive() error {
	return s.saveArchiveEnvelope(&archiveEnvelope{Version: jsonstore.SchemaVersion, Tasks: []models.Task{}})
}

// ---- helpers ----

func findTask(tasks []models.Task, id string) *models.Task {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}

// generateRandomAlphaID generates a random 4-character lowercase alphabetic
// string. On a crypto/rand failure it falls back to a deterministic-but-valid
// sequence rather than erroring.
func generateRandomAlphaID() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
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
