package models

import (
	"strings"
	"time"
)

// Note represents an observation or progress update on a task
type Note struct {
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// HistoryEntry records a stage transition for audit purposes.
type HistoryEntry struct {
	From   string    `json:"from"`
	To     string    `json:"to"`
	By     string    `json:"by,omitempty"`
	At     time.Time `json:"at"`
	Reason string    `json:"reason,omitempty"`
}

// Task represents a task in the work queue
type Task struct {
	ID                 string         `json:"id"`
	Name               string         `json:"name"`
	Description        string         `json:"description,omitempty"`
	Approach           string         `json:"approach,omitempty"`
	Verify             string         `json:"verify,omitempty"`
	Result             string         `json:"result,omitempty"`
	Outcome            string         `json:"outcome,omitempty"`
	AcceptanceCriteria string         `json:"acceptanceCriteria,omitempty"`
	ScopeOut           string         `json:"scopeOut,omitempty"`
	AffectedAreas      string         `json:"affectedAreas,omitempty"`
	TestStrategy       string         `json:"testStrategy,omitempty"`
	Risks              string         `json:"risks,omitempty"`
	Report             string         `json:"report,omitempty"`
	Parent             *string        `json:"parent"`
	Status             string         `json:"status"`
	BlockedBy          []string       `json:"blockedBy,omitempty"`
	Owner              *string        `json:"owner,omitempty"`
	Notes              []Note         `json:"notes,omitempty"`
	History            []HistoryEntry `json:"history,omitempty"`
	ManualBlockReason  string         `json:"manualBlockReason,omitempty"`
	BlockedFromStage   string         `json:"blockedFromStage,omitempty"`
	Created            time.Time      `json:"created"`
	Updated            time.Time      `json:"updated"`
}

// Valid status values
const (
	StatusCaptured   = "captured"
	StatusRefined    = "refined"
	StatusPlanned    = "planned"
	StatusReady      = "ready"
	StatusInProgress = "in-progress"
	StatusInReview   = "in-review"
	StatusDone       = "done"
)

// StageOrder defines the canonical forward order of lifecycle stages.
var StageOrder = []string{
	StatusCaptured, StatusRefined, StatusPlanned, StatusReady,
	StatusInProgress, StatusInReview, StatusDone,
}

// StageIndex returns the index of the given status in StageOrder, or -1 if invalid.
func StageIndex(status string) int {
	for i, s := range StageOrder {
		if s == status {
			return i
		}
	}
	return -1
}

// HasStructuredFields returns true when all three structured fields are non-empty.
// Used to distinguish legacy (pre-v4) tasks from structured tasks.
func (t *Task) HasStructuredFields() bool {
	return t.Approach != "" && t.Verify != "" && t.Result != ""
}

// IsValidStatus checks if a status value is valid
func IsValidStatus(status string) bool {
	return StageIndex(status) >= 0
}

// IsValidTaskID checks if an ID is a valid 4-character lowercase alphabetic string
func IsValidTaskID(id string) bool {
	if len(id) != 4 {
		return false
	}
	for _, c := range id {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

// NormalizeTaskID converts an ID to lowercase for case-insensitive input
func NormalizeTaskID(id string) string {
	return strings.ToLower(id)
}
