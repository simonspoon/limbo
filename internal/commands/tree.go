package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/spf13/cobra"
)

var treePretty bool
var treeShowAll bool

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Display tasks in a hierarchical tree view",
	Long:  `Display all tasks in a hierarchical tree structure showing parent-child relationships.`,
	RunE:  runTree,
}

func init() {
	// tree defaults to pretty since JSON hierarchy is awkward
	treeCmd.Flags().BoolVar(&treePretty, "pretty", true, "Pretty print output (default true for tree)")
	treeCmd.Flags().BoolVar(&treeShowAll, "show-all", false, "Show all tasks including completed")
}

func runTree(cmd *cobra.Command, args []string) error {
	// Load storage
	store, err := storage.NewStorage()
	if err != nil {
		return err
	}

	// Load all tasks
	tasks, err := store.LoadAll()
	if err != nil {
		return err
	}

	if !treeShowAll {
		tasks = filterCompletedTasks(tasks)
	}

	if len(tasks) == 0 {
		if treePretty {
			fmt.Println("No tasks found")
		} else {
			fmt.Println("[]")
		}
		return nil
	}

	if !treePretty {
		out, _ := json.Marshal(tasks)
		fmt.Println(string(out))
		return nil
	}

	// Sort tasks by creation time
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Created.Before(tasks[j].Created)
	})

	// Build task map for easy lookup
	taskMap := make(map[string]models.Task)
	for i := range tasks {
		taskMap[tasks[i].ID] = tasks[i]
	}

	// Find root tasks (tasks with no parent)
	var roots []models.Task
	for i := range tasks {
		if tasks[i].Parent == nil {
			roots = append(roots, tasks[i])
		}
	}

	// Print tree for each root
	for i := range roots {
		isLast := i == len(roots)-1
		printTaskTree(os.Stdout, &roots[i], taskMap, "", isLast)
	}

	return nil
}

func printTaskTree(w io.Writer, task *models.Task, taskMap map[string]models.Task, prefix string, isLast bool) {
	boldWhite := color.New(color.Bold, color.FgWhite)
	gray := color.New(color.FgHiBlack)
	statusColor := getStatusColor(task.Status)

	var marker string
	if prefix == "" {
		marker = ""
	} else if isLast {
		marker = "└─ "
	} else {
		marker = "├─ "
	}

	// Format: ID  Name  [STATUS]
	_, _ = fmt.Fprint(w, prefix+marker)
	_, _ = gray.Fprintf(w, "%s  ", task.ID)
	_, _ = boldWhite.Fprint(w, task.Name)
	_, _ = fmt.Fprint(w, "  ")
	_, _ = statusColor.Fprintf(w, "[%s]", formatStatus(task.Status))
	_, _ = fmt.Fprintln(w)

	// Find children
	var children []models.Task
	for id := range taskMap {
		t := taskMap[id]
		if t.Parent != nil && *t.Parent == task.ID {
			children = append(children, t)
		}
	}

	// Sort children by creation time
	sort.Slice(children, func(i, j int) bool {
		return children[i].Created.Before(children[j].Created)
	})

	// Print children recursively
	for i := range children {
		childIsLast := i == len(children)-1
		var childPrefix string
		if prefix == "" {
			childPrefix = "  "
		} else if isLast {
			childPrefix = prefix + "   "
		} else {
			childPrefix = prefix + "│  "
		}
		printTaskTree(w, &children[i], taskMap, childPrefix, childIsLast)
	}
}

func getStatusColor(status string) *color.Color {
	switch status {
	case models.StatusTodo:
		return color.New(color.FgCyan)
	case models.StatusInProgress:
		return color.New(color.FgYellow)
	case models.StatusDone:
		return color.New(color.FgGreen)
	default:
		return color.New(color.FgWhite)
	}
}

func formatStatus(status string) string {
	switch status {
	case models.StatusInProgress:
		return "IN-PROG"
	case models.StatusDone:
		return "DONE"
	case models.StatusTodo:
		return "TODO"
	default:
		return status
	}
}
