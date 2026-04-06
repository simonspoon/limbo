# Data Model

This document is a precise reference for the data structures used by limbo. All types are defined in Go source; JSON representations are what appears in `.limbo/tasks.json` and in command output.

## Source Files

| Type | Source file |
|------|-------------|
| `Task`, `Note`, status constants | `internal/models/task.go` |
| `TaskStore`, `NextResult` | `internal/storage/storage.go` |
| `WatchEvent` | `internal/commands/watch.go` |

---

## Task

Defined in `internal/models/task.go`.

```go
type Task struct {
    ID                 string         `json:"id"`
    Name               string         `json:"name"`
    Description        string         `json:"description,omitempty"`
    Approach           string         `json:"approach,omitempty"`
    Verify             string         `json:"verify,omitempty"`
    Result             string         `json:"result,omitempty"`
    Outcome            string         `json:"outcome,omitempty"`
    AcceptanceCriteria string         `json:"acceptanceCriteria,omitempty"`
    ScopeOut           string         `json:"scopeOut,omitempty"`
    AffectedAreas      string         `json:"affectedAreas,omitempty"`
    TestStrategy       string         `json:"testStrategy,omitempty"`
    Risks              string         `json:"risks,omitempty"`
    Report             string         `json:"report,omitempty"`
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
```

| Field | Go type | JSON tag | Description |
|-------|---------|----------|-------------|
| `ID` | `string` | `"id"` | 4-character lowercase alphabetic string (e.g. `"abcd"`). Generated via `crypto/rand`. User input is normalized to lowercase via `NormalizeTaskID`. |
| `Name` | `string` | `"name"` | Task title. Required. |
| `Description` | `string` | `"description,omitempty"` | Optional free-text details. Omitted from JSON when empty. |
| `Approach` | `string` | `"approach,omitempty"` | Chosen solution path — what to do and why this approach. Replaces the former `Action` field. |
| `Verify` | `string` | `"verify,omitempty"` | Runnable commands or steps to confirm work succeeded. |
| `Result` | `string` | `"result,omitempty"` | Template for what to report back when done. |
| `Outcome` | `string` | `"outcome,omitempty"` | Actual result reported when marking `done`. Set via `limbo status --outcome`. |
| `AcceptanceCriteria` | `string` | `"acceptanceCriteria,omitempty"` | Observable conditions that define "done" in human terms. Required at `captured → refined` gate. |
| `ScopeOut` | `string` | `"scopeOut,omitempty"` | Explicitly excluded work. Required at `captured → refined` gate. |
| `AffectedAreas` | `string` | `"affectedAreas,omitempty"` | Files, modules, dependencies involved. Required at `refined → planned` gate. |
| `TestStrategy` | `string` | `"testStrategy,omitempty"` | What tests exist, what new tests are needed. Required at `refined → planned` gate. |
| `Risks` | `string` | `"risks,omitempty"` | What could go wrong; assumptions being made. Required at `refined → planned` gate. |
| `Report` | `string` | `"report,omitempty"` | Executor's summary of what changed and caveats. Required at `in-progress → in-review` gate. |
| `Parent` | `*string` | `"parent"` | Pointer to the parent task's ID. `null` in JSON means the task is a root task. Always present in JSON (not omitempty). |
| `Status` | `string` | `"status"` | Lifecycle stage. One of: `"captured"`, `"refined"`, `"planned"`, `"ready"`, `"in-progress"`, `"in-review"`, `"done"`. |
| `BlockedBy` | `[]string` | `"blockedBy,omitempty"` | List of task IDs that must reach `"done"` before this task can be started. Omitted from JSON when empty. |
| `Owner` | `*string` | `"owner,omitempty"` | Agent name that has claimed this task. `null` / omitted when unclaimed. |
| `Notes` | `[]Note` | `"notes,omitempty"` | Append-only list of timestamped observations. Omitted from JSON when empty. |
| `History` | `[]HistoryEntry` | `"history,omitempty"` | Audit trail of stage transitions. Stored in the JSON index, not context files. |
| `ManualBlockReason` | `string` | `"manualBlockReason,omitempty"` | Reason for manual block. Non-empty means the task is manually blocked. Stored in JSON index. |
| `BlockedFromStage` | `string` | `"blockedFromStage,omitempty"` | Stage the task was in when manually blocked. Restored on unblock. Stored in JSON index. |
| `Created` | `time.Time` | `"created"` | Creation timestamp. Serialized as RFC3339Nano. |
| `Updated` | `time.Time` | `"updated"` | Last-modified timestamp. Serialized as RFC3339Nano. |

