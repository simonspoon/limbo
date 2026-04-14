package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/simonspoon/limbo/internal/models"
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
		"Approach":    "Implement JWT login and token refresh endpoints.",
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

	// Write in random order (known sections only — custom "## Foo"
	// headings inside content are now preserved in place by parseContextFile
	// rather than round-tripping as top-level sections).
	sections := map[string]string{
		"Notes":       "### 2026-02-20T10:00:00Z\nStarted",
		"Description": "Some desc",
		"Verify":      "Run tests",
		"Approach":    "Do the thing",
		"Result":      "Report back",
		"Outcome":     "It worked",
	}

	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	// Read the raw file and verify ordering
	data, err := os.ReadFile(store.contextFilePath("abcd"))
	require.NoError(t, err)
	content := string(data)

	// Find positions of each section header
	approachPos := indexOf(content, "## Approach")
	verifyPos := indexOf(content, "## Verify")
	resultPos := indexOf(content, "## Result")
	outcomePos := indexOf(content, "## Outcome")
	descPos := indexOf(content, "## Description")
	notesPos := indexOf(content, "## Notes")

	assert.True(t, approachPos < verifyPos, "Approach should come before Verify")
	assert.True(t, verifyPos < resultPos, "Verify should come before Result")
	assert.True(t, resultPos < outcomePos, "Result should come before Outcome")
	assert.True(t, outcomePos < descPos, "Outcome should come before Description")
	assert.True(t, descPos < notesPos, "Description should come before Notes (Notes always last)")
}

func TestWriteContext_EmptySectionsOmitted(t *testing.T) {
	store := setupContextTest(t)

	sections := map[string]string{
		"Approach":    "Do something",
		"Verify":      "",
		"Result":      "   ",
		"Description": "A real description",
	}

	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	data, err := os.ReadFile(store.contextFilePath("abcd"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "## Approach")
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
		"Approach": "Do the thing",
		"Verify":   "Check it",
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

	assert.Equal(t, "Do the thing", got["Approach"])
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
		"Approach": "Do the thing",
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
	dir := t.TempDir()
	store := NewStorageAt(dir)
	expected := filepath.Join(dir, LimboDir, ContextDirName, "abcd")
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
	content := "## Approach\n\n## Verify\nRun tests\n"
	dir := store.ContextDir("abcd")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(store.contextFilePath("abcd"), []byte(content), 0644))

	got, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Equal(t, "", got["Approach"])
	assert.Equal(t, "Run tests", got["Verify"])
}

