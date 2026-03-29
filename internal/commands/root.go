package commands

import (
	"fmt"
	"os"

	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags "-X github.com/simonspoon/limbo/internal/commands.Version=..."
var Version = "dev"

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

// getStorage returns a Storage instance for the current project.
func getStorage() (*storage.Storage, error) {
	return storage.NewStorage()
}

func init() {
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
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(pruneCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(blockCmd)
	rootCmd.AddCommand(unblockCmd)
	rootCmd.AddCommand(noteCmd)
	rootCmd.AddCommand(claimCmd)
	rootCmd.AddCommand(unclaimCmd)
	rootCmd.AddCommand(editCmd)
	rootCmd.AddCommand(templateCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(archiveCmd)
}
