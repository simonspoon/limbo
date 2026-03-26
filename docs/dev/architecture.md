# Architecture

## Package Layout

```
cmd/limbo/main.go              Entry point
internal/commands/             Cobra command implementations (one file per command)
internal/models/               Task and Note structs, status constants
internal/storage/              JSON file storage and all business logic
```

### cmd/limbo/main.go

The entry point is minimal. It delegates entirely to the commands package:

```go
func main() {
    commands.Execute()
}
```

`commands.Execute()` calls `rootCmd.Execute()` from cobra and exits on error (see `internal/commands/root.go`).

### internal/commands/

Each subcommand lives in its own file. The full list of commands registered in `root.go`:

`init`, `add`, `list`, `show`, `status`, `delete`, `edit`, `parent`, `unparent`, `tree`, `next`, `prune`, `watch`, `block`, `unblock`, `note`, `claim`, `unclaim`, `search`, `template`, `export`, `import`

All commands follow the same pattern: call `getStorage()` (defined in `root.go`), perform operations on the returned `*Storage`, then print JSON by default or human-readable output when `--pretty` is passed.

`getStorage()` is a helper that respects the `--global` persistent flag and the `LIMBO_ROOT` environment variable. When either is set, it calls `storage.NewStorageGlobal(rootOverride)` instead of `storage.NewStorage()`. A package-level `globalFlag` variable holds the `--global` flag state, registered as a persistent flag on `rootCmd`.

See `internal/commands/root.go` for the `init()` function that wires all subcommands to `rootCmd`.

### internal/models/task.go

Defines the two core structs and the three valid status constants:

```go
type Note struct {
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}

type Task struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description,omitempty"`
    Parent      *string   `json:"parent"`
    Status      string    `json:"status"`
    BlockedBy   []string  `json:"blockedBy,omitempty"`
    Owner       *string   `json:"owner,omitempty"`
    Notes       []Note    `json:"notes,omitempty"`
    Created     time.Time `json:"created"`
    Updated     time.Time `json:"updated"`
}

