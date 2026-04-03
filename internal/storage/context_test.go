package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupContextTest(t *testing.T) *Storage {
	t.Helper()
	dir := t.TempDir()
	store := NewStorageAt(dir)
	require.NoError(t, store.Init())
	return store
}

func TestReadContext_FileDoesNotExist(t *testing.T) {
	store := setupContextTest(t)

	sections, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Empty(t, sections)
}

func TestWriteAndReadContext_RoundTrip(t *testing.T) {
	store := setupContextTest(t)

	input := map[string]string{
		"Action":      "Implement JWT login and token refresh endpoints.",
		"Verify":      "1. go test ./internal/auth/... passes\n2. curl POST /login returns 200",
		"Result":      "List endpoints added and test results",
		"Description": "Add JWT-based login and token refresh",
	}

	err := store.WriteContext("abcd", input)
	require.NoError(t, err)

	got, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Equal(t, input, got)
}

func TestWriteContext_SectionOrdering(t *testing.T) {
	store := setupContextTest(t)

	// Write in random order
	sections := map[string]string{
		"Notes":       "### 2026-02-20T10:00:00Z\nStarted",
		"Description": "Some desc",
		"Verify":      "Run tests",
		"Action":      "Do the thing",
		"Zebra":       "Custom section Z",
		"Result":      "Report back",
		"Alpha":       "Custom section A",
		"Outcome":     "It worked",
	}

	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	// Read the raw file and verify ordering
	data, err := os.ReadFile(store.contextFilePath("abcd"))
	require.NoError(t, err)
	content := string(data)

	// Find positions of each section header
	actionPos := indexOf(content, "## Action")
	verifyPos := indexOf(content, "## Verify")
	resultPos := indexOf(content, "## Result")
	outcomePos := indexOf(content, "## Outcome")
	descPos := indexOf(content, "## Description")
	alphaPos := indexOf(content, "## Alpha")
	zebraPos := indexOf(content, "## Zebra")
	notesPos := indexOf(content, "## Notes")

	assert.True(t, actionPos < verifyPos, "Action should come before Verify")
	assert.True(t, verifyPos < resultPos, "Verify should come before Result")
	assert.True(t, resultPos < outcomePos, "Result should come before Outcome")
	assert.True(t, outcomePos < descPos, "Outcome should come before Description")
	assert.True(t, descPos < alphaPos, "Description should come before Alpha (custom)")
	assert.True(t, alphaPos < zebraPos, "Alpha should come before Zebra (alphabetical)")
	assert.True(t, zebraPos < notesPos, "Zebra should come before Notes (Notes always last)")
}

func TestWriteContext_EmptySectionsOmitted(t *testing.T) {
	store := setupContextTest(t)

	sections := map[string]string{
		"Action":      "Do something",
		"Verify":      "",
		"Result":      "   ",
		"Description": "A real description",
	}

	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	data, err := os.ReadFile(store.contextFilePath("abcd"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "## Action")
	assert.Contains(t, content, "## Description")
	assert.NotContains(t, content, "## Verify")
	assert.NotContains(t, content, "## Result")
}

func TestAppendNote_EmptyFile(t *testing.T) {
	store := setupContextTest(t)

	ts := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	err := store.AppendNote("abcd", "Started with login endpoint", ts)
	require.NoError(t, err)

	// Verify file was created
	data, err := os.ReadFile(store.contextFilePath("abcd"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "## Notes")
	assert.Contains(t, content, "### 2026-02-20T10:00:00Z")
	assert.Contains(t, content, "Started with login endpoint")
}

func TestAppendNote_ExistingFileNoNotes(t *testing.T) {
	store := setupContextTest(t)

	// Write initial content without Notes
	sections := map[string]string{
		"Action": "Do the thing",
		"Verify": "Check it",
	}
	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	// Append a note
	ts := time.Date(2026, 2, 20, 11, 0, 0, 0, time.UTC)
	err = store.AppendNote("abcd", "Progress update", ts)
	require.NoError(t, err)

	// Read back and verify both original sections and Notes exist
	got, err := store.ReadContext("abcd")
	require.NoError(t, err)

	assert.Equal(t, "Do the thing", got["Action"])
	assert.Equal(t, "Check it", got["Verify"])
	assert.Contains(t, got["Notes"], "### 2026-02-20T11:00:00Z")
	assert.Contains(t, got["Notes"], "Progress update")
}

func TestAppendNote_ExistingNotes(t *testing.T) {
	store := setupContextTest(t)

	// Add first note
	ts1 := time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC)
	err := store.AppendNote("abcd", "First note", ts1)
	require.NoError(t, err)

	// Add second note
	ts2 := time.Date(2026, 2, 20, 11, 30, 0, 0, time.UTC)
	err = store.AppendNote("abcd", "Second note", ts2)
	require.NoError(t, err)

	// Read back and verify both notes exist
	got, err := store.ReadContext("abcd")
	require.NoError(t, err)

	assert.Contains(t, got["Notes"], "### 2026-02-20T10:00:00Z")
	assert.Contains(t, got["Notes"], "First note")
	assert.Contains(t, got["Notes"], "### 2026-02-20T11:30:00Z")
	assert.Contains(t, got["Notes"], "Second note")
}

func TestParseNotes_MultipleNotes(t *testing.T) {
	notesContent := `### 2026-02-20T10:00:00Z
Started with login endpoint

### 2026-02-20T11:30:00Z
Scope clarified: refresh tokens use rotating scheme`

	notes := ParseNotes(notesContent)
	require.Len(t, notes, 2)

	assert.Equal(t, time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC), notes[0].Timestamp)
	assert.Equal(t, "Started with login endpoint", notes[0].Content)

	assert.Equal(t, time.Date(2026, 2, 20, 11, 30, 0, 0, time.UTC), notes[1].Timestamp)
	assert.Equal(t, "Scope clarified: refresh tokens use rotating scheme", notes[1].Content)
}

