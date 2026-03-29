# Getting Started

limbo is a CLI-based task manager designed for use by LLMs and AI agents. It stores tasks in a single JSON file and outputs JSON by default for easy programmatic parsing.

## Installation

**Via go install:**

```bash
go install github.com/simonspoon/limbo/cmd/limbo@latest
```

**Build from source:**

```bash
git clone https://github.com/simonspoon/limbo.git
cd limbo
go build -o limbo ./cmd/limbo
```

## Initialize a Project

Run `limbo init` from your project root to create `.limbo/tasks.json` in the current directory:

```bash
limbo init
```

limbo's storage walks up directories to find `.limbo/` — the same way git finds `.git/`. This means you can run limbo commands from any subdirectory of your project and it will find the right task file. Run `limbo init` from the project root so all subdirectories can discover it.

## Basic Task Lifecycle

Here is a concrete example of the typical workflow:

**1. Add a top-level task:**

```bash
limbo add "Build the feature"
```

This returns JSON including the new task's ID:

```json
{"id": "abcd", "name": "Build the feature", "status": "todo", ...}
```

**2. Add subtasks using the parent ID:**

```bash
limbo add "Write tests" --parent abcd
limbo add "Update documentation" --parent abcd
```

**3. Set a task to in-progress:**

```bash
limbo status abcd in-progress
```

**4. Complete a task:**

```bash
limbo status abcd done
```

Note: a task cannot be marked `done` if it has children that are not yet done. Complete all subtasks first.

## Viewing Tasks

**JSON list of all tasks:**

```bash
limbo list
```

**Human-readable list grouped by status:**

```bash
limbo list --pretty
```

**Hierarchical tree view (pretty by default):**

```bash
limbo tree
```

**Details for a single task:**

```bash
limbo show abcd
```

By default, `list`, `tree`, and `watch` hide completed tasks that are fully resolved (top-level done tasks or done children of done parents). Use `--show-all` to see everything:

```bash
limbo list --show-all
limbo tree --show-all
```

## Getting the Next Task

```bash
limbo next
```

`limbo next` uses depth-first traversal to return the most relevant task to work on. It finds the deepest in-progress task and returns its todo children. If there are no in-progress tasks, it returns candidates from the top level. Blocked tasks are always skipped.

When context exists (an in-progress task is found), the response looks like:

```json
{"task": {"id": "abcd", "name": "Write tests", ...}}
```

When no task is in-progress, it returns candidates:

```json
{"candidates": [...]}
```

## Output Format

All commands output JSON by default. Add `--pretty` to any command for human-readable output with colors:

```bash
limbo list --pretty
limbo show abcd --pretty
limbo next --pretty
```
