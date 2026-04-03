# Command Reference

All limbo commands output JSON by default for easy machine parsing. Pass `--pretty` to any command for human-readable, colored output.

Task IDs are 4-character lowercase alphabetic strings (e.g., `abcd`). IDs are case-insensitive — `ABCD` and `abcd` refer to the same task.

## Global Flags

These flags are available on every command:

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output (default varies per command) |

---

## Setup

### `limbo init`

Initialize limbo in the current directory. Creates `.limbo/` with `tasks.json` and `context/` directory.

**Usage**

```
limbo init [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"success": true, "path": "/path/to/directory"}
```

**Errors**

- `.limbo/` already exists in the current directory.

---

## Task Management

### `limbo add <name>`

Add a new task with the given name. New tasks start with status `captured`.

**Usage**

```
limbo add <name> [flags]
```

**Flags**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--approach` | | `""` | Chosen solution path — what to do and why |
| `--action` | | `""` | Hidden alias for `--approach` (backward compat) |
| `--verify` | | `""` | How to confirm the work succeeded |
| `--result` | | `""` | Template for what to report back |
| `--acceptance-criteria` | | `""` | Observable conditions that define "done" |
| `--scope-out` | | `""` | What is explicitly out of scope |
| `--affected-areas` | | `""` | Areas of the codebase affected |
| `--test-strategy` | | `""` | How to test the changes |
| `--risks` | | `""` | Known risks and mitigations |
| `--report` | | `""` | Completion report |
| `--description` | `-d` | `""` | Task description |
| `--parent` | | `""` | Parent task ID |
| `--pretty` | | `false` | Human-readable output |

**Output (JSON)**

Outputs only the new task's ID as a plain string (not JSON).

```
abcd
```

**Constraints and errors**

- `--parent` must refer to an existing task.
- Cannot add a child to a task with status `done`.

---

### `limbo edit <id>`

Modify an existing task's mutable fields. Only specified flags are updated (partial update).

**Usage**

```
limbo edit <id> [flags]
```

**Flags**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--name` | | `""` | Task name |
| `--description` | `-d` | `""` | Task description |
| `--approach` | | `""` | Chosen solution path |
| `--action` | | `""` | Hidden alias for `--approach` (backward compat) |
| `--verify` | | `""` | How to confirm the work succeeded |
| `--result` | | `""` | Template for what to report back |
| `--acceptance-criteria` | | `""` | Observable conditions that define "done" |
| `--scope-out` | | `""` | What is explicitly out of scope |
| `--affected-areas` | | `""` | Areas of the codebase affected |
| `--test-strategy` | | `""` | How to test the changes |
| `--risks` | | `""` | Known risks and mitigations |
| `--report` | | `""` | Completion report |
| `--pretty` | | `false` | Human-readable output showing the updated task |

**Output (JSON)**

```json
{"id": "abcd", "updated": "2024-01-15T10:30:00.000000-04:00"}
```

**Constraints and errors**

- At least one editable flag must be specified (error if no flags provided).
- Task must exist.
- Fields not specified in flags are left unchanged.
- Non-editable fields (status, parent, blockedBy, owner, notes, history, created) are preserved. Use dedicated commands (`status`, `parent`/`unparent`, `block`/`unblock`, `claim`/`unclaim`, `note`) for those.

---

### `limbo status <id> <status>`

Update the status of a task.

**Usage**

```
limbo status <id> <status> [flags]
```

Valid values for `<status>`: `captured`, `refined`, `planned`, `ready`, `in-progress`, `in-review`, `done`.

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--outcome` | `""` | Actual result to record when marking done |
| `--reason` | `""` | Required for backward transitions (e.g., refined → captured) |
| `--by` | `""` | Who triggered the transition (recorded in history) |
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"id": "abcd", "status": "done"}
```

**Gate validation**

Forward transitions enforce required fields at each step:

| Transition | Required fields |
|-----------|----------------|
| captured → refined | `acceptance_criteria`, `scope_out` |
| refined → planned | `approach`, `affected_areas`, `test_strategy`, `risks` |
| planned → ready | `verify` |
| ready → in-progress | task must be claimed (has owner) |
| in-progress → in-review | `report` |
| in-review → done | `outcome` |

Multi-stage jumps (e.g., captured → planned) validate all intermediate gates. If any gate fails, the transition is rejected with a message listing the missing fields.

**Constraints and errors**

