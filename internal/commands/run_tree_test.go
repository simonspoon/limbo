package commands

import (
	"sort"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idsOf returns the IDs of the given tasks in a deterministic (sorted) order
// so tests can compare against expected sets with assert.Equal.
func idsOf(tasks []models.Task) []string {
	out := make([]string, len(tasks))
	for i := range tasks {
		out[i] = tasks[i].ID
	}
	sort.Strings(out)
	return out
}

// mkTask builds a task for in-memory FindNextLeaf tests. parentID is the empty
// string for roots; Created is mandatory because the function sorts on it.
func mkTask(id, parentID, status string, created time.Time) models.Task {
	t := models.Task{
		ID:      id,
		Name:    id,
		Status:  status,
		Created: created,
		Updated: created,
	}
	if parentID != "" {
		p := parentID
		t.Parent = &p
	}
	return t
}

// saveTasks persists a slice of tasks in order. Used to set up LoadSubtree /
// BottomUpCleanup fixtures.
func saveTasks(t *testing.T, store *storage.Storage, tasks []models.Task) {
	t.Helper()
	for i := range tasks {
		require.NoError(t, store.SaveTask(&tasks[i]))
	}
}

// -----------------------------------------------------------------------------
// LoadSubtree tests
// -----------------------------------------------------------------------------

func TestTreeWalk_LoadSubtree_Empty(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	saveTasks(t, store, []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
	})

	subtree, err := LoadSubtree(store, "aaaa")
	require.NoError(t, err)
	assert.Empty(t, subtree, "root with no descendants should return empty subtree")
}

func TestTreeWalk_LoadSubtree_Chain(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	saveTasks(t, store, []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
		mkTask("bbbb", "aaaa", models.StatusCaptured, now),
		mkTask("cccc", "bbbb", models.StatusCaptured, now),
		mkTask("dddd", "cccc", models.StatusCaptured, now),
	})

	subtree, err := LoadSubtree(store, "aaaa")
	require.NoError(t, err)
	assert.Equal(t, []string{"bbbb", "cccc", "dddd"}, idsOf(subtree))
}

func TestTreeWalk_LoadSubtree_Star(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	saveTasks(t, store, []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
		mkTask("bbbb", "aaaa", models.StatusCaptured, now),
		mkTask("cccc", "aaaa", models.StatusCaptured, now),
		mkTask("dddd", "aaaa", models.StatusCaptured, now),
		mkTask("eeee", "aaaa", models.StatusCaptured, now),
	})

	subtree, err := LoadSubtree(store, "aaaa")
	require.NoError(t, err)
	assert.Equal(t, []string{"bbbb", "cccc", "dddd", "eeee"}, idsOf(subtree))
}

func TestTreeWalk_LoadSubtree_Mixed(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// A -> { B -> { C, D }, E -> F }
	now := time.Now()
	saveTasks(t, store, []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
		mkTask("bbbb", "aaaa", models.StatusCaptured, now),
		mkTask("cccc", "bbbb", models.StatusCaptured, now),
		mkTask("dddd", "bbbb", models.StatusCaptured, now),
		mkTask("eeee", "aaaa", models.StatusCaptured, now),
		mkTask("ffff", "eeee", models.StatusCaptured, now),
	})

	subtree, err := LoadSubtree(store, "aaaa")
	require.NoError(t, err)
	assert.Equal(t, []string{"bbbb", "cccc", "dddd", "eeee", "ffff"}, idsOf(subtree))

	// Sub-root: B yields {C, D}.
	subB, err := LoadSubtree(store, "bbbb")
	require.NoError(t, err)
	assert.Equal(t, []string{"cccc", "dddd"}, idsOf(subB))
}

func TestTreeWalk_LoadSubtree_MissingRoot(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	saveTasks(t, store, []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
	})

	_, err = LoadSubtree(store, "zzzz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task not found")
}

