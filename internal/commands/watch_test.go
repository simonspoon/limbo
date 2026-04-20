package commands

import (
	"bytes"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/simonspoon/limbo/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestDetectChanges(t *testing.T) {
	now := time.Now()

	prev := map[string]models.Task{
		"aaaa": {ID: "aaaa", Name: "Task 1", Updated: now},
		"aaab": {ID: "aaab", Name: "Task 2", Updated: now},
		"aaac": {ID: "aaac", Name: "Task 3", Updated: now},
	}

	updatedTime := now.Add(time.Second)
	curr := map[string]models.Task{
		"aaaa": {ID: "aaaa", Name: "Task 1", Updated: now},                 // unchanged
		"aaab": {ID: "aaab", Name: "Task 2 Updated", Updated: updatedTime}, // updated
		"aaad": {ID: "aaad", Name: "Task 4", Updated: now},                 // added
	}

	added, updated, deleted := detectChanges(prev, curr)

	assert.ElementsMatch(t, []string{"aaad"}, added)
	assert.ElementsMatch(t, []string{"aaab"}, updated)
	assert.ElementsMatch(t, []string{"aaac"}, deleted)
}

func TestDetectChangesEmpty(t *testing.T) {
	prev := map[string]models.Task{}
	curr := map[string]models.Task{}

	added, updated, deleted := detectChanges(prev, curr)

	assert.Empty(t, added)
	assert.Empty(t, updated)
	assert.Empty(t, deleted)
}

func TestDetectChangesAllNew(t *testing.T) {
	now := time.Now()
	prev := map[string]models.Task{}
	curr := map[string]models.Task{
		"aaaa": {ID: "aaaa", Name: "Task 1", Updated: now},
		"aaab": {ID: "aaab", Name: "Task 2", Updated: now},
	}

	added, updated, deleted := detectChanges(prev, curr)

	assert.ElementsMatch(t, []string{"aaaa", "aaab"}, added)
	assert.Empty(t, updated)
	assert.Empty(t, deleted)
}

func TestDetectChangesAllDeleted(t *testing.T) {
	now := time.Now()
	prev := map[string]models.Task{
		"aaaa": {ID: "aaaa", Name: "Task 1", Updated: now},
		"aaab": {ID: "aaab", Name: "Task 2", Updated: now},
	}
	curr := map[string]models.Task{}

	added, updated, deleted := detectChanges(prev, curr)

	assert.Empty(t, added)
	assert.Empty(t, updated)
	assert.ElementsMatch(t, []string{"aaaa", "aaab"}, deleted)
}

func TestFilterByStatus(t *testing.T) {
	tasks := []models.Task{
		{ID: "aaaa", Name: "Task 1", Status: models.StatusCaptured},
		{ID: "aaab", Name: "Task 2", Status: models.StatusInProgress},
		{ID: "aaac", Name: "Task 3", Status: models.StatusDone},
		{ID: "aaad", Name: "Task 4", Status: models.StatusCaptured},
	}

	filtered := filterByStatus(tasks, models.StatusCaptured)
	assert.Len(t, filtered, 2)
	for _, task := range filtered {
		assert.Equal(t, models.StatusCaptured, task.Status)
	}

	filtered = filterByStatus(tasks, models.StatusInProgress)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "aaab", filtered[0].ID)

	filtered = filterByStatus(tasks, models.StatusDone)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "aaac", filtered[0].ID)
}

func TestFilterByStatusEmpty(t *testing.T) {
	tasks := []models.Task{
		{ID: "aaaa", Name: "Task 1", Status: models.StatusCaptured},
	}

	filtered := filterByStatus(tasks, models.StatusDone)
	assert.Empty(t, filtered)
}

func TestToTaskMap(t *testing.T) {
	tasks := []models.Task{
		{ID: "aaaa", Name: "Task 1"},
		{ID: "aaab", Name: "Task 2"},
		{ID: "aaac", Name: "Task 3"},
	}

	m := toTaskMap(tasks)

	assert.Len(t, m, 3)
	assert.Equal(t, "Task 1", m["aaaa"].Name)
	assert.Equal(t, "Task 2", m["aaab"].Name)
	assert.Equal(t, "Task 3", m["aaac"].Name)
}

func TestCountByStatus(t *testing.T) {
	tasks := []models.Task{
		{ID: "aaaa", Status: models.StatusCaptured},
		{ID: "aaab", Status: models.StatusCaptured},
		{ID: "aaac", Status: models.StatusInProgress},
		{ID: "aaad", Status: models.StatusDone},
		{ID: "aaae", Status: models.StatusDone},
		{ID: "aaaf", Status: models.StatusDone},
	}

	assert.Equal(t, 2, countByStatus(tasks, models.StatusCaptured))
	assert.Equal(t, 1, countByStatus(tasks, models.StatusInProgress))
	assert.Equal(t, 3, countByStatus(tasks, models.StatusDone))
}

