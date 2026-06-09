package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoc_IsADR(t *testing.T) {
	assert.True(t, IsADR(&Doc{Type: DocTypeADR}))
	assert.False(t, IsADR(&Doc{Type: "prd"}))
	assert.False(t, IsADR(&Doc{Type: ""}))
	assert.False(t, IsADR(nil))
}

func TestDoc_IsValidADRTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{ADRStatusProposed, ADRStatusAccepted, true},
		{ADRStatusAccepted, ADRStatusSuperseded, true},
		// backward
		{ADRStatusAccepted, ADRStatusProposed, false},
		{ADRStatusSuperseded, ADRStatusProposed, false},
		{ADRStatusSuperseded, ADRStatusAccepted, false},
		// skip-ahead
		{ADRStatusProposed, ADRStatusSuperseded, false},
		// same-status
		{ADRStatusProposed, ADRStatusProposed, false},
		{ADRStatusAccepted, ADRStatusAccepted, false},
		// unknown strings
		{ADRStatusProposed, "bogus", false},
		{"bogus", ADRStatusAccepted, false},
		{"", ADRStatusProposed, false},
		{ADRStatusProposed, "", false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, IsValidADRTransition(c.from, c.to),
			"transition %q -> %q", c.from, c.to)
	}
}

func TestDoc_IsValidADRStatus(t *testing.T) {
	assert.True(t, IsValidADRStatus(ADRStatusProposed))
	assert.True(t, IsValidADRStatus(ADRStatusAccepted))
	assert.True(t, IsValidADRStatus(ADRStatusSuperseded))
	assert.False(t, IsValidADRStatus("bogus"))
	assert.False(t, IsValidADRStatus(""))
}

func TestDoc_IsValidDocID(t *testing.T) {
	assert.True(t, IsValidDocID("abcd"))
	assert.False(t, IsValidDocID("ABCD"))
	assert.False(t, IsValidDocID("ab"))
	assert.False(t, IsValidDocID("abcde"))
	assert.False(t, IsValidDocID("ab1d"))
	assert.False(t, IsValidDocID("../."))
	assert.False(t, IsValidDocID(""))
}

func TestDoc_SlugGeneration(t *testing.T) {
	cases := []struct {
		title, want string
	}{
		{"My PRD", "my-prd"},
		{"Use X for Y!", "use-x-for-y"},
		{"  Leading/Trailing  ", "leading-trailing"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"ADR 001: Choose Postgres", "adr-001-choose-postgres"},
		{"", "doc"},
		{"!!!", "doc"},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, SlugifyTitle(c.title), "slug for %q", c.title)
	}
}
