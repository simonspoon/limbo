package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var showPretty bool

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show task details",
	Long:  `Display detailed information about a task.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	showCmd.Flags().BoolVar(&showPretty, "pretty", false, "Pretty print output")
}

type blockerInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type showResult struct {
	*models.Task
	Blockers []blockerInfo `json:"blockers,omitempty"`
	Blocks   []blockerInfo `json:"blocks,omitempty"`
}

func runShow(cmd *cobra.Command, args []string) error {
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

	// Load task
	task, err := store.LoadTask(id)
	if err != nil {
		return err
	}

	// Load all tasks for dependency resolution
	allTasks, err := store.LoadAll()
	if err != nil {
		return err
	}

	// Resolve blockers: for each ID in BlockedBy, resolve to {id, name, status}
	var blockers []blockerInfo
	for _, blockerID := range task.BlockedBy {
		if info := findBlockerInfo(allTasks, blockerID); info != nil {
			blockers = append(blockers, *info)
		}
	}

	// Reverse lookup: find all tasks whose BlockedBy contains this task's ID
	var blocks []blockerInfo
	for i := range allTasks {
		for _, depID := range allTasks[i].BlockedBy {
			if depID == id {
				blocks = append(blocks, blockerInfo{
					ID:     allTasks[i].ID,
					Name:   allTasks[i].Name,
					Status: allTasks[i].Status,
				})
				break
			}
		}
	}

	if showPretty {
		printTaskDetails(task, blockers, blocks)
	} else {
		result := showResult{
			Task:     task,
			Blockers: blockers,
			Blocks:   blocks,
		}
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}

	return nil
}

func findBlockerInfo(tasks []models.Task, id string) *blockerInfo {
	for i := range tasks {
		if tasks[i].ID == id {
			return &blockerInfo{
				ID:     tasks[i].ID,
				Name:   tasks[i].Name,
				Status: tasks[i].Status,
			}
		}
	}
	return nil
}

func printTaskDetails(task *models.Task, blockers, blocks []blockerInfo) {
	cyan := color.New(color.FgCyan, color.Bold)
	white := color.New(color.FgWhite)
	gray := color.New(color.FgHiBlack)
	yellow := color.New(color.FgYellow)

	separator := strings.Repeat("-", 60)
	cyan.Println(separator)
	cyan.Printf("Task: %s\n", task.ID)
	cyan.Println(separator)
	fmt.Println()

	white.Printf("Name:        %s\n", task.Name)

	if task.Description != "" {
		white.Printf("Description: %s\n", task.Description)
	}

	if task.Action != "" {
		white.Printf("Action:      %s\n", task.Action)
	}
	if task.Verify != "" {
		white.Printf("Verify:      %s\n", task.Verify)
	}
	if task.Result != "" {
		white.Printf("Result:      %s\n", task.Result)
	}
	if task.Outcome != "" {
		green := color.New(color.FgGreen)
		green.Printf("Outcome:     %s\n", task.Outcome)
	}

	white.Printf("Status:      %s\n", task.Status)

	if task.Parent != nil {
		white.Printf("Parent:      %s\n", *task.Parent)
	} else {
		white.Println("Parent:      none")
	}

	if task.Owner != nil {
		white.Printf("Owner:       %s\n", *task.Owner)
	}

	if len(blockers) > 0 {
		fmt.Println()
		yellow.Println("Blocked by:")
		for _, b := range blockers {
			white.Printf("  %s - %s (%s)\n", b.ID, b.Name, b.Status)
		}
	}

	if len(blocks) > 0 {
		fmt.Println()
		yellow.Println("Blocks:")
		for _, b := range blocks {
			white.Printf("  %s - %s (%s)\n", b.ID, b.Name, b.Status)
		}
	}

	gray.Printf("Created:     %s\n", task.Created.Format("2006-01-02 15:04:05"))
	gray.Printf("Updated:     %s\n", task.Updated.Format("2006-01-02 15:04:05"))

	if len(task.Notes) > 0 {
		fmt.Println()
		yellow.Println("Notes:")
		for _, note := range task.Notes {
			gray.Printf("  [%s] ", note.Timestamp.Format("2006-01-02 15:04"))
			white.Printf("%s\n", note.Content)
		}
	}
}
