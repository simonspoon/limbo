package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/simonspoon/limbo/internal/store/taskstore"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X github.com/simonspoon/limbo/internal/commands.Version=..."
var Version = "dev"

// noClimbFlag backs the --no-climb persistent flag.
var noClimbFlag bool

var rootCmd = &cobra.Command{
	Use:     "limbo",
	Version: Version,
	Short:   "CLI Project Manager - A lightweight JSON-based task queue for LLMs",
	Long: `limbo is a CLI-based task manager designed for use by LLMs and agents.
It uses a single JSON file for storage and outputs JSON by default for easy parsing.`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getStorage resolves the central, per-project storage root for the current
// working directory and returns a taskstore facade rooted there. It climbs for
// a project anchor (honoring --no-climb / LIMBO_NO_CLIMB), derives the project
// ID, and computes the platform-conventional storage root (LIMBO_HOME honored).
// When no anchor is found or no ID can be derived it returns an error directing
// the user to run `limbo init`.
func getStorage() (*taskstore.Store, error) {
	projectRoot, centralRoot, err := resolveRoots()
	if err != nil {
		return nil, err
	}

	legacyDir := filepath.Join(projectRoot, ".limbo")
	legacyExists := fileExists(filepath.Join(legacyDir, "tasks.json"))
	centralExists := fileExists(filepath.Join(centralRoot, "tasks.json"))

	switch {
	case legacyExists && centralExists:
		// (A18) Both stores exist. Central wins, but warn loudly (multi-line,
		// naming both paths, recommending migrate) on EVERY invocation until the
		// legacy dir is removed or renamed. No data is touched.
		fmt.Fprintln(os.Stderr, "limbo: WARNING: both a legacy in-tree store and a central store were found.")
		fmt.Fprintf(os.Stderr, "  legacy (ignored): %s\n", legacyDir)
		fmt.Fprintf(os.Stderr, "  central (in use): %s\n", centralRoot)
		fmt.Fprintln(os.Stderr, "  Using the central store. Run 'limbo migrate' (or delete/rename the legacy dir) to silence this warning.")
		return taskstore.New(centralRoot), nil
	case legacyExists:
		// (A19) Only a legacy in-tree store exists. Use it transparently for
		// this invocation (read path only — no schema mutation, A23) and warn
		// on a single line to recommend migrate.
		fmt.Fprintf(os.Stderr, "limbo: using legacy in-tree store at %s; run 'limbo migrate' to move it to the central location.\n", legacyDir)
		return taskstore.New(legacyDir), nil
	default:
		// Only central (or neither) — current behavior, no warning.
		return taskstore.New(centralRoot), nil
	}
}

// fileExists reports whether path exists as a regular (non-directory) file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func init() {
	// Persistent flags available on all subcommands.
	rootCmd.PersistentFlags().BoolVar(&noClimbFlag, "no-climb", false,
		"do not search parent directories for a project anchor (equivalent to LIMBO_NO_CLIMB=1)")

	// Global mutating-command guard (A7). Stored as a flag value with a
	// Changed() check at use sites, since 0 is a valid revision.
	rootCmd.PersistentFlags().Int("if-revision", 0,
		"only mutate when the store's current revision equals N (no-op on read-only commands)")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(parentCmd)
	rootCmd.AddCommand(unparentCmd)
	rootCmd.AddCommand(treeCmd)
	rootCmd.AddCommand(pruneCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(blockCmd)
	rootCmd.AddCommand(unblockCmd)
	rootCmd.AddCommand(noteCmd)
	rootCmd.AddCommand(claimCmd)
	rootCmd.AddCommand(unclaimCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(archiveCmd)
}
