package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var (
	statusPretty  bool
	statusOutcome string
	statusReason  string
	statusBy      string
)

var statusCmd = &cobra.Command{
	Use:   "status <id> <status>",
	Short: "Update task status",
	Long:  `Update the status of a task. Valid statuses: captured, refined, planned, ready, in-progress, in-review, done`,
	Args:  cobra.ExactArgs(2),
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusPretty, "pretty", false, "Pretty print output")
	statusCmd.Flags().StringVar(&statusOutcome, "outcome", "", "Actual result when marking done")
	statusCmd.Flags().StringVar(&statusReason, "reason", "", "Reason for backward transition")
	statusCmd.Flags().StringVar(&statusBy, "by", "", "Who triggered the transition")
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
		return fmt.Errorf("invalid status %q. Must be: captured, refined, planned, ready, in-progress, in-review, done", newStatus)
	}

	// Load storage
	store, err := getStorage()
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

	// Set outcome if provided
	if newStatus == models.StatusDone && statusOutcome != "" {
		task.Outcome = statusOutcome
	}

	// Validate transition constraints
	if err := validateStatusTransition(task, newStatus); err != nil {
		return err
	}

	// Record history entry
	oldStatus := task.Status
	task.History = append(task.History, models.HistoryEntry{
		From:   oldStatus,
		To:     newStatus,
		By:     statusBy,
		At:     time.Now(),
		Reason: statusReason,
	})

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

	printStatusUpdate(task)
	return nil
}

func printStatusUpdate(task *models.Task) {
	if statusPretty {
		green := color.New(color.FgGreen)
		green.Printf("Updated task %s status: %s\n", task.ID, task.Status)
	} else {
		out, _ := json.Marshal(struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}{task.ID, task.Status})
		fmt.Println(string(out))
	}
}

func validateStatusTransition(task *models.Task, newStatus string) error {
	// Manually blocked tasks cannot transition at all
	if task.ManualBlockReason != "" {
		return fmt.Errorf("cannot transition task %s: manually blocked (%s)", task.ID, task.ManualBlockReason)
	}

	return nil
}