- Forward transitions are the default. Backward transitions require `--reason`.
- Manually blocked tasks cannot transition until unblocked.
- Cannot transition to `in-progress` if dependency-blocked (`blockedBy` contains incomplete tasks).
- Cannot mark `done` if the task has undone children.
- When marked `done`, the task is auto-removed from all other tasks' `blockedBy` lists.
- Structured tasks (those with `approach`, `verify`, and `result` all set) require `--outcome` when marking `done`.
- Every transition records a `HistoryEntry` (from, to, by, at, reason).

---

### `limbo delete <id>`

Delete a task.

**Usage**

```
limbo delete <id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"success": true, "id": "abcd"}
```

**Constraints and errors**

- Cannot delete a task that has undone children. Complete or delete children first.
- Children of the deleted task are orphaned: their `parent` field is set to `null`.
- The deleted task is automatically removed from the `blockedBy` list of all other tasks.

---

### `limbo prune`

Archive all completed tasks. Moves tasks with status `done` that have no undone children to `.limbo/archive.json`. Use `limbo archive` subcommands to manage archived tasks.

**Usage**

```
limbo prune [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"archived": ["abcd", "efgh"], "count": 2}
```

If there are no tasks to prune: `{"archived": [], "count": 0}`.

---

### `limbo parent <id> <parent-id>`

Set the parent of a task, creating a hierarchical relationship. The task becomes a child of the specified parent.

**Usage**

```
limbo parent <id> <parent-id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"id": "abcd", "parent": "efgh"}
```

**Constraints and errors**

- Both tasks must exist.
- Cannot set a task as its own parent.
- Cannot use a `done` task as a parent.
- Circular parent chains are rejected (e.g., A → B → A).

---

### `limbo unparent <id>`

Remove a task's parent, making it a top-level task.

**Usage**

```
limbo unparent <id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"id": "abcd", "parent": null}
```

**Notes**

- If the task is already a top-level task (no parent), the command succeeds without error.

---

## Search

### `limbo search <query>`

Search tasks by matching a query string against task name and description (case-insensitive substring match).

**Usage**

```
limbo search <query> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--show-all` | `false` | Show all tasks including completed |
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

Returns a JSON array of matching task objects, sorted by creation time. Returns `null` if no matches.

**Notes**

- By default, completed tasks are hidden (same visibility rules as `list`). Use `--show-all` to include them.

---

## Viewing

### `limbo list`

List tasks with optional filtering.

**Usage**

```
limbo list [flags]
```

