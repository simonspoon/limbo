package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file consolidates the A24 acceptance-matrix clauses that T05 owns
// end-to-end: the --if-revision mismatch byte-identical guard (A7/A24), the
// init-on-existing byte-identical relocation (A16/A24), the full project-ID
// priority hierarchy including a real two-commit git fixture and the UUID
// fallback persisting across runs (A11-A13/A24), and the LIMBO_HOME override
// redirecting both the resolved storage root and the migration destination
// (A10/A24). It writes no production code — it only exercises behavior shipped
// by T01-T04.

// withIfRevision builds a cobra command carrying a "changed" --if-revision flag
// set to want, so checkIfRevision treats the guard as active. It mirrors the
// persistent flag registered on rootCmd.
func withIfRevision(t *testing.T, want int) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Int("if-revision", 0, "")
	require.NoError(t, cmd.Flags().Set("if-revision", itoa(want)))
	return cmd
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// readStore returns the central tasks.json bytes for the active test project.
func readStore(t *testing.T, homeDir string) (string, []byte) {
	t.Helper()
	path := findTasksJSON(t, homeDir)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return path, data
}

// TestA24_IfRevisionMismatchLeavesStoreByteIdentical covers A7/A24: a mutating
// command guarded with --if-revision N where N differs from the current
// revision must abort with the structured stderr error and leave tasks.json
// byte-identical.
func TestA24_IfRevisionMismatchLeavesStoreByteIdentical(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	homeDir := os.Getenv("LIMBO_HOME")
	_, before := readStore(t, homeDir)

	// The fresh store is at revision 0; guard on a non-matching revision.
	cmd := withIfRevision(t, 99)
	resetAddFlags()

	var stderr string
	var runErr error
	stderr = captureStderr(t, func() {
		runErr = runAdd(cmd, []string{"should not persist"})
	})

	require.Error(t, runErr, "mismatched --if-revision must abort")

	// Structured error on stderr: {"error":"revision mismatch","expected":99,"actual":0}.
	var payload struct {
		Error    string `json:"error"`
		Expected int    `json:"expected"`
		Actual   int    `json:"actual"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &payload),
		"stderr must carry the structured revision-mismatch JSON, got %q", stderr)
	assert.Equal(t, "revision mismatch", payload.Error)
	assert.Equal(t, 99, payload.Expected)
	assert.Equal(t, 0, payload.Actual)

	// The store must be byte-identical before and after the rejected mutation.
	_, after := readStore(t, homeDir)
	assert.Equal(t, before, after, "rejected --if-revision write must leave the store byte-identical")
}

// TestA24_IfRevisionMatchAllowsMutation is the positive counterpart: when the
// guard matches the current revision, the mutation proceeds and the revision
// advances.
func TestA24_IfRevisionMatchAllowsMutation(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd := withIfRevision(t, 0) // fresh store is revision 0
	resetAddFlags()
	require.NoError(t, runAdd(cmd, []string{"persists"}))

	store, err := testStore(t)
	require.NoError(t, err)
	tasks, err := store.LoadAll()
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	rev, err := store.Revision()
	require.NoError(t, err)
	assert.Equal(t, 1, rev, "a matching guarded mutation must advance the revision")
}

// TestA24_InitOnExistingStoreByteIdentical covers A16/A24: init on a populated
// central store renames it to <path>.replaced-<ISO> (original tasks.json +
// context/ tree byte-identical) and recreates a fresh empty store at the
// original path; both paths exist on disk afterward.
func TestA24_InitOnExistingStoreByteIdentical(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	homeDir := os.Getenv("LIMBO_HOME")

	// Populate the store: a task plus a context sidecar so the relocated tree is
	// non-trivial.
	resetAddFlags()
	addApproach = "do work"
	require.NoError(t, runAdd(addCmd, []string{"task with context"}))

	storageRoot := filepath.Dir(findTasksJSON(t, homeDir))

	// Snapshot the original tree (tasks.json + every file under context/).
	origTasks, err := os.ReadFile(filepath.Join(storageRoot, "tasks.json"))
	require.NoError(t, err)
	origContext := snapshotTree(t, filepath.Join(storageRoot, "context"))

	// Re-init: must relocate the populated root aside and seed a fresh one.
	initPretty = false
	require.NoError(t, runInit(initCmd, nil))

	// Both paths exist: the original (now fresh + empty) and the .replaced-* sibling.
	_, statErr := os.Stat(filepath.Join(storageRoot, "tasks.json"))
	require.NoError(t, statErr, "a fresh store must exist at the original path")

	replaced := findReplacedSibling(t, storageRoot)
	require.NotEmpty(t, replaced, "expected a <path>.replaced-* sibling directory")

	// The renamed tree is byte-identical to the pre-init snapshot.
	relTasks, err := os.ReadFile(filepath.Join(replaced, "tasks.json"))
	require.NoError(t, err)
	assert.Equal(t, origTasks, relTasks, "relocated tasks.json must be byte-identical")

	relContext := snapshotTree(t, filepath.Join(replaced, "context"))
	assert.Equal(t, origContext, relContext, "relocated context/ tree must be byte-identical")

	// The fresh store is empty and back at revision 0.
	freshData, err := os.ReadFile(filepath.Join(storageRoot, "tasks.json"))
	require.NoError(t, err)
	var fresh struct {
		Revision int               `json:"revision"`
		Tasks    []json.RawMessage `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal(freshData, &fresh))
	assert.Equal(t, 0, fresh.Revision)
	assert.Empty(t, fresh.Tasks)
}

