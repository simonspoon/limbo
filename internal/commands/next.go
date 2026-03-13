package commands

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var (
	nextPretty    bool
	nextUnclaimed bool
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Get the next task to work on",
	Long: `Returns the next task using depth-first traversal.

When in-progress tasks exist: returns todo children (then siblings) of the deepest in-progress task, walking up the hierarchy as needed.
When no in-progress tasks: returns a list of root-level todo candidates.

Blocked tasks are always skipped. Use --unclaimed to also skip tasks that have an owner.`,
	RunE: runNext,
}

func init() {
	nextCmd.Flags().BoolVar(&nextPretty, "pretty", false, "Pretty print output")
	nextCmd.Flags().BoolVar(&nextUnclaimed, "unclaimed", false, "Skip tasks that have an owner")
}

func runNext(cmd *cobra.Command, args []string) error {
	// Load storage
	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	// Get next task
	result, err := store.GetNextTaskFiltered(nextUnclaimed)
	if err != nil {
		return err
	}

	// Handle single task result
	if result.Task != nil {
		if nextPretty {
			cyan := color.New(color.FgCyan)
			cyan.Printf("Next task: %s - %s\n", result.Task.ID, result.Task.Name)
			if result.Task.Description != "" {
				fmt.Printf("Description: %s\n", result.Task.Description)
			}
			if result.Task.Action != "" {
				fmt.Printf("Action:      %s\n", result.Task.Action)
			}
			if result.Task.Verify != "" {
				fmt.Printf("Verify:      %s\n", result.Task.Verify)
			}
			if result.Task.Result != "" {
				fmt.Printf("Result:      %s\n", result.Task.Result)
			}
		} else {
			out, _ := json.Marshal(result)
			fmt.Println(string(out))
		}
		return nil
	}

	// Handle candidates list
	if len(result.Candidates) > 0 {
		if nextPretty {
			yellow := color.New(color.FgYellow)
			yellow.Println("No task in progress. Available candidates:")
			for i := range result.Candidates {
				fmt.Printf("  %d. %s - %s\n", i+1, result.Candidates[i].ID, result.Candidates[i].Name)
			}
		} else {
			out, _ := json.Marshal(result)
			fmt.Println(string(out))
		}
		return nil
	}

	// No tasks at all
	if nextPretty {
		if result.BlockedCount > 0 {
			fmt.Printf("No unblocked tasks. %d task(s) blocked.\n", result.BlockedCount)
		} else {
			fmt.Println("No tasks in queue")
		}
	} else {
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}
	return nil
}
