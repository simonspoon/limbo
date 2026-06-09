package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetDocFlags clears doc command flag state between tests.
func resetDocFlags() {
	docAddTitle = ""
	docAddType = ""
	docAddBody = ""
	docAddSlug = ""
	docAddPretty = false
	docShowPretty = false
}

// newDocCmd returns a throwaway cobra command with stderr wired to a buffer so
// jsonError output can be asserted. The RunE funcs read flags from package
// globals, so the caller sets those directly.
func newDocCmd() (*cobra.Command, *bytes.Buffer) {
	var errBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&errBuf)
	cmd.SetOut(&bytes.Buffer{})
	return cmd, &errBuf
}

// captureStdout redirects os.Stdout for the duration of fn and returns whatever
// was written to it (the commands print results with fmt.Println to stdout).
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

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

// addDoc is a test helper that adds a doc and returns its id.
func addDoc(t *testing.T, title, typ, body string) string {
	t.Helper()
	resetDocFlags()
	docAddTitle = title
	docAddType = typ
	docAddBody = body
	cmd, _ := newDocCmd()
	out := captureStdout(t, func() {
		require.NoError(t, runDocAdd(cmd, nil))
	})
	id := bytes.TrimSpace([]byte(out))
	require.True(t, models.IsValidDocID(string(id)), "expected a valid doc id, got %q", string(id))
	return string(id)
}

// --- AC-1 ---

func TestDocList_EmptyProjectReturnsEmptyArray(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd, _ := newDocCmd()
	out := captureStdout(t, func() {
		require.NoError(t, runDocList(cmd, nil))
	})
	var docs []models.Doc
	require.NoError(t, json.Unmarshal([]byte(out), &docs))
	assert.Len(t, docs, 0)
	assert.Equal(t, "[]", bytesString(out))
}

func bytesString(s string) string { return string(bytes.TrimSpace([]byte(s))) }

func TestDocList_FieldsPresent(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	addDoc(t, "First", "prd", "a")
	addDoc(t, "Second", "arch", "b")

	cmd, _ := newDocCmd()
	out := captureStdout(t, func() {
		require.NoError(t, runDocList(cmd, nil))
	})
	var docs []models.Doc
	require.NoError(t, json.Unmarshal([]byte(out), &docs))
	require.Len(t, docs, 2)
	for _, d := range docs {
		assert.NotEmpty(t, d.ID)
		assert.NotEmpty(t, d.Slug)
		assert.NotEmpty(t, d.Title)
		assert.NotEmpty(t, d.Type)
		assert.False(t, d.Created.IsZero())
		assert.False(t, d.Updated.IsZero())
	}
}

// --- AC-2 ---

func TestDocAdd_BodyFileExistsAndRoundTrips(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	body := "# My PRD\n\nLine two with \"quotes\" and unicode café ☕\n"
	id := addDoc(t, "My PRD", "prd", body)

	store, err := getStorage()
	require.NoError(t, err)
	bodyPath := filepath.Join(store.GetRootDir(), "docs", id, "body.md")
	onDisk, err := os.ReadFile(bodyPath)
	require.NoError(t, err)
	assert.Equal(t, body, string(onDisk))

	// round-trips through doc show
	cmd, _ := newDocCmd()
	out := captureStdout(t, func() {
		require.NoError(t, runDocShow(cmd, []string{id}))
	})
	var res docShowResult
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Equal(t, body, res.Body)
}

// --- AC-3 ---

func TestDocIndex_FileReadableWithoutCLI(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	id := addDoc(t, "Indexed", "prd", "body")

	store, err := getStorage()
	require.NoError(t, err)
	indexPath := filepath.Join(store.GetRootDir(), "docs", "index.json")
	data, err := os.ReadFile(indexPath)
	require.NoError(t, err)

	var env struct {
		Docs []models.Doc `json:"docs"`
	}
	require.NoError(t, json.Unmarshal(data, &env))
	var found *models.Doc
	for i := range env.Docs {
		if env.Docs[i].ID == id {
			found = &env.Docs[i]
		}
	}
	require.NotNil(t, found)
	assert.NotEmpty(t, found.ID)
	assert.NotEmpty(t, found.Slug)
	assert.Equal(t, "Indexed", found.Title)
	assert.Equal(t, "prd", found.Type)
	assert.False(t, found.Created.IsZero())
	assert.False(t, found.Updated.IsZero())
}

