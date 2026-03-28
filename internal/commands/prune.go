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

// findPruneableIDs returns IDs of done tasks with no undone descendants.
func findPruneableIDs(tasks []models.Task, store interface {
	HasUndoneChildren(string) (bool, error)
}) ([]string, error) {
	var ids []string
	for i := range tasks {
		if tasks[i].Status != models.StatusDone {
			continue
		}
		hasUndone, err := store.HasUndoneChildren(tasks[i].ID)
		if err != nil {
			return nil, err
		}
		if !hasUndone {
			ids = append(ids, tasks[i].ID)
		}
	}
	return ids, nil
}

// collectTasksByID returns tasks whose IDs are in the given set.
func collectTasksByID(tasks []models.Task, ids []string) []models.Task {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	var result []models.Task
	for i := range tasks {
		if idSet[tasks[i].ID] {
			result = append(result, tasks[i])
		}
	}
	return result
}

func runPrune(cmd *cobra.Command, args []string) error {
	store, err := getStorage()
	if err != nil {
		return err
	}

	tasks, err := store.LoadAll()
	if err != nil {
		return err
	}

	toPrune, err := findPruneableIDs(tasks, store)
	if err != nil {
		return err
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

	for _, id := range toPrune {
		if err := store.RemoveFromAllBlockedBy(id); err != nil {
			return err
		}
	}

	if err := store.ArchiveTasks(collectTasksByID(tasks, toPrune)); err != nil {
		return err
	}

	if err := store.DeleteTasks(toPrune); err != nil {
		return err
	}

	result := pruneResult{Archived: toPrune, Count: len(toPrune)}
	if prunePretty {
		green := color.New(color.FgGreen)
		green.Printf("Archived %d completed task(s)\n", len(toPrune))
	} else {
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}

	return nil
}
