package commands

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var (
	listStatus    string
	listPretty    bool
	listOwner     string
	listUnclaimed bool
	listBlocked   bool
	listUnblocked bool
	listShowAll   bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	Long:  `List tasks with optional filtering by status, owner, or blocked state.`,
	RunE:  runList,
}

func init() {
	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "Filter by status (todo|in-progress|done)")
	listCmd.Flags().BoolVar(&listPretty, "pretty", false, "Pretty print output")
	listCmd.Flags().StringVar(&listOwner, "owner", "", "Filter to tasks owned by this agent")
	listCmd.Flags().BoolVar(&listUnclaimed, "unclaimed", false, "Filter to tasks with no owner")
	listCmd.Flags().BoolVar(&listBlocked, "blocked", false, "Show only blocked tasks")
	listCmd.Flags().BoolVar(&listUnblocked, "unblocked", false, "Show only unblocked tasks")
	listCmd.Flags().BoolVar(&listShowAll, "show-all", false, "Show all tasks including completed")
}

func runList(cmd *cobra.Command, args []string) error {
	if err := validateListFlags(); err != nil {
		return err
	}

	store, err := getStorage()
	if err != nil {
		return err
	}

	tasks, err := store.LoadAll()
	if err != nil {
		return err
	}

	tasks, err = applyListFilters(tasks, store)
	if err != nil {
		return err
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Created.Before(tasks[j].Created)
	})

	if listPretty {
		printTasksPretty(tasks)
	} else {
		out, _ := json.Marshal(tasks)
		fmt.Println(string(out))
	}

	return nil
}

func validateListFlags() error {
	if listOwner != "" && listUnclaimed {
		return fmt.Errorf("--owner and --unclaimed are mutually exclusive")
	}
	if listBlocked && listUnblocked {
		return fmt.Errorf("--blocked and --unblocked are mutually exclusive")
	}
	if listStatus != "" && !models.IsValidStatus(listStatus) {
		return fmt.Errorf("invalid status %q. Must be: captured, refined, planned, ready, in-progress, in-review, done", listStatus)
	}
	return nil
}

func applyListFilters(tasks []models.Task, store *storage.Storage) ([]models.Task, error) {
	if listStatus != "" {
		tasks = filterTasksByStatus(tasks, listStatus)
	}
	if listOwner != "" {
		tasks = filterByOwner(tasks, listOwner)
	}
	if listUnclaimed {
		tasks = filterUnclaimed(tasks)
	}
	if listBlocked {
		var err error
		tasks, err = filterBlocked(tasks, store, true)
		if err != nil {
			return nil, err
		}
	}
	if listUnblocked {
		var err error
		tasks, err = filterBlocked(tasks, store, false)
		if err != nil {
			return nil, err
		}
	}
	if !listShowAll {
		tasks = filterCompletedTasks(tasks)
	}
	return tasks, nil
}

func filterTasksByStatus(tasks []models.Task, status string) []models.Task {
	var filtered []models.Task
	for i := range tasks {
		if tasks[i].Status == status {
			filtered = append(filtered, tasks[i])
		}
	}
	return filtered
}

func filterByOwner(tasks []models.Task, owner string) []models.Task {
	var filtered []models.Task
	for i := range tasks {
		if tasks[i].Owner != nil && *tasks[i].Owner == owner {
			filtered = append(filtered, tasks[i])
		}
	}
	return filtered
}

func filterUnclaimed(tasks []models.Task) []models.Task {
	var filtered []models.Task
	for i := range tasks {
		if tasks[i].Owner == nil {
			filtered = append(filtered, tasks[i])
		}
	}
	return filtered
}

func filterBlocked(tasks []models.Task, store *storage.Storage, wantBlocked bool) ([]models.Task, error) {
	var filtered []models.Task
	for i := range tasks {
		blocked, err := store.IsBlocked(&tasks[i])
		if err != nil {
			return nil, err
		}
		if blocked == wantBlocked {
			filtered = append(filtered, tasks[i])
		}
	}
	return filtered, nil
}

func printTasksPretty(tasks []models.Task) {
	if len(tasks) == 0 {
		fmt.Println("No tasks found.")
		return
	}

	// Group by status
	grouped := make(map[string][]models.Task)
	for i := range tasks {
		grouped[tasks[i].Status] = append(grouped[tasks[i].Status], tasks[i])
	}

	// Status order
	statuses := []string{
		models.StatusCaptured,
		models.StatusRefined,
		models.StatusPlanned,
		models.StatusReady,
		models.StatusInProgress,
		models.StatusInReview,
		models.StatusDone,
	}

	// Colors
	statusColors := map[string]*color.Color{
		models.StatusCaptured:   color.New(color.FgCyan),
		models.StatusRefined:    color.New(color.FgBlue),
		models.StatusPlanned:    color.New(color.FgMagenta),
		models.StatusReady:      color.New(color.FgWhite, color.Bold),
		models.StatusInProgress: color.New(color.FgYellow),
		models.StatusInReview:   color.New(color.FgHiYellow),
		models.StatusDone:       color.New(color.FgGreen),
	}

	for _, status := range statuses {
		group := grouped[status]
		if len(group) == 0 {
			continue
		}

		statusColor := statusColors[status]
		statusColor.Printf("\n%s (%d)\n", status, len(group))

		for i := range group {
			fmt.Printf("  %s  %s\n", group[i].ID, group[i].Name)
		}
	}
}