func TestTreeWalk_LoadSubtree_CycleGuard(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// Build a synthetic parent cycle: aaaa.Parent = bbbb, bbbb.Parent = aaaa.
	// storage.SaveTask does not validate parent acyclicity, so this persists
	// as-is and exercises the cycle guard in LoadSubtree.
	now := time.Now()
	a := mkTask("aaaa", "bbbb", models.StatusCaptured, now)
	b := mkTask("bbbb", "aaaa", models.StatusCaptured, now)
	require.NoError(t, store.SaveTask(&a))
	require.NoError(t, store.SaveTask(&b))

	_, err = LoadSubtree(store, "aaaa")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

// -----------------------------------------------------------------------------
// BottomUpCleanup tests
// -----------------------------------------------------------------------------

func TestTreeWalk_BottomUpCleanup_AllDoneChain(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	// A=Captured, B=Done, C=Done  (A -> B -> C). B is a non-leaf whose only
	// child C is done, so B is already done by assumption. A should be
	// promoted because its only child B is done.
	saveTasks(t, store, []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
		mkTask("bbbb", "aaaa", models.StatusDone, now),
		mkTask("cccc", "bbbb", models.StatusDone, now),
	})

	subtree, err := LoadSubtree(store, "aaaa")
	require.NoError(t, err)
	// Include the root itself — BottomUpCleanup works on any pre-loaded slice.
	root, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	slice := append([]models.Task{*root}, subtree...)

	require.NoError(t, BottomUpCleanup(store, slice))

	// In-memory mutation.
	for i := range slice {
		if slice[i].ID == "aaaa" {
			assert.Equal(t, models.StatusDone, slice[i].Status)
			assert.Equal(t, "All subtasks completed", slice[i].Outcome)
		}
	}

	// Persisted.
	persisted, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, persisted.Status)
	assert.Equal(t, "All subtasks completed", persisted.Outcome)
}

func TestTreeWalk_BottomUpCleanup_PartialMixed(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// A with { B=Done, C=Captured } — A must NOT be promoted.
	now := time.Now()
	saveTasks(t, store, []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
		mkTask("bbbb", "aaaa", models.StatusDone, now),
		mkTask("cccc", "aaaa", models.StatusCaptured, now),
	})

	all, err := store.LoadAllIndex()
	require.NoError(t, err)

	require.NoError(t, BottomUpCleanup(store, all))

	persisted, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusCaptured, persisted.Status)
	assert.Empty(t, persisted.Outcome)
}

func TestTreeWalk_BottomUpCleanup_Cascade(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	// 3-level cascade: G (Captured) -> P (Captured) -> {C1=Done, C2=Done}.
	// One call should promote P first (all children done) then G (its only
	// remaining child P was just promoted).
	now := time.Now()
	saveTasks(t, store, []models.Task{
		mkTask("gggg", "", models.StatusCaptured, now),
		mkTask("pppp", "gggg", models.StatusCaptured, now),
		mkTask("cccc", "pppp", models.StatusDone, now),
		mkTask("dddd", "pppp", models.StatusDone, now),
	})

	all, err := store.LoadAllIndex()
	require.NoError(t, err)

	require.NoError(t, BottomUpCleanup(store, all))

	for _, id := range []string{"pppp", "gggg"} {
		task, err := store.LoadTask(id)
		require.NoError(t, err)
		assert.Equal(t, models.StatusDone, task.Status, "task %s should be promoted", id)
		assert.Equal(t, "All subtasks completed", task.Outcome)
	}
}

func TestTreeWalk_BottomUpCleanup_Idempotent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	saveTasks(t, store, []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
		mkTask("bbbb", "aaaa", models.StatusDone, now),
		mkTask("cccc", "aaaa", models.StatusDone, now),
	})

	// First call — should promote aaaa.
	all1, err := store.LoadAllIndex()
	require.NoError(t, err)
	require.NoError(t, BottomUpCleanup(store, all1))

	after1, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	require.Equal(t, models.StatusDone, after1.Status)
	require.Equal(t, "All subtasks completed", after1.Outcome)
	firstUpdated := after1.Updated

	// Second call — must be a no-op.
	all2, err := store.LoadAllIndex()
	require.NoError(t, err)
	require.NoError(t, BottomUpCleanup(store, all2))

	after2, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, after2.Status)
	assert.Equal(t, "All subtasks completed", after2.Outcome)
	assert.True(t, firstUpdated.Equal(after2.Updated),
		"second BottomUpCleanup call must not rewrite the task")
}

func TestTreeWalk_BottomUpCleanup_LeafUntouched(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	saveTasks(t, store, []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
	})

	all, err := store.LoadAllIndex()
	require.NoError(t, err)
	require.NoError(t, BottomUpCleanup(store, all))

	persisted, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusCaptured, persisted.Status)
	assert.Empty(t, persisted.Outcome)
}

func TestTreeWalk_BottomUpCleanup_ManuallyDoneRootSkipped(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	store, err := storage.NewStorage()
	require.NoError(t, err)

	now := time.Now()
	// Root is already Done with a custom outcome; BottomUpCleanup must NOT
	// overwrite that outcome even though all children are also Done.
	root := mkTask("aaaa", "", models.StatusDone, now)
	root.Outcome = "Manually wrapped up"
	child := mkTask("bbbb", "aaaa", models.StatusDone, now)
	require.NoError(t, store.SaveTask(&root))
	require.NoError(t, store.SaveTask(&child))

	all, err := store.LoadAllIndex()
	require.NoError(t, err)
	require.NoError(t, BottomUpCleanup(store, all))

	persisted, err := store.LoadTask("aaaa")
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, persisted.Status)
	assert.Equal(t, "Manually wrapped up", persisted.Outcome)
}