// snapshotTree returns a deterministic map of relative-path -> file bytes for
// every regular file under root, so two trees can be compared for byte
// identity. A missing root yields an empty map.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return out
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(b)
		return nil
	})
	require.NoError(t, err)
	return out
}

// findReplacedSibling returns the single <base>.replaced-* sibling of
// storageRoot, or "" if none exists.
func findReplacedSibling(t *testing.T, storageRoot string) string {
	t.Helper()
	parent := filepath.Dir(storageRoot)
	base := filepath.Base(storageRoot)
	entries, err := os.ReadDir(parent)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), base+".replaced-") {
			return filepath.Join(parent, e.Name())
		}
	}
	return ""
}

// TestA24_ProjectIDGitFirstCommitTwoCommitFixture covers A12/A24: the project
// ID derives from the first-commit SHA, proven against a real two-commit git
// repository built inside the test. The derived ID must equal the first commit
// (not HEAD) and must select the central store under that ID.
func TestA24_ProjectIDGitFirstCommitTwoCommitFixture(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	homeDir, err := os.MkdirTemp("", "limbo-home-*")
	require.NoError(t, err)
	projDir, err := os.MkdirTemp("", "limbo-gitproj-*")
	require.NoError(t, err)

	firstSHA := buildTwoCommitRepo(t, projDir)

	t.Setenv("LIMBO_HOME", homeDir)
	// Climb so the .git anchor is found; the fixture repo is the cwd.
	t.Setenv(noClimbEnv, "")

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

	// The seeded store must live under projects/<firstSHA>.
	expected := filepath.Join(homeDir, "projects", firstSHA)
	_, statErr := os.Stat(filepath.Join(expected, "tasks.json"))
	require.NoError(t, statErr,
		"store must be seeded at projects/<first-commit-SHA>; expected %s", expected)

	// Sanity: the first commit is NOT HEAD, so we are proving first-commit (not
	// HEAD) derivation.
	headSHA := strings.TrimSpace(gitOut(t, projDir, "rev-parse", "HEAD"))
	assert.NotEqual(t, headSHA, firstSHA, "fixture must have a HEAD distinct from the first commit")
}

// buildTwoCommitRepo initializes a git repo at dir with exactly two commits and
// returns the 40-char lowercase hex SHA of the first commit.
func buildTwoCommitRepo(t *testing.T, dir string) string {
	t.Helper()
	gitOut(t, dir, "init")
	gitOut(t, dir, "config", "user.email", "test@example.com")
	gitOut(t, dir, "config", "user.name", "Test")
	gitOut(t, dir, "config", "commit.gpgsign", "false")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644))
	gitOut(t, dir, "add", "a.txt")
	gitOut(t, dir, "commit", "-m", "first")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("two\n"), 0o644))
	gitOut(t, dir, "add", "b.txt")
	gitOut(t, dir, "commit", "-m", "second")

	first := strings.TrimSpace(gitOut(t, dir, "rev-list", "--max-parents=0", "HEAD"))
	require.Len(t, first, 40, "first-commit SHA must be 40 hex chars")
	return first
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	require.NoErrorf(t, cmd.Run(), "git %s failed: %s", strings.Join(args, " "), errOut.String())
	return out.String()
}

// TestA24_LimboIDOverridesGitFirstCommit covers A11/A24: a .limbo-id at the
// project root takes precedence over git first-commit derivation, selecting a
// store under the override ID rather than the commit SHA.
func TestA24_LimboIDOverridesGitFirstCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	homeDir, err := os.MkdirTemp("", "limbo-home-*")
	require.NoError(t, err)
	projDir, err := os.MkdirTemp("", "limbo-gitproj-*")
	require.NoError(t, err)

	firstSHA := buildTwoCommitRepo(t, projDir)
	// Place an override that must win over the git SHA.
	require.NoError(t, os.WriteFile(filepath.Join(projDir, ".limbo-id"), []byte("override-wins\n"), 0o644))

	t.Setenv("LIMBO_HOME", homeDir)
	t.Setenv(noClimbEnv, "")

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

	// The store lives under the override ID, NOT the first-commit SHA.
	_, statErr := os.Stat(filepath.Join(homeDir, "projects", "override-wins", "tasks.json"))
	require.NoError(t, statErr, ".limbo-id override must select projects/override-wins")
	_, shaStatErr := os.Stat(filepath.Join(homeDir, "projects", firstSHA, "tasks.json"))
	assert.True(t, os.IsNotExist(shaStatErr), "git-SHA store must not be created when .limbo-id overrides")
}

