package jsonstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
)

// newStore returns a jsonstore rooted at a fresh tempdir.
func newStore(t *testing.T) *Store {
	t.Helper()
	return New(t.TempDir())
}

// TestRoundTrip exercises the Store contract end to end: SaveAll a set with
// structured content fields, AddTask another, Load it all back, and assert
// equality of the sidecar-merged fields plus the revision counter.
func TestRoundTrip(t *testing.T) {
	s := newStore(t)

	// Fresh store: empty, revision 0.
	tasks, err := s.Load()
	if err != nil {
		t.Fatalf("Load fresh: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("fresh store should be empty, got %d", len(tasks))
	}
	rev, err := s.Revision()
	if err != nil {
		t.Fatalf("Revision fresh: %v", err)
	}
	if rev != 0 {
		t.Fatalf("fresh revision = %d, want 0", rev)
	}

	now := time.Now().UTC().Truncate(time.Second)
	t1 := models.Task{
		ID:                 "aaaa",
		Name:               "first",
		Status:             models.StatusReady,
		Description:        "a description with ## not a heading inside",
		Approach:           "do the thing",
		Verify:             "run the tests",
		Result:             "it worked",
		Outcome:            "shipped",
		AcceptanceCriteria: "all green",
		ScopeOut:           "not this",
		AffectedAreas:      "store",
		TestStrategy:       "table driven",
		Risks:              "none",
		Report:             "final report",
		Created:            now,
		Updated:            now,
		Notes: []models.Note{
			{Content: "first note", Timestamp: now},
		},
	}
	if err := s.SaveAll([]models.Task{t1}); err != nil {
		t.Fatalf("SaveAll: %v", err)
	}

	// Revision incremented by exactly 1.
	rev, err = s.Revision()
	if err != nil {
		t.Fatalf("Revision after SaveAll: %v", err)
	}
	if rev != 1 {
		t.Fatalf("revision after SaveAll = %d, want 1", rev)
	}

	t2 := models.Task{
		ID:       "bbbb",
		Name:     "second",
		Status:   models.StatusCaptured,
		Approach: "second approach",
		Created:  now,
		Updated:  now,
	}
	if err := s.AddTask(t2); err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	rev, err = s.Revision()
	if err != nil {
		t.Fatalf("Revision after AddTask: %v", err)
	}
	if rev != 2 {
		t.Fatalf("revision after AddTask = %d, want 2", rev)
	}

	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Load returned %d tasks, want 2", len(got))
	}

	byID := map[string]models.Task{}
	for _, tk := range got {
		byID[tk.ID] = tk
	}

	g1 := byID["aaaa"]
	if g1.Description != t1.Description {
		t.Errorf("Description mismatch: got %q want %q", g1.Description, t1.Description)
	}
	if g1.Approach != t1.Approach || g1.Verify != t1.Verify || g1.Result != t1.Result {
		t.Errorf("structured fields mismatch: %+v", g1)
	}
	if g1.Outcome != t1.Outcome || g1.AcceptanceCriteria != t1.AcceptanceCriteria ||
		g1.ScopeOut != t1.ScopeOut || g1.AffectedAreas != t1.AffectedAreas ||
		g1.TestStrategy != t1.TestStrategy || g1.Risks != t1.Risks || g1.Report != t1.Report {
		t.Errorf("extended structured fields mismatch: %+v", g1)
	}
	if len(g1.Notes) != 1 || g1.Notes[0].Content != "first note" {
		t.Errorf("notes mismatch: %+v", g1.Notes)
	}
	if !g1.Notes[0].Timestamp.Equal(now) {
		t.Errorf("note timestamp mismatch: got %v want %v", g1.Notes[0].Timestamp, now)
	}

	// Read-only Load must not change the revision.
	rev2, err := s.Revision()
	if err != nil {
		t.Fatalf("Revision after Load: %v", err)
	}
	if rev2 != 2 {
		t.Fatalf("read-only ops changed revision: %d != 2", rev2)
	}

	// The JSON index on disk must be stripped of content (it lives in sidecars).
	raw, err := os.ReadFile(s.tasksPath())
	if err != nil {
		t.Fatalf("read tasks.json: %v", err)
	}
	if strings.Contains(string(raw), "do the thing") {
		t.Errorf("structured content leaked into tasks.json index")
	}
	if !strings.Contains(string(raw), `"version": "7.0.0"`) {
		t.Errorf("tasks.json missing schema version 7.0.0:\n%s", raw)
	}
}