// --- AC-4 ---

func TestDocLink_Bidirectional(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	taskID := addTaskForTest(t, "Linked Task")
	docID := addDoc(t, "Linked Doc", "prd", "body")

	cmd, _ := newDocCmd()
	require.NoError(t, runDocLink(cmd, []string{docID, taskID}))

	// doc show reports the linked task
	c2, _ := newDocCmd()
	out := captureStdout(t, func() {
		require.NoError(t, runDocShow(c2, []string{docID}))
	})
	var res docShowResult
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Contains(t, res.LinkedTasks, taskID)

	// task show reports the linked doc
	store, err := getStorage()
	require.NoError(t, err)
	task, err := store.LoadTask(taskID)
	require.NoError(t, err)
	assert.Contains(t, task.LinkedDocs, docID)
}

func TestDocLink_SurvivesRoundTrip(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	taskID := addTaskForTest(t, "RT Task")
	docID := addDoc(t, "RT Doc", "prd", "body")

	cmd, _ := newDocCmd()
	require.NoError(t, runDocLink(cmd, []string{docID, taskID}))

	// Force a FRESH reload from disk (R1 regression guard): a brand-new
	// getStorage() facade reading the persisted index, not an in-memory object.
	store, err := getStorage()
	require.NoError(t, err)

	task, err := store.LoadTask(taskID)
	require.NoError(t, err)
	assert.Contains(t, task.LinkedDocs, docID, "task->doc link must survive a store round-trip (R1: not stripped)")

	doc, err := store.Docs().Get(docID)
	require.NoError(t, err)
	assert.Contains(t, doc.LinkedTasks, taskID, "doc->task link must survive a store round-trip")
}

func TestDocLink_InvalidTaskRejected(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	docID := addDoc(t, "Doc", "prd", "body")

	cmd, errBuf := newDocCmd()
	err := runDocLink(cmd, []string{docID, "zzzz"})
	require.Error(t, err)
	assertJSONError(t, errBuf)
}

// --- AC-5 ---

func TestDocDelete_RemovesEntryAndDir(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	id := addDoc(t, "Doomed", "prd", "bye")
	store, err := getStorage()
	require.NoError(t, err)
	docDir := filepath.Join(store.GetRootDir(), "docs", id)

	cmd, _ := newDocCmd()
	out := captureStdout(t, func() {
		require.NoError(t, runDocDelete(cmd, []string{id}))
	})
	assert.Contains(t, out, "\"deleted\":true")

	// gone from index
	docs, err := store.Docs().List()
	require.NoError(t, err)
	for _, d := range docs {
		assert.NotEqual(t, id, d.ID)
	}
	// dir removed
	_, statErr := os.Stat(docDir)
	assert.True(t, os.IsNotExist(statErr))
}

func TestDocDelete_NonexistentErrors(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd, errBuf := newDocCmd()
	err := runDocDelete(cmd, []string{"zzzz"})
	require.Error(t, err)
	assertJSONError(t, errBuf)
}

// --- AC-7 ---

func TestExport_WarnsWhenDocsExist(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	addDoc(t, "Doc", "prd", "body")

	store, err := getStorage()
	require.NoError(t, err)
	docsPath := filepath.Join(store.GetRootDir(), "docs")

	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	out := captureStdout(t, func() {
		require.NoError(t, runExport(cmd, nil))
	})

	assert.Contains(t, errBuf.String(), "export does not include project docs")
	assert.Contains(t, errBuf.String(), docsPath)
	// stdout JSON still valid
	var env map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &env))
}

func TestExport_NoWarnWhenNoDocs(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	out := captureStdout(t, func() {
		require.NoError(t, runExport(cmd, nil))
	})
	assert.NotContains(t, errBuf.String(), "export does not include project docs")
	var env map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &env))
}

// --- AC-8 ---

