package commands

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var prunePretty bool

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Delete all completed tasks",
	Long:  `Delete all tasks with status 'done' that have no undone children. Safe operation - won't delete tasks with incomplete subtasks.`,
	RunE:  runPrune,
}

func init() {
	pruneCmd.Flags().BoolVar(&prunePretty, "pretty", false, "Pretty print output")
}

type pruneResult struct {
	Deleted []string `json:"deleted"`
	Count   int      `json:"count"`
}

func runPrune(cmd *cobra.Command, args []string) error {
	// Load storage
	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	// Load all tasks
	tasks, err := store.LoadAll()
	if err != nil {
		return err
	}

	// Find tasks that can be pruned (done and no undone children)
	var toPrune []string
	for i := range tasks {
		if tasks[i].Status != models.StatusDone {
			continue
		}

		// Check for undone children
		hasUndone, err := store.HasUndoneChildren(tasks[i].ID)
		if err != nil {
			return err
		}
		if hasUndone {
			continue
		}

		toPrune = append(toPrune, tasks[i].ID)
	}

	if len(toPrune) == 0 {
		result := pruneResult{Deleted: []string{}, Count: 0}
		if prunePretty {
			fmt.Println("No completed tasks to prune")
		} else {
			out, _ := json.Marshal(result)
			fmt.Println(string(out))
		}
		return nil
	}

	// Clean up BlockedBy references before deleting
	for _, id := range toPrune {
		if err := store.RemoveFromAllBlockedBy(id); err != nil {
			return err
		}
	}

	// Delete the tasks
	if err := store.DeleteTasks(toPrune); err != nil {
		return err
	}

	result := pruneResult{
		Deleted: toPrune,
		Count:   len(toPrune),
	}

	if prunePretty {
		green := color.New(color.FgGreen)
		green.Printf("Pruned %d completed task(s)\n", len(toPrune))
	} else {
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}

	return nil
}
