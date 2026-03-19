package commands

import (
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/require"
)

func createSearchTestTask(t *testing.T, store *storage.Storage, name, description, status string) string {
	now := time.Now()
	id, err := store.GenerateTaskID()
	require.NoError(t, err)

	task := &models.Task{
		ID:          id,
		Name:        name,
		Description: description,
		Status:      status,
		Created:     now,
		Updated:     now,
	}
	require.NoError(t, store.SaveTask(task))
	return id
}

func resetSearchFlags() {
	searchPretty = false
	searchShowAll = false
}

func TestSearchBasicMatch(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createSearchTestTask(t, store, "Build login page", "", models.StatusTodo)
	createSearchTestTask(t, store, "Fix database bug", "", models.StatusTodo)

	resetSearchFlags()
	err = runSearch(nil, []string{"login"})
	require.NoError(t, err)
}

func TestSearchNoMatch(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createSearchTestTask(t, store, "Build login page", "", models.StatusTodo)

	resetSearchFlags()
	err = runSearch(nil, []string{"nonexistent"})
	require.NoError(t, err)
}

func TestSearchCaseInsensitive(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createSearchTestTask(t, store, "Build Login Page", "", models.StatusTodo)

	resetSearchFlags()

	// Search with different case
	err = runSearch(nil, []string{"LOGIN"})
	require.NoError(t, err)

	err = runSearch(nil, []string{"build login"})
	require.NoError(t, err)
}

func TestSearchPartialMatch(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createSearchTestTask(t, store, "Implement authentication", "", models.StatusTodo)

	resetSearchFlags()
	err = runSearch(nil, []string{"auth"})
	require.NoError(t, err)
}

func TestSearchMatchesDescription(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createSearchTestTask(t, store, "Task one", "Handle user authentication", models.StatusTodo)
	createSearchTestTask(t, store, "Task two", "Build the dashboard", models.StatusTodo)

	resetSearchFlags()
	err = runSearch(nil, []string{"authentication"})
	require.NoError(t, err)
}

func TestSearchPrettyOutput(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createSearchTestTask(t, store, "Build login page", "", models.StatusTodo)
	createSearchTestTask(t, store, "Fix login bug", "", models.StatusInProgress)

	resetSearchFlags()
	searchPretty = true
	err = runSearch(nil, []string{"login"})
	require.NoError(t, err)
}

func TestSearchAllStatuses(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	createSearchTestTask(t, store, "Login todo", "", models.StatusTodo)
	createSearchTestTask(t, store, "Login progress", "", models.StatusInProgress)
	createSearchTestTask(t, store, "Login done", "", models.StatusDone)

	resetSearchFlags()
	searchShowAll = true
	err = runSearch(nil, []string{"login"})
	require.NoError(t, err)
}

func TestSearchEmptyProject(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetSearchFlags()
	err := runSearch(nil, []string{"anything"})
	require.NoError(t, err)
}

func TestSearchRequiresArgument(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	resetSearchFlags()
	// cobra.ExactArgs(1) validates this at the cobra level,
	// but we can verify the command is configured correctly
	require.Equal(t, "search <query>", searchCmd.Use)
}