### HasStructuredFields

```go
func (t *Task) HasStructuredFields() bool
```

Returns `true` when `Approach`, `Verify`, and `Result` are all non-empty. This is a utility method on the model; limbo does not use it for enforcement. External tools may use it to distinguish structured tasks from unstructured ones.

---

## Note

Defined in `internal/models/task.go`.

```go
type Note struct {
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}
```

| Field | Go type | JSON tag | Description |
|-------|---------|----------|-------------|
| `Content` | `string` | `"content"` | The text of the note. |
| `Timestamp` | `time.Time` | `"timestamp"` | When the note was added. Serialized as RFC3339Nano. |

Notes are append-only. Existing notes are never modified or deleted.

---

## HistoryEntry

Defined in `internal/models/task.go`.

```go
type HistoryEntry struct {
    From   string    `json:"from"`
    To     string    `json:"to"`
    By     string    `json:"by,omitempty"`
    At     time.Time `json:"at"`
    Reason string    `json:"reason,omitempty"`
}
```

| Field | Go type | JSON tag | Description |
|-------|---------|----------|-------------|
| `From` | `string` | `"from"` | Stage before the transition. For manual block events, this is the pre-block stage. |
| `To` | `string` | `"to"` | Stage after the transition. `"blocked"` for manual block events. |
| `By` | `string` | `"by,omitempty"` | Who triggered the transition (e.g., agent name). Set via `--by` flag. |
| `At` | `time.Time` | `"at"` | When the transition occurred. Serialized as RFC3339Nano. |
| `Reason` | `string` | `"reason,omitempty"` | Required on backward transitions and manual blocks. |

History entries are recorded automatically on every `limbo status` transition and on manual block/unblock events. They are stored in the JSON index alongside the task metadata, not in context files.

---

## Status Constants and Lifecycle

Defined in `internal/models/task.go`.

```go
const (
    StatusCaptured   = "captured"
    StatusRefined    = "refined"
    StatusPlanned    = "planned"
    StatusReady      = "ready"
    StatusInProgress = "in-progress"
    StatusInReview   = "in-review"
    StatusDone       = "done"
)

var StageOrder = []string{
    StatusCaptured, StatusRefined, StatusPlanned, StatusReady,
    StatusInProgress, StatusInReview, StatusDone,
}
```

| Constant | Value | Meaning |
|----------|-------|---------|
| `StatusCaptured` | `"captured"` | Raw idea captured. Default status for new tasks. |
| `StatusRefined` | `"refined"` | Acceptance criteria and scope defined. |
| `StatusPlanned` | `"planned"` | Approach, affected areas, test strategy, and risks documented. |
| `StatusReady` | `"ready"` | Verify steps validated as concrete. Ready for execution. |
| `StatusInProgress` | `"in-progress"` | Work is actively underway (task must be claimed). |
| `StatusInReview` | `"in-review"` | Implementation complete, under independent verification. |
| `StatusDone` | `"done"` | Work is complete and verified. |

### Status Transitions

limbo is a pure task store -- status transitions have no field requirements. Any valid status can be set at any time, in any direction. The only constraint is that manually blocked tasks cannot transition until unblocked.

