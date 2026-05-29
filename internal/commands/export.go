package commands

import (
	"encoding/json"
	"fmt"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export tasks to JSON",
	Long:  `Export all tasks as JSON to stdout. Pipe to a file for backup or transfer between projects.`,
	Args:  cobra.NoArgs,
	RunE:  runExport,
}

// exportEnvelope is the export wire format. The legacy version string is left
// at 4.0.0 (the export format is independent of the on-disk schema); the
// additive top-level revision integer surfaces the store's current revision
// (A22).
type exportEnvelope struct {
	Version  string        `json:"version"`
	Revision int           `json:"revision"`
	Tasks    []models.Task `json:"tasks"`
}

func runExport(cmd *cobra.Command, args []string) error {
	store, err := getStorage()
	if err != nil {
		return err
	}

	tasks, err := store.LoadAll()
	if err != nil {
		return err
	}

	revision, err := store.Revision()
	if err != nil {
		return err
	}

	exportData := exportEnvelope{
		Version:  "4.0.0",
		Revision: revision,
		Tasks:    tasks,
	}

	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
