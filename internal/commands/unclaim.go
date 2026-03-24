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

var unclaimPretty bool

var unclaimCmd = &cobra.Command{
	Use:   "unclaim <id>",
	Short: "Remove ownership from a task",
	Long:  `Clear the owner of a task.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runUnclaim,
}

func init() {
	unclaimCmd.Flags().BoolVar(&unclaimPretty, "pretty", false, "Pretty print output")
}

func runUnclaim(cmd *cobra.Command, args []string) error {
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

	if task.Owner == nil {
		return fmt.Errorf("task %s has no owner", id)
	}

	task.Owner = nil
	task.Updated = time.Now()

	if err := store.SaveTask(task); err != nil {
		return err
	}

	if unclaimPretty {
		green := color.New(color.FgGreen)
		green.Printf("Task %s ownership cleared\n", id)
	} else {
		out, _ := json.Marshal(struct {
			ID    string  `json:"id"`
			Owner *string `json:"owner"`
		}{task.ID, nil})
		fmt.Println(string(out))
	}

	return nil
}
