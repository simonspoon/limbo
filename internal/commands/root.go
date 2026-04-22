package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X github.com/simonspoon/limbo/internal/commands.Version=..."
var Version = "dev"

// noClimbFlag backs the --no-climb persistent flag.
var noClimbFlag bool

// climbWarnOnce guards the home-dir-climb warning so it emits at most once
// per invocation even when getStorage() is called multiple times.
var climbWarnOnce sync.Once

var rootCmd = &cobra.Command{
	Use:     "limbo",
	Version: Version,
	Short:   "CLI Project Manager - A lightweight JSON-based task queue for LLMs",
	Long: `limbo is a CLI-based task manager designed for use by LLMs and agents.
It uses a single JSON file for storage and outputs JSON by default for easy parsing.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if noClimbFlag {
			if err := os.Setenv(storage.NoClimbEnv, "1"); err != nil {
				return err
			}
		}
		return nil
	},
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getStorage returns a Storage instance for the current project. When the
// resolved .limbo root is an ancestor of cwd AND matches $HOME, a one-shot
// warning is emitted to stderr to surface accidental home-dir store use.
func getStorage() (*storage.Storage, error) {
	store, err := storage.NewStorage()
	if err != nil {
		return nil, err
	}
	maybeWarnHomeClimb(store.GetRootDir())
	return store, nil
}

// maybeWarnHomeClimb emits a one-shot stderr warning when the resolved
// limbo root is an ancestor of the current working directory AND equals
// the user's home directory. This catches the common case where a project
// subdir with no local .limbo silently uses ~/.limbo, contaminating the
// home-dir backlog with project tasks.
func maybeWarnHomeClimb(rootDir string) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}

	// Resolve symlinks so macOS /var vs /private/var comparisons work.
	resolvedRoot := evalSymlinksOrSelf(rootDir)
	resolvedCwd := evalSymlinksOrSelf(cwd)
	resolvedHome := evalSymlinksOrSelf(home)

	// Only warn when the store was found above cwd (climb occurred) AND
	// that ancestor is the home directory.
	if resolvedRoot == resolvedCwd {
		return
	}
	if resolvedRoot != resolvedHome {
		return
	}

	climbWarnOnce.Do(func() {
		fmt.Fprintf(os.Stderr,
			"warning: using limbo store at %s (ancestor of %s).\n"+
				"  store matches $HOME — may cause home-dir backlog contamination.\n"+
				"  run 'limbo init' here, or set %s=1 / pass --no-climb to disable parent search.\n",
			rootDir, cwd, storage.NoClimbEnv)
	})
}

// evalSymlinksOrSelf resolves symlinks, falling back to the input on error.
func evalSymlinksOrSelf(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func init() {
	// Persistent flags available on all subcommands.
	rootCmd.PersistentFlags().BoolVar(&noClimbFlag, "no-climb", false,
		"do not search parent directories for .limbo (equivalent to LIMBO_NO_CLIMB=1)")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
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
