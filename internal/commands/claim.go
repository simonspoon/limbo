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
	claimPretty bool
	claimForce  bool
)

var claimCmd = &cobra.Command{
	Use:   "claim <id> <agent-name>",
	Short: "Claim ownership of a task",
	Long:  `Set the owner of a task to the specified agent name.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runClaim,
}

func init() {
	claimCmd.Flags().BoolVar(&claimPretty, "pretty", false, "Pretty print output")
	claimCmd.Flags().BoolVar(&claimForce, "force", false, "Force claim even if already owned")
}

func runClaim(cmd *cobra.Command, args []string) error {
	id := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(id) {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	agentName := args[1]
	if agentName == "" {
		return fmt.Errorf("agent name cannot be empty")
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

	// Check if already owned by different agent
	if task.Owner != nil && *task.Owner != agentName && !claimForce {
		return fmt.Errorf("task %s is already owned by %s (use --force to override)", id, *task.Owner)
	}

	task.Owner = &agentName
	task.Updated = time.Now()

	if err := store.SaveTask(task); err != nil {
		return err
	}

	if claimPretty {
		green := color.New(color.FgGreen)
		green.Printf("Task %s claimed by %s\n", id, agentName)
	} else {
		out, _ := json.Marshal(struct {
			ID    string `json:"id"`
			Owner string `json:"owner"`
		}{task.ID, *task.Owner})
		fmt.Println(string(out))
	}

	return nil
}
