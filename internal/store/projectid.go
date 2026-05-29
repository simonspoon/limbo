package store

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// LimboIDFile is the override file that, when present at the project root,
// supplies the project ID directly (A11).
const LimboIDFile = ".limbo-id"

// ErrNoProjectID is the distinct signal returned by ResolveProjectID when no
// ID can be derived: there is no .limbo-id override (A11) and the project root
// is not inside a git working tree with at least one commit (A12). Callers
// (notably `limbo init`) branch on this to fall back to UUID generation (A13);
// non-init commands surface it as "run limbo init".
//
// Use errors.Is(err, ErrNoProjectID) to test for it.
var ErrNoProjectID = errors.New("no project id available")

// gitSHARe matches a 40-character lowercase hex git object name.
var gitSHARe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ResolveProjectID derives the project ID for the project rooted at root,
// following the fixed priority hierarchy:
//
//  1. .limbo-id override (A11): if root/.limbo-id exists, its first non-empty
//     trimmed line is the ID. Contents are opaque; the only validation is
//     non-emptiness.
//  2. git first-commit SHA (A12): otherwise, if root is inside a git working
//     tree with at least one commit, the ID is the 40-char lowercase hex SHA
//     of the first commit reachable from HEAD, equivalent to
//     `git rev-list --max-parents=0 HEAD | tail -1`.
//  3. no id available: otherwise ResolveProjectID returns ErrNoProjectID so the
//     caller can branch to UUID generation (A13). This function never
//     generates or writes an ID itself.
//
// The runGit hook lets tests inject a fake git without a real repository; pass
// nil to use the real `git` binary executed with root as its working
// directory.
func ResolveProjectID(root string, runGit func(args ...string) (string, error)) (string, error) {
	// (1) .limbo-id override.
	if id, ok, err := readLimboID(filepath.Join(root, LimboIDFile)); err != nil {
		return "", err
	} else if ok {
		return id, nil
	}

	// (2) git first-commit SHA.
	if runGit == nil {
		runGit = gitRunner(root)
	}
	out, err := runGit("rev-list", "--max-parents=0", "HEAD")
	if err == nil {
		if sha := firstCommitSHA(out); sha != "" {
			return sha, nil
		}
	}

	// (3) no id available — caller handles the UUID fallback.
	return "", ErrNoProjectID
}

// readLimboID reads the .limbo-id override at path. It returns the first
// non-empty trimmed line and ok=true when such a line exists. A missing file
// is reported as ok=false with a nil error; a file that exists but contains
// only blank lines is likewise ok=false (treated as absent so the next
// resolution tier applies).
func readLimboID(path string) (id string, ok bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line, true, nil
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return "", false, scanErr
	}
	return "", false, nil
}

// firstCommitSHA extracts the first-commit SHA from the output of
// `git rev-list --max-parents=0 HEAD`. A repository with a linear history has a
// single root commit; histories with merged-in unrelated roots may list
// several, in which case the last line — the one `tail -1` would select per
// A12 — is used. Only a well-formed 40-char lowercase hex line is accepted.
func firstCommitSHA(out string) string {
	var last string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		last = line
	}
	if gitSHARe.MatchString(last) {
		return last
	}
	return ""
}

// gitRunner returns a runGit function that executes the real `git` binary with
// dir as its working directory.
func gitRunner(dir string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
}
