package commands

import (
	"encoding/json"
	"fmt"
	"strings"
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

	// Set outcome before gate validation so in-review->done gate can pass
	if newStatus == models.StatusDone && statusOutcome != "" {
		task.Outcome = statusOutcome
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

func validateStatusTransition(store *storage.Storage, task *models.Task, newStatus string) error {
	// Manually blocked tasks cannot transition at all
	if task.ManualBlockReason != "" {
		return fmt.Errorf("cannot transition task %s: manually blocked (%s)", task.ID, task.ManualBlockReason)
	}

	oldIndex := models.StageIndex(task.Status)
	newIndex := models.StageIndex(newStatus)

	// Backward transition: require --reason
	if newIndex < oldIndex {
		if statusReason == "" {
			return fmt.Errorf("cannot transition %s backward from %s to %s: --reason is required", task.ID, task.Status, newStatus)
		}
		// Backward transitions skip gate validation
		return nil
	}

	// Forward transition: validate gates for each intermediate step
	for i := oldIndex; i < newIndex; i++ {
		from := models.StageOrder[i]
		to := models.StageOrder[i+1]
		if err := validateGate(store, task, from, to); err != nil {
			return err
		}
	}

	return nil
}

// checkRequired returns an error listing any empty fields from the name→value pairs.
func checkRequired(taskID, from, to string, fields ...string) error {
	var missing []string
	for i := 0; i < len(fields); i += 2 {
		if fields[i+1] == "" {
			missing = append(missing, fields[i])
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("cannot transition %s from %s to %s: missing required fields: %s",
			taskID, from, to, strings.Join(missing, ", "))
	}
	return nil
}

func validateGate(store *storage.Storage, task *models.Task, from, to string) error {
	switch from + "->" + to {
	case models.StatusCaptured + "->" + models.StatusRefined:
		return checkRequired(task.ID, from, to,
			"acceptance_criteria", task.AcceptanceCriteria,
			"scope_out", task.ScopeOut)

	case models.StatusRefined + "->" + models.StatusPlanned:
		return checkRequired(task.ID, from, to,
			"approach", task.Approach,
			"affected_areas", task.AffectedAreas,
			"test_strategy", task.TestStrategy,
			"risks", task.Risks)

	case models.StatusPlanned + "->" + models.StatusReady:
		return checkRequired(task.ID, from, to, "verify", task.Verify)

	case models.StatusReady + "->" + models.StatusInProgress:
		if task.Owner == nil {
			return fmt.Errorf("cannot transition %s from %s to %s: task must be claimed (no owner)",
				task.ID, from, to)
		}
		blocked, err := store.IsBlocked(task)
		if err != nil {
			return err
		}
		if blocked {
			return fmt.Errorf("cannot start task %s: blocked by %v", task.ID, task.BlockedBy)
		}

	case models.StatusInProgress + "->" + models.StatusInReview:
		return checkRequired(task.ID, from, to, "report", task.Report)

	case models.StatusInReview + "->" + models.StatusDone:
		if err := checkRequired(task.ID, from, to, "outcome", task.Outcome); err != nil {
			return err
		}
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
