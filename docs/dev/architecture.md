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

`init`, `add`, `list`, `show`, `status`, `delete`, `edit`, `parent`, `unparent`, `tree`, `next`, `prune`, `watch`, `block`, `unblock`, `note`, `claim`, `unclaim`, `search`, `template`, `export`, `import`, `archive`

All commands follow the same pattern: call `getStorage()` (defined in `root.go`), perform operations on the returned `*Storage`, then print JSON by default or human-readable output when `--pretty` is passed.

`getStorage()` is a thin wrapper that delegates to `storage.NewStorage()`.

See `internal/commands/root.go` for the `init()` function that wires all subcommands to `rootCmd`.

### internal/models/task.go

Defines the two core structs and the three valid status constants:

```go
type Note struct {
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}

type Task struct {
    ID                 string         `json:"id"`
    Name               string         `json:"name"`
    Description        string         `json:"description,omitempty"`
    Approach           string         `json:"approach,omitempty"`
    // ... content fields (Verify, Result, Outcome, AcceptanceCriteria,
    //     ScopeOut, AffectedAreas, TestStrategy, Risks, Report)
    Parent             *string        `json:"parent"`
    Status             string         `json:"status"`
    BlockedBy          []string       `json:"blockedBy,omitempty"`
    Owner              *string        `json:"owner,omitempty"`
    Notes              []Note         `json:"notes,omitempty"`
    History            []HistoryEntry `json:"history,omitempty"`
    ManualBlockReason  string         `json:"manualBlockReason,omitempty"`
    BlockedFromStage   string         `json:"blockedFromStage,omitempty"`
    Created            time.Time      `json:"created"`
    Updated            time.Time      `json:"updated"`
}

const (
    StatusCaptured   = "captured"
    StatusRefined    = "refined"
    StatusPlanned    = "planned"
    StatusReady      = "ready"
    StatusInProgress = "in-progress"
    StatusInReview   = "in-review"
    StatusDone       = "done"
)
```

`Parent` and `Owner` are nullable pointers so they serialize as `null` (not omitted) when unset. `BlockedBy`, `Notes`, `History`, and content fields use `omitempty` and are absent from JSON when empty.

Helper functions: `IsValidStatus`, `IsValidTaskID` (enforces 4-character lowercase alpha), `NormalizeTaskID` (lowercases input), `StageIndex` (returns position in the 7-stage lifecycle, -1 if invalid).

### internal/storage/storage.go

All business logic lives here. Commands do not manipulate task slices directly; they call methods on `*Storage`.

---

## Storage Design

### Two-Tier Layout

limbo uses a split storage model:

```
<project-root>/.limbo/
    tasks.json              # JSON index — metadata only
    context/                # Per-task content files
        <id>/
            context.md      # H2-delimited markdown (approach, verify, result, etc.)
            attachments/    # Reserved for future binary attachments
    archive.json            # Archived tasks (complete data, created by prune)
```

**JSON index (`tasks.json`)** contains fast-query metadata (id, name, status, parent, blockedBy, owner, created, updated) plus structured operational fields (history, manualBlockReason, blockedFromStage). Content fields (description, approach, verify, result, outcome, acceptanceCriteria, scopeOut, affectedAreas, testStrategy, risks, report, notes) are stored in per-task context files.

**Context files (`context/<id>/context.md`)** use H2-delimited markdown sections. Known sections are ordered: Approach, Verify, Result, Outcome, AcceptanceCriteria, ScopeOut, AffectedAreas, TestStrategy, Risks, Report, Description, then custom sections alphabetically, Notes always last. Empty sections are omitted.

The split is transparent to commands — `SaveTask` writes to both tiers, `LoadTask`/`LoadAll` merge them back into a single Task struct.

### File Location Constants

```go
const (
    LimboDir       = ".limbo"
    TasksFile      = "tasks.json"
    ArchiveFile    = "archive.json"
    ContextDirName = "context"
)
```

### TaskStore (on-disk format)

```go
type TaskStore struct {
    Version string        `json:"version"`
    Tasks   []models.Task `json:"tasks"`
}
```

