package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "limbo",
	Version: "0.1.0",
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
}
