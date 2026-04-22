package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/spf13/cobra"
)

var treePretty bool
var treeShowAll bool

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Display tasks in a hierarchical tree view",
	Long: `Display all tasks in a hierarchical tree structure showing parent-child relationships.

Tree defaults to a pretty, human-readable format (--pretty=true). Pass --pretty=false
for JSON output. The --json flag is accepted as a no-op for script compatibility;
it does NOT override the --pretty default. To force JSON, use --pretty=false.`,
	RunE: runTree,
}

func init() {
	// tree defaults to pretty since JSON hierarchy is awkward
	treeCmd.Flags().BoolVar(&treePretty, "pretty", true, "Pretty print output (default true for tree)")
	treeCmd.Flags().BoolVar(&treeShowAll, "show-all", false, "Show all tasks including completed")
	// --json is a no-op accepted for script compatibility. Tree defaults to
	// --pretty=true, so --json alone does NOT produce JSON; callers wanting
	// JSON output must pass --pretty=false.
	treeCmd.Flags().Bool("json", false, "Accepted for compatibility (no-op; use --pretty=false for JSON)")
}

func runTree(cmd *cobra.Command, args []string) error {
	// Load storage
	store, err := getStorage()
	if err != nil {
		return err
	}

	// Load all tasks
	tasks, err := store.LoadAll()
	if err != nil {
		return err
	}

	// Snapshot unfiltered task map for blocked detection so blocker names
	// resolve even when filterCompletedTasks would hide done blockers.
	// Mirrors watch.go loading an unfiltered map alongside the render slice.
	allTaskMap := toTaskMap(tasks)

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

	// Print tree for each root. Match watch --pretty blocked rendering:
	// 🚫 prefix on blocked tasks + dimmed ↳ sub-lines for reason and
	// non-done blocker names.
	for i := range roots {
		isLast := i == len(roots)-1
		printTaskTree(os.Stdout, &roots[i], taskMap, "", isLast, true, allTaskMap)
	}

	return nil
}

// printTaskTree renders a task subtree.
//
// When showBlocked is true, blocked tasks are prefixed with 🚫 before the
// name and an indented dimmed sub-line describes the block context
// (manual reason and/or "blocked by: <name>" per non-done blocker).
// allTaskMap is the unfiltered task map used to resolve blocker status +
// names; it may be nil when showBlocked is false.
func printTaskTree(w io.Writer, task *models.Task, taskMap map[string]models.Task, prefix string, isLast bool, showBlocked bool, allTaskMap map[string]models.Task) {
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

	blocked := showBlocked && isTaskBlocked(task, allTaskMap)

	// Format: ID  [🚫 ]Name  [STATUS]
	_, _ = fmt.Fprint(w, prefix+marker)
	_, _ = gray.Fprintf(w, "%s  ", task.ID)
	if blocked {
		_, _ = fmt.Fprint(w, "🚫 ")
	}
	_, _ = boldWhite.Fprint(w, task.Name)
	_, _ = fmt.Fprint(w, "  ")
	_, _ = statusColor.Fprintf(w, "[%s]", formatStatus(task.Status))
	_, _ = fmt.Fprintln(w)

	// Compute child indentation now so the block sub-line aligns with
	// where children would appear.
	var childPrefix string
	if prefix == "" {
		childPrefix = "  "
	} else if isLast {
		childPrefix = prefix + "   "
	} else {
		childPrefix = prefix + "│  "
	}

	// Block context sub-line(s)
	if blocked {
		if task.ManualBlockReason != "" {
			_, _ = gray.Fprintf(w, "%s↳ %s\n", childPrefix, task.ManualBlockReason)
		}
		for _, blockerID := range task.BlockedBy {
			blocker, ok := allTaskMap[blockerID]
			if ok && blocker.Status == models.StatusDone {
				continue
			}
			name := blockerID
			if ok && blocker.Name != "" {
				name = blocker.Name
			}
			_, _ = gray.Fprintf(w, "%s↳ blocked by: %s\n", childPrefix, name)
		}
	}

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
		printTaskTree(w, &children[i], taskMap, childPrefix, childIsLast, showBlocked, allTaskMap)
	}
}

func getStatusColor(status string) *color.Color {
	switch status {
	case models.StatusCaptured:
		return color.New(color.FgCyan)
	case models.StatusRefined:
		return color.New(color.FgBlue)
	case models.StatusPlanned:
		return color.New(color.FgMagenta)
	case models.StatusReady:
		return color.New(color.FgWhite, color.Bold)
	case models.StatusInProgress:
		return color.New(color.FgYellow)
	case models.StatusInReview:
		return color.New(color.FgHiYellow)
	case models.StatusDone:
		return color.New(color.FgGreen)
	default:
		return color.New(color.FgWhite)
	}
}

func formatStatus(status string) string {
	switch status {
	case models.StatusCaptured:
		return "CAPTURED"
	case models.StatusRefined:
		return "REFINED"
	case models.StatusPlanned:
		return "PLANNED"
	case models.StatusReady:
		return "READY"
	case models.StatusInProgress:
		return "IN-PROG"
	case models.StatusInReview:
		return "REVIEW"
	case models.StatusDone:
		return "DONE"
	default:
		return strings.ToUpper(status)
	}
}
