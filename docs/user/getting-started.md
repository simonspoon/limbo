# Getting Started

limbo is a CLI-based task manager designed for use by LLMs and AI agents. It uses two-tier file-based storage (a JSON index for metadata plus per-task markdown files for content) and outputs JSON by default for easy programmatic parsing.

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

Run `limbo init` from your project root to create the `.limbo/` directory (containing `tasks.json` and `context/`):

```bash
limbo init
```

limbo's storage walks up directories to find `.limbo/` — the same way git finds `.git/`. This means you can run limbo commands from any subdirectory of your project and it will find the right task file. Run `limbo init` from the project root so all subdirectories can discover it.

If the resolved store happens to be your home directory (`$HOME/.limbo/`), limbo prints a one-shot warning on stderr so a missing local `.limbo/` doesn't silently contaminate a home-dir store. Pass `--no-climb` (or set `LIMBO_NO_CLIMB=1`) to disable parent-directory search entirely.

## Basic Task Lifecycle

limbo uses a 7-stage lifecycle: `captured → refined → planned → ready → in-progress → in-review → done`. Each forward transition enforces required fields (gates).

**1. Add a task (starts as `captured`):**

```bash
limbo add "Build the feature"
```

**2. Refine it (requires acceptance criteria and scope):**

```bash
limbo edit abcd --acceptance-criteria "Users can log in and out" --scope-out "No OAuth"
limbo status abcd refined
```

**3. Plan it (requires approach, affected areas, test strategy, risks):**

```bash
limbo edit abcd --approach "JWT-based auth" --affected-areas "internal/auth/" \
  --test-strategy "Unit + integration tests" --risks "Token rotation edge cases"
limbo status abcd planned
```

**4. Mark ready (requires verify steps):**

```bash
limbo edit abcd --verify "go test ./internal/auth/... passes"
limbo status abcd ready
```

**5. Claim and start:**

```bash
limbo claim abcd agent-1
limbo status abcd in-progress
```

**6. Submit for review (requires report):**

```bash
limbo edit abcd --report "Added login/logout endpoints, all tests pass"
limbo status abcd in-review
```

**7. Complete:**

```bash
limbo status abcd done --outcome "JWT auth implemented; 12 tests passing"
```

limbo is a pure task store -- status transitions have no field requirements. You can jump to any status at any time.

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

## Finding Available Work

```bash
limbo list --status ready --unblocked
```

Use `list` with filters to find tasks that are ready to work on. Combine flags for specific queries:

```bash
limbo list --status ready --unblocked --unclaimed  # Unowned ready tasks
limbo list --blocked                                # Tasks waiting on dependencies
```

## Output Format

All commands output JSON by default. Add `--pretty` to any command for human-readable output with colors:

```bash
limbo list --pretty
limbo show abcd --pretty
```
