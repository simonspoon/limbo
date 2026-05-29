package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initEnv sets up an isolated central-storage home and a fresh project dir
// (with a .limbo-id anchor) and chdir's into it. It returns the project dir and
// the LIMBO_HOME so tests can assert on the seeded store.
func initEnv(t *testing.T) (projDir, homeDir string) {
	t.Helper()

	homeDir, err := os.MkdirTemp("", "limbo-home-*")
	require.NoError(t, err)
	projDir, err = os.MkdirTemp("", "limbo-proj-*")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(projDir, ".limbo-id"), []byte("init-test-id\n"), 0o644))

	t.Setenv("LIMBO_HOME", homeDir)
	t.Setenv(noClimbEnv, "1")

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projDir))
	t.Cleanup(func() {
		os.Chdir(origDir)
		os.RemoveAll(projDir)
		os.RemoveAll(homeDir)
	})
	return projDir, homeDir
}

// findTasksJSON returns the single tasks.json seeded under home/projects/<id>.
func findTasksJSON(t *testing.T, homeDir string) string {
	t.Helper()
	var found string
	err := filepath.Walk(homeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "tasks.json" {
			found = path
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, found, "no central tasks.json was seeded")
	return found
}

func TestInitCommand(t *testing.T) {
	_, homeDir := initEnv(t)

	initPretty = false
	require.NoError(t, runInit(initCmd, nil))

	tasksPath := findTasksJSON(t, homeDir)

	// The seed envelope must be schema 7.0.0 at revision 0.
	data, err := os.ReadFile(tasksPath)
	require.NoError(t, err)
	var env struct {
		Version  string `json:"version"`
		Revision int    `json:"revision"`
	}
	require.NoError(t, json.Unmarshal(data, &env))
	assert.Equal(t, "7.0.0", env.Version)
	assert.Equal(t, 0, env.Revision)

	// A context directory is created alongside.
	contextDir := filepath.Join(filepath.Dir(tasksPath), "context")
	info, err := os.Stat(contextDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestInitCommandRelocatesExistingStore(t *testing.T) {
	_, homeDir := initEnv(t)

	initPretty = false
	require.NoError(t, runInit(initCmd, nil))

	// Mutate the store so the relocation has something to move.
	resetAddFlags()
	require.NoError(t, runAdd(addCmd, []string{"existing task"}))

	storageRoot := filepath.Dir(findTasksJSON(t, homeDir))

	// A second init should succeed (no "already exists" error) and relocate
	// the populated store aside.
	require.NoError(t, runInit(initCmd, nil))

	entries, err := os.ReadDir(filepath.Dir(storageRoot))
	require.NoError(t, err)
	base := filepath.Base(storageRoot)
	relocated := false
	for _, e := range entries {
		if e.IsDir() && e.Name() != base && len(e.Name()) > len(base) && e.Name()[:len(base)] == base {
			relocated = true
		}
	}
	assert.True(t, relocated, "expected a .replaced-* sibling directory")
}

func TestInitCommandPrettyOutput(t *testing.T) {
	initEnv(t)

	initPretty = true
	defer func() { initPretty = false }()

	require.NoError(t, runInit(initCmd, nil))
}

func TestInitGeneratesUUIDWhenNoGitNoID(t *testing.T) {
	homeDir, err := os.MkdirTemp("", "limbo-home-*")
	require.NoError(t, err)
	projDir, err := os.MkdirTemp("", "limbo-proj-*")
	require.NoError(t, err)
	t.Setenv("LIMBO_HOME", homeDir)
	t.Setenv(noClimbEnv, "1")

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(projDir))
	t.Cleanup(func() {
		os.Chdir(origDir)
		os.RemoveAll(projDir)
		os.RemoveAll(homeDir)
	})

	initPretty = false
	require.NoError(t, runInit(initCmd, nil))

	idData, err := os.ReadFile(filepath.Join(projDir, ".limbo-id"))
	require.NoError(t, err)
	assert.NotEmpty(t, string(idData), "init must write a non-empty .limbo-id")
}
