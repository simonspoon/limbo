package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidStatus(t *testing.T) {
	// Valid statuses
	assert.True(t, IsValidStatus(StatusTodo))
	assert.True(t, IsValidStatus(StatusInProgress))
	assert.True(t, IsValidStatus(StatusDone))

	// Invalid statuses
	assert.False(t, IsValidStatus(""))
	assert.False(t, IsValidStatus("invalid"))
	assert.False(t, IsValidStatus("DONE"))        // case sensitive
	assert.False(t, IsValidStatus("TODO"))        // case sensitive
	assert.False(t, IsValidStatus("in_progress")) // wrong format
}

func TestIsValidTaskID(t *testing.T) {
	// Valid IDs: exactly 4 lowercase alpha chars
	assert.True(t, IsValidTaskID("abcd"))
	assert.True(t, IsValidTaskID("zzzz"))
	assert.True(t, IsValidTaskID("aaaa"))

	// Too short
	assert.False(t, IsValidTaskID(""))
	assert.False(t, IsValidTaskID("abc"))
	assert.False(t, IsValidTaskID("a"))

	// Too long
	assert.False(t, IsValidTaskID("abcde"))
	assert.False(t, IsValidTaskID("abcdef"))

	// Contains digits
	assert.False(t, IsValidTaskID("abc1"))
	assert.False(t, IsValidTaskID("1234"))

	// Contains uppercase
	assert.False(t, IsValidTaskID("ABCD"))
	assert.False(t, IsValidTaskID("Abcd"))

	// Contains special characters
	assert.False(t, IsValidTaskID("ab-c"))
	assert.False(t, IsValidTaskID("ab c"))
	assert.False(t, IsValidTaskID("ab_c"))
}

func TestNormalizeTaskID(t *testing.T) {
	// Already lowercase
	assert.Equal(t, "abcd", NormalizeTaskID("abcd"))

	// Uppercase → lowercase
	assert.Equal(t, "abcd", NormalizeTaskID("ABCD"))
	assert.Equal(t, "abcd", NormalizeTaskID("AbCd"))

	// Mixed with non-alpha (normalize doesn't validate, just lowercases)
	assert.Equal(t, "ab1c", NormalizeTaskID("AB1C"))

	// Empty string
	assert.Equal(t, "", NormalizeTaskID(""))
}

func TestHasStructuredFields(t *testing.T) {
	// All three set → true
	task := &Task{Action: "do X", Verify: "check Y", Result: "report Z"}
	assert.True(t, task.HasStructuredFields())

	// Missing Action → false
	task = &Task{Action: "", Verify: "check Y", Result: "report Z"}
	assert.False(t, task.HasStructuredFields())

	// Missing Verify → false
	task = &Task{Action: "do X", Verify: "", Result: "report Z"}
	assert.False(t, task.HasStructuredFields())

	// Missing Result → false
	task = &Task{Action: "do X", Verify: "check Y", Result: ""}
	assert.False(t, task.HasStructuredFields())

	// All empty → false (legacy task)
	task = &Task{}
	assert.False(t, task.HasStructuredFields())
}
