# limbo

A CLI-based task manager designed for use by LLMs and AI agents.

limbo uses a single JSON file (`.limbo/tasks.json`) for storage and outputs JSON by default for easy parsing. It supports hierarchical task structures with progressive decomposition workflows.

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

# Add tasks (--action, --verify, --result are required)
limbo add "Implement user authentication" \
  --action "Build JWT login and token refresh" \
  --verify "go test ./... passes" \
  --result "List of endpoints added and test output"
limbo add "Add login endpoint" --parent <task-id> \
  --action "Implement POST /login handler" \
  --verify "Integration test passes" \
  --result "Handler file path and test results"

# View tasks
limbo list                    # JSON output
limbo list --pretty           # Human-readable output
limbo tree                    # Hierarchical view (pretty by default)

# Update task status
limbo status <task-id> in-progress
limbo status <task-id> done --outcome "Implemented; all tests pass"

# Get next task (depth-first traversal)
limbo next

# Watch for changes
limbo watch --pretty
```

## Command Reference

| Command | Description |
|---------|-------------|
| `init` | Initialize limbo in the current directory |
| `add <name>` | Add a new task (`--action`, `--verify`, `--result` required; `--parent`, `--description`/`-d`) |
| `edit <id>` | Edit a task's fields (`--name`, `--description`/`-d`, `--action`, `--verify`, `--result`) |
| `list` | List all tasks |
| `tree` | Display tasks in a tree structure (`--show-all`) |
| `show <id>` | Show details for a specific task |
| `status <id> <status>` | Update task status (`todo`, `in-progress`, `done`); `--outcome` required for structured tasks when marking `done` |
| `next` | Get the next task to work on |
| `parent <id> <parent-id>` | Set a task's parent |
| `unparent <id>` | Remove a task's parent |
| `delete <id>` | Delete a task |
| `prune` | Remove all completed tasks |
| `watch` | Watch tasks for live updates |
| `block <blocker> <blocked>` | Add dependency (blocked waits for blocker) |
| `unblock <blocker> <blocked>` | Remove dependency |
| `note <id> "message"` | Add a timestamped note to a task |
| `claim <id> <agent>` | Claim task ownership |
| `unclaim <id>` | Release task ownership |
| `search <query>` | Search tasks by name or description (case-insensitive) |
| `template list` | List available task templates |
| `template show <name>` | Preview a template's task hierarchy |
| `template apply <name>` | Apply a template to create tasks (`--parent`) |
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

The `next` command supports:
- `--unclaimed` - Skip tasks that have an owner

## Usage with AI Agents

limbo is designed for integration with LLMs and AI agents like Claude Code. The JSON output makes it easy to parse task information programmatically.

Example workflow:
```bash
# Agent checks for next task
limbo next

# Returns JSON like:
# {"task": {"id": "abcd", "name": "Implement feature X", ...}}

# Agent claims and starts task
limbo claim abcd agent-1
limbo status abcd in-progress

# Agent adds progress notes
limbo note abcd "Started implementation"
limbo note abcd "Found edge case, handling it"

# Agent completes work, marks done (--outcome required for structured tasks)
limbo status abcd done --outcome "Implemented feature X; all tests pass"
```

### Multi-Agent Coordination

limbo supports multiple agents working on the same task queue:

```bash
# Agent claims an unclaimed task
limbo next --unclaimed
limbo claim <id> agent-1

# Other agents skip claimed tasks
limbo next --unclaimed  # won't return agent-1's task

# Set up task dependencies
limbo block <prereq-id> <dependent-id>
# dependent task won't appear in `next` until prereq is done

# When prereq completes, dependent is auto-unblocked
limbo status <prereq-id> done
```

### Progressive Decomposition

The `next` command uses depth-first traversal to support progressive decomposition:
- Finds the deepest in-progress task
- Returns its todo children (or siblings if none)
- Walks up the hierarchy when no todos exist at the current level

This allows agents to break down large tasks into smaller subtasks just-in-time.

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

## Storage

Tasks are stored in `.limbo/tasks.json` in your project directory. The storage system walks up directories to find the `.limbo/` folder (similar to how git finds `.git/`).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.