func TestDocAdd_ADRDefaultsToProposed(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	id := addDoc(t, "Use X", models.DocTypeADR, "")

	// via show
	cmd, _ := newDocCmd()
	out := captureStdout(t, func() {
		require.NoError(t, runDocShow(cmd, []string{id}))
	})
	var res docShowResult
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Equal(t, models.ADRStatusProposed, res.Status)

	// via index.json
	store, err := getStorage()
	require.NoError(t, err)
	d, err := store.Docs().Get(id)
	require.NoError(t, err)
	assert.Equal(t, models.ADRStatusProposed, d.Status)
}

// --- AC-9 ---

func TestDocStatus_ValidTransitions(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	id := addDoc(t, "ADR", models.DocTypeADR, "")

	cmd, _ := newDocCmd()
	require.NoError(t, runDocStatus(cmd, []string{id, models.ADRStatusAccepted}))

	store, err := getStorage()
	require.NoError(t, err)
	d, err := store.Docs().Get(id)
	require.NoError(t, err)
	assert.Equal(t, models.ADRStatusAccepted, d.Status)

	c2, _ := newDocCmd()
	require.NoError(t, runDocStatus(c2, []string{id, models.ADRStatusSuperseded}))
	d2, err := store.Docs().Get(id)
	require.NoError(t, err)
	assert.Equal(t, models.ADRStatusSuperseded, d2.Status)
}

func TestDocStatus_InvalidRejected(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	id := addDoc(t, "ADR", models.DocTypeADR, "")
	// advance to accepted so we can attempt a backward/bogus move
	c0, _ := newDocCmd()
	require.NoError(t, runDocStatus(c0, []string{id, models.ADRStatusAccepted}))

	// bogus status
	c1, errBuf1 := newDocCmd()
	err := runDocStatus(c1, []string{id, "bogus"})
	require.Error(t, err)
	assertJSONError(t, errBuf1)

	// backward transition accepted -> proposed
	c2, errBuf2 := newDocCmd()
	err = runDocStatus(c2, []string{id, models.ADRStatusProposed})
	require.Error(t, err)
	assertJSONError(t, errBuf2)

	// stored status UNCHANGED (still accepted) after a fresh reload
	store, err := getStorage()
	require.NoError(t, err)
	d, err := store.Docs().Get(id)
	require.NoError(t, err)
	assert.Equal(t, models.ADRStatusAccepted, d.Status)
}

func TestDocStatus_NonExistentDoc(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	cmd, errBuf := newDocCmd()
	err := runDocStatus(cmd, []string{"zzzz", models.ADRStatusAccepted})
	require.Error(t, err)
	assertJSONError(t, errBuf)
}

// --- AC-10 ---

func TestDocSupersede_Bidirectional(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	oldID := addDoc(t, "Old ADR", models.DocTypeADR, "")
	newID := addDoc(t, "New ADR", models.DocTypeADR, "")

	cmd, _ := newDocCmd()
	require.NoError(t, runDocSupersede(cmd, []string{oldID, newID}))

	// fresh reload for round-trip
	store, err := getStorage()
	require.NoError(t, err)
	oldDoc, err := store.Docs().Get(oldID)
	require.NoError(t, err)
	assert.Equal(t, newID, oldDoc.SupersededBy)
	assert.Equal(t, models.ADRStatusSuperseded, oldDoc.Status)

	newDoc, err := store.Docs().Get(newID)
	require.NoError(t, err)
	assert.Equal(t, oldID, newDoc.Supersedes)
}

func TestDocSupersede_RejectsNonADRAndMissing(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	adrID := addDoc(t, "ADR", models.DocTypeADR, "")
	prdID := addDoc(t, "PRD", "prd", "")

	// old is non-ADR
	c1, errBuf1 := newDocCmd()
	err := runDocSupersede(c1, []string{prdID, adrID})
	require.Error(t, err)
	assertJSONError(t, errBuf1)

	// new is non-ADR
	c2, errBuf2 := newDocCmd()
	err = runDocSupersede(c2, []string{adrID, prdID})
	require.Error(t, err)
	assertJSONError(t, errBuf2)

	// missing id
	c3, errBuf3 := newDocCmd()
	err = runDocSupersede(c3, []string{adrID, "zzzz"})
	require.Error(t, err)
	assertJSONError(t, errBuf3)

	// self-supersede
	c4, errBuf4 := newDocCmd()
	err = runDocSupersede(c4, []string{adrID, adrID})
	require.Error(t, err)
	assertJSONError(t, errBuf4)
}

