package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/simonspoon/clipm/internal/models"
	"github.com/simonspoon/clipm/internal/storage"
	"github.com/spf13/cobra"
)

var statusPretty bool
var statusOutcome string

var statusCmd = &cobra.Command{
	Use:   "status <id> <status>",
	Short: "Update task status",
	Long:  `Update the status of a task. Valid statuses: todo, in-progress, done`,
	Args:  cobra.ExactArgs(2),
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusPretty, "pretty", false, "Pretty print output")
	statusCmd.Flags().StringVar(&statusOutcome, "outcome", "", "Actual result when marking done")
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Normalize and validate task ID
	id := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(id) {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	// Get new status
	newStatus := args[1]

	// Validate status
	if !models.IsValidStatus(newStatus) {
		return fmt.Errorf("invalid status %q. Must be: todo, in-progress, done", newStatus)
	}

	// Load storage
	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	// Load the task
	task, err := store.LoadTask(id)
	if err != nil {
		if err == storage.ErrTaskNotFound {
			return fmt.Errorf("task %s not found", id)
		}
		return err
	}

	// Validate transition constraints
	if err := validateStatusTransition(store, task, newStatus); err != nil {
		return err
	}

	// Require --outcome for structured tasks being marked done
	if newStatus == models.StatusDone && task.HasStructuredFields() {
		if statusOutcome == "" {
			return fmt.Errorf("structured task %s requires --outcome when marking done", task.ID)
		}
	}

	// Set outcome when marking done
	if newStatus == models.StatusDone && statusOutcome != "" {
		task.Outcome = statusOutcome
	}

	// Update status and timestamp
	task.Status = newStatus
	task.Updated = time.Now()

	// Save the task
	if err := store.SaveTask(task); err != nil {
		return err
	}

	// Auto-remove from all BlockedBy lists when marked done
	if newStatus == models.StatusDone {
		if err := store.RemoveFromAllBlockedBy(id); err != nil {
			return err
		}
	}

	if statusPretty {
		green := color.New(color.FgGreen)
		green.Printf("Updated task %s status: %s\n", task.ID, newStatus)
	} else {
		out, _ := json.Marshal(struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}{task.ID, task.Status})
		fmt.Println(string(out))
	}

	return nil
}

func validateStatusTransition(store *storage.Storage, task *models.Task, newStatus string) error {
	if newStatus == models.StatusInProgress {
		blocked, err := store.IsBlocked(task)
		if err != nil {
			return err
		}
		if blocked {
			return fmt.Errorf("cannot start task %s: blocked by %v", task.ID, task.BlockedBy)
		}
	}

	if newStatus == models.StatusDone {
		hasUndone, err := store.HasUndoneChildren(task.ID)
		if err != nil {
			return err
		}
		if hasUndone {
			return fmt.Errorf("cannot mark task as done: has undone children")
		}
	}

	return nil
}
