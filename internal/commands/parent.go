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

var parentPretty bool

var parentCmd = &cobra.Command{
	Use:   "parent <id> <parent-id>",
	Short: "Set a task's parent",
	Long:  `Set the parent of a task to create a hierarchical relationship. Prevents circular dependencies.`,
	Args:  cobra.ExactArgs(2),
	RunE:  runParent,
}

func init() {
	parentCmd.Flags().BoolVar(&parentPretty, "pretty", false, "Pretty print output")
}

func runParent(cmd *cobra.Command, args []string) error {
	// Normalize and validate task IDs
	childID := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(childID) {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}
	parentID := models.NormalizeTaskID(args[1])
	if !models.IsValidTaskID(parentID) {
		return fmt.Errorf("invalid parent ID: %s", args[1])
	}

	// Can't parent to self
	if childID == parentID {
		return fmt.Errorf("cannot set task as its own parent")
	}

	// Load storage
	store, err := getStorage()
	if err != nil {
		return err
	}

	// Check child task exists
	childTask, err := store.LoadTask(childID)
	if err != nil {
		return fmt.Errorf("task %s not found", childID)
	}

	// Check parent task exists
	parentTask, err := store.LoadTask(parentID)
	if err != nil {
		return fmt.Errorf("parent task %s not found", parentID)
	}

	// Check parent is not done
	if parentTask.Status == models.StatusDone {
		return fmt.Errorf("cannot set done task %s as parent", parentID)
	}

	// Check for circular dependencies
	if wouldCreateCycle(store, childID, parentID) {
		return fmt.Errorf("cannot set parent - would create circular dependency")
	}

	// Update parent and timestamp
	childTask.Parent = &parentID
	childTask.Updated = time.Now()

	// Save the task
	if err := store.SaveTask(childTask); err != nil {
		return err
	}

	if parentPretty {
		green := color.New(color.FgGreen)
		green.Printf("Task %s is now a child of %s\n", childID, parentID)
	} else {
		out, _ := json.Marshal(struct {
			ID     string `json:"id"`
			Parent string `json:"parent"`
		}{childTask.ID, *childTask.Parent})
		fmt.Println(string(out))
	}

	return nil
}

// wouldCreateCycle checks if setting parentID as the parent of childID would create a cycle
func wouldCreateCycle(store *storage.Storage, childID, parentID string) bool {
	// Traverse up the parent chain from the proposed parent
	// If we encounter childID, we have a cycle
	currentID := parentID
	visited := make(map[string]bool)

	for {
		// Detect loops in existing structure
		if visited[currentID] {
			return true
		}
		visited[currentID] = true

		// If we reached the child, we have a cycle
		if currentID == childID {
			return true
		}

		// Get the current task's parent
		task, err := store.LoadTask(currentID)
		if err != nil || task.Parent == nil {
			// Reached a top-level task or error, no cycle
			return false
		}

		// Move up to parent
		currentID = *task.Parent
	}
}
