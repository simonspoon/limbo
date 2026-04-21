# Agent Integration Guide

limbo is a CLI task manager built specifically for LLMs and AI agents. This guide covers how to integrate limbo into agent workflows.

## Why limbo for agents

- **JSON output by default** -- every command returns machine-parseable JSON, no scraping needed
- **Pure task store** -- no workflow logic, no gate validation. Agents/orchestrators enforce their own rules
- **Ownership system** -- multiple agents can coordinate on the same task queue without conflicts
- **File-based storage** -- `.limbo/` directory with a JSON index and per-task context files, no server or daemon required

## Single-Agent Workflow

A basic agent loop: find available work, claim it, do the work, mark it done.

```bash
# Find available work
limbo list --status ready --unblocked

# Claim and start
limbo claim abcd agent-1
limbo status abcd in-progress

# Log progress
limbo note abcd "Started implementation"
limbo note abcd "Found edge case, handling it"

# Complete
limbo status abcd done --outcome "Implemented feature X; all tests pass"
```

## Multi-Agent Coordination

When multiple agents share the same task queue, use ownership and dependencies to avoid conflicts.

### Claiming tasks

```bash
# Agent picks an unclaimed task
limbo list --status ready --unblocked --unclaimed
limbo claim <id> agent-1
```

`limbo claim` fails if the task is already owned by another agent. Use `--force` to override ownership if needed.

### Dependencies

Dependencies prevent agents from starting work before prerequisites are complete.

```bash
# Mark task B as blocked by task A
limbo block <prereq-id> <dependent-id>

# The dependent won't appear in --unblocked results until the prereq is done
limbo list --status ready --unblocked  # skips blocked tasks

# When the prereq is marked done, it's auto-removed from all BlockedBy lists
limbo status <prereq-id> done
```

### Filtering

Use `list` flags to inspect the task queue:

```bash
limbo list --owner agent-1       # tasks owned by agent-1
limbo list --unclaimed            # tasks with no owner
limbo list --status in-progress   # tasks currently being worked on
limbo list --blocked              # tasks waiting on dependencies
limbo list --unblocked            # tasks ready to start
```

## Progressive Decomposition

Agents can break down large tasks into subtasks using parent/child relationships:

```bash
# Agent decomposes a broad task into subtasks
limbo claim abcd agent-1
limbo status abcd in-progress
limbo add "Design auth schema" --parent abcd \
  --approach "Design the database schema for auth" \
  --verify "Schema reviewed and approved" \
  --result "Schema file path and summary of design decisions"
limbo add "Implement login endpoint" --parent abcd \
  --approach "Implement POST /login in auth package" \
  --verify "go test ./internal/auth/... passes" \
  --result "Handler file path and passing test output"

# Find available subtasks
limbo list --status ready --unblocked
```

The orchestrator/agent layer is responsible for deciding which task to pick up next. limbo provides the data (hierarchy, status, blocking) but does not implement scheduling logic.

## Watch Mode for Orchestrators

The `watch` command monitors the `.limbo/` directory for changes and outputs updates in real-time. This is useful for orchestrator processes that coordinate multiple agents.

### JSON mode (default)

Outputs newline-delimited JSON events:

```bash
limbo watch
```

Event types:

| Event | Description | Key fields |
|-------|-------------|------------|
| `snapshot` | Initial full task list on startup | `tasks` (array of task objects) |
| `added` | A new task was created | `task` (single task object) |
| `updated` | A task was modified | `task` (single task object) |
| `deleted` | A task was removed | `taskId` (string) |

Every event includes a `timestamp` field.

Example events:

```json
{"type":"snapshot","tasks":[{"id":"abcd","name":"Task 1","status":"captured",...}],"timestamp":"..."}
{"type":"added","task":{"id":"efgh","name":"New task","status":"captured",...},"timestamp":"..."}
{"type":"updated","task":{"id":"abcd","name":"Task 1","status":"in-progress",...},"timestamp":"..."}
{"type":"deleted","taskId":"abcd","timestamp":"..."}
```

### Pretty mode

Clears the screen and renders a hierarchical tree view that auto-refreshes:

```bash
limbo watch --pretty
```

Press `q` or `Ctrl+C` to exit.

The header shows a task count by status including a `blocked` bucket (`N todo · N in-progress · N blocked · N done`). Blocked tasks are prefixed with `🚫` and followed by indented `↳` sub-lines showing the manual block reason (if set) and/or each non-done dependency blocker by name. If a blocker ID cannot be resolved in the task map (e.g., after a prune or partial import), the raw ID is shown instead of a name. This rendering is specific to `watch --pretty`; `limbo tree` does not show blocked indicators.

### Watch options

```bash
# Filter events to a specific status
limbo watch --status in-progress

# Custom polling interval (default 500ms)
limbo watch --interval 1s

# Show all tasks including completed
limbo watch --show-all
```

## Backup and Transfer

Export and import tasks between projects or for backup:

```bash
# Export all tasks to a file
limbo export > backup.json

# Import into another project (tasks get new IDs, references are remapped)
limbo import backup.json

# Replace all existing tasks with imported ones
limbo import backup.json --replace
```

## Key Constraints

- limbo is a pure task store -- no gate validation, no field requirements for status transitions
- `--outcome`, `--approach`, `--verify`, `--result` are all optional flags (recommended but not enforced)
- `--reason` is optional for all transitions (useful for audit trail)
- Children cannot be added to `done` tasks
- Deleting a task orphans its children (sets their parent to nil)
- `limbo prune` archives all `done` tasks (moves to `archive.json` and cleans up context directories)
- Notes are append-only
- Manually blocked tasks cannot transition until unblocked
- When marked `done`, auto-removed from all other tasks' `blockedBy` lists
- Manual block (`limbo block <id> --reason "..."`) freezes a task; unblock (`limbo unblock <id>`) restores it
