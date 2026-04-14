package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/simonspoon/limbo/internal/models"
)

// ContextDirName is the directory name under .limbo that holds per-task context.
const ContextDirName = "context"

// contextFileName is the file inside each task's context directory.
const contextFileName = "context.md"

// knownSectionOrder defines the fixed display order for known sections.
// Any section not in this list is considered custom and sorted alphabetically
// after the known sections but before Notes.
var knownSectionOrder = map[string]int{
	"Approach":           0,
	"Verify":             1,
	"Result":             2,
	"Outcome":            3,
	"AcceptanceCriteria": 4,
	"ScopeOut":           5,
	"AffectedAreas":      6,
	"TestStrategy":       7,
	"Risks":              8,
	"Report":             9,
	"Description":        10,
}

// knownSectionNames is the allow-list of "## <name>" headings that
// parseContextFile recognizes as section boundaries. Any other "## " line
// is treated as part of the current section's content. This lets content
// fields (Description, Approach, etc.) contain arbitrary markdown headings
// without getting truncated on read.
//
// The set includes every key of knownSectionOrder plus:
//   - "Notes"  — written by AppendNote
//   - "Action" — legacy v4→v5 compat; mergeContext falls back to Action
//     when Approach is empty, and migrateFromV5 rewrites Action→Approach
//     by reading parsed sections.
var knownSectionNames = func() map[string]bool {
	m := make(map[string]bool, len(knownSectionOrder)+2)
	for name := range knownSectionOrder {
		m[name] = true
	}
	m["Notes"] = true
	m["Action"] = true
	return m
}()

// ContextDir returns the path to a task's context directory.
func (s *Storage) ContextDir(id string) string {
	return filepath.Join(s.rootDir, LimboDir, ContextDirName, id)
}

// contextFilePath returns the path to a task's context.md file.
func (s *Storage) contextFilePath(id string) string {
	return filepath.Join(s.ContextDir(id), contextFileName)
}

// ReadContext reads and parses a task's context.md file.
// Returns a map of section name to content (trimmed).
// Returns empty map (not error) if file doesn't exist.
func (s *Storage) ReadContext(id string) (map[string]string, error) {
	data, err := os.ReadFile(s.contextFilePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("failed to read context file: %w", err)
	}

	return parseContextFile(string(data)), nil
}

// WriteContext writes a section map to a task's context.md file.
// Creates the context directory if needed.
// Sections are ordered per the ordering rules. Empty sections are omitted.
func (s *Storage) WriteContext(id string, sections map[string]string) error {
	dir := s.ContextDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create context directory: %w", err)
	}

	content := renderContextFile(sections)
	if err := os.WriteFile(s.contextFilePath(id), []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write context file: %w", err)
	}
	return nil
}

// AppendNote appends a timestamped note to the Notes section of context.md.
// Creates the file/directory if needed.
func (s *Storage) AppendNote(id, content string, timestamp time.Time) error {
	dir := s.ContextDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create context directory: %w", err)
	}

	filePath := s.contextFilePath(id)
	noteEntry := fmt.Sprintf("### %s\n%s\n", timestamp.Format(time.RFC3339), content)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new file with just a Notes section
			fileContent := "## Notes\n" + noteEntry
			return os.WriteFile(filePath, []byte(fileContent), 0644)
		}
		return fmt.Errorf("failed to read context file: %w", err)
	}

	fileStr := string(data)

	// Check if Notes section exists by looking for "## Notes" at start of a line
	notesIdx := findNotesSection(fileStr)
	if notesIdx == -1 {
		// No Notes section — append one at the end
		if !strings.HasSuffix(fileStr, "\n") {
			fileStr += "\n"
		}
		fileStr += "\n## Notes\n" + noteEntry
	} else {
		// Append to existing Notes section
		if !strings.HasSuffix(fileStr, "\n") {
			fileStr += "\n"
		}
		fileStr += "\n" + noteEntry
	}

	return os.WriteFile(filePath, []byte(fileStr), 0644)
}

// DeleteContext removes a task's entire context directory.
// No error if directory doesn't exist.
func (s *Storage) DeleteContext(id string) error {
	dir := s.ContextDir(id)
	err := os.RemoveAll(dir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete context directory: %w", err)
	}
	return nil
}