func TestParseNotes_WithAuthorSuffix(t *testing.T) {
	notesContent := "### 2026-02-20T11:30:00Z \u2014 pm\nScope clarified: refresh tokens use rotating scheme"

	notes := ParseNotes(notesContent)
	require.Len(t, notes, 1)

	assert.Equal(t, time.Date(2026, 2, 20, 11, 30, 0, 0, time.UTC), notes[0].Timestamp)
	assert.Contains(t, notes[0].Content, "\u2014 pm")
	assert.Contains(t, notes[0].Content, "Scope clarified")
}

func TestParseNotes_EmptyContent(t *testing.T) {
	notes := ParseNotes("")
	assert.Nil(t, notes)

	notes = ParseNotes("   \n  ")
	assert.Nil(t, notes)
}

func TestDeleteContext_RemovesDirectory(t *testing.T) {
	store := setupContextTest(t)

	// Write some content first
	sections := map[string]string{
		"Action": "Do the thing",
	}
	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	// Verify directory exists
	_, err = os.Stat(store.ContextDir("abcd"))
	require.NoError(t, err)

	// Delete context
	err = store.DeleteContext("abcd")
	require.NoError(t, err)

	// Verify directory is gone
	_, err = os.Stat(store.ContextDir("abcd"))
	assert.True(t, os.IsNotExist(err))
}

func TestDeleteContext_NoErrorIfMissing(t *testing.T) {
	store := setupContextTest(t)

	err := store.DeleteContext("zzzz")
	assert.NoError(t, err)
}

func TestContextDir_Path(t *testing.T) {
	store := NewStorageAt("/tmp/project")
	expected := filepath.Join("/tmp/project", LimboDir, ContextDirName, "abcd")
	assert.Equal(t, expected, store.ContextDir("abcd"))
}

func TestReadContext_EmptyFile(t *testing.T) {
	store := setupContextTest(t)

	// Create directory and empty file
	dir := store.ContextDir("abcd")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(store.contextFilePath("abcd"), []byte(""), 0644))

	sections, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Empty(t, sections)
}

func TestWriteAndReadContext_SectionWithNoContent(t *testing.T) {
	store := setupContextTest(t)

	// Manually write a file with a section that has only a heading
	content := "## Action\n\n## Verify\nRun tests\n"
	dir := store.ContextDir("abcd")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(store.contextFilePath("abcd"), []byte(content), 0644))

	got, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Equal(t, "", got["Action"])
	assert.Equal(t, "Run tests", got["Verify"])
}

func TestWriteAndReadContext_MultiLineContent(t *testing.T) {
	store := setupContextTest(t)

	sections := map[string]string{
		"Action": "Step 1: Do X\nStep 2: Do Y\n\nStep 3: Do Z",
		"Verify": "- check A\n- check B\n- check C",
	}

	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	got, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Equal(t, sections, got)
}

func TestWriteAndReadContext_CodeBlocksPreserved(t *testing.T) {
	store := setupContextTest(t)

	sections := map[string]string{
		"Action": "Run this:\n```markdown\n## Not a section\nSome content\n```\nThen check output",
	}

	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	got, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Equal(t, sections["Action"], got["Action"])
	// Verify we didn't split on ## inside code block
	_, hasNotASection := got["Not a section"]
	assert.False(t, hasNotASection)
}

func TestParseNotes_MultiLineNoteContent(t *testing.T) {
	notesContent := `### 2026-02-20T10:00:00Z
First line of note.
Second line of note.

Third paragraph.

### 2026-02-20T11:00:00Z
Short note`

	notes := ParseNotes(notesContent)
	require.Len(t, notes, 2)

	assert.Equal(t, "First line of note.\nSecond line of note.\n\nThird paragraph.", notes[0].Content)
	assert.Equal(t, "Short note", notes[1].Content)
}

// indexOf returns the byte position of substr in s, or -1 if not found.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
