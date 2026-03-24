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

var unblockPretty bool

var unblockCmd = &cobra.Command{
	Use:   "unblock <blocker-id> <blocked-id>",
	Short: "Remove a dependency between tasks",
	Long:  `Remove blocker-id from blocked-id's dependencies.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runUnblock,
}

func init() {
	unblockCmd.Flags().BoolVar(&unblockPretty, "pretty", false, "Pretty print output")
}

func runUnblock(cmd *cobra.Command, args []string) error {
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
