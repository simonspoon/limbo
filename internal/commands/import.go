package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var (
	importReplace bool
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import tasks from a JSON file",
	Long: `Import tasks from a JSON file previously created by 'limbo export'.
By default, imported tasks are added alongside existing tasks with new IDs.
Use --replace to clear existing tasks first.`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

func init() {
	importCmd.Flags().BoolVar(&importReplace, "replace", false, "Replace all existing tasks instead of merging")
}

func runImport(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var importData storage.TaskStore
	if err := json.Unmarshal(data, &importData); err != nil {
		return fmt.Errorf("failed to parse import file: %w", err)
	}

	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	if importReplace {
		// Delete all existing tasks
		existing, err := store.LoadAll()
		if err != nil {
			return err
		}
		ids := make([]string, len(existing))
		for i := range existing {
			ids[i] = existing[i].ID
		}
		if len(ids) > 0 {
			if err := store.DeleteTasks(ids); err != nil {
				return err
			}
		}
	}

	// Build mapping from old IDs to new IDs
	idMapping := make(map[string]string)
	for i := range importData.Tasks {
		newID, err := store.GenerateTaskID()
		if err != nil {
			return err
		}
		idMapping[importData.Tasks[i].ID] = newID
	}

	// Import tasks with remapped IDs
	for i := range importData.Tasks {
		task := importData.Tasks[i]
		task.ID = idMapping[task.ID]

		// Remap parent reference
		if task.Parent != nil {
			if newParent, ok := idMapping[*task.Parent]; ok {
				task.Parent = &newParent
			} else {
				task.Parent = nil // parent not in import set
			}
		}

		// Remap blockedBy references
		if len(task.BlockedBy) > 0 {
			newBlockedBy := make([]string, 0, len(task.BlockedBy))
			for _, oldID := range task.BlockedBy {
				if newID, ok := idMapping[oldID]; ok {
					newBlockedBy = append(newBlockedBy, newID)
				}
				// Drop references to tasks not in the import set
			}
			task.BlockedBy = newBlockedBy
		}

		if err := store.SaveTask(&task); err != nil {
			return fmt.Errorf("failed to save task %s: %w", task.Name, err)
		}
	}

	result := map[string]interface{}{
		"imported": len(importData.Tasks),
		"mode":     "merge",
	}
	if importReplace {
		result["mode"] = "replace"
	}

	out, _ := json.Marshal(result)
	fmt.Println(string(out))
	return nil
}

// resetImportFlags resets import flags for testing
func resetImportFlags() {
	importReplace = false
}
