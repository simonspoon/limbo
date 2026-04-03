# Agent Integration Guide

limbo is a CLI task manager built specifically for LLMs and AI agents. This guide covers how to integrate limbo into agent workflows.

## Why limbo for agents

- **JSON output by default** -- every command returns machine-parseable JSON, no scraping needed
- **Depth-first `next` command** -- supports progressive decomposition, letting agents break large tasks into subtasks and work through them systematically
- **Ownership system** -- multiple agents can coordinate on the same task queue without conflicts
- **File-based storage** -- `.limbo/` directory with a JSON index and per-task context files, no server or daemon required

## Single-Agent Workflow

A basic agent loop: get the next task, claim it, do the work, mark it done.

```bash
# Check for next task
limbo next
# Returns: {"task": {"id": "abcd", "name": "Implement feature X", ...}}

# Claim and start
limbo claim abcd agent-1
limbo status abcd in-progress

# Log progress
limbo note abcd "Started implementation"
limbo note abcd "Found edge case, handling it"

# Complete (--outcome required for structured tasks)
limbo status abcd done --outcome "Implemented feature X; all tests pass"
```

The `next` command returns one of two shapes:

- `{"task": {...}}` -- when there is an in-progress context that narrows the result to a single suggestion
- `{"candidates": [...]}` -- when no tasks are in-progress, returning all available top-level `ready` tasks

## Multi-Agent Coordination

When multiple agents share the same task queue, use ownership and dependencies to avoid conflicts.

### Claiming tasks

```bash
# Agent picks an unclaimed task
limbo next --unclaimed
limbo claim <id> agent-1

# Other agents skip claimed tasks
limbo next --unclaimed  # won't return agent-1's task
```

`limbo claim` fails if the task is already owned by another agent. Use `--force` to override ownership if needed.

### Dependencies

Dependencies prevent agents from starting work before prerequisites are complete.

```bash
# Mark task B as blocked by task A
limbo block <prereq-id> <dependent-id>

# The dependent task won't appear in `next` until the prereq is done
limbo next  # skips blocked tasks

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

The `next` command uses depth-first traversal to support progressive decomposition workflows:

1. It finds the deepest in-progress task in the hierarchy
2. Returns that task's `ready` children first
3. If no `ready` children exist, returns `ready` siblings
4. If no `ready` tasks at that level, walks up the hierarchy and repeats

Note: only tasks in the `ready` stage appear in `next` results. Tasks must pass through the planning stages (captured → refined → planned → ready) before they can be picked up.

This means agents can break down large tasks on the fly:

```bash
# Agent gets a broad task
limbo next
# {"task": {"id": "abcd", "name": "Build authentication system"}}

# Agent decomposes it into subtasks
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
limbo add "Implement logout endpoint" --parent abcd \
  --approach "Implement POST /logout in auth package" \
  --verify "go test ./internal/auth/... passes" \
  --result "Handler file path and passing test output"

# Next call now returns the first subtask
limbo next
# {"task": {"id": "efgh", "name": "Design auth schema"}}
```

A task cannot be marked `done` if it has undone children, so agents must complete all subtasks before finishing the parent.

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

### Watch options

```bash
# Filter events to a specific status
limbo watch --status in-progress

# Custom polling interval (default 500ms)
limbo watch --interval 1s

# Show all tasks including completed
limbo watch --show-all
```

## Templates

Templates scaffold entire task hierarchies with a single command. Use them instead of manually creating tasks with `limbo add`.

**Built-in templates:** `bug-fix`, `feature`, `swe-full-cycle`.

```bash
# List available templates
limbo template list

# Preview what a template creates
limbo template show swe-full-cycle --pretty

# Apply a template — creates all tasks with dependencies pre-wired
limbo template apply feature

# Nest a template under an existing parent task
limbo template apply bug-fix --parent abcd
```

Templates define parent/child relationships and block dependencies, so agents don't need to wire them manually. Prefer `template apply` over decomposing tasks by hand when a matching template exists.

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

- Tasks cannot be marked `done` if they have undone children
- Tasks cannot be set to `in-progress` if they are blocked by incomplete dependencies
- Children cannot be added to `done` tasks
- Deleting a task orphans its children (sets their parent to nil)
- `limbo prune` archives all `done` tasks (moves to `archive.json` and cleans up context directories)
- Notes are append-only
- Structured tasks (those created with `--approach`, `--verify`, `--result`) require `--outcome` when marking `done`
- Forward transitions enforce gate validation (required fields per stage)
- Backward transitions require `--reason`
- Manual block (`limbo block <id> --reason "..."`) freezes a task; unblock (`limbo unblock <id>`) restores it