**Flags**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--status` | `-s` | `""` | Filter by status: `captured`, `refined`, `planned`, `ready`, `in-progress`, `in-review`, or `done` |
| `--owner` | | `""` | Show only tasks owned by this agent name |
| `--unclaimed` | | `false` | Show only tasks with no owner |
| `--blocked` | | `false` | Show only blocked tasks |
| `--unblocked` | | `false` | Show only unblocked tasks |
| `--show-all` | | `false` | Show all tasks, including completed |
| `--pretty` | | `false` | Human-readable output grouped by status |

**Output (JSON)**

Returns a JSON array of task objects, sorted by creation time.

**Mutually exclusive flags**

- `--owner` and `--unclaimed` cannot be used together.
- `--blocked` and `--unblocked` cannot be used together.

**Visibility**

By default, "fully resolved" done tasks are hidden. See the [Visibility Rules](#visibility-rules) section.

---

### `limbo tree`

Display tasks as a hierarchical tree showing parent-child relationships.

**Usage**

```
limbo tree [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `true` | Human-readable tree output (default is `true` for this command) |
| `--show-all` | `false` | Show all tasks, including completed |

**Output**

Pretty mode (default): renders an indented tree with status labels (`[CAPTURED]`, `[REFINED]`, `[PLANNED]`, `[READY]`, `[IN-PROG]`, `[REVIEW]`, `[DONE]`), using colors. JSON mode: returns a flat array of task objects.

**Visibility**

By default, "fully resolved" done tasks are hidden. See the [Visibility Rules](#visibility-rules) section.

---

### `limbo show <id>`

Display detailed information about a single task, including its blockers and the tasks it blocks.

**Usage**

```
limbo show <id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output including notes |

**Output (JSON)**

```json
{
  "id": "abcd",
  "name": "My task",
  "description": "...",
  "approach": "...",
  "verify": "...",
  "result": "...",
  "outcome": "...",
  "acceptanceCriteria": "...",
  "scopeOut": "...",
  "affectedAreas": "...",
  "testStrategy": "...",
  "risks": "...",
  "report": "...",
  "parent": null,
  "status": "captured",
  "history": [...],
  "blockedBy": ["efgh"],
  "owner": null,
  "notes": [...],
  "created": "...",
  "updated": "...",
  "blockers": [{"id": "efgh", "name": "Other task", "status": "in-progress"}],
  "blocks": []
}
```

The `blockers` field resolves each ID in `blockedBy` to `{id, name, status}`. The `blocks` field is the reverse: tasks that depend on this task.

---

### `limbo next`

Return the next task to work on, using depth-first traversal.

**Usage**

```
limbo next [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--unclaimed` | `false` | Skip tasks that have an owner |
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

The output shape varies depending on context:

- When in-progress tasks exist, returns the narrowed next task:
  ```json
  {"task": { ...task object... }}
  ```

- When no in-progress tasks exist, returns a list of root-level candidates:
  ```json
  {"candidates": [ ...task objects... ]}
  ```

- When all remaining tasks are blocked:
  ```json
  {"blockedCount": 3}
  ```

**Traversal behavior**

When in-progress tasks exist, `next` finds the deepest in-progress task in the hierarchy, then returns its `ready` children. If there are no `ready` children, it returns `ready` siblings. It walks up the hierarchy as needed. Blocked tasks are always skipped. With `--unclaimed`, tasks that have an owner are also skipped.

Note: `next` only surfaces tasks in the `ready` stage — tasks must pass through the planning stages (captured → refined → planned → ready) before they appear in `next` results.

---

## Dependencies

### `limbo block`

Two modes: **dependency block** (2 args) and **manual block** (1 arg).

**Dependency block: `limbo block <blocker-id> <blocked-id>`**

Add a dependency: `<blocked-id>` will wait for `<blocker-id>` to be `done` before it can be started.

```
limbo block <blocker-id> <blocked-id> [flags]
```

**Manual block: `limbo block <id> --reason "..."`**

Manually block a task. The task's current stage is saved and all status transitions are rejected until unblocked. Records a HistoryEntry.

```
limbo block <id> --reason "waiting on design review" [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--reason` | `""` | Reason for manual block (required for 1-arg mode) |
| `--by` | `""` | Who blocked the task |
| `--pretty` | `false` | Human-readable output |

**Constraints and errors**

- Dependency block: both tasks must exist, no self-block, no blocking on done tasks, no duplicates, no cycles.
- Manual block: task must exist, `--reason` required, cannot block an already manually blocked task.

---

### `limbo unblock`

Two modes: **remove dependency** (2 args) and **remove manual block** (1 arg).

**Remove dependency: `limbo unblock <blocker-id> <blocked-id>`**

Remove `<blocker-id>` from `<blocked-id>`'s `blockedBy` list.

```
limbo unblock <blocker-id> <blocked-id> [flags]
```

**Remove manual block: `limbo unblock <id>`**

Remove manual block and restore the task to its previous stage. Records a HistoryEntry.

```
limbo unblock <id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--by` | `""` | Who unblocked the task |
| `--pretty` | `false` | Human-readable output |

**Errors**

- Dependency unblock: task is not blocked by the specified blocker.
- Manual unblock: task is not manually blocked.

---

## Ownership

### `limbo claim <id> <agent-name>`

Set the owner of a task to the specified agent name.

**Usage**

```
limbo claim <id> <agent-name> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | `false` | Override existing owner |
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"id": "abcd", "owner": "agent-1"}
```

**Constraints and errors**

- If the task is already owned by a different agent, the command fails unless `--force` is passed.
- Claiming a task you already own (same agent name) succeeds without `--force`.

---

### `limbo unclaim <id>`

Remove the owner from a task.

**Usage**

```
limbo unclaim <id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"id": "abcd", "owner": null}
```

**Errors**

- Task has no owner.

---

## Notes

### `limbo note <id> <message>`

Append a timestamped note to a task. Notes are append-only and cannot be edited or deleted.

**Usage**

```
limbo note <id> <message> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"id": "abcd", "noteCount": 2}
```

**Errors**

- Message cannot be empty.
- Task not found.

---

## Watch

### `limbo watch`

Continuously monitor tasks and display updates. Press `q` or `Ctrl+C` to exit.

**Usage**

```
limbo watch [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--interval` | `500ms` | Polling interval (e.g., `1s`, `200ms`) |
| `--status` | `""` | Filter by status (any valid lifecycle stage) |
| `--show-all` | `false` | Show all tasks, including completed |
| `--pretty` | `false` | Human-readable output: clears screen and redraws hierarchical tree |

**Output (JSON mode)**

In JSON mode, the first tick emits a `snapshot` event containing all current tasks. Subsequent ticks emit change events for tasks that were added, updated, or deleted.

Event types:

| Type | Fields |
|------|--------|
| `snapshot` | `type`, `tasks`, `timestamp` |
| `added` | `type`, `task`, `timestamp` |
| `updated` | `type`, `task`, `timestamp` |
| `deleted` | `type`, `taskId`, `timestamp` |

Example events:

```json
{"type":"snapshot","tasks":[...],"timestamp":"..."}
{"type":"added","task":{...},"timestamp":"..."}
{"type":"updated","task":{...},"timestamp":"..."}
{"type":"deleted","taskId":"abcd","timestamp":"..."}
```

**Output (pretty mode)**

Clears the terminal screen on each tick and redraws the task hierarchy as a tree (same format as `limbo tree --pretty`). A header shows the current time and a count of tasks by status. Press `q` or `Ctrl+C` to exit.

**Visibility**

By default, "fully resolved" done tasks are hidden. See the [Visibility Rules](#visibility-rules) section.

---

## Templates

### `limbo template list`

List all available templates (built-in and project-local).

**Usage**

```
limbo template list [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
[{"name": "bug-fix", "description": "..."}, {"name": "feature", "description": "..."}]
```

**Built-in templates:** `bug-fix`, `feature`, `swe-full-cycle`.

---

### `limbo template show <name>`

Display the task hierarchy a template would create without actually creating any tasks.

**Usage**

```
limbo template show <name> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable tree output |

**Output (JSON)**

Returns the full template definition with task hierarchy.

---

### `limbo template apply <name>`

Apply a template, creating all tasks with parent/child relationships and block dependencies.

**Usage**

```
limbo template apply <name> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--parent` | `""` | Parent task ID to nest the template's tasks under |
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"createdIDs": ["abcd", "efgh", "ijkl", ...]}
```

**Notes**

- `--pretty` is a persistent flag — it applies to all `template` subcommands.
- Templates define task hierarchies with dependencies pre-wired. Use `template show` to preview before applying.

---

## Data Portability

### `limbo export`

Export all tasks as JSON to stdout. Pipe to a file for backup or transfer between projects.

**Usage**

```
limbo export
```

**Output (JSON)**

```json
{"version": "6.0.0", "tasks": [...]}
```

The output includes full task data (content fields merged from context files). This can be imported with `limbo import`.

---

### `limbo import <file>`

Import tasks from a JSON file previously created by `limbo export`.

**Usage**

```
limbo import <file> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--replace` | `false` | Clear all existing tasks before importing |

**Output (JSON)**

```json
{"imported": 5, "mode": "merge"}
```

**Behavior**

- **Merge mode (default):** Imported tasks are added alongside existing tasks. All task IDs are remapped to avoid conflicts. Parent and blocker references within the imported set are remapped accordingly.
- **Replace mode (`--replace`):** All existing tasks are deleted before importing.
- References to tasks outside the imported set (e.g., a parent ID not in the import file) are dropped.

---

## Archive

### `limbo archive list`

List all archived tasks.

**Usage**

```
limbo archive list [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

Returns a JSON array of archived task objects, sorted by creation time. Returns `[]` if the archive is empty.

---

### `limbo archive show <id>`

Display detailed information about an archived task.

**Usage**

```
limbo archive show <id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

Returns the full task object (same shape as `limbo show`).

**Errors**

- Task not found in archive.

---

### `limbo archive restore <id>`

Move an archived task back to the active store with status `done`.

**Usage**

```
limbo archive restore <id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"restored": "abcd", "warning": ""}
```

The `warning` field is populated when stale references were cleaned up during restore (e.g., orphaned parent pointer, removed invalid blockers).

**Constraints and errors**

- Fails if the task ID already exists in the active store.
- Stale `BlockedBy` references (IDs not in the active store) are removed.
- If the task's `Parent` no longer exists in the active store, the parent pointer is cleared (task becomes a root task).

---

### `limbo archive purge`

Permanently delete all archived tasks. This cannot be undone.

**Usage**

```
limbo archive purge [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"purged": 5}
```

---

## Visibility Rules

By default, `list`, `tree`, and `watch` hide done tasks that have no remaining active work. Specifically, a done task is hidden unless its parent exists and is itself not done (i.e., it is a completed subtask of an ongoing parent task).

Pass `--show-all` to any of these commands to display all tasks regardless of status.
