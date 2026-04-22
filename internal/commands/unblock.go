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
	unblockPretty bool
	unblockBy     string
)

var unblockCmd = &cobra.Command{
	Use:   "unblock <predecessor-id> [successor-id]",
	Short: "Unblock a task (manual or dependency)",
	Long: `Unblock a task. Two modes — argument order mirrors "limbo block".

Remove dependency (two args):
    limbo unblock <predecessor-id> <successor-id>

  Remove predecessor from successor's blockedBy list. Example:
  "limbo unblock A B" removes the A-blocks-B edge (A was the predecessor
  that had to complete before B).

Remove manual block (one arg):
    limbo unblock <id>

  Remove manual block and restore the task to its previous stage.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runUnblock,
}

func init() {
	unblockCmd.Flags().BoolVar(&unblockPretty, "pretty", false, "Pretty print output")
	unblockCmd.Flags().StringVar(&unblockBy, "by", "", "Who unblocked the task")
}

func runUnblock(cmd *cobra.Command, args []string) error {
	if len(args) == 1 {
		return runManualUnblock(args)
	}
	return runDependencyUnblock(args)
}

func runDependencyUnblock(args []string) error {
	blockerID := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(blockerID) {
		return fmt.Errorf("invalid blocker ID: %s", args[0])
	}
	blockedID := models.NormalizeTaskID(args[1])
	if !models.IsValidTaskID(blockedID) {
		return fmt.Errorf("invalid blocked ID: %s", args[1])
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	blocked, err := store.LoadTask(blockedID)
	if err != nil {
		if err == storage.ErrTaskNotFound {
			return fmt.Errorf("task %s not found", blockedID)
		}
		return err
	}

	// Find and remove blocker
	found := false
	newBlockedBy := make([]string, 0, len(blocked.BlockedBy))
	for _, id := range blocked.BlockedBy {
		if id == blockerID {
			found = true
			continue
		}
		newBlockedBy = append(newBlockedBy, id)
	}

	if !found {
		return fmt.Errorf("task %s is not blocked by %s", blockedID, blockerID)
	}

	blocked.BlockedBy = newBlockedBy
	blocked.Updated = time.Now()

	if err := store.SaveTask(blocked); err != nil {
		return err
	}

	if unblockPretty {
		green := color.New(color.FgGreen)
		green.Printf("Task %s is no longer blocked by %s\n", blockedID, blockerID)
	} else {
		out, _ := json.Marshal(struct {
			ID        string   `json:"id"`
			BlockedBy []string `json:"blockedBy"`
		}{blocked.ID, blocked.BlockedBy})
		fmt.Println(string(out))
	}

	return nil
}

func runManualUnblock(args []string) error {
	id := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(id) {
		return fmt.Errorf("invalid task ID: %s", args[0])
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

	if task.ManualBlockReason == "" {
		return fmt.Errorf("task %s is not manually blocked", id)
	}

	task.History = append(task.History, models.HistoryEntry{
		From: "blocked",
		To:   task.BlockedFromStage,
		By:   unblockBy,
		At:   time.Now(),
	})
	task.Status = task.BlockedFromStage
	task.ManualBlockReason = ""
	task.BlockedFromStage = ""
	task.Updated = time.Now()

	if err := store.SaveTask(task); err != nil {
		return err
	}

	if unblockPretty {
		green := color.New(color.FgGreen)
		green.Printf("Task %s unblocked, restored to %s\n", id, task.Status)
	} else {
		out, _ := json.Marshal(struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}{task.ID, task.Status})
		fmt.Println(string(out))
	}

	return nil
}
