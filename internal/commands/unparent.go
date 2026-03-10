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

var unparentPretty bool

var unparentCmd = &cobra.Command{
	Use:   "unparent <id>",
	Short: "Remove a task's parent",
	Long:  `Remove the parent relationship from a task, making it a top-level task.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runUnparent,
}

func init() {
	unparentCmd.Flags().BoolVar(&unparentPretty, "pretty", false, "Pretty print output")
}

func runUnparent(cmd *cobra.Command, args []string) error {
	// Normalize and validate task ID
	id := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(id) {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	// Load storage
	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	// Load the task
	task, err := store.LoadTask(id)
	if err != nil {
		return fmt.Errorf("task %s not found", id)
	}

	// Check if task already has no parent
	if task.Parent == nil {
		if unparentPretty {
			yellow := color.New(color.FgYellow)
			yellow.Printf("Task %s is already a top-level task\n", id)
		} else {
			out, _ := json.Marshal(struct {
				ID     string  `json:"id"`
				Parent *string `json:"parent"`
			}{task.ID, nil})
			fmt.Println(string(out))
		}
		return nil
	}

	// Remove parent and update timestamp
	task.Parent = nil
	task.Updated = time.Now()

	// Save the task
	if err := store.SaveTask(task); err != nil {
		return err
	}

	if unparentPretty {
		green := color.New(color.FgGreen)
		green.Printf("Task %s is now a top-level task\n", id)
	} else {
		out, _ := json.Marshal(struct {
			ID     string  `json:"id"`
			Parent *string `json:"parent"`
		}{task.ID, nil})
		fmt.Println(string(out))
	}

	return nil
}