const (
    StatusTodo       = "todo"
    StatusInProgress = "in-progress"
    StatusDone       = "done"
)
```

`Parent` and `Owner` are nullable pointers so they serialize as `null` (not omitted) when unset. `BlockedBy`, `Notes`, and `Description` use `omitempty` and are absent from JSON when empty.

Helper functions: `IsValidStatus`, `IsValidTaskID` (enforces 4-character lowercase alpha), `NormalizeTaskID` (lowercases input for case-insensitive acceptance).

### internal/storage/storage.go

All business logic lives here. Commands do not manipulate task slices directly; they call methods on `*Storage`.

---

## Storage Design

### File Location

Tasks are stored at `<project-root>/.limbo/tasks.json`. The constants are:

```go
const (
    LimboDir  = ".limbo"
    TasksFile = "tasks.json"
)
```

### TaskStore (on-disk format)

```go
type TaskStore struct {
    Version string        `json:"version"`
    Tasks   []models.Task `json:"tasks"`
}
```

The current version string is `"4.0.0"`. The file is written with `json.MarshalIndent` using two-space indentation (see `storage.go:590`).

### Storage struct

```go
type Storage struct {
    rootDir string
}
```

`rootDir` is the absolute path to the directory containing `.limbo/`. It is set during construction and never changes.

### Directory Discovery

`NewStorage()` calls `findProjectRoot()`, which walks up the directory tree from the current working directory until it finds a directory containing `.limbo/`, mirroring how git finds `.git/` (see `storage.go:54-72`):

```go
func findProjectRoot() (string, error) {
    dir, err := os.Getwd()
    // ...
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
```

`NewStorageAt(dir string)` bypasses discovery and is used in tests to point at a temporary directory.

`NewStorageGlobal(rootOverride string)` creates a storage instance rooted at the user's home directory (or `rootOverride` if non-empty). Unlike `NewStorage()`, it does not walk up the directory tree — it checks for `.limbo/` directly at the target location and returns an error if not found, prompting the user to run `limbo --global init` first.

### Core Storage Methods

**LoadAll** loads the full task list (see `storage.go:97`):

```go
func (s *Storage) LoadAll() ([]models.Task, error)
```

**LoadTask** finds a single task by ID, returns `ErrTaskNotFound` if absent (see `storage.go:106`):

```go
func (s *Storage) LoadTask(id string) (*models.Task, error)
```

**SaveTask** performs an upsert: it scans the task slice for a matching ID, updates in place if found, or appends if not found, then writes the full store back to disk (see `storage.go:121-142`):

```go
func (s *Storage) SaveTask(task *models.Task) error
```

**DeleteTask** and **DeleteTasks** rebuild the slice excluding the target ID(s) and write back (see `storage.go:145`, `storage.go:170`).

**loadStore** / **saveStore** are unexported helpers that handle JSON marshaling and file I/O. `loadStore` also handles schema migration on first read: v2.0.0 stores (int64 IDs) are migrated directly to v4.0.0; v3.0.0 stores are migrated to v4.0.0 (new structured fields default to `""`). A backup is written before each migration (see `storage.go:477`, `storage.go:587`).

### Task ID Generation

IDs are 4-character lowercase alphabetic strings (e.g., `abcd`). `GenerateTaskID` uses `crypto/rand` to generate candidates and checks against existing IDs for uniqueness, retrying up to 100 times (see `storage.go:692`). The alphabet is `a-z` only, giving 26^4 = 456,976 possible values.

---

## GetNextTask: Depth-First Algorithm

`GetNextTask` and `GetNextTaskFiltered` implement the depth-first traversal used by the `next` command (see `storage.go:219-277`).

### Entry Points

```go
func (s *Storage) GetNextTask() (*NextResult, error)
func (s *Storage) GetNextTaskFiltered(unclaimedOnly bool) (*NextResult, error)
```

`GetNextTask` delegates to `GetNextTaskFiltered(false)`. When `unclaimedOnly` is true, tasks that have a non-nil `Owner` are excluded from results.

### NextResult

```go
type NextResult struct {
    Task         *models.Task  `json:"task,omitempty"`
    Candidates   []models.Task `json:"candidates,omitempty"`
    BlockedCount int           `json:"blockedCount,omitempty"`
}
```

Exactly one of `Task` or `Candidates` is populated, or neither (when no work is available). `BlockedCount` is set when the result set is empty to indicate how many todo tasks are waiting on blockers.

### Step 1: Find the Deepest In-Progress Task

`getDeepestInProgress` locates the in-progress task that has no in-progress children (see `storage.go:291-310`):

1. Build a set of task IDs that have at least one in-progress child.
2. Scan all tasks; any in-progress task not in that set is a leaf candidate.
3. Among candidates, pick the one with the earliest `Created` timestamp (oldest first).

This identifies the "current focus" in the tree.

### Step 2: Walk Up from the Deepest Task

Starting from the deepest in-progress task, the algorithm walks up the hierarchy (see `storage.go:246-275`):

1. **Check todo children** of the current task via `getTodoChildren` — returns todo tasks whose `Parent` matches the current task ID, sorted by `Created` ascending, skipping blocked tasks.
2. If children exist, return the first one as `{task: ...}` and stop.
3. **Check todo siblings** via `getTodoSiblings` — returns todo tasks sharing the same parent, sorted by `Created` ascending, skipping blocked tasks.
4. If siblings exist, return the first one as `{task: ...}` and stop.
5. Move `current` to the parent task and repeat from step 1.
6. If the root is reached with no results, return `{blockedCount: N}`.

### Step 3: No In-Progress Tasks

When `getDeepestInProgress` returns nil (no in-progress tasks exist), `getRootTodos` collects all todo tasks with `Parent == nil`, skipping blocked tasks, sorted by `Created` ascending (see `storage.go:366-380`). These are returned as `{candidates: [...]}`.

### Blocking Check

`isTaskBlocked` returns true if any ID in `task.BlockedBy` refers to a task whose status is not `done` (see `storage.go:394-405`). A missing blocker (deleted task) is treated as non-blocking.

---

## Data Flow

A typical command execution follows this path:

1. Cobra dispatches to the command's `RunE` function.
2. The command calls `getStorage()`, which checks the `--global` flag and `LIMBO_ROOT` env var. If either is set, it calls `storage.NewStorageGlobal()` targeting `~/.limbo/` (or the override path). Otherwise it calls `storage.NewStorage()`, which auto-discovers the `.limbo/` directory by walking up from `os.Getwd()`.
3. The command calls one or more storage methods (`LoadAll`, `LoadTask`, `SaveTask`, etc.).
4. Each storage method calls the unexported `loadStore`, which reads and JSON-unmarshals `tasks.json` from `<rootDir>/.limbo/tasks.json`.
5. The method operates on the in-memory `TaskStore`, then calls `saveStore` to write the updated JSON back to disk.
6. The command marshals its result to JSON and prints to stdout (or uses `--pretty` for human-readable output).

There is no in-memory cache; every storage method call performs a full file read and (if mutating) a full file write. This keeps concurrency semantics simple at the cost of I/O efficiency, which is acceptable for CLI use.

---

## Dependency and Ownership Rules (Enforced in Commands)

These constraints are enforced in the individual command files in `internal/commands/`, not in the storage layer:

- A task cannot be marked `done` if it has undone descendants (`HasUndoneChildren`, see `storage.go:418`).
- A task cannot be set to `in-progress` if it is blocked (`IsBlocked`, see `storage.go:608`).
- Children cannot be added to a `done` task.
- When a task is marked `done`, `RemoveFromAllBlockedBy` removes it from all other tasks' `BlockedBy` lists (see `storage.go:666`).
- `WouldCreateCycle` uses BFS over the `BlockedBy` graph to detect dependency cycles before adding a new `block` edge (see `storage.go:628`).
- `claim` fails if `Owner` is already set; `--force` overrides.
- `delete` calls `OrphanChildren` to set `Parent = nil` on direct children before removing the task (see `storage.go:441`).
