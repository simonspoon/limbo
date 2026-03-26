# Contributing

This guide covers what you need to know to add or modify code in limbo.

## Dev Setup

Clone the repository and enter the project directory:

```bash
git clone https://github.com/simonspoon/limbo.git
cd limbo
```

Build the binary:

```bash
go build -o limbo ./cmd/limbo
```

Run all tests:

```bash
go test ./...
```

Run a single test by name:

```bash
go test ./internal/commands -run TestAddCommand
```

Run tests with coverage:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Run the linter:

```bash
golangci-lint run
```

**Running `golangci-lint run` before committing is required. CI will fail if there are linter errors.**

## Package Structure

```
limbo/
├── cmd/limbo/main.go         # Entry point, calls commands.Execute()
├── internal/
│   ├── commands/             # Cobra command implementations (one file per command)
│   ├── models/               # Task model and status constants
│   └── storage/              # JSON file storage operations
```

## Adding a New Command

Follow the exact pattern used throughout `internal/commands/`. The `add` command (`internal/commands/add.go`) is a good reference.

### 1. Create `internal/commands/newcmd.go`

Define package-level flag variables, the cobra command, and the run function:

```go
package commands

import (
    "github.com/spf13/cobra"
)

var (
    newCmdFlag bool
)

var newCmd = &cobra.Command{
    Use:   "new <arg>",
    Short: "One-line description of the command",
    Long:  `Longer description of what the command does.`,
    Args:  cobra.ExactArgs(1),
    RunE:  runNew,
}

func init() {
    newCmd.Flags().BoolVar(&newCmdFlag, "flag-name", false, "Flag description")
}

func runNew(cmd *cobra.Command, args []string) error {
    store, err := getStorage()
    if err != nil {
        return err
    }
    // implementation
    _ = store
    return nil
}
```

Key points:
- Declare flag variables at package level (not inside functions).
- The `cobra.Command` is a package-level `var` named after the command (e.g., `newCmd`).
- Use the `Use`, `Short`, `Long`, and `RunE` fields.
- Register flags in `func init()` using `newCmd.Flags().BoolVar(...)`, `StringVar(...)`, etc.
- The run function is a named function (`runNew`) referenced in `RunE`, not an anonymous closure. This makes it directly callable from tests.
- Use `getStorage()` (not `storage.NewStorage()` directly) to obtain a `*Storage`. This helper respects the `--global` flag and `LIMBO_ROOT` env var.

### 2. Register the command in `internal/commands/root.go`

Add the command to the `init()` function in `root.go`:

```go
func init() {
    // existing commands...
    rootCmd.AddCommand(newCmd)
}
```

### 3. Create `internal/commands/newcmd_test.go`

```go
package commands

import (
    "testing"

    "github.com/simonspoon/limbo/internal/storage"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestNewCommand(t *testing.T) {
    _, cleanup := setupTestEnv(t)
    defer cleanup()

    // Reset flags to defaults before each test
    newCmdFlag = false

    err := runNew(nil, []string{"some-arg"})
    require.NoError(t, err)

    store, err := storage.NewStorage()
    require.NoError(t, err)

    tasks, err := store.LoadAll()
    require.NoError(t, err)
    assert.Len(t, tasks, 1)
}
```

## Testing Conventions

- Use `storage.NewStorageAt(dir)` with a temp directory — never use `storage.NewStorage()` directly in tests.
- Call `store.Init()` to initialize the storage before use.
- Test files live in the same package as the code they test and follow the `*_test.go` naming pattern.
- Use the shared `setupTestEnv(t)` helper defined in `add_test.go` for command tests. It creates a temp directory, changes into it, and initializes a limbo store.
- Reset all package-level flag variables to their defaults at the start of each test case.

Example using `setupTestEnv`:

```go
func TestSomething(t *testing.T) {
    _, cleanup := setupTestEnv(t)
    defer cleanup()

    // package-level flag resets
    newCmdFlag = false

    err := runNew(nil, []string{"arg"})
    require.NoError(t, err)
}
```

Example using `NewStorageAt` directly (for storage-level tests):

```go
func TestStorageThing(t *testing.T) {
    dir := t.TempDir()
    store := storage.NewStorageAt(dir)
    require.NoError(t, store.Init())

    // test against store
}
```

## PR Checklist

Before opening a pull request:

- [ ] All tests pass (`go test ./...`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Documentation updated if the change affects user-facing behavior or architecture
- [ ] Commit messages are clear and describe the change

## Source References

- Entry point: `cmd/limbo/main.go`
- Command pattern: `internal/commands/add.go` and `internal/commands/add_test.go`
- Command registration: `internal/commands/root.go`
- Task model: `internal/models/`
- Storage: `internal/storage/`
