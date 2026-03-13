package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var initPretty bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new limbo project",
	Long:  `Initialize a new limbo project by creating the .limbo directory structure.`,
	RunE:  runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initPretty, "pretty", false, "Pretty print output")
}

type initResult struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
}

func runInit(cmd *cobra.Command, args []string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create storage at current directory
	store := storage.NewStorageAt(cwd)

	// Initialize the project
	if err := store.Init(); err != nil {
		return err
	}

	result := initResult{
		Success: true,
		Path:    cwd,
	}

	if initPretty {
		green := color.New(color.FgGreen)
		green.Printf("Initialized limbo in %s\n", cwd)
	} else {
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}

	return nil
}
