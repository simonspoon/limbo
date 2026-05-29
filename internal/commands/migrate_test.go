package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyEnv sets up an isolated central-storage home and a project dir that
// holds a .limbo-id anchor plus a legacy in-tree .limbo/ store seeded with one
// task and a context sidecar. It chdir's into the project and returns the
// project dir, the legacy dir, and the LIMBO_HOME. No central store is created
// (no `limbo init`), so callers exercise the legacy/central detection paths
// directly.
func legacyEnv(t *testing.T) (projDir, legacyDir, homeDir string) {
	t.Helper()

	homeDir, err := os.MkdirTemp("", "limbo-home-*")
	require.NoError(t, err)
	projDir, err = os.MkdirTemp("", "limbo-proj-*")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(projDir, ".limbo-id"), []byte("legacy-test-id\n"), 0o644))

	legacyDir = filepath.Join(projDir, ".limbo")
	require.NoError(t, os.MkdirAll(filepath.Join(legacyDir, "context", "aaaa"), 0o755))
	legacy := `{"version":"6.0.0","tasks":[{"id":"aaaa","name":"legacy task","status":"captured"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "tasks.json"), []byte(legacy), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(legacyDir, "context", "aaaa", "context.md"),
		[]byte("## Description\nlegacy body\n"), 0o644))

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
	return projDir, legacyDir, homeDir
}

// TestMigrateRoundTrip covers A17: migrate copies the legacy store (tasks +
// sidecars) to the central root with the revision incremented, renames the
// source aside instead of deleting it, and is idempotent on re-run.
func TestMigrateRoundTrip(t *testing.T) {
	projDir, legacyDir, homeDir := legacyEnv(t)

	require.NoError(t, runMigrate(migrateCmd, nil))

	// The central store now exists, carries the migrated task, and has its
	// revision bumped from the legacy 0 to 1.
	centralTasks := findTasksJSON(t, homeDir)
	data, err := os.ReadFile(centralTasks)
	require.NoError(t, err)
	var env struct {
		Version  string `json:"version"`
		Revision int    `json:"revision"`
		Tasks    []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal(data, &env))
	assert.Equal(t, "7.0.0", env.Version, "destination must be rewritten to 7.0.0")
	assert.Equal(t, 1, env.Revision, "destination revision must be legacy+1")
	require.Len(t, env.Tasks, 1)
	assert.Equal(t, "aaaa", env.Tasks[0].ID)

	// The context sidecar was carried over verbatim.
	sidecar := filepath.Join(filepath.Dir(centralTasks), "context", "aaaa", "context.md")
	body, err := os.ReadFile(sidecar)
	require.NoError(t, err)
	assert.Contains(t, string(body), "legacy body")

	// The source was renamed (not deleted): the .limbo dir is gone but a
	// .limbo.migrated-* sibling exists.
	_, statErr := os.Stat(legacyDir)
	assert.True(t, os.IsNotExist(statErr), "legacy .limbo must be renamed away")
	entries, err := os.ReadDir(projDir)
	require.NoError(t, err)
	migratedFound := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".limbo.migrated-") {
			migratedFound = true
		}
	}
	assert.True(t, migratedFound, "expected a .limbo.migrated-* sibling")

	// Re-running migrate is idempotent: no legacy store remains.
	require.NoError(t, runMigrate(migrateCmd, nil))
}

// TestMigrateRefusesWhenCentralExists covers the A17 reconcile guard: migrate
// must refuse (return an error) and touch nothing when a central store already
// exists alongside a legacy one.
func TestMigrateRefusesWhenCentralExists(t *testing.T) {
	_, legacyDir, homeDir := legacyEnv(t)

	// Seed a central store so both stores coexist.
	initPretty = false
	require.NoError(t, runInit(initCmd, nil))
	centralTasks := findTasksJSON(t, homeDir)
	before, err := os.ReadFile(centralTasks)
	require.NoError(t, err)

	err = runMigrate(migrateCmd, nil)
	require.Error(t, err, "migrate must refuse when a central store already exists")

	// Nothing was renamed or rewritten.
	_, statErr := os.Stat(legacyDir)
	assert.NoError(t, statErr, "legacy dir must remain untouched on refusal")
	after, err := os.ReadFile(centralTasks)
	require.NoError(t, err)
	assert.Equal(t, before, after, "central store must be untouched on refusal")
}

// TestBothPresentWarning covers A18: when both legacy and central stores exist,
// getStorage prefers the central store and emits a multi-line stderr warning
// naming both paths and recommending migrate, without mutating anything.
func TestBothPresentWarning(t *testing.T) {
	projDir, legacyDir, homeDir := legacyEnv(t)

	// Seed the central store so both coexist.
	initPretty = false
	require.NoError(t, runInit(initCmd, nil))
	centralTasks := findTasksJSON(t, homeDir)
	centralRoot := filepath.Dir(centralTasks)

	// Capture stderr around getStorage.
	stderr := captureStderr(t, func() {
		st, err := getStorage()
		require.NoError(t, err)
		// Central wins: the facade is rooted at the central store, which has no
		// tasks (the legacy task is NOT visible).
		tasks, err := st.LoadAll()
		require.NoError(t, err)
		assert.Empty(t, tasks, "central store wins; legacy task must not appear")
		assert.Equal(t, centralRoot, st.GetRootDir())
	})

	assert.Contains(t, stderr, legacyDir, "warning must name the legacy dir")
	assert.Contains(t, stderr, centralRoot, "warning must name the central dir")
	assert.Contains(t, strings.ToLower(stderr), "migrate", "warning must recommend migrate")
	assert.GreaterOrEqual(t, strings.Count(stderr, "\n"), 1, "warning must be multi-line")

	// The legacy store was not mutated (still 6.0.0).
	legBytes, err := os.ReadFile(filepath.Join(legacyDir, "tasks.json"))
	require.NoError(t, err)
	var legEnv struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(legBytes, &legEnv))
	assert.Equal(t, "6.0.0", legEnv.Version, "legacy store must not be mutated")
	_ = projDir
}

// TestLegacyFallbackNoMutation covers A19/A23: when only a legacy store exists,
// getStorage uses it transparently with a single-line warning and never mutates
// the schema version.
func TestLegacyFallbackNoMutation(t *testing.T) {
	_, legacyDir, _ := legacyEnv(t)

	stderr := captureStderr(t, func() {
		store, err := getStorage()
		require.NoError(t, err)
		// The legacy task is visible through the in-tree fallback.
		tasks, err := store.LoadAll()
		require.NoError(t, err)
		require.Len(t, tasks, 1)
		assert.Equal(t, "aaaa", tasks[0].ID)
		assert.Equal(t, ".limbo", filepath.Base(store.GetRootDir()),
			"legacy fallback facade must be rooted at the in-tree .limbo dir")
	})

	assert.Contains(t, strings.ToLower(stderr), "migrate", "legacy fallback must recommend migrate")

	// Reading through the fallback must not rewrite the legacy schema version.
	legBytes, err := os.ReadFile(filepath.Join(legacyDir, "tasks.json"))
	require.NoError(t, err)
	var legEnv struct {
		Version string `json:"version"`
	}
	require.NoError(t, json.Unmarshal(legBytes, &legEnv))
	assert.Equal(t, "6.0.0", legEnv.Version, "legacy fallback must not mutate the schema version")
}

// captureStderr redirects os.Stderr for the duration of fn and returns whatever
// was written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()

	fn()
	_ = w.Close()
	return <-done
}