func TestIsTaskBlocked(t *testing.T) {
	// Setup: one done task, one non-done task, reference map
	doneBlocker := models.Task{ID: "bbbb", Name: "Done Blocker", Status: models.StatusDone}
	todoBlocker := models.Task{ID: "cccc", Name: "Todo Blocker", Status: models.StatusCaptured}
	allMap := map[string]models.Task{
		"bbbb": doneBlocker,
		"cccc": todoBlocker,
	}

	t.Run("manual block only", func(t *testing.T) {
		task := &models.Task{ID: "aaaa", ManualBlockReason: "stuck on review"}
		assert.True(t, isTaskBlocked(task, allMap))
	})

	t.Run("dep block with non-done blocker", func(t *testing.T) {
		task := &models.Task{ID: "aaaa", BlockedBy: []string{"cccc"}}
		assert.True(t, isTaskBlocked(task, allMap))
	})

	t.Run("dep block with only done blockers", func(t *testing.T) {
		task := &models.Task{ID: "aaaa", BlockedBy: []string{"bbbb"}}
		assert.False(t, isTaskBlocked(task, allMap))
	})

	t.Run("dep block with unknown blocker id treated as blocked", func(t *testing.T) {
		task := &models.Task{ID: "aaaa", BlockedBy: []string{"zzzz"}}
		assert.True(t, isTaskBlocked(task, allMap))
	})

	t.Run("neither manual nor deps", func(t *testing.T) {
		task := &models.Task{ID: "aaaa"}
		assert.False(t, isTaskBlocked(task, allMap))
	})

	t.Run("manual plus dep done still blocked", func(t *testing.T) {
		task := &models.Task{ID: "aaaa", ManualBlockReason: "waiting", BlockedBy: []string{"bbbb"}}
		assert.True(t, isTaskBlocked(task, allMap))
	})
}

func TestCountBlocked(t *testing.T) {
	allMap := map[string]models.Task{
		"bbbb": {ID: "bbbb", Status: models.StatusDone},
		"cccc": {ID: "cccc", Status: models.StatusCaptured},
	}
	tasks := []models.Task{
		{ID: "t001", Status: models.StatusCaptured},                                    // not blocked
		{ID: "t002", Status: models.StatusInProgress, ManualBlockReason: "halt"},       // manual
		{ID: "t003", Status: models.StatusCaptured, BlockedBy: []string{"cccc"}},       // dep-blocked (todo blocker)
		{ID: "t004", Status: models.StatusCaptured, BlockedBy: []string{"bbbb"}},       // not blocked (done blocker)
		{ID: "t005", Status: models.StatusReady, BlockedBy: []string{"cccc", "bbbb"}},  // dep-blocked
	}
	assert.Equal(t, 3, countBlocked(tasks, allMap))
}

func TestPrintTaskTree_BlockedRendering(t *testing.T) {
	// Ensure color output is always emitted so substring assertions stay
	// deterministic across environments.
	prev := color.NoColor
	color.NoColor = true
	t.Cleanup(func() { color.NoColor = prev })

	now := time.Now()
	blocker := models.Task{ID: "bbbb", Name: "Upstream Job", Status: models.StatusInProgress, Created: now, Updated: now}
	manual := models.Task{ID: "aaaa", Name: "Manual Block Task", Status: models.StatusInProgress, ManualBlockReason: "waiting on CI", Created: now, Updated: now}
	dep := models.Task{ID: "aaac", Name: "Dep Block Task", Status: models.StatusCaptured, BlockedBy: []string{"bbbb"}, Created: now, Updated: now}
	plain := models.Task{ID: "aaad", Name: "Plain Task", Status: models.StatusCaptured, Created: now, Updated: now}

	taskMap := map[string]models.Task{
		manual.ID:  manual,
		dep.ID:     dep,
		plain.ID:   plain,
		blocker.ID: blocker,
	}
	allMap := taskMap

	t.Run("showBlocked true emits prefix and sub-line for manual", func(t *testing.T) {
		var buf bytes.Buffer
		m := manual
		printTaskTree(&buf, &m, taskMap, "", true, true, allMap)
		out := buf.String()
		assert.Contains(t, out, "🚫 Manual Block Task")
		assert.Contains(t, out, "↳ waiting on CI")
	})

	t.Run("showBlocked true emits prefix and blocked-by for dep", func(t *testing.T) {
		var buf bytes.Buffer
		d := dep
		printTaskTree(&buf, &d, taskMap, "", true, true, allMap)
		out := buf.String()
		assert.Contains(t, out, "🚫 Dep Block Task")
		assert.Contains(t, out, "↳ blocked by: Upstream Job")
	})

	t.Run("showBlocked false suppresses prefix and sub-line", func(t *testing.T) {
		var buf bytes.Buffer
		m := manual
		printTaskTree(&buf, &m, taskMap, "", true, false, nil)
		out := buf.String()
		assert.NotContains(t, out, "🚫")
		assert.NotContains(t, out, "↳")
	})

	t.Run("non-blocked task has no prefix even when showBlocked true", func(t *testing.T) {
		var buf bytes.Buffer
		p := plain
		printTaskTree(&buf, &p, taskMap, "", true, true, allMap)
		out := buf.String()
		assert.NotContains(t, out, "🚫")
		assert.NotContains(t, out, "↳")
	})

	t.Run("dep blocker already done does not emit blocked-by line", func(t *testing.T) {
		doneBlocker := models.Task{ID: "bbbb", Name: "Done Job", Status: models.StatusDone}
		allMapDone := map[string]models.Task{"bbbb": doneBlocker}
		task := models.Task{ID: "aaaa", Name: "Has Done Blocker", Status: models.StatusReady, BlockedBy: []string{"bbbb"}}
		var buf bytes.Buffer
		printTaskTree(&buf, &task, map[string]models.Task{task.ID: task}, "", true, true, allMapDone)
		out := buf.String()
		assert.NotContains(t, out, "🚫")
		assert.NotContains(t, out, "↳")
	})
}

func TestWatchInvalidStatus(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Set invalid status
	watchStatus = "invalid"
	watchPretty = false
	watchInterval = 100 * time.Millisecond

	err := runWatch(nil, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}
