package commands

import (
	"fmt"
	"sort"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
)

// LoadSubtree returns all transitive descendants of rootID (excluding rootID
// itself) by walking the parent->children graph built from storage.LoadAllIndex.
//
// A single disk read is performed (metadata only, no context files). A visited
// set guards against parent-pointer cycles: if one is detected, an error is
// returned instead of an infinite loop. If rootID is not present in the store,
// an error is returned.
//
// The returned slice contains value copies of the descendant tasks.
func LoadSubtree(store *storage.Storage, rootID string) ([]models.Task, error) {
	all, err := store.LoadAllIndex()
	if err != nil {
		return nil, err
	}

	// Confirm rootID exists in the loaded set.
	rootFound := false
	for i := range all {
		if all[i].ID == rootID {
			rootFound = true
			break
		}
	}
	if !rootFound {
		return nil, fmt.Errorf("task not found: %s", rootID)
	}

	// Build parentID -> []*Task map from the full set.
	children := make(map[string][]*models.Task, len(all))
	for i := range all {
		t := &all[i]
		if t.Parent != nil {
			children[*t.Parent] = append(children[*t.Parent], t)
		}
	}

	// BFS from rootID, skipping rootID itself in the result.
	visited := make(map[string]bool, len(all))
	visited[rootID] = true
	queue := []string{rootID}
	var result []models.Task

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, child := range children[current] {
			if visited[child.ID] {
				return nil, fmt.Errorf("cycle detected at task %s", child.ID)
			}
			visited[child.ID] = true
			result = append(result, *child)
			queue = append(queue, child.ID)
		}
	}

	return result, nil
}

// BottomUpCleanup walks the pre-loaded task slice in post-order and promotes
// non-leaf tasks whose children are all StatusDone. Promotion mutates the
// in-memory slice in place (via pointers into the slice) AND persists via
// store.SaveTask, so a subsequent FindNextLeaf on the same slice observes the
// new statuses without re-reading from disk.
//
// Rules:
//   - Leaves (no children in the slice) are never touched.
//   - Nodes already StatusDone are skipped (idempotent; respects manually-done
//     nodes with bespoke outcomes).
//   - A non-leaf whose children are all StatusDone is promoted: Status is set
//     to StatusDone and Outcome to "All subtasks completed".
//
// A visited-set guards against parent-pointer cycles in the slice; cycles
// return an error rather than infinite-looping.
func BottomUpCleanup(store *storage.Storage, tasks []models.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	// Build byID map pointing into the slice so mutations are observable to
	// the caller, plus a parent->children map.
	byID := make(map[string]*models.Task, len(tasks))
	for i := range tasks {
		byID[tasks[i].ID] = &tasks[i]
	}
	childMap := make(map[string][]*models.Task, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		if t.Parent != nil {
			if _, ok := byID[*t.Parent]; ok {
				childMap[*t.Parent] = append(childMap[*t.Parent], t)
			}
		}
	}

	// Roots of the slice = tasks whose Parent is nil OR whose parent is not in
	// the slice. Using these as entry points ensures every node is reachable
	// in the post-order walk below.
	var roots []*models.Task
	for i := range tasks {
		t := &tasks[i]
		if t.Parent == nil {
			roots = append(roots, t)
			continue
		}
		if _, ok := byID[*t.Parent]; !ok {
			roots = append(roots, t)
		}
	}

	visited := make(map[string]bool, len(tasks))
	// onStack tracks the current DFS stack so we can distinguish a back-edge
	// (cycle) from a cross-visit (which shouldn't happen in a tree but is safe
	// to handle).
	onStack := make(map[string]bool, len(tasks))

	var walk func(node *models.Task) error
	walk = func(node *models.Task) error {
		if onStack[node.ID] {
			return fmt.Errorf("cycle detected at task %s", node.ID)
		}
		if visited[node.ID] {
			return nil
		}
		onStack[node.ID] = true
		for _, child := range childMap[node.ID] {
			if err := walk(child); err != nil {
				return err
			}
		}
		onStack[node.ID] = false
		visited[node.ID] = true

		// Post-order processing: evaluate promotion after children are done.
		if node.Status == models.StatusDone {
			return nil
		}
		kids := childMap[node.ID]
		if len(kids) == 0 {
			return nil // leaf: never touched
		}
		for _, child := range kids {
			if child.Status != models.StatusDone {
				return nil
			}
		}
		node.Status = models.StatusDone
		node.Outcome = "All subtasks completed"
		// Persist via the store. SaveTask takes a pointer; passing the slice
		// pointer keeps a single source of truth.
		if err := store.SaveTask(node); err != nil {
			return err
		}
		return nil
	}

	for _, root := range roots {
		if err := walk(root); err != nil {
			return err
		}
	}
	return nil
}

// FindNextLeaf returns the earliest-Created eligible leaf in tasks, or nil if
// none qualifies. It is pure: no store access, no disk I/O. Callers that want
// cleanup-aware results should run BottomUpCleanup on the same slice first.
//
// A task is an eligible leaf when:
//   - It has zero children within the provided slice (strict leaf).
//   - Its Status is not StatusDone.
//   - Its ManualBlockReason is empty.
//   - Every BlockedBy entry either refers to an ID not present in the slice
//     (dangling blockers treated as resolved, matching storage.IsBlocked
//     semantics) or refers to a task in the slice with Status == StatusDone.
//
// Ties on Created.UnixNano are broken by lexicographic ID ascending so that
// the result is deterministic even when generateRandomAlphaID produces two
// tasks in the same nanosecond.
func FindNextLeaf(tasks []models.Task) *models.Task {
	if len(tasks) == 0 {
		return nil
	}

	// Build ID -> *Task for blocker lookup.
	byID := make(map[string]*models.Task, len(tasks))
	for i := range tasks {
		byID[tasks[i].ID] = &tasks[i]
	}

	// Build ID -> has children flag (strict leaf detection).
	hasChild := make(map[string]bool, len(tasks))
	for i := range tasks {
		t := &tasks[i]
		if t.Parent == nil {
			continue
		}
		hasChild[*t.Parent] = true
	}

	var candidates []*models.Task
	for i := range tasks {
		t := &tasks[i]
		if hasChild[t.ID] {
			continue // not a strict leaf
		}
		if t.Status == models.StatusDone {
			continue
		}
		if t.ManualBlockReason != "" {
			continue
		}
		blocked := false
		for _, blockerID := range t.BlockedBy {
			blocker, ok := byID[blockerID]
			if !ok {
				continue // dangling = resolved
			}
			if blocker.Status != models.StatusDone {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		candidates = append(candidates, t)
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].Created.Equal(candidates[j].Created) {
			return candidates[i].Created.Before(candidates[j].Created)
		}
		return candidates[i].ID < candidates[j].ID
	})

	return candidates[0]
}
