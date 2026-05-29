package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/store"
	"github.com/simonspoon/limbo/internal/store/taskstore"
	"github.com/spf13/cobra"
)

var initPretty bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new limbo project",
	Long: `Initialize a new limbo project.

limbo resolves a per-project storage location under a central, platform-
conventional data directory (overridable with LIMBO_HOME) keyed by a stable
project ID. The project ID is taken from a .limbo-id file, then the git
first-commit SHA, and otherwise a freshly generated UUID written to .limbo-id.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initPretty, "pretty", false, "Pretty print output")
}

type initResult struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
}

func runInit(cmd *cobra.Command, args []string) error {
	// (A15) Determine the project root. init uses cwd when no anchor is found
	// or when climbing is disabled.
	root, err := initProjectRoot()
	if err != nil {
		return err
	}

	// Resolve or derive the project ID. On ErrNoProjectID (no .limbo-id, no
	// git history) generate a UUID v4 and persist it to <root>/.limbo-id (A13).
	id, err := store.ResolveProjectID(root, nil)
	if err != nil {
		if !errors.Is(err, store.ErrNoProjectID) {
			return err
		}
		id, err = generateUUIDv4()
		if err != nil {
			return err
		}
		idPath := filepath.Join(root, store.LimboIDFile)
		if werr := os.WriteFile(idPath, []byte(id+"\n"), 0o644); werr != nil {
			return fmt.Errorf("failed to write %s: %w", store.LimboIDFile, werr)
		}
	}

	// Compute the central storage root for this project ID.
	storageRoot, err := centralStorageRoot(id)
	if err != nil {
		return err
	}

	// (A16) If the storage root already holds a tasks.json, rename the existing
	// directory aside before reinitializing, and announce the relocation.
	if _, statErr := os.Stat(filepath.Join(storageRoot, "tasks.json")); statErr == nil {
		replaced := storageRoot + ".replaced-" + time.Now().UTC().Format("20060102T150405Z")
		if err := os.Rename(storageRoot, replaced); err != nil {
			return fmt.Errorf("failed to relocate existing store: %w", err)
		}
		fmt.Printf("relocated existing store to %s\n", replaced)
	}

	// Seed the empty store at revision 0 and create the context directory.
	ts := taskstore.New(storageRoot)
	if err := ts.Seed(); err != nil {
		return err
	}

	result := initResult{Success: true, Path: storageRoot}
	if initPretty {
		green := color.New(color.FgGreen)
		green.Printf("Initialized limbo store at %s\n", storageRoot)
	} else {
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}
	return nil
}

// initProjectRoot resolves the project root for init: it climbs for an anchor
// (honoring no-climb), but falls back to cwd when no anchor is found (A15)
// instead of erroring the way non-init commands do.
func initProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	root, err := store.FindProjectRoot(cwd, wantNoClimb())
	if err != nil {
		var nf *store.ProjectRootNotFoundError
		if errors.As(err, &nf) {
			return cwd, nil
		}
		return "", err
	}
	return root, nil
}
