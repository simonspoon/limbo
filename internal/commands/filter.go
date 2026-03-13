package commands

import "github.com/simonspoon/limbo/internal/models"

// filterCompletedTasks removes done tasks that are "fully resolved" from the display.
// A done task is hidden if:
//   - it has no parent (top-level done task), OR
//   - its parent is also done
//
// A done task is shown only if its parent exists AND is not done (i.e., it's a completed subtask of active work).
func filterCompletedTasks(tasks []models.Task) []models.Task {
	// Build a map of task ID -> task for O(1) parent lookups
	byID := make(map[string]models.Task, len(tasks))
	for i := range tasks {
		byID[tasks[i].ID] = tasks[i]
	}

	var result []models.Task
	for i := range tasks {
		if tasks[i].Status != models.StatusDone {
			// Always keep non-done tasks
			result = append(result, tasks[i])
			continue
		}

		// Done task: keep only if it has a parent that is not done
		if tasks[i].Parent != nil {
			if parent, ok := byID[*tasks[i].Parent]; ok && parent.Status != models.StatusDone {
				result = append(result, tasks[i])
			}
		}
	}

	return result
}
