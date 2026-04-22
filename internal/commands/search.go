package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/spf13/cobra"
)

var (
	searchPretty  bool
	searchShowAll bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search tasks by name or description",
	Long: `Search tasks by matching a query string against task name and description (case-insensitive substring match).

Output is JSON by default. Use --pretty for a human-readable format. The --json
flag is accepted as a no-op for script compatibility (JSON is already the default).`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().BoolVar(&searchPretty, "pretty", false, "Pretty print output")
	searchCmd.Flags().BoolVar(&searchShowAll, "show-all", false, "Show all tasks including completed")
	// --json is a no-op: JSON is already the default output. Accepted so
	// scripts that pass --json defensively do not fail with "unknown flag".
	searchCmd.Flags().Bool("json", false, "Emit JSON output (no-op; JSON is the default)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.ToLower(args[0])

	store, err := getStorage()
	if err != nil {
		return err
	}

	tasks, err := store.LoadAll()
	if err != nil {
		return err
	}

	if !searchShowAll {
		tasks = filterCompletedTasks(tasks)
	}

	var matched []models.Task
	for i := range tasks {
		name := strings.ToLower(tasks[i].Name)
		desc := strings.ToLower(tasks[i].Description)
		if strings.Contains(name, query) || strings.Contains(desc, query) {
			matched = append(matched, tasks[i])
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].Created.Before(matched[j].Created)
	})

	if searchPretty {
		printTasksPretty(matched)
	} else {
		out, _ := json.Marshal(matched)
		fmt.Println(string(out))
	}

	return nil
}