// ParseNotes extracts individual notes from a Notes section string.
// Each note starts with "### <RFC3339 timestamp>" optionally followed by " — <author>".
// Author suffixes are included in the note Content.
func ParseNotes(notesContent string) []models.Note {
	if strings.TrimSpace(notesContent) == "" {
		return nil
	}

	var notes []models.Note
	lines := strings.Split(notesContent, "\n")

	var currentTimestamp time.Time
	var currentLines []string
	inNote := false

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			// Flush previous note
			if inNote {
				notes = append(notes, models.Note{
					Timestamp: currentTimestamp,
					Content:   strings.TrimSpace(strings.Join(currentLines, "\n")),
				})
			}

			// Parse the heading: "### 2026-02-20T10:00:00Z" or "### 2026-02-20T10:00:00Z — pm"
			heading := strings.TrimPrefix(line, "### ")
			ts, content := parseNoteHeading(heading)
			currentTimestamp = ts
			currentLines = nil
			if content != "" {
				currentLines = append(currentLines, content)
			}
			inNote = true
		} else if inNote {
			currentLines = append(currentLines, line)
		}
	}

	// Flush last note
	if inNote {
		notes = append(notes, models.Note{
			Timestamp: currentTimestamp,
			Content:   strings.TrimSpace(strings.Join(currentLines, "\n")),
		})
	}

	return notes
}

// parseNoteHeading parses a note heading like "2026-02-20T10:00:00Z — pm".
// Returns the timestamp and any author suffix as content.
func parseNoteHeading(heading string) (time.Time, string) {
	// Try to split on " — " (em dash with spaces) for author
	parts := strings.SplitN(heading, " \u2014 ", 2)
	tsStr := strings.TrimSpace(parts[0])

	ts, _ := time.Parse(time.RFC3339, tsStr)

	if len(parts) == 2 {
		return ts, "\u2014 " + strings.TrimSpace(parts[1])
	}
	return ts, ""
}

// parseContextFile parses a context.md file into a section map.
//
// Only "## <name>" lines whose <name> is in knownSectionNames are treated
// as section boundaries. Any other "## ..." line is kept verbatim as part
// of the current section's content, so content fields can safely contain
// markdown subheadings.
//
// Residual limitation: a content field that contains a literal "## <name>"
// line where <name> happens to be a known section (e.g. a Description that
// contains a line "## Approach") will still be hijacked and split into two
// sections. Avoid embedding literal known-section headings inside content.
func parseContextFile(content string) map[string]string {
	sections := make(map[string]string)
	if strings.TrimSpace(content) == "" {
		return sections
	}

	lines := strings.Split(content, "\n")
	var currentSection string
	var currentLines []string
	inCodeBlock := false

	for _, line := range lines {
		// Track code blocks to avoid splitting on ## inside them
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
		}

		if !inCodeBlock && strings.HasPrefix(line, "## ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if knownSectionNames[name] {
				// Flush previous section
				if currentSection != "" {
					sections[currentSection] = normalizeSectionContent(currentLines)
				}
				currentSection = name
				currentLines = nil
				continue
			}
			// Unknown "## ..." heading — treat as section content.
			if currentSection != "" {
				currentLines = append(currentLines, line)
			}
			continue
		}

		if currentSection != "" {
			currentLines = append(currentLines, line)
		}
	}

	// Flush last section
	if currentSection != "" {
		sections[currentSection] = normalizeSectionContent(currentLines)
	}

	return sections
}

// normalizeSectionContent joins lines and trims to a single trailing newline per section.
func normalizeSectionContent(lines []string) string {
	content := strings.Join(lines, "\n")
	return strings.TrimSpace(content)
}

// renderContextFile renders a section map to a context.md string.
// Sections are ordered per the ordering rules. Empty sections are omitted.
func renderContextFile(sections map[string]string) string {
	ordered := sortSections(sections)

	var parts []string
	for _, name := range ordered {
		content := sections[name]
		if strings.TrimSpace(content) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("## %s\n%s", name, content))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n") + "\n"
}

// sortSections returns section names in the correct display order:
// 1. Known sections in fixed order
// 2. Custom sections alphabetically
// 3. Notes always last
func sortSections(sections map[string]string) []string {
	var known []string
	var custom []string
	hasNotes := false

	for name := range sections {
		if strings.TrimSpace(sections[name]) == "" {
			continue
		}
		if name == "Notes" {
			hasNotes = true
			continue
		}
		if _, ok := knownSectionOrder[name]; ok {
			known = append(known, name)
		} else {
			custom = append(custom, name)
		}
	}

	sort.Slice(known, func(i, j int) bool {
		return knownSectionOrder[known[i]] < knownSectionOrder[known[j]]
	})
	sort.Strings(custom)

	result := make([]string, 0, len(known)+len(custom)+1)
	result = append(result, known...)
	result = append(result, custom...)
	if hasNotes {
		result = append(result, "Notes")
	}
	return result
}

// findNotesSection finds the byte offset of "## Notes" in the file content.
// Returns -1 if not found.
func findNotesSection(content string) int {
	lines := strings.Split(content, "\n")
	offset := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "## Notes" {
			return offset
		}
		offset += len(line) + 1 // +1 for the newline
	}
	return -1
}