// TestUpdateDeleteAppendNote covers the remaining mutating ops including the
// not-found error paths.
func TestUpdateDeleteAppendNote(t *testing.T) {
	s := newStore(t)
	base := models.Task{ID: "cccc", Name: "c", Status: models.StatusReady}
	if err := s.AddTask(base); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	// UpdateTask on a missing ID errors.
	if err := s.UpdateTask(models.Task{ID: "zzzz", Name: "z"}); err == nil {
		t.Fatalf("UpdateTask on absent ID should error")
	}

	upd := base
	upd.Name = "c-updated"
	upd.Approach = "new approach"
	if err := s.UpdateTask(upd); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	if err := s.AppendNote("cccc", "progress note"); err != nil {
		t.Fatalf("AppendNote: %v", err)
	}
	if err := s.AppendNote("nope", "x"); err == nil {
		t.Fatalf("AppendNote on absent ID should error")
	}

	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].Name != "c-updated" || got[0].Approach != "new approach" {
		t.Fatalf("update not reflected: %+v", got)
	}
	if len(got[0].Notes) != 1 || got[0].Notes[0].Content != "progress note" {
		t.Fatalf("note not appended: %+v", got[0].Notes)
	}

	// ReadContext returns the raw sidecar body; WriteContext overwrites it.
	body, err := s.ReadContext("cccc")
	if err != nil {
		t.Fatalf("ReadContext: %v", err)
	}
	if !strings.Contains(body, "new approach") || !strings.Contains(body, "progress note") {
		t.Fatalf("ReadContext body missing content: %q", body)
	}
	if err := s.WriteContext("cccc", "## Approach\nrewritten\n"); err != nil {
		t.Fatalf("WriteContext: %v", err)
	}
	body, _ = s.ReadContext("cccc")
	if !strings.Contains(body, "rewritten") {
		t.Fatalf("WriteContext did not overwrite: %q", body)
	}

	// ReadContext on a task with no sidecar returns empty, nil.
	empty, err := s.ReadContext("absent")
	if err != nil || empty != "" {
		t.Fatalf("ReadContext absent = (%q, %v), want empty/nil", empty, err)
	}

	// DeleteTask removes the task and its sidecar; absent ID errors.
	if err := s.DeleteTask("cccc"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if err := s.DeleteTask("cccc"); err == nil {
		t.Fatalf("DeleteTask on absent ID should error")
	}
	got, _ = s.Load()
	if len(got) != 0 {
		t.Fatalf("task not deleted: %+v", got)
	}
	if _, err := os.Stat(s.contextDir("cccc")); !os.IsNotExist(err) {
		t.Fatalf("sidecar dir not removed after delete")
	}
}

// TestConcurrentAddTask is the 100-goroutine concurrency stress: 100
// goroutines each AddTask a distinct task. The flock serialization plus
// read-modify-write under one exclusive lock hold must ensure all 100 land and
// tasks.json stays parseable. The literal 100 below is load-bearing.
func TestConcurrentAddTask(t *testing.T) {
	s := newStore(t)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("t%03d", i)
			task := models.Task{
				ID:       id,
				Name:     fmt.Sprintf("task %d", i),
				Status:   models.StatusReady,
				Approach: fmt.Sprintf("approach for goroutine %d", i),
			}
			if err := s.AddTask(task); err != nil {
				t.Errorf("AddTask(%s): %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load after %d concurrent goroutines: %v", n, err)
	}
	if len(got) != n {
		t.Fatalf("after 100 concurrent AddTask: got %d tasks, want %d (lost-update window)", len(got), n)
	}

	// Revision must equal the number of successful mutations.
	rev, err := s.Revision()
	if err != nil {
		t.Fatalf("Revision: %v", err)
	}
	if rev != n {
		t.Fatalf("revision after 100 mutations = %d, want %d", rev, n)
	}

	// tasks.json must still parse.
	if _, err := s.Load(); err != nil {
		t.Fatalf("tasks.json unparseable after stress: %v", err)
	}
}

