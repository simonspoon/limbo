<p align="center">
  <img src="icon.png" width="128" height="128" alt="limbo">
</p>

# limbo

A CLI-based task manager designed for use by LLMs and AI agents.

limbo uses two-tier file-based storage: a lean JSON index (`.limbo/tasks.json`) for metadata and per-task markdown files (`.limbo/context/<id>/context.md`) for content. All commands output JSON by default for easy parsing. It supports hierarchical task structures with progressive decomposition workflows.

## Installation

### Homebrew

```bash
brew install simonspoon/tap/limbo
```

### Go install

```bash
go install github.com/simonspoon/limbo/cmd/limbo@latest
```

### From source

```bash
git clone https://github.com/simonspoon/limbo.git
cd limbo
go build -o limbo ./cmd/limbo
```

## Quick Start

```bash
# Initialize limbo in your project
limbo init

# Add tasks (simple or structured)
limbo add "Fix login bug"
limbo add "Implement user authentication" \
  --approach "Build JWT login and token refresh" \
  --verify "go test ./... passes" \
  --result "List of endpoints added and test output"
limbo add "Add login endpoint" --parent <task-id> \
  --approach "Implement POST /login handler" \
  --verify "Integration test passes" \
  --result "Handler file path and test results"

# View tasks
limbo list                    # JSON output
limbo list --pretty           # Human-readable output
limbo tree                    # Hierarchical view (pretty by default)

# Update task status (no gate validation -- pure task store)
limbo status <task-id> in-progress
limbo status <task-id> done --outcome "Implemented; all tests pass"

# Watch for changes
limbo watch --pretty
```

## Command Reference

| Command | Description |
|---------|-------------|
| `init` | Initialize limbo in the current directory |
| `add <name>` | Add a new task (`--approach`, `--verify`, `--result`, `--parent`, `--description`/`-d`, plus lifecycle fields) |
| `edit <id>` | Edit a task's fields (`--name`, `--description`/`-d`, `--approach`, `--verify`, `--result`, plus lifecycle fields) |
| `list` | List all tasks |
| `tree` | Display tasks in a tree structure (`--show-all`) |
| `show <id>` | Show details for a specific task |
| `status <id> <status>` | Update task status (`captured`, `refined`, `planned`, `ready`, `in-progress`, `in-review`, `done`) |
| `parent <id> <parent-id>` | Set a task's parent |
| `unparent <id>` | Remove a task's parent |
| `delete <id>` | Delete a task |
| `prune` | Archive all completed tasks to `.limbo/archive.json` |
| `archive list` | List archived tasks |
| `archive show <id>` | Show details for an archived task |
| `archive restore <id>` | Restore an archived task to the active store |
| `archive purge` | Permanently delete all archived tasks |
| `watch` | Watch tasks for live updates |
| `block <blocker> <blocked>` | Add dependency; or `block <id> --reason "..."` for manual block |
| `unblock <blocker> <blocked>` | Remove dependency; or `unblock <id>` to remove manual block |
| `note <id> "message"` | Add a timestamped note to a task |
| `claim <id> <agent>` | Claim task ownership |
| `unclaim <id>` | Release task ownership |
| `search <query>` | Search tasks by name or description (case-insensitive) |
| `export` | Export all tasks as JSON to stdout |
| `import <file>` | Import tasks from a JSON file (`--replace`) |

All commands output JSON by default. Use `--pretty` for human-readable output with colors.

### Completed Task Visibility

By default, `list`, `tree`, and `watch` hide "fully resolved" done tasks. A done task is only shown if its parent exists and is not done (i.e., it's a completed subtask of active work). Top-level done tasks and done children of done parents are hidden.

Use `--show-all` on any of these commands to see all tasks including completed.

### Filtering

The `list` command supports filtering:
- `--status <status>` - Filter by status
- `--owner <name>` - Filter by owner
- `--unclaimed` - Show only unowned tasks
- `--blocked` / `--unblocked` - Filter by blocked state
- `--show-all` - Show all tasks including completed

## Usage with AI Agents

limbo is designed for integration with LLMs and AI agents like Claude Code. The JSON output makes it easy to parse task information programmatically.

Example workflow:
```bash
# Agent finds available work
limbo list --status ready --unblocked

# Agent claims and starts task
limbo claim abcd agent-1
limbo status abcd in-progress

# Agent adds progress notes
limbo note abcd "Started implementation"
limbo note abcd "Found edge case, handling it"

# Agent completes work, marks done
limbo status abcd done --outcome "Implemented feature X; all tests pass"
```

### Multi-Agent Coordination

limbo supports multiple agents working on the same task queue:

```bash
# Agent finds and claims an unclaimed task
limbo list --status ready --unblocked --unclaimed
limbo claim <id> agent-1

# Set up task dependencies
limbo block <prereq-id> <dependent-id>
# dependent won't appear in --unblocked results until prereq is done

# When prereq completes, dependent is auto-unblocked
limbo status <prereq-id> done
```

### Watch Mode

The `watch` command monitors tasks.json for changes and outputs updates in real-time:

```bash
# JSON mode (default) - outputs newline-delimited events
limbo watch

# Pretty mode - clears screen and redraws hierarchical tree
limbo watch --pretty

# Filter by status
limbo watch --status in-progress --pretty

# Show all tasks including completed
limbo watch --show-all --pretty

# Custom polling interval
limbo watch --interval 1s
```

JSON mode outputs events:
- `snapshot` - Initial task list on startup
- `added` - New task created
- `updated` - Task modified
- `deleted` - Task removed

**Pretty mode** renders a live hierarchical tree. The header line shows a count of tasks by status, including a `blocked` bucket:

```
limbo watch - 15:04:05
Tasks: 2 todo · 0 in-progress · 1 blocked · 0 done
```

The blocked count reflects only the tasks currently visible (respects any active `--status` filter).

Blocked tasks are prefixed with `🚫` before the task name. Indented `↳` sub-lines describe why:

Blocked task rendering (example abbreviated — real output includes all visible tasks):

```
abcd  🚫 blocked task  [CAPTURED]
  ↳ waiting on review
  ↳ blocked by: dep task
```

- The first `↳` line shows the manual block reason, if one was set.
- Each remaining `↳` line identifies a non-done dependency blocker by name (or by raw ID if the blocker cannot be resolved).

Note: blocked visibility is specific to `watch --pretty`. The `limbo tree` command does not show `🚫` prefixes or `↳` sub-lines.

## Storage

limbo uses two-tier storage within the `.limbo/` directory:

- **`tasks.json`** — lightweight JSON index containing task metadata (id, name, status, parent, blockedBy, owner, timestamps)
- **`context/<id>/context.md`** — per-task markdown file with content fields (approach, verify, result, outcome, acceptance criteria, scope out, affected areas, test strategy, risks, report, description, notes) using H2 sections
- **`archive.json`** — archived tasks (complete data, created by `limbo prune`)

The storage system walks up directories to find the `.limbo/` folder (similar to how git finds `.git/`). The split is transparent — commands like `show` and `edit` merge both tiers automatically.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.
