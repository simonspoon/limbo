package commands

import (
	"testing"
	"time"

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
