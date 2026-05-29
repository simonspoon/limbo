package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// gitDirName is the directory whose presence anchors a project root during the
// climb.
const gitDirName = ".git"

// ProjectRootNotFoundError is the structured error returned by FindProjectRoot
// when neither a .git directory nor a .limbo-id file is found at or above the
// start directory before the filesystem root (A14). It carries the directory
// the search started from so callers can produce a precise message and direct
// the user to `limbo init`.
type ProjectRootNotFoundError struct {
	// StartDir is the directory the (failed) search began at.
	StartDir string
	// NoClimb records whether the search was confined to StartDir.
	NoClimb bool
}

func (e *ProjectRootNotFoundError) Error() string {
	if e.NoClimb {
		return fmt.Sprintf("no limbo project anchor (.git or %s) in %s (climb disabled). Run 'limbo init' first", LimboIDFile, e.StartDir)
	}
	return fmt.Sprintf("no limbo project anchor (.git or %s) at or above %s. Run 'limbo init' first", LimboIDFile, e.StartDir)
}

// FindProjectRoot walks up from startDir looking for either a .git directory or
// a .limbo-id file, stopping at and returning the first directory that contains
// either (A14).
//
// When noClimb is true the search is confined to startDir: if startDir itself
// holds an anchor it is returned, otherwise a *ProjectRootNotFoundError is
// returned without ascending. When noClimb is false the search ascends one
// parent at a time and returns a *ProjectRootNotFoundError once it reaches the
// filesystem root without a match.
//
// A returned *ProjectRootNotFoundError can be tested with errors.As.
func FindProjectRoot(startDir string, noClimb bool) (string, error) {
	dir := filepath.Clean(startDir)

	for {
		if hasAnchor(dir) {
			return dir, nil
		}

		if noClimb {
			return "", &ProjectRootNotFoundError{StartDir: startDir, NoClimb: true}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without a match.
			return "", &ProjectRootNotFoundError{StartDir: startDir, NoClimb: false}
		}
		dir = parent
	}
}

// hasAnchor reports whether dir contains a .git directory or a .limbo-id file.
// A .git entry of any kind (directory in a normal clone, file in a git
// worktree or submodule) counts as an anchor; a .limbo-id must be a regular
// file presence to count.
func hasAnchor(dir string) bool {
	if _, err := os.Lstat(filepath.Join(dir, gitDirName)); err == nil {
		return true
	}
	if _, err := os.Lstat(filepath.Join(dir, LimboIDFile)); err == nil {
		return true
	}
	return false
}
