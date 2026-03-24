package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	watchInterval time.Duration
	watchPretty   bool
	watchStatus   string
	watchShowAll  bool
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch tasks for changes",
	Long:  `Continuously monitor tasks and display updates. Press q or Ctrl+C to exit.`,
	RunE:  runWatch,
}

func init() {
	watchCmd.Flags().DurationVar(&watchInterval, "interval", 500*time.Millisecond, "Polling interval")
	watchCmd.Flags().BoolVar(&watchPretty, "pretty", false, "Human-readable output (clear & redraw)")
	watchCmd.Flags().StringVar(&watchStatus, "status", "", "Filter by status (todo|in-progress|done)")
	watchCmd.Flags().BoolVar(&watchShowAll, "show-all", false, "Show all tasks including completed")
}

// WatchEvent represents a change event for JSON output
type WatchEvent struct {
	Type      string        `json:"type"`
	Task      *models.Task  `json:"task,omitempty"`
	Tasks     []models.Task `json:"tasks,omitempty"`
	TaskID    string        `json:"taskId,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

func runWatch(cmd *cobra.Command, args []string) error {
	store, err := getStorage()
	if err != nil {
		return err
	}

	// Validate status filter
	if watchStatus != "" && !models.IsValidStatus(watchStatus) {
		return fmt.Errorf("invalid status %q. Must be: todo, in-progress, done", watchStatus)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Enter raw mode when pretty-printing to an interactive terminal
	rawMode, cleanup := setupRawMode(watchPretty, cancel)
	if cleanup != nil {
		defer cleanup()
	}

	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	var prevTasks map[string]models.Task
	first := true

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			tasks, err := store.LoadAll()
			if err != nil {
				continue
			}

			// Filter by status if specified
			if watchStatus != "" {
				tasks = filterByStatus(tasks, watchStatus)
			}

			if !watchShowAll {
				tasks = filterCompletedTasks(tasks)
			}

			// Sort by created time
			sort.Slice(tasks, func(i, j int) bool {
				return tasks[i].Created.Before(tasks[j].Created)
			})

			currTasks := toTaskMap(tasks)

			if watchPretty {
				clearAndRender(tasks, rawMode)
			} else {
				if first {
					outputSnapshot(tasks)
					first = false
				} else {
					outputChanges(prevTasks, currTasks)
				}
			}

			prevTasks = currTasks
		}
	}
}

func filterByStatus(tasks []models.Task, status string) []models.Task {
	var filtered []models.Task
	for i := range tasks {
		if tasks[i].Status == status {
			filtered = append(filtered, tasks[i])
		}
	}
	return filtered
}

func toTaskMap(tasks []models.Task) map[string]models.Task {
	m := make(map[string]models.Task)
	for i := range tasks {
		m[tasks[i].ID] = tasks[i]
	}
	return m
}

func detectChanges(prev, curr map[string]models.Task) (added, updated, deleted []string) {
	for id := range curr {
		task := curr[id]
		if _, exists := prev[id]; !exists {
			added = append(added, id)
		} else if !prev[id].Updated.Equal(task.Updated) {
			updated = append(updated, id)
		}
	}
	for id := range prev {
		if _, exists := curr[id]; !exists {
			deleted = append(deleted, id)
		}
	}
	return
}

func outputSnapshot(tasks []models.Task) {
	event := WatchEvent{
		Type:      "snapshot",
		Tasks:     tasks,
		Timestamp: time.Now(),
	}
	out, _ := json.Marshal(event)
	fmt.Println(string(out))
}

func outputChanges(prev, curr map[string]models.Task) {
	added, updated, deleted := detectChanges(prev, curr)

	now := time.Now()

	for _, id := range added {
		task := curr[id]
		event := WatchEvent{
			Type:      "added",
			Task:      &task,
			Timestamp: now,
		}
		out, _ := json.Marshal(event)
		fmt.Println(string(out))
	}

	for _, id := range updated {
		task := curr[id]
		event := WatchEvent{
			Type:      "updated",
			Task:      &task,
			Timestamp: now,
		}
		out, _ := json.Marshal(event)
		fmt.Println(string(out))
	}

	for _, id := range deleted {
		event := WatchEvent{
			Type:      "deleted",
			TaskID:    id,
			Timestamp: now,
		}
		out, _ := json.Marshal(event)
		fmt.Println(string(out))
	}
}

func clearAndRender(tasks []models.Task, rawMode bool) {
	var buf bytes.Buffer

	// Clear screen using ANSI escape codes
	fmt.Fprint(&buf, "\033[H\033[2J")

	// Header
	fmt.Fprintf(&buf, "limbo watch - %s\n", time.Now().Format("15:04:05"))
	fmt.Fprintf(&buf, "Tasks: %d todo, %d in-progress, %d done\n\n",
		countByStatus(tasks, models.StatusTodo),
		countByStatus(tasks, models.StatusInProgress),
		countByStatus(tasks, models.StatusDone))

	if len(tasks) == 0 {
		fmt.Fprintln(&buf, "No tasks found.")
	} else {
		// Build task map for tree rendering
		taskMap := make(map[string]models.Task)
		for i := range tasks {
			taskMap[tasks[i].ID] = tasks[i]
		}

		// Find root tasks
		var roots []models.Task
		for i := range tasks {
			if tasks[i].Parent == nil {
				roots = append(roots, tasks[i])
			}
		}

		// Print tree for each root
		for i := range roots {
			isLast := i == len(roots)-1
			printTaskTree(&buf, &roots[i], taskMap, "", isLast)
		}
	}

	fmt.Fprintln(&buf, "\nPress q to quit")

	output := buf.String()
	if rawMode {
		output = strings.ReplaceAll(output, "\n", "\r\n")
	}
	_, _ = fmt.Fprint(os.Stdout, output)
}

// setupRawMode enters raw terminal mode when pretty-printing to an interactive
// terminal. It returns whether raw mode is active and a cleanup function to
// restore the terminal (nil if raw mode was not entered).
func setupRawMode(pretty bool, cancel context.CancelFunc) (bool, func()) {
	if !pretty {
		return false, nil
	}
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return false, nil
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return false, nil
	}

	// Read keystrokes in background
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				return
			}
			switch buf[0] {
			case 'q', 'Q', 3: // 'q', 'Q', or Ctrl+C
				cancel()
				return
			}
		}
	}()

	return true, func() { _ = term.Restore(fd, oldState) }
}

func countByStatus(tasks []models.Task, status string) int {
	count := 0
	for i := range tasks {
		if tasks[i].Status == status {
			count++
		}
	}
	return count
}