Workflow rules (e.g., requiring specific fields before advancing stages) are enforced by the orchestrator/agent layer, not by limbo.

### Manual Block

Manual block is an overlay — not a lifecycle stage. When a task is manually blocked (`limbo block <id> --reason "..."`), `ManualBlockReason` is set and `BlockedFromStage` records the current stage. All status transitions are rejected until unblocked. On unblock, the task's status is restored from `BlockedFromStage`.

### Helper Functions

`StageIndex(status string) int` — returns the index in `StageOrder` (0-6), or -1 for invalid statuses.

`IsValidStatus(status string) bool` — returns true if the status is one of the 7 valid stages.

---

## TaskStore

Defined in `internal/storage/storage.go`. This is the root object of `.limbo/tasks.json`.

```go
type TaskStore struct {
    Version string        `json:"version"`
    Tasks   []models.Task `json:"tasks"`
}
```

| Field | Go type | JSON tag | Description |
|-------|---------|----------|-------------|
| `Version` | `string` | `"version"` | Schema version. Currently `"6.0.0"`. |
| `Tasks` | `[]models.Task` | `"tasks"` | Flat list of all tasks. Content fields (Description, Approach, Verify, Result, Outcome, AcceptanceCriteria, ScopeOut, AffectedAreas, TestStrategy, Risks, Report, Notes) are empty in the JSON index — they are stored in per-task context files. Metadata fields (History, ManualBlockReason, BlockedFromStage) stay in the JSON index. |

**Migration:** On load, older stores are migrated automatically: v2→v4 (int64 to string IDs), v3→v4 (add structured fields), v4→v5 (split content into context files, Action→Approach rename), v5→v6 (7-stage lifecycle, `todo`→`captured` status mapping). All migrations chain. A backup is created before each step (`.bak`, `.v3.bak`, `.v4.bak`, `.v5.bak`).

**Archive:** The `prune` command moves completed tasks to `.limbo/archive.json`, which uses the same `TaskStore` format. The archive file is created lazily on first prune (not by `limbo init`). `GenerateTaskID` checks both `tasks.json` and `archive.json` for ID collisions, so archived task IDs are never reused.

---

## NextResult

Defined in `internal/storage/storage.go`. Returned by the `next` command.

```go
type NextResult struct {
    Task         *models.Task  `json:"task,omitempty"`
    Candidates   []models.Task `json:"candidates,omitempty"`
    BlockedCount int           `json:"blockedCount,omitempty"`
}
```

| Field | Go type | JSON tag | Description |
|-------|---------|----------|-------------|
| `Task` | `*models.Task` | `"task,omitempty"` | The single recommended next task. Present when an in-progress task provides context and a specific next step is identified. |
| `Candidates` | `[]models.Task` | `"candidates,omitempty"` | List of candidate tasks when there is no in-progress context to narrow the choice. |
| `BlockedCount` | `int` | `"blockedCount,omitempty"` | Number of tasks skipped because all of their blockers are incomplete. Present when nothing is available. |

Exactly one of `Task` or `Candidates` will be populated in a successful response. `BlockedCount` supplements either field when applicable.

---

## WatchEvent

Defined in `internal/commands/watch.go`. Emitted to stdout by `limbo watch` (JSON mode) whenever the task store changes.

```go
type WatchEvent struct {
    Type      string        `json:"type"`
    Task      *models.Task  `json:"task,omitempty"`
    Tasks     []models.Task `json:"tasks,omitempty"`
    TaskID    string        `json:"taskId,omitempty"`
    Timestamp time.Time     `json:"timestamp"`
}
```

