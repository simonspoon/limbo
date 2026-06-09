package models

import (
	"strings"
	"time"
)

// Doc represents a project-scoped document (PRD, architecture overview, ADR,
// etc.) stored in the central store. The document body is NOT a field on Doc:
// it lives in a markdown sidecar at <root>/docs/<id>/body.md, mirroring how
// Task content lives in <root>/context/<id>/context.md. Only the metadata
// index (this struct) is persisted in <root>/docs/index.json.
type Doc struct {
	ID           string    `json:"id"`
	Slug         string    `json:"slug"`
	Title        string    `json:"title"`
	Type         string    `json:"type"`
	Status       string    `json:"status,omitempty"`
	LinkedTasks  []string  `json:"linkedTasks,omitempty"`
	Supersedes   string    `json:"supersedes,omitempty"`
	SupersededBy string    `json:"supersededBy,omitempty"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
}

// DocTypeADR is the one document type that carries a status lifecycle. All
// other type strings are free-form (generic doc bag) and carry no status.
const DocTypeADR = "adr"

// ADR lifecycle status values. The lifecycle is forward-only:
// proposed -> accepted -> superseded.
const (
	ADRStatusProposed   = "proposed"
	ADRStatusAccepted   = "accepted"
	ADRStatusSuperseded = "superseded"
)

// adrStatusOrder is the canonical forward order of ADR statuses.
var adrStatusOrder = []string{ADRStatusProposed, ADRStatusAccepted, ADRStatusSuperseded}

// IsADR reports whether the doc carries the ADR lifecycle.
func IsADR(d *Doc) bool {
	return d != nil && d.Type == DocTypeADR
}

// IsValidADRStatus reports whether s is one of the defined ADR statuses.
func IsValidADRStatus(s string) bool {
	for _, v := range adrStatusOrder {
		if v == s {
			return true
		}
	}
	return false
}

// IsValidADRTransition reports whether moving an ADR's status from `from` to
// `to` is allowed. Transitions are forward-only along
// proposed -> accepted -> superseded; same-status, backward, skip-ahead, and
// unknown-status moves are all rejected. (Skip-ahead such as
// proposed -> superseded via `doc status` is disallowed; supersede is the only
// path that sets superseded, and it does so via `doc supersede`.)
func IsValidADRTransition(from, to string) bool {
	fi := adrStatusIndex(from)
	ti := adrStatusIndex(to)
	if fi < 0 || ti < 0 {
		return false
	}
	return ti == fi+1
}

func adrStatusIndex(s string) int {
	for i, v := range adrStatusOrder {
		if v == s {
			return i
		}
	}
	return -1
}

// IsValidDocID checks if an ID is a valid 4-character lowercase alphabetic
// string. Doc IDs reuse the task ID scheme so command-layer validation mirrors
// task commands and no unvalidated value reaches a filesystem path.
func IsValidDocID(id string) bool {
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

// NormalizeDocID converts an ID to lowercase for case-insensitive input.
func NormalizeDocID(id string) string {
	return strings.ToLower(id)
}

// SlugifyTitle derives a kebab-case slug from a title: lowercase, runs of
// non-alphanumeric characters collapse to a single '-', leading/trailing '-'
// trimmed. An empty or all-punctuation title yields "doc".
func SlugifyTitle(title string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "doc"
	}
	return slug
}
