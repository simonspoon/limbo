// Package store defines the typed storage seam that all of limbo's task and
// sidecar IO routes through, together with the pure location/identity
// primitives (central-path resolution, project-ID derivation, and the
// project-root climb) that every storage backend builds on.
//
// This package intentionally contains no backend implementation. The default
// JSON backend that satisfies the Store interface lives in a sibling package
// (internal/store/jsonstore, added in a later task). Keeping the interface and
// the pure resolvers here lets callers depend on the seam without depending on
// any concrete backend.
package store

import "github.com/simonspoon/limbo/internal/models"

// Store is the typed seam through which all task and sidecar IO flows. Every
// call site that today reads or writes tasks.json or a context sidecar is
// expected to route through this interface so that the backend (JSON today,
// possibly sqlite later) can change without touching callers.
//
// Context sidecars are addressed by task ID and carry the per-task markdown
// content (Description, Approach, Verify, Result, etc.) that lives outside the
// JSON index. ReadContext returns the raw markdown body for a task;
// WriteContext replaces it. The mapping between structured Task fields and the
// markdown section layout is the backend's concern, not the caller's.
type Store interface {
	// Load returns all tasks with their sidecar content merged in. It is a
	// read-only operation and must not change the store's revision.
	Load() ([]models.Task, error)

	// SaveAll persists the full task set, replacing the prior contents. A
	// successful SaveAll is a mutation and increments the store revision by
	// exactly one.
	SaveAll(tasks []models.Task) error

	// Revision returns the store's current monotonically-increasing revision
	// counter. A freshly initialized store reports 0. Read-only operations
	// never change it.
	Revision() (int, error)

	// AddTask appends a new task to the store. It is a mutation.
	AddTask(task models.Task) error

	// UpdateTask replaces an existing task (matched by ID). It is a mutation
	// and returns an error if no task with the given ID exists.
	UpdateTask(task models.Task) error

	// DeleteTask removes a task (and its sidecar) by ID. It is a mutation and
	// returns an error if no task with the given ID exists.
	DeleteTask(id string) error

	// AppendNote appends a timestamped note to the named task. It is a
	// mutation.
	AppendNote(id, content string) error

	// ReadContext returns the raw markdown sidecar body for the given task ID.
	// It returns an empty string (and a nil error) when the task has no
	// sidecar. It is a read-only operation.
	ReadContext(taskID string) (string, error)

	// WriteContext replaces the markdown sidecar body for the given task ID.
	// It is a mutation.
	WriteContext(taskID, content string) error
}