The current version string is `"6.0.0"`. Tasks in the JSON index have content fields stripped (empty strings / nil), but metadata fields (`History`, `ManualBlockReason`, `BlockedFromStage`) are preserved. The file is written with `json.MarshalIndent` using two-space indentation.

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

### Core Storage Methods

**LoadAll** loads the full task list, merging content from context files:

```go
func (s *Storage) LoadAll() ([]models.Task, error)
```

**LoadAllIndex** loads tasks from the JSON index only (no context file I/O). Use for commands that only need metadata (list, tree, next):

```go
func (s *Storage) LoadAllIndex() ([]models.Task, error)
```

**LoadTask** finds a single task by ID, merges content from its context file, returns `ErrTaskNotFound` if absent:

```go
func (s *Storage) LoadTask(id string) (*models.Task, error)
```

**SaveTask** performs an upsert with two-tier split: extracts content fields into `context/<id>/context.md`, strips them from the task, then writes the metadata-only task to the JSON index. If all content fields are empty, deletes the context directory instead:

```go
func (s *Storage) SaveTask(task *models.Task) error
```

**DeleteTask** and **DeleteTasks** remove from the JSON index and delete the corresponding `context/<id>/` directory.

**loadStore** / **saveStore** are unexported helpers that handle JSON marshaling and file I/O. `loadStore` also handles schema migration on first read: v2→v4 (int64 to string IDs), v3→v4 (add structured fields), v4→v5 (split content into context files, Action→Approach), v5→v6 (7-stage lifecycle, todo→captured). All migrations chain. A backup is written before each step.

### Task ID Generation

IDs are 4-character lowercase alphabetic strings (e.g., `abcd`). `GenerateTaskID` uses `crypto/rand` to generate candidates and checks against existing IDs for uniqueness, retrying up to 100 times (see `storage.go:692`). The alphabet is `a-z` only, giving 26^4 = 456,976 possible values.

---

## Data Flow

A typical command execution follows this path:

1. Cobra dispatches to the command's `RunE` function.
2. The command calls `getStorage()`, which delegates to `storage.NewStorage()`. This auto-discovers the `.limbo/` directory by walking up from `os.Getwd()`.
3. The command calls one or more storage methods (`LoadAll`, `LoadTask`, `SaveTask`, etc.).
4. **Load path:** `loadStore` reads the JSON index from `tasks.json`. Then `mergeContext` reads `context/<id>/context.md` for each task and populates content fields. `LoadAllIndex` skips the merge step.
5. **Save path:** `extractContext` builds a section map from the task's content fields and writes it to `context/<id>/context.md`. The task copy has content fields stripped, then is written to the JSON index via `saveStore`.
6. The command marshals its result to JSON and prints to stdout (or uses `--pretty` for human-readable output).

There is no in-memory cache; every storage method call performs a full file read and (if mutating) a full file write. The two-tier split keeps the JSON index small and fast for metadata-only queries while storing potentially large content in separate files.

---

## Lifecycle, Dependency and Ownership Rules (Enforced in Commands)

These constraints are enforced in the individual command files in `internal/commands/`, not in the storage layer:

- **No gate validation**: limbo is a pure task store. Status transitions are unconditional (except for manual blocks).
- **Manually blocked tasks** (`ManualBlockReason != ""`) cannot transition at all until unblocked.
- Children cannot be added to a `done` task.
- When a task is marked `done`, `RemoveFromAllBlockedBy` removes it from all other tasks' `BlockedBy` lists.
- `WouldCreateCycle` uses BFS over the `BlockedBy` graph to detect dependency cycles before adding a new `block` edge.
- `block` with 1 arg + `--reason` creates a manual block; with 2 args creates a dependency block.
- `unblock` with 1 arg removes a manual block and restores the previous stage; with 2 args removes a dependency.
- `claim` fails if `Owner` is already set; `--force` overrides.
- `delete` calls `OrphanChildren` to set `Parent = nil` on direct children before removing the task.
- Every successful `status` transition records a `HistoryEntry` with from/to/by/at/reason.