// TestAtomicWriteCrashSafety fault-injects a failing os.Rename so the new
// write never lands, then asserts tasks.json still holds the OLD parseable
// content — old-or-new only, never half-written.
func TestAtomicWriteCrashSafety(t *testing.T) {
	s := newStore(t)

	orig := models.Task{ID: "orig", Name: "original", Status: models.StatusReady}
	if err := s.AddTask(orig); err != nil {
		t.Fatalf("seed AddTask: %v", err)
	}

	before, err := os.ReadFile(s.tasksPath())
	if err != nil {
		t.Fatalf("read tasks.json before: %v", err)
	}

	// Inject a rename failure for the duration of the next write.
	saved := osRename
	osRename = func(oldpath, newpath string) error {
		return fmt.Errorf("injected rename failure")
	}
	err = s.AddTask(models.Task{ID: "newt", Name: "new", Status: models.StatusReady})
	osRename = saved
	if err == nil {
		t.Fatalf("AddTask should have failed with injected rename error")
	}

	after, err := os.ReadFile(s.tasksPath())
	if err != nil {
		t.Fatalf("read tasks.json after: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("tasks.json changed despite failed rename (half-written):\nbefore=%s\nafter=%s", before, after)
	}

	// And it must still parse to the OLD content (only the original task).
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load after injected failure: %v", err)
	}
	if len(got) != 1 || got[0].ID != "orig" {
		t.Fatalf("expected only original task after failed write, got %+v", got)
	}
}

// TestBakRecovery writes a valid store, corrupts tasks.json with garbage while
// leaving a valid hardlinked .bak, and asserts Load recovers from the .bak and
// emits a stderr warning.
func TestBakRecovery(t *testing.T) {
	s := newStore(t)

	// Two writes guarantee a .bak hardlink exists holding the prior content.
	if err := s.AddTask(models.Task{ID: "keep", Name: "keep", Status: models.StatusReady}); err != nil {
		t.Fatalf("first AddTask: %v", err)
	}
	if err := s.AddTask(models.Task{ID: "also", Name: "also", Status: models.StatusReady}); err != nil {
		t.Fatalf("second AddTask: %v", err)
	}

	// .bak must exist and parse.
	bak := filepath.Join(s.root, "tasks.json.bak")
	if _, err := os.Stat(bak); err != nil {
		t.Fatalf("expected tasks.json.bak to exist: %v", err)
	}

	// Corrupt tasks.json with garbage; leave .bak intact.
	if err := os.WriteFile(s.tasksPath(), []byte("{not valid json at all"), 0o644); err != nil {
		t.Fatalf("corrupt tasks.json: %v", err)
	}

	// Capture stderr to confirm the recovery warning is emitted.
	r, w, _ := os.Pipe()
	savedStderr := os.Stderr
	os.Stderr = w

	got, loadErr := s.Load()

	w.Close()
	os.Stderr = savedStderr
	var buf strings.Builder
	tmp := make([]byte, 4096)
	for {
		nr, _ := r.Read(tmp)
		if nr > 0 {
			buf.Write(tmp[:nr])
		}
		if nr == 0 {
			break
		}
	}
	r.Close()

	if loadErr != nil {
		t.Fatalf("Load should recover from .bak, got error: %v", loadErr)
	}
	// The .bak holds the state after the first write (one task: "keep").
	if len(got) != 1 || got[0].ID != "keep" {
		t.Fatalf("recovered content mismatch: %+v", got)
	}
	warning := buf.String()
	if !strings.Contains(warning, "bak") {
		t.Fatalf("expected stderr warning naming the .bak fallback, got: %q", warning)
	}
}

// TestBakRecoveryBothCorrupt asserts that when both tasks.json and its .bak
// fail to parse, Load returns a structured error and does NOT overwrite or
// truncate either file.
func TestBakRecoveryBothCorrupt(t *testing.T) {
	s := newStore(t)
	if err := s.AddTask(models.Task{ID: "xxxx", Name: "x", Status: models.StatusReady}); err != nil {
		t.Fatalf("AddTask: %v", err)
	}
	if err := s.AddTask(models.Task{ID: "yyyy", Name: "y", Status: models.StatusReady}); err != nil {
		t.Fatalf("AddTask: %v", err)
	}

	bak := filepath.Join(s.root, "tasks.json.bak")
	// Corrupt both files. Note os.WriteFile breaks the hardlink, so the two
	// are now independent garbage files.
	if err := os.WriteFile(s.tasksPath(), []byte("garbage1"), 0o644); err != nil {
		t.Fatalf("corrupt tasks.json: %v", err)
	}
	if err := os.WriteFile(bak, []byte("garbage2"), 0o644); err != nil {
		t.Fatalf("corrupt .bak: %v", err)
	}

	if _, err := s.Load(); err == nil {
		t.Fatalf("Load should error when both tasks.json and .bak are corrupt")
	}

	// Neither file should have been overwritten/truncated by Load.
	d1, _ := os.ReadFile(s.tasksPath())
	d2, _ := os.ReadFile(bak)
	if string(d1) != "garbage1" || string(d2) != "garbage2" {
		t.Fatalf("Load must not overwrite on parse failure: %q %q", d1, d2)
	}
}