// -----------------------------------------------------------------------------
// FindNextLeaf tests (pure — no storage)
// -----------------------------------------------------------------------------

func TestTreeWalk_FindNextLeaf_Nil(t *testing.T) {
	assert.Nil(t, FindNextLeaf(nil))
	assert.Nil(t, FindNextLeaf([]models.Task{}))
}

func TestTreeWalk_FindNextLeaf_EarliestCreatedWins(t *testing.T) {
	now := time.Now()
	tasks := []models.Task{
		mkTask("cccc", "", models.StatusCaptured, now.Add(2*time.Millisecond)),
		mkTask("aaaa", "", models.StatusCaptured, now.Add(0*time.Millisecond)),
		mkTask("bbbb", "", models.StatusCaptured, now.Add(1*time.Millisecond)),
	}

	got := FindNextLeaf(tasks)
	require.NotNil(t, got)
	assert.Equal(t, "aaaa", got.ID)
}

func TestTreeWalk_FindNextLeaf_CreatedTieBreaksByID(t *testing.T) {
	// Identical Created — deterministic tiebreak by ID ascending.
	now := time.Now()
	tasks := []models.Task{
		mkTask("cccc", "", models.StatusCaptured, now),
		mkTask("aaaa", "", models.StatusCaptured, now),
		mkTask("bbbb", "", models.StatusCaptured, now),
	}

	got := FindNextLeaf(tasks)
	require.NotNil(t, got)
	assert.Equal(t, "aaaa", got.ID)
}

func TestTreeWalk_FindNextLeaf_SkipsDone(t *testing.T) {
	now := time.Now()
	tasks := []models.Task{
		mkTask("aaaa", "", models.StatusDone, now),
		mkTask("bbbb", "", models.StatusCaptured, now.Add(time.Millisecond)),
	}

	got := FindNextLeaf(tasks)
	require.NotNil(t, got)
	assert.Equal(t, "bbbb", got.ID)
}

func TestTreeWalk_FindNextLeaf_SkipsManualBlock(t *testing.T) {
	now := time.Now()
	blocked := mkTask("aaaa", "", models.StatusCaptured, now)
	blocked.ManualBlockReason = "waiting on design review"
	tasks := []models.Task{
		blocked,
		mkTask("bbbb", "", models.StatusCaptured, now.Add(time.Millisecond)),
	}

	got := FindNextLeaf(tasks)
	require.NotNil(t, got)
	assert.Equal(t, "bbbb", got.ID)
}

func TestTreeWalk_FindNextLeaf_SkipsBlockedByNonDone(t *testing.T) {
	now := time.Now()
	blocker := mkTask("aaaa", "", models.StatusCaptured, now)
	blocked := mkTask("bbbb", "", models.StatusCaptured, now.Add(time.Millisecond))
	blocked.BlockedBy = []string{"aaaa"}
	later := mkTask("cccc", "", models.StatusCaptured, now.Add(2*time.Millisecond))

	// aaaa itself is the earliest eligible leaf. Expect aaaa first — but we
	// want to prove bbbb is skipped, so make aaaa ineligible via manual block
	// and assert cccc is chosen.
	blockerBlocked := blocker
	blockerBlocked.ManualBlockReason = "external"
	tasks := []models.Task{blockerBlocked, blocked, later}

	got := FindNextLeaf(tasks)
	require.NotNil(t, got)
	assert.Equal(t, "cccc", got.ID,
		"bbbb is blocked by non-done aaaa, aaaa is manually blocked, so cccc wins")
}

func TestTreeWalk_FindNextLeaf_BlockedByDanglingCountsResolved(t *testing.T) {
	// BlockedBy references an ID that is NOT in the slice. That counts as
	// resolved (matches storage.IsBlocked semantics), so the task is eligible.
	now := time.Now()
	t1 := mkTask("aaaa", "", models.StatusCaptured, now)
	t1.BlockedBy = []string{"zzzz"} // dangling

	tasks := []models.Task{t1}

	got := FindNextLeaf(tasks)
	require.NotNil(t, got)
	assert.Equal(t, "aaaa", got.ID)
}

func TestTreeWalk_FindNextLeaf_SkipsNonLeaf(t *testing.T) {
	// A has child B. A must not be returned even though it is the earliest
	// and has no blockers — FindNextLeaf only returns strict leaves.
	now := time.Now()
	tasks := []models.Task{
		mkTask("aaaa", "", models.StatusCaptured, now),
		mkTask("bbbb", "aaaa", models.StatusCaptured, now.Add(time.Millisecond)),
	}

	got := FindNextLeaf(tasks)
	require.NotNil(t, got)
	assert.Equal(t, "bbbb", got.ID)
}