| Field | Go type | JSON tag | Description |
|-------|---------|----------|-------------|
| `Type` | `string` | `"type"` | Event kind. One of `"snapshot"`, `"added"`, `"updated"`, `"deleted"`. |
| `Task` | `*models.Task` | `"task,omitempty"` | The affected task. Present for `"added"` and `"updated"` events. |
| `Tasks` | `[]models.Task` | `"tasks,omitempty"` | Full task list at the time of the snapshot. Present for `"snapshot"` events only. |
| `TaskID` | `string` | `"taskId,omitempty"` | ID of the deleted task. Present for `"deleted"` events only. |
| `Timestamp` | `time.Time` | `"timestamp"` | When the event was emitted. Serialized as RFC3339Nano. |

### WatchEvent type values

| `type` value | When emitted | Populated fields |
|--------------|--------------|-----------------|
| `"snapshot"` | First tick after `watch` starts | `Tasks`, `Timestamp` |
| `"added"` | A task that did not exist in the previous tick now exists | `Task`, `Timestamp` |
| `"updated"` | A task existed in the previous tick and its data has changed | `Task`, `Timestamp` |
| `"deleted"` | A task that existed in the previous tick no longer exists | `TaskID`, `Timestamp` |

---

## tasks.json Example (v6)

In v6, `tasks.json` contains metadata plus structured operational fields (History, ManualBlock). Content fields are stored in context files.

```json
{
  "version": "6.0.0",
  "tasks": [
    {
      "id": "abcd",
      "name": "Implement authentication",
      "parent": null,
      "status": "in-progress",
      "owner": "agent-1",
      "history": [
        {"from": "captured", "to": "refined", "by": "pm", "at": "2026-02-20T09:30:00Z"},
        {"from": "refined", "to": "planned", "by": "pm", "at": "2026-02-20T09:45:00Z"},
        {"from": "planned", "to": "ready", "by": "pm", "at": "2026-02-20T09:50:00Z"},
        {"from": "ready", "to": "in-progress", "by": "tl", "at": "2026-02-20T10:00:00Z"}
      ],
      "created": "2026-02-20T09:00:00.000000000Z",
      "updated": "2026-02-20T10:00:00.000000000Z"
    },
    {
      "id": "efgh",
      "name": "Write login handler",
      "parent": "abcd",
      "status": "captured",
      "created": "2026-02-20T09:01:00.000000000Z",
      "updated": "2026-02-20T09:01:00.000000000Z"
    },
    {
      "id": "ijkl",
      "name": "Write token refresh handler",
      "parent": "abcd",
      "status": "captured",
      "blockedBy": ["efgh"],
      "created": "2026-02-20T09:02:00.000000000Z",
      "updated": "2026-02-20T09:02:00.000000000Z"
    }
  ]
}
```

The corresponding content lives in context files. For example, `.limbo/context/abcd/context.md`:

```markdown
## Approach
Implement JWT login and token refresh endpoints

## Verify
Run integration tests: go test ./...

## Result
List endpoints added and test results

## AcceptanceCriteria
Login returns JWT, refresh rotates tokens, invalid tokens return 401

## ScopeOut
No OAuth2 or social login

## AffectedAreas
internal/auth/, internal/handlers/

## TestStrategy
Unit tests for token generation, integration tests for endpoints

## Risks
Token rotation edge cases under concurrent requests

## Description
Add JWT-based login and token refresh

## Notes
### 2026-02-20T10:00:00Z
Started with login endpoint
```

Notes on the example:

- `"parent": null` serializes as a JSON `null` (the field is never omitted because the struct tag has no `omitempty`).
- Content fields (description, approach, verify, result, outcome, acceptance criteria, scope out, affected areas, test strategy, risks, report, notes) are absent from the JSON — they live in context files.
- Metadata fields (`history`, `manualBlockReason`, `blockedFromStage`) stay in the JSON index for fast queries.
- Fields with `omitempty` (`blockedBy`, `owner`, `history`) are absent when empty, as shown for `efgh`.
- `blockedBy` for `ijkl` means `efgh` must reach `"done"` before `ijkl` can be started.
- Timestamps use RFC3339Nano format in JSON, RFC3339 in context files.
