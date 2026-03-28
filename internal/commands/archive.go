package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/spf13/cobra"
)

var archiveListPretty bool
var archiveShowPretty bool
var archiveRestorePretty bool
var archivePurgePretty bool

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Manage archived tasks",
	Long:  `View, restore, or purge tasks that were archived by the prune command.`,
}

var archiveListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all archived tasks",
	Long:  `List all tasks in the archive.`,
	RunE:  runArchiveList,
}

var archiveShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show archived task details",
	Long:  `Display detailed information about an archived task.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runArchiveShow,
}

var archiveRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore an archived task",
	Long:  `Move an archived task back to the active store with status 'done'. Fails if the task ID already exists in the active store.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runArchiveRestore,
}

var archivePurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Permanently delete all archived tasks",
	Long:  `Permanently delete all tasks from the archive. This cannot be undone.`,
	RunE:  runArchivePurge,
}

func init() {
	archiveListCmd.Flags().BoolVar(&archiveListPretty, "pretty", false, "Pretty print output")
	archiveShowCmd.Flags().BoolVar(&archiveShowPretty, "pretty", false, "Pretty print output")
	archiveRestoreCmd.Flags().BoolVar(&archiveRestorePretty, "pretty", false, "Pretty print output")
	archivePurgeCmd.Flags().BoolVar(&archivePurgePretty, "pretty", false, "Pretty print output")

	archiveCmd.AddCommand(archiveListCmd)
	archiveCmd.AddCommand(archiveShowCmd)
	archiveCmd.AddCommand(archiveRestoreCmd)
	archiveCmd.AddCommand(archivePurgeCmd)
}

func runArchiveList(cmd *cobra.Command, args []string) error {
	store, err := getStorage()
	if err != nil {
		return err
	}

	tasks, err := store.LoadArchive()
	if err != nil {
		return err
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Created.Before(tasks[j].Created)
	})

	if archiveListPretty {
		if len(tasks) == 0 {
			fmt.Println("No archived tasks.")
			return nil
		}
		gray := color.New(color.FgHiBlack)
		gray.Printf("Archive (%d)\n", len(tasks))
		for i := range tasks {
			fmt.Printf("  %s  %s\n", tasks[i].ID, tasks[i].Name)
		}
	} else {
		out, _ := json.Marshal(tasks)
		fmt.Println(string(out))
	}

	return nil
}

func runArchiveShow(cmd *cobra.Command, args []string) error {
	id := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(id) {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	task, err := store.LoadArchivedTask(id)
	if err != nil {
		return err
	}

	if archiveShowPretty {
		printTaskDetails(task, nil, nil)
	} else {
		out, _ := json.Marshal(task)
		fmt.Println(string(out))
	}

	return nil
}

type restoreResult struct {
	Restored string `json:"restored"`
	Warning  string `json:"warning,omitempty"`
}

func runArchiveRestore(cmd *cobra.Command, args []string) error {
	id := models.NormalizeTaskID(args[0])
	if !models.IsValidTaskID(id) {
		return fmt.Errorf("invalid task ID: %s", args[0])
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	// Check for ID collision in active store
	_, err = store.LoadTask(id)
	if err == nil {
		return fmt.Errorf("task ID %s already exists in active store", id)
	}

	// Remove from archive
	task, err := store.UnarchiveTask(id)
	if err != nil {
		return err
	}

	var warnings []string

	// Clear stale BlockedBy references (check each against active store)
	if len(task.BlockedBy) > 0 {
		var validBlockers []string
		for _, blockerID := range task.BlockedBy {
			if _, err := store.LoadTask(blockerID); err == nil {
				validBlockers = append(validBlockers, blockerID)
			}
		}
		if len(validBlockers) != len(task.BlockedBy) {
			warnings = append(warnings, "cleared stale BlockedBy references")
		}
		task.BlockedBy = validBlockers
	}

	// Orphan Parent if not in active store
	if task.Parent != nil {
		if _, err := store.LoadTask(*task.Parent); err != nil {
			warnings = append(warnings, fmt.Sprintf("orphaned parent %s (not in active store)", *task.Parent))
			task.Parent = nil
		}
	}

	// Restore as done
	task.Status = models.StatusDone
	task.Updated = time.Now()

	if err := store.SaveTask(task); err != nil {
		return err
	}

	warningStr := ""
	if len(warnings) > 0 {
		warningStr = fmt.Sprintf("%v", warnings)
	}

	if archiveRestorePretty {
		green := color.New(color.FgGreen)
		green.Printf("Restored task %s: %s\n", task.ID, task.Name)
		if warningStr != "" {
			yellow := color.New(color.FgYellow)
			yellow.Printf("Warnings: %s\n", warningStr)
		}
	} else {
		result := restoreResult{
			Restored: id,
			Warning:  warningStr,
		}
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}

	return nil
}

type purgeResult struct {
	Purged int `json:"purged"`
}

func runArchivePurge(cmd *cobra.Command, args []string) error {
	store, err := getStorage()
	if err != nil {
		return err
	}

	// Count archived tasks before purging
	tasks, err := store.LoadArchive()
	if err != nil {
		return err
	}

	count := len(tasks)

	if err := store.PurgeArchive(); err != nil {
		return err
	}

	if archivePurgePretty {
		if count == 0 {
			fmt.Println("No archived tasks to purge")
		} else {
			green := color.New(color.FgGreen)
			green.Printf("Purged %d archived task(s)\n", count)
		}
	} else {
		result := purgeResult{Purged: count}
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}

	return nil
}
