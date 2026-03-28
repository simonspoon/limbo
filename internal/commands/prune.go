package commands

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/spf13/cobra"
)

var prunePretty bool

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Archive all completed tasks",
	Long:  `Archive all tasks with status 'done' that have no undone children to .limbo/archive.json. Safe operation - won't archive tasks with incomplete subtasks.`,
	RunE:  runPrune,
}

func init() {
	pruneCmd.Flags().BoolVar(&prunePretty, "pretty", false, "Pretty print output")
}

type pruneResult struct {
	Archived []string `json:"archived"`
	Count    int      `json:"count"`
}

func runPrune(cmd *cobra.Command, args []string) error {
	// Load storage
	store, err := getStorage()
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
		result := pruneResult{Archived: []string{}, Count: 0}
		if prunePretty {
			fmt.Println("No completed tasks to prune")
		} else {
			out, _ := json.Marshal(result)
			fmt.Println(string(out))
		}
		return nil
	}

	// Clean up BlockedBy references before archiving
	for _, id := range toPrune {
		if err := store.RemoveFromAllBlockedBy(id); err != nil {
			return err
		}
	}

	// Collect full task objects for archiving
	pruneSet := make(map[string]bool)
	for _, id := range toPrune {
		pruneSet[id] = true
	}
	var toArchive []models.Task
	for i := range tasks {
		if pruneSet[tasks[i].ID] {
			toArchive = append(toArchive, tasks[i])
		}
	}

	// Archive first, then delete (crash safety: duplicate > data loss)
	if err := store.ArchiveTasks(toArchive); err != nil {
		return err
	}

	if err := store.DeleteTasks(toPrune); err != nil {
		return err
	}

	result := pruneResult{
		Archived: toPrune,
		Count:    len(toPrune),
	}

	if prunePretty {
		green := color.New(color.FgGreen)
		green.Printf("Archived %d completed task(s)\n", len(toPrune))
	} else {
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}

	return nil
}
