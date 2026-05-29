package jsonstore

import (
	"fmt"
	"os"

	"github.com/simonspoon/limbo/internal/models"
)

// ErrTaskNotFound is returned by mutations that target a task ID absent from
// the store.
var ErrTaskNotFound = fmt.Errorf("task not found")

// Load returns all tasks with their sidecar content merged in. It is
// read-only: it takes a shared lock and never changes the revision. A
// fresh/missing store returns an empty slice and a nil error.
func (s *Store) Load() ([]models.Task, error) {
	var tasks []models.Task
	err := s.withSharedLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		out := make([]models.Task, len(env.Tasks))
		copy(out, env.Tasks)
		for i := range out {
			if err := s.mergeContext(&out[i]); err != nil {
				return err
			}
		}
		tasks = out
		return nil
	})
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		tasks = []models.Task{}
	}
	return tasks, nil
}

// SaveAll persists the full task set, replacing prior contents. It splits each
// task's structured content into its sidecar, strips those fields from the
// JSON index, increments the revision by exactly one, and writes atomically —
// all under a single exclusive lock hold.
func (s *Store) SaveAll(tasks []models.Task) error {
	return s.withExclusiveLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		return s.persistLocked(env, tasks)
	})
}

// persistLocked writes the given task set as the new store contents, bumping
// the revision. The caller must hold the exclusive lock and have already read
// the current envelope (for its revision).
func (s *Store) persistLocked(current *envelope, tasks []models.Task) error {
	index := make([]models.Task, len(tasks))
	for i := range tasks {
		t := tasks[i]
		if err := s.writeContextFor(&t); err != nil {
			return err
		}
		index[i] = stripContent(t)
	}
	next := &envelope{
		Version:  SchemaVersion,
		Revision: current.Revision + 1,
		Tasks:    index,
	}
	return s.writeEnvelope(next)
}

// Revision returns the store's current revision counter. Read-only.
func (s *Store) Revision() (int, error) {
	var rev int
	err := s.withSharedLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		rev = env.Revision
		return nil
	})
	return rev, err
}

// AddTask appends a new task. It reads, mutates, and writes entirely within a
// single exclusive lock hold so concurrent adders never lose updates.
func (s *Store) AddTask(task models.Task) error {
	return s.withExclusiveLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		merged := s.mergeAll(env.Tasks)
		merged = append(merged, task)
		return s.persistLocked(env, merged)
	})
}

// UpdateTask replaces an existing task matched by ID. Errors if the ID is
// absent. Read-modify-write under one exclusive lock hold.
func (s *Store) UpdateTask(task models.Task) error {
	return s.withExclusiveLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		merged := s.mergeAll(env.Tasks)
		found := false
		for i := range merged {
			if merged[i].ID == task.ID {
				merged[i] = task
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: %s", ErrTaskNotFound, task.ID)
		}
		return s.persistLocked(env, merged)
	})
}

// DeleteTask removes a task (and its sidecar) by ID. Errors if the ID is
// absent. Read-modify-write under one exclusive lock hold.
func (s *Store) DeleteTask(id string) error {
	return s.withExclusiveLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		merged := s.mergeAll(env.Tasks)
		out := make([]models.Task, 0, len(merged))
		found := false
		for i := range merged {
			if merged[i].ID == id {
				found = true
				continue
			}
			out = append(out, merged[i])
		}
		if !found {
			return fmt.Errorf("%w: %s", ErrTaskNotFound, id)
		}
		if err := s.persistLocked(env, out); err != nil {
			return err
		}
		// Best-effort removal of the orphaned sidecar directory.
		_ = os.RemoveAll(s.contextDir(id))
		return nil
	})
}

// AppendNote appends a timestamped note to the named task's Notes section.
// Errors if the ID is absent. Read-modify-write under one exclusive lock hold.
func (s *Store) AppendNote(id, content string) error {
	return s.withExclusiveLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		merged := s.mergeAll(env.Tasks)
		idx := -1
		for i := range merged {
			if merged[i].ID == id {
				idx = i
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("%w: %s", ErrTaskNotFound, id)
		}
		merged[idx].Notes = append(merged[idx].Notes, models.Note{
			Content:   content,
			Timestamp: nowUTC(),
		})
		return s.persistLocked(env, merged)
	})
}

// ReadContext returns the raw markdown sidecar body for a task, or an empty
// string (nil error) when no sidecar exists. Read-only.
func (s *Store) ReadContext(taskID string) (string, error) {
	var body string
	err := s.withSharedLock(func() error {
		data, err := os.ReadFile(s.contextFilePath(taskID))
		if err != nil {
			if os.IsNotExist(err) {
				body = ""
				return nil
			}
			return fmt.Errorf("read context sidecar: %w", err)
		}
		body = string(data)
		return nil
	})
	return body, err
}

// WriteContext overwrites the raw markdown sidecar body for a task. Mutation.
func (s *Store) WriteContext(taskID, content string) error {
	return s.withExclusiveLock(func() error {
		dir := s.contextDir(taskID)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create context dir: %w", err)
		}
		if err := os.WriteFile(s.contextFilePath(taskID), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write context sidecar: %w", err)
		}
		return nil
	})
}

// mergeAll merges sidecar content into a copy of every task in the slice. Used
// by mutations so a read-modify-write cycle preserves structured content that
// lives only in the sidecars (the JSON index holds it stripped).
func (s *Store) mergeAll(tasks []models.Task) []models.Task {
	out := make([]models.Task, len(tasks))
	copy(out, tasks)
	for i := range out {
		_ = s.mergeContext(&out[i])
	}
	return out
}

// writeContextFor renders a task's structured content into its sidecar, or
// removes the sidecar directory when the task has no content at all.
func (s *Store) writeContextFor(task *models.Task) error {
	sections := extractContext(task)
	if len(sections) == 0 {
		_ = os.RemoveAll(s.contextDir(task.ID))
		return nil
	}
	dir := s.contextDir(task.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create context dir: %w", err)
	}
	if err := os.WriteFile(s.contextFilePath(task.ID), []byte(renderContextFile(sections)), 0o644); err != nil {
		return fmt.Errorf("write context sidecar: %w", err)
	}
	return nil
}

// mergeContext reads a task's sidecar and populates its structured content
// fields. Absent sidecar leaves the task untouched.
func (s *Store) mergeContext(task *models.Task) error {
	data, err := os.ReadFile(s.contextFilePath(task.ID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read context sidecar: %w", err)
	}
	sections := parseContextFile(string(data))
	if len(sections) == 0 {
		return nil
	}
	task.Description = sections["Description"]
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
		task.Notes = parseNotes(notesStr)
	}
	return nil
}

// stripContent returns a copy of task with its structured content fields and
// notes cleared, suitable for the JSON index (the content lives in the
// sidecar).
func stripContent(task models.Task) models.Task {
	task.Description = ""
	task.Approach = ""
	task.Verify = ""
	task.Result = ""
	task.Outcome = ""
	task.AcceptanceCriteria = ""
	task.ScopeOut = ""
	task.AffectedAreas = ""
	task.TestStrategy = ""
	task.Risks = ""
	task.Report = ""
	task.Notes = nil
	return task
}