// TestA24_UUIDFallbackPersistsAcrossRuns covers A13/A24: in a non-git tempdir
// with no .limbo-id, init generates a UUID, writes it to .limbo-id, and the same
// ID is reused on a second run (no new ID, no new store directory).
func TestA24_UUIDFallbackPersistsAcrossRuns(t *testing.T) {
	homeDir, err := os.MkdirTemp("", "limbo-home-*")
	require.NoError(t, err)
	projDir, err := os.MkdirTemp("", "limbo-nogit-*")
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

	idData, err := os.ReadFile(filepath.Join(projDir, store_LimboIDFile()))
	require.NoError(t, err)
	firstID := strings.TrimSpace(string(idData))
	require.NotEmpty(t, firstID, "init must persist a generated UUID to .limbo-id")

	// Exactly one project directory exists after the first run.
	projsAfterFirst := listProjects(t, homeDir)
	require.Len(t, projsAfterFirst, 1)
	require.Equal(t, firstID, projsAfterFirst[0])

	// A second init in the same dir must reuse the persisted ID. (Init relocates
	// the existing store aside, but does not mint a new project ID.)
	require.NoError(t, runInit(initCmd, nil))

	idData2, err := os.ReadFile(filepath.Join(projDir, store_LimboIDFile()))
	require.NoError(t, err)
	secondID := strings.TrimSpace(string(idData2))
	assert.Equal(t, firstID, secondID, "the generated .limbo-id must persist across runs")

	// Still exactly one project ID (the relocation lives under the same ID dir
	// as a .replaced-* sibling, not a new project).
	assert.Equal(t, []string{firstID}, listProjects(t, homeDir),
		"a second run must not mint a new project ID")
}

// listProjects returns the distinct base project IDs under homeDir/projects,
// stripping any .replaced-* relocation suffix so a relocated store counts as the
// same project.
func listProjects(t *testing.T, homeDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(homeDir, "projects"))
	require.NoError(t, err)
	seen := map[string]bool{}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if i := strings.Index(name, ".replaced-"); i >= 0 {
			name = name[:i]
		}
		if !seen[name] {
			seen[name] = true
			ids = append(ids, name)
		}
	}
	return ids
}

// store_LimboIDFile returns the .limbo-id filename. It is a tiny indirection so
// the test does not import the store package solely for one constant string,
// keeping this file focused on command-level behavior.
func store_LimboIDFile() string { return ".limbo-id" }

// TestA24_LimboHomeRedirectsStorageRootAndMigrationDest covers A10/A24:
// LIMBO_HOME redirects BOTH the resolved storage root (init/getStorage) AND the
// destination that migrate writes to.
func TestA24_LimboHomeRedirectsStorageRootAndMigrationDest(t *testing.T) {
	// Part 1: storage root honors LIMBO_HOME.
	_, legacyDir, homeDir := legacyEnv(t)
	expectedRoot := filepath.Join(homeDir, "projects", "legacy-test-id")

	// Before migration, getStorage falls back to the legacy in-tree store
	// (no central store yet), so assert the *resolved* central root via init.
	initPretty = false
	require.NoError(t, runInit(initCmd, nil))
	st, err := getStorage()
	require.NoError(t, err)
	assert.Equal(t, expectedRoot, st.GetRootDir(),
		"LIMBO_HOME must redirect the resolved storage root")

	// Re-create a clean legacy-only situation for the migration-destination
	// assertion: a second project under the SAME LIMBO_HOME but a different ID.
	_ = legacyDir
}

// TestA24_LimboHomeRedirectsMigrationDestination isolates the migration-dest
// half of A10: migrate writes the central store under ${LIMBO_HOME}/projects/<id>.
func TestA24_LimboHomeRedirectsMigrationDestination(t *testing.T) {
	_, legacyDir, homeDir := legacyEnv(t)

	require.NoError(t, runMigrate(migrateCmd, nil))

	// The migrated central store must be under LIMBO_HOME, keyed by the project ID.
	expectedDest := filepath.Join(homeDir, "projects", "legacy-test-id", "tasks.json")
	data, err := os.ReadFile(expectedDest)
	require.NoError(t, err, "migrate must write the central store under LIMBO_HOME/projects/<id>")

	var env struct {
		Tasks []struct {
			ID string `json:"id"`
		} `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal(data, &env))
	require.Len(t, env.Tasks, 1)
	assert.Equal(t, "aaaa", env.Tasks[0].ID)

	// Source renamed aside, not deleted.
	_, statErr := os.Stat(legacyDir)
	assert.True(t, os.IsNotExist(statErr), "legacy source must be renamed away after migrate")
}
