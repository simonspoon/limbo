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
	blockPretty bool
	blockReason string
	blockBy     string
)

var blockCmd = &cobra.Command{
	Use:   "block <predecessor-id> [successor-id]",
	Short: "Block a task (manual or dependency)",
	Long: `Block a task. Two modes — argument order matters.

Dependency block (two args):
    limbo block <predecessor-id> <successor-id>

  Predecessor must complete before successor can start; successor gains
  predecessor in its blockedBy list. Example: "limbo block A B" means
  A blocks B — A must complete before B can start.

Manual block (one arg + --reason):
    limbo block <id> --reason "..."

  Manually block a task. Current stage is saved and status transitions
  are rejected until unblocked.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runBlock,
}

func init() {
	blockCmd.Flags().BoolVar(&blockPretty, "pretty", false, "Pretty print output")
	blockCmd.Flags().StringVar(&blockReason, "reason", "", "Reason for manually blocking (required for manual block)")
	blockCmd.Flags().StringVar(&blockBy, "by", "", "Who blocked the task")
}

func runBlock(cmd *cobra.Command, args []string) error {
	if len(args) == 1 {
		return runManualBlock(args)
	}
	return runDependencyBlock(args)
}

func runDependencyBlock(args []string) error {
	blockerID, blockedID, err := parseBlockArgs(args)
	if err != nil {
		return err
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	blocker, blocked, err := loadBlockTasks(store, blockerID, blockedID)
	if err != nil {
		return err
	}

	if err := validateBlock(store, blocker, blocked, blockerID, blockedID); err != nil {
		return err
	}

	blocked.BlockedBy = append(blocked.BlockedBy, blockerID)
	blocked.Updated = time.Now()

	if err := store.SaveTask(blocked); err != nil {
		return err
	}

	if blockPretty {
		green := color.New(color.FgGreen)
		green.Printf("Task %s is now blocked by %s\n", blockedID, blockerID)
	} else {
		out, _ := json.Marshal(struct {
			ID        string   `json:"id"`
			BlockedBy []string `json:"blockedBy"`
		}{blocked.ID, blocked.BlockedBy})
		fmt.Println(string(out))
	}

	return nil
}

func runManualBlock(args []string) error {
	id := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(id) {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	if blockReason == "" {
		return fmt.Errorf("--reason is required for manual block")
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	task, err := store.LoadTask(id)
	if err != nil {
		if err == storage.ErrTaskNotFound {
			return fmt.Errorf("task %s not found", id)
		}
		return err
	}

	if task.ManualBlockReason != "" {
		return fmt.Errorf("task %s is already manually blocked", id)
	}

	task.ManualBlockReason = blockReason
	task.BlockedFromStage = task.Status
	task.History = append(task.History, models.HistoryEntry{
		From:   task.Status,
		To:     "blocked",
		By:     blockBy,
		At:     time.Now(),
		Reason: blockReason,
	})
	task.Updated = time.Now()

	if err := store.SaveTask(task); err != nil {
		return err
	}

	if blockPretty {
		green := color.New(color.FgGreen)
		green.Printf("Task %s manually blocked: %s\n", id, blockReason)
	} else {
		out, _ := json.Marshal(struct {
			ID                string `json:"id"`
			ManualBlockReason string `json:"manualBlockReason"`
			BlockedFromStage  string `json:"blockedFromStage"`
		}{task.ID, task.ManualBlockReason, task.BlockedFromStage})
		fmt.Println(string(out))
	}

	return nil
}

func parseBlockArgs(args []string) (blockerID, blockedID string, err error) {
	blockerID = models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(blockerID) {
		return "", "", fmt.Errorf("invalid blocker ID: %s", args[0])
	}
	blockedID = models.NormalizeTaskID(args[1])
	if !models.IsValidTaskID(blockedID) {
		return "", "", fmt.Errorf("invalid blocked ID: %s", args[1])
	}
	if blockerID == blockedID {
		return "", "", fmt.Errorf("a task cannot block itself")
	}
	return blockerID, blockedID, nil
}

func loadBlockTasks(store *storage.Storage, blockerID, blockedID string) (*models.Task, *models.Task, error) {
	blocker, err := store.LoadTask(blockerID)
	if err != nil {
		if err == storage.ErrTaskNotFound {
			return nil, nil, fmt.Errorf("blocker task %s not found", blockerID)
		}
		return nil, nil, err
	}

	blocked, err := store.LoadTask(blockedID)
	if err != nil {
		if err == storage.ErrTaskNotFound {
			return nil, nil, fmt.Errorf("blocked task %s not found", blockedID)
		}
		return nil, nil, err
	}
	return blocker, blocked, nil
}

func validateBlock(store *storage.Storage, blocker, blocked *models.Task, blockerID, blockedID string) error {
	if blocker.Status == models.StatusDone {
		return fmt.Errorf("cannot block on completed task %s", blockerID)
	}

	hasCycle, err := store.WouldCreateCycle(blockerID, blockedID)
	if err != nil {
		return err
	}
	if hasCycle {
		return fmt.Errorf("cannot add dependency: would create a cycle")
	}

	for _, id := range blocked.BlockedBy {
		if id == blockerID {
			return fmt.Errorf("task %s is already blocked by %s", blockedID, blockerID)
		}
	}
	return nil
}
