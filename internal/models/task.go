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

// Task represents a task in the work queue
type Task struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Action      string    `json:"action,omitempty"`
	Verify      string    `json:"verify,omitempty"`
	Result      string    `json:"result,omitempty"`
	Outcome     string    `json:"outcome,omitempty"`
	Parent      *string   `json:"parent"`
	Status      string    `json:"status"`
	BlockedBy   []string  `json:"blockedBy,omitempty"`
	Owner       *string   `json:"owner,omitempty"`
	Notes       []Note    `json:"notes,omitempty"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
}

// Valid status values
const (
	StatusTodo       = "todo"
	StatusInProgress = "in-progress"
	StatusDone       = "done"
)

// HasStructuredFields returns true when all three structured fields are non-empty.
// Used to distinguish legacy (pre-v4) tasks from structured tasks.
func (t *Task) HasStructuredFields() bool {
	return t.Action != "" && t.Verify != "" && t.Result != ""
}

// IsValidStatus checks if a status value is valid
func IsValidStatus(status string) bool {
	return status == StatusTodo || status == StatusInProgress || status == StatusDone
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