func TestWriteAndReadContext_MultiLineContent(t *testing.T) {
	store := setupContextTest(t)

	sections := map[string]string{
		"Approach": "Step 1: Do X\nStep 2: Do Y\n\nStep 3: Do Z",
		"Verify":   "- check A\n- check B\n- check C",
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
		"Approach": "Run this:\n```markdown\n## Not a section\nSome content\n```\nThen check output",
	}

	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	got, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Equal(t, sections["Approach"], got["Approach"])
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

func TestWriteAndReadContext_NewFields(t *testing.T) {
	store := setupContextTest(t)

	input := map[string]string{
		"Approach":           "Design the API endpoints",
		"Verify":             "Run integration tests",
		"Result":             "All tests pass",
		"AcceptanceCriteria": "Endpoints return correct status codes",
		"ScopeOut":           "No frontend changes",
		"AffectedAreas":      "internal/api, internal/handlers",
		"TestStrategy":       "Unit + integration tests",
		"Risks":              "Breaking change to existing clients",
		"Report":             "Summary of changes and test results",
	}

	err := store.WriteContext("abcd", input)
	require.NoError(t, err)

	got, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Equal(t, input, got)
}

func TestWriteContext_NewFieldOrdering(t *testing.T) {
	store := setupContextTest(t)

	sections := map[string]string{
		"Report":             "Final report",
		"Risks":              "Some risks",
		"TestStrategy":       "Test plan",
		"AffectedAreas":      "Some areas",
		"ScopeOut":           "Not in scope",
		"AcceptanceCriteria": "Must pass",
		"Description":        "A description",
		"Approach":           "The approach",
		"Verify":             "Run tests",
		"Result":             "Report results",
		"Outcome":            "Shipped",
	}

	err := store.WriteContext("abcd", sections)
	require.NoError(t, err)

	data, err := os.ReadFile(store.contextFilePath("abcd"))
	require.NoError(t, err)
	content := string(data)

	approachPos := indexOf(content, "## Approach")
	verifyPos := indexOf(content, "## Verify")
	resultPos := indexOf(content, "## Result")
	outcomePos := indexOf(content, "## Outcome")
	acPos := indexOf(content, "## AcceptanceCriteria")
	scopePos := indexOf(content, "## ScopeOut")
	areasPos := indexOf(content, "## AffectedAreas")
	testPos := indexOf(content, "## TestStrategy")
	risksPos := indexOf(content, "## Risks")
	reportPos := indexOf(content, "## Report")
	descPos := indexOf(content, "## Description")

	assert.True(t, approachPos < verifyPos, "Approach before Verify")
	assert.True(t, verifyPos < resultPos, "Verify before Result")
	assert.True(t, resultPos < outcomePos, "Result before Outcome")
	assert.True(t, outcomePos < acPos, "Outcome before AcceptanceCriteria")
	assert.True(t, acPos < scopePos, "AcceptanceCriteria before ScopeOut")
	assert.True(t, scopePos < areasPos, "ScopeOut before AffectedAreas")
	assert.True(t, areasPos < testPos, "AffectedAreas before TestStrategy")
	assert.True(t, testPos < risksPos, "TestStrategy before Risks")
	assert.True(t, risksPos < reportPos, "Risks before Report")
	assert.True(t, reportPos < descPos, "Report before Description")
}

func TestMergeContext_ActionFallback(t *testing.T) {
	store := setupContextTest(t)

	// Write a context file with "Action" section (simulating stale v5 data)
	dir := store.ContextDir("abcd")
	require.NoError(t, os.MkdirAll(dir, 0755))
	content := "## Action\nDo the old thing\n\n## Verify\nCheck it\n"
	require.NoError(t, os.WriteFile(store.contextFilePath("abcd"), []byte(content), 0644))

	// mergeContext should map Action → Approach
	task := &models.Task{ID: "abcd"}
	err := store.mergeContext(task)
	require.NoError(t, err)

	assert.Equal(t, "Do the old thing", task.Approach)
	assert.Equal(t, "Check it", task.Verify)
}

func TestParseContextFile_H2InsideSectionPreserved(t *testing.T) {
	cases := []struct {
		name    string
		content string
		section string
		want    string
	}{
		{
			name: "description with h2 between paragraphs",
			content: "## Description\n" +
				"First paragraph.\n" +
				"\n" +
				"## SomeHeading\n" +
				"\n" +
				"Second paragraph.\n",
			section: "Description",
			want: "First paragraph.\n" +
				"\n" +
				"## SomeHeading\n" +
				"\n" +
				"Second paragraph.",
		},
		{
			name: "approach with multiple unknown h2 lines",
			content: "## Approach\n" +
				"Intro.\n" +
				"\n" +
				"## Step One\n" +
				"Do A.\n" +
				"\n" +
				"## Step Two\n" +
				"Do B.\n",
			section: "Approach",
			want: "Intro.\n" +
				"\n" +
				"## Step One\n" +
				"Do A.\n" +
				"\n" +
				"## Step Two\n" +
				"Do B.",
		},
		{
			name: "h2 immediately after section header",
			content: "## Description\n" +
				"## Leading Heading\n" +
				"\n" +
				"Body text.\n",
			section: "Description",
			want: "## Leading Heading\n" +
				"\n" +
				"Body text.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sections := parseContextFile(tc.content)
			assert.Equal(t, tc.want, sections[tc.section])
			// The unknown heading must not have become its own section.
			_, hasLeading := sections["Leading Heading"]
			_, hasSome := sections["SomeHeading"]
			_, hasStep1 := sections["Step One"]
			_, hasStep2 := sections["Step Two"]
			assert.False(t, hasLeading)
			assert.False(t, hasSome)
			assert.False(t, hasStep1)
			assert.False(t, hasStep2)
		})
	}
}

func TestWriteAndReadContext_DescriptionWithH2Heading(t *testing.T) {
	store := setupContextTest(t)

	desc := "First paragraph.\n" +
		"\n" +
		"## Heading\n" +
		"\n" +
		"Second paragraph.\n" +
		"\n" +
		"Third paragraph."

	input := map[string]string{
		"Description": desc,
	}

	err := store.WriteContext("abcd", input)
	require.NoError(t, err)

	got, err := store.ReadContext("abcd")
	require.NoError(t, err)
	assert.Equal(t, desc, got["Description"])
	// The embedded "## Heading" must not have leaked into its own section.
	_, hasPhantom := got["Heading"]
	assert.False(t, hasPhantom)
}

func TestParseContextFile_KnownHeaderStillSplits(t *testing.T) {
	content := "## Description\n" +
		"A real description.\n" +
		"\n" +
		"## Verify\n" +
		"Run tests.\n"

	sections := parseContextFile(content)
	assert.Equal(t, "A real description.", sections["Description"])
	assert.Equal(t, "Run tests.", sections["Verify"])
}

func TestParseContextFile_NotesAndActionRecognized(t *testing.T) {
	// Both "## Notes" and "## Action" must still be treated as section
	// boundaries: Notes is written by AppendNote, Action is legacy v4→v5
	// compat used by mergeContext and the v5→v6 migration.
	content := "## Action\n" +
		"Do the old thing.\n" +
		"\n" +
		"## Verify\n" +
		"Check it.\n" +
		"\n" +
		"## Notes\n" +
		"### 2026-02-20T10:00:00Z\n" +
		"Some note.\n"

	sections := parseContextFile(content)
	assert.Equal(t, "Do the old thing.", sections["Action"])
	assert.Equal(t, "Check it.", sections["Verify"])
	assert.Contains(t, sections["Notes"], "### 2026-02-20T10:00:00Z")
	assert.Contains(t, sections["Notes"], "Some note.")
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