// --- AC-11 ---

func TestDoc_NonADRHasNoStatus(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	id := addDoc(t, "Plain PRD", "prd", "body")

	cmd, _ := newDocCmd()
	out := captureStdout(t, func() {
		require.NoError(t, runDocShow(cmd, []string{id}))
	})
	// status omitempty: absent from JSON
	assert.NotContains(t, out, "\"status\"")
	var res docShowResult
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Empty(t, res.Status)

	// index.json entry carries no status
	store, err := getStorage()
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(store.GetRootDir(), "docs", "index.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "\"status\"")
}

func TestDoc_StatusRejectedOnNonADR(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	id := addDoc(t, "Plain", "prd", "")

	cmd, errBuf := newDocCmd()
	err := runDocStatus(cmd, []string{id, models.ADRStatusAccepted})
	require.Error(t, err)
	assertJSONError(t, errBuf)

	// not mutated
	store, err := getStorage()
	require.NoError(t, err)
	d, err := store.Docs().Get(id)
	require.NoError(t, err)
	assert.Empty(t, d.Status)
}

func TestDoc_SupersedeRejectedOnNonADR(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	a := addDoc(t, "PlainA", "prd", "")
	b := addDoc(t, "PlainB", "prd", "")

	cmd, errBuf := newDocCmd()
	err := runDocSupersede(cmd, []string{a, b})
	require.Error(t, err)
	assertJSONError(t, errBuf)

	// neither mutated
	store, err := getStorage()
	require.NoError(t, err)
	da, _ := store.Docs().Get(a)
	db, _ := store.Docs().Get(b)
	assert.Empty(t, da.SupersededBy)
	assert.Empty(t, da.Status)
	assert.Empty(t, db.Supersedes)
}

func TestDoc_NonADRLifecycleUnaffectedByADRs(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	// Behavior with zero ADRs present.
	p1 := addDoc(t, "Doc One", "prd", "body")
	store, err := getStorage()
	require.NoError(t, err)
	d1, err := store.Docs().Get(p1)
	require.NoError(t, err)
	assert.Empty(t, d1.Status)

	// Now add an ADR and repeat the same operations on non-ADR docs.
	addDoc(t, "Some ADR", models.DocTypeADR, "")

	p2 := addDoc(t, "Doc Two", "prd", "body")
	d2, err := store.Docs().Get(p2)
	require.NoError(t, err)
	assert.Empty(t, d2.Status, "non-ADR status behavior must be identical whether or not ADRs exist")

	// link + delete a non-ADR doc with ADRs present
	taskID := addTaskForTest(t, "T")
	cl, _ := newDocCmd()
	require.NoError(t, runDocLink(cl, []string{p2, taskID}))
	cd, _ := newDocCmd()
	_ = captureStdout(t, func() {
		require.NoError(t, runDocDelete(cd, []string{p2}))
	})
	_, getErr := store.Docs().Get(p2)
	assert.Error(t, getErr)
}

// --- helpers ---

func assertJSONError(t *testing.T, errBuf *bytes.Buffer) {
	t.Helper()
	var obj struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(errBuf.Bytes()), &obj),
		"stderr must contain a parseable {\"error\":...} object, got %q", errBuf.String())
	assert.NotEmpty(t, obj.Error)
}

// addTaskForTest creates a task via runAdd and returns its id.
func addTaskForTest(t *testing.T, name string) string {
	t.Helper()
	resetAddFlags()
	store, err := getStorage()
	require.NoError(t, err)
	before, err := store.LoadAll()
	require.NoError(t, err)
	beforeIDs := make(map[string]bool)
	for _, tk := range before {
		beforeIDs[tk.ID] = true
	}
	_ = captureStdout(t, func() {
		require.NoError(t, runAdd(nil, []string{name}))
	})
	after, err := store.LoadAll()
	require.NoError(t, err)
	for _, tk := range after {
		if !beforeIDs[tk.ID] {
			return tk.ID
		}
	}
	t.Fatalf("could not determine newly added task id")
	return ""
}
