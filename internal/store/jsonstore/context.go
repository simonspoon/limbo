package jsonstore

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/simonspoon/limbo/internal/models"
)

// nowUTC is the time source for AppendNote timestamps. The Store interface
// provides no timestamp argument, so notes are stamped with the current UTC
// time. Kept as a package var so tests can pin it if needed.
var nowUTC = func() time.Time { return time.Now().UTC() }

// knownSectionOrder defines the fixed display order for known sections in the
// rendered sidecar. Sections not in this list sort alphabetically after the
// known ones but before Notes.
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
// parseContextFile treats as section boundaries. Any other "## " line is kept
// verbatim as the current section's content, so content fields can carry
// arbitrary markdown subheadings. "Notes" is written by AppendNote; "Action"
// is legacy compatibility (mapped to Approach on merge).
var knownSectionNames = func() map[string]bool {
	m := make(map[string]bool, len(knownSectionOrder)+2)
	for name := range knownSectionOrder {
		m[name] = true
	}
	m["Notes"] = true
	m["Action"] = true
	return m
}()

// extractContext builds a section map from a task's structured content fields
// (including a rendered Notes section).
func extractContext(task *models.Task) map[string]string {
	sections := make(map[string]string)
	if task.Approach != "" {
		sections["Approach"] = task.Approach
	}
	if task.Verify != "" {
		sections["Verify"] = task.Verify
	}
	if task.Result != "" {
		sections["Result"] = task.Result
	}
	if task.Outcome != "" {
		sections["Outcome"] = task.Outcome
	}
	if task.AcceptanceCriteria != "" {
		sections["AcceptanceCriteria"] = task.AcceptanceCriteria
	}
	if task.ScopeOut != "" {
		sections["ScopeOut"] = task.ScopeOut
	}
	if task.AffectedAreas != "" {
		sections["AffectedAreas"] = task.AffectedAreas
	}
	if task.TestStrategy != "" {
		sections["TestStrategy"] = task.TestStrategy
	}
	if task.Risks != "" {
		sections["Risks"] = task.Risks
	}
	if task.Report != "" {
		sections["Report"] = task.Report
	}
	if task.Description != "" {
		sections["Description"] = task.Description
	}
	if len(task.Notes) > 0 {
		sections["Notes"] = renderNotes(task.Notes)
	}
	return sections
}

// renderNotes renders a note slice into the "### <RFC3339>\n<content>" format.
func renderNotes(notes []models.Note) string {
	var b strings.Builder
	for i, note := range notes {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("### ")
		b.WriteString(note.Timestamp.Format(time.RFC3339))
		b.WriteString("\n")
		b.WriteString(note.Content)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// parseContextFile parses a context.md body into a section map. Only known
// "## <name>" headings are treated as boundaries; other "## ..." lines and any
// lines inside fenced code blocks are kept as the current section's content.
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
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
		}

		if !inCodeBlock && strings.HasPrefix(line, "## ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if knownSectionNames[name] {
				if currentSection != "" {
					sections[currentSection] = normalizeSectionContent(currentLines)
				}
				currentSection = name
				currentLines = nil
				continue
			}
			if currentSection != "" {
				currentLines = append(currentLines, line)
			}
			continue
		}

		if currentSection != "" {
			currentLines = append(currentLines, line)
		}
	}

	if currentSection != "" {
		sections[currentSection] = normalizeSectionContent(currentLines)
	}

	return sections
}

func normalizeSectionContent(lines []string) string {
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// renderContextFile renders a section map into a context.md body. Empty
// sections are omitted; sections are ordered per sortSections.
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

// sortSections orders section names: known sections in fixed order, then
// custom sections alphabetically, with Notes always last.
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

// parseNotes extracts individual notes from a Notes section body. Each note
// begins with "### <RFC3339 timestamp>" optionally followed by " — <author>"
// (the author suffix is folded into the note content).
func parseNotes(notesContent string) []models.Note {
	if strings.TrimSpace(notesContent) == "" {
		return nil
	}

	var notes []models.Note
	lines := strings.Split(notesContent, "\n")

	var currentTimestamp time.Time
	var currentLines []string
	inNote := false

	flush := func() {
		notes = append(notes, models.Note{
			Timestamp: currentTimestamp,
			Content:   strings.TrimSpace(strings.Join(currentLines, "\n")),
		})
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") {
			if inNote {
				flush()
			}
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

	if inNote {
		flush()
	}

	return notes
}

// parseNoteHeading parses a note heading like "2026-02-20T10:00:00Z — pm",
// returning the timestamp and any author suffix folded into content.
func parseNoteHeading(heading string) (time.Time, string) {
	parts := strings.SplitN(heading, " — ", 2)
	tsStr := strings.TrimSpace(parts[0])
	ts, _ := time.Parse(time.RFC3339, tsStr)
	if len(parts) == 2 {
		return ts, "— " + strings.TrimSpace(parts[1])
	}
	return ts, ""
}
