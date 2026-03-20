package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/simonspoon/limbo/internal/models"
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
	data, err := os.ReadFile(args[0])
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
		if err := deleteAllTasks(store); err != nil {
			return err
		}
	}

	idMapping, err := buildIDMapping(store, importData.Tasks)
	if err != nil {
		return err
	}

	for i := range importData.Tasks {
		task := importData.Tasks[i]
		remapTaskIDs(&task, idMapping)
		if err := store.SaveTask(&task); err != nil {
			return fmt.Errorf("failed to save task %s: %w", task.Name, err)
		}
	}

	mode := "merge"
	if importReplace {
		mode = "replace"
	}
	out, _ := json.Marshal(map[string]interface{}{
		"imported": len(importData.Tasks),
		"mode":     mode,
	})
	fmt.Println(string(out))
	return nil
}

func deleteAllTasks(store *storage.Storage) error {
	existing, err := store.LoadAll()
	if err != nil {
		return err
	}
	ids := make([]string, len(existing))
	for i := range existing {
		ids[i] = existing[i].ID
	}
	if len(ids) > 0 {
		return store.DeleteTasks(ids)
	}
	return nil
}

func buildIDMapping(store *storage.Storage, tasks []models.Task) (map[string]string, error) {
	m := make(map[string]string, len(tasks))
	for i := range tasks {
		newID, err := store.GenerateTaskID()
		if err != nil {
			return nil, err
		}
		m[tasks[i].ID] = newID
	}
	return m, nil
}

func remapTaskIDs(task *models.Task, idMapping map[string]string) {
	task.ID = idMapping[task.ID]

	if task.Parent != nil {
		if newParent, ok := idMapping[*task.Parent]; ok {
			task.Parent = &newParent
		} else {
			task.Parent = nil
		}
	}

	if len(task.BlockedBy) > 0 {
		newBlockedBy := make([]string, 0, len(task.BlockedBy))
		for _, oldID := range task.BlockedBy {
			if newID, ok := idMapping[oldID]; ok {
				newBlockedBy = append(newBlockedBy, newID)
			}
		}
		task.BlockedBy = newBlockedBy
	}
}

// resetImportFlags resets import flags for testing
func resetImportFlags() {
	importReplace = false
}
