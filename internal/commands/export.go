package commands

import (
	"encoding/json"
	"fmt"

	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export tasks to JSON",
	Long:  `Export all tasks as JSON to stdout. Pipe to a file for backup or transfer between projects.`,
	Args:  cobra.NoArgs,
	RunE:  runExport,
}

func runExport(cmd *cobra.Command, args []string) error {
	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	tasks, err := store.LoadAll()
	if err != nil {
		return err
	}

	exportData := storage.TaskStore{
		Version: "4.0.0",
		Tasks:   tasks,
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
