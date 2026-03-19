# Command Reference

All limbo commands output JSON by default for easy machine parsing. Pass `--pretty` to any command for human-readable, colored output.

Task IDs are 4-character lowercase alphabetic strings (e.g., `abcd`). IDs are case-insensitive — `ABCD` and `abcd` refer to the same task.

---

## Setup

### `limbo init`

Initialize limbo in the current directory. Creates `.limbo/tasks.json`.

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

Add a new task with the given name. New tasks start with status `todo`.

**Usage**

```
limbo add <name> [flags]
```

**Flags**

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--action` | | *(required)* | What concrete work to perform |
| `--verify` | | *(required)* | How to confirm the action succeeded |
| `--result` | | *(required)* | Template for what to report back |
| `--description` | `-d` | `""` | Task description |
| `--parent` | | `""` | Parent task ID |
| `--pretty` | | `false` | Human-readable output |

**Output (JSON)**

Outputs only the new task's ID as a plain string (not JSON).

```
abcd
```

**Constraints and errors**

- `--action`, `--verify`, and `--result` are required.
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
| `--action` | | `""` | What concrete work to perform |
| `--verify` | | `""` | How to confirm the action succeeded |
| `--result` | | `""` | Template for what to report back |
| `--pretty` | | `false` | Human-readable output showing the updated task |

**Output (JSON)**

```json
{"id": "abcd", "updated": "2024-01-15T10:30:00.000000-04:00"}
```

**Constraints and errors**

- At least one editable flag must be specified (error if no flags provided).
- Task must exist.
- Fields not specified in flags are left unchanged.
- Non-editable fields (status, parent, blockedBy, owner, notes, created) are preserved. Use dedicated commands (`status`, `parent`/`unparent`, `block`/`unblock`, `claim`/`unclaim`, `note`) for those.

---

### `limbo status <id> <status>`

Update the status of a task.

**Usage**

```
limbo status <id> <status> [flags]
```

Valid values for `<status>`: `todo`, `in-progress`, `done`.

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--outcome` | `""` | Actual result to record when marking done |
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"id": "abcd", "status": "done"}
```

**Constraints and errors**

- Cannot set a task to `in-progress` if it has incomplete blockers (tasks in its `blockedBy` list that are not `done`).
- Cannot set a task to `done` if it has children that are not `done`.
- When a task is marked `done`, it is automatically removed from the `blockedBy` list of all other tasks.
- Structured tasks (those with `action`, `verify`, and `result` all set) require `--outcome` when marking `done`.

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

Remove all completed tasks. Only deletes tasks with status `done` that have no undone children.

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
{"deleted": ["abcd", "efgh"], "count": 2}
```

If there are no tasks to prune: `{"deleted": [], "count": 0}`.

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
| `--status` | `-s` | `""` | Filter by status: `todo`, `in-progress`, or `done` |
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

Pretty mode (default): renders an indented tree with status labels (`[TODO]`, `[IN-PROG]`, `[DONE]`), using colors. JSON mode: returns a flat array of task objects.

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
  "action": "...",
  "verify": "...",
  "result": "...",
  "outcome": "...",
  "parent": null,
  "status": "todo",
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

When in-progress tasks exist, `next` finds the deepest in-progress task in the hierarchy, then returns its `todo` children. If there are no `todo` children, it returns `todo` siblings. It walks up the hierarchy as needed. Blocked tasks are always skipped. With `--unclaimed`, tasks that have an owner are also skipped.

---

## Dependencies

### `limbo block <blocker-id> <blocked-id>`

Add a dependency: `<blocked-id>` will wait for `<blocker-id>` to be `done` before it can be started.

**Usage**

```
limbo block <blocker-id> <blocked-id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"id": "abcd", "blockedBy": ["efgh"]}
```

**Constraints and errors**

- Both tasks must exist.
- A task cannot block itself.
- Cannot block on a task that is already `done`.
- The dependency already exists (duplicate).
- Circular dependencies are rejected (e.g., A blocks B, B blocks A).

---

### `limbo unblock <blocker-id> <blocked-id>`

Remove a dependency: remove `<blocker-id>` from `<blocked-id>`'s `blockedBy` list.

**Usage**

```
limbo unblock <blocker-id> <blocked-id> [flags]
```

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | `false` | Human-readable output |

**Output (JSON)**

```json
{"id": "abcd", "blockedBy": []}
```

**Errors**

- `<blocked-id>` is not currently blocked by `<blocker-id>`.

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
| `--status` | `""` | Filter by status: `todo`, `in-progress`, or `done` |
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

## Visibility Rules

By default, `list`, `tree`, and `watch` hide done tasks that have no remaining active work. Specifically, a done task is hidden unless its parent exists and is itself not done (i.e., it is a completed subtask of an ongoing parent task).

Pass `--show-all` to any of these commands to display all tasks regardless of status.
