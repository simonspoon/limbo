package store

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const fakeSHA = "1234567890abcdef1234567890abcdef12345678"

// fakeGit returns a runGit stub that responds to rev-list with the given
// output and error.
func fakeGit(out string, err error) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		return out, err
	}
}

func TestResolveProjectID_LimboIDOverride(t *testing.T) {
	// A11: .limbo-id first non-empty trimmed line wins over git.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, LimboIDFile), []byte("\n  \n  my-project-id  \nignored\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// git would return a SHA, but the override must take precedence.
	id, err := ResolveProjectID(root, fakeGit(fakeSHA+"\n", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "my-project-id" {
		t.Fatalf("id = %q, want %q (first non-empty trimmed line)", id, "my-project-id")
	}
}

func TestResolveProjectID_LimboIDBlankFileFallsThrough(t *testing.T) {
	// A .limbo-id with only blank lines is treated as absent, so git applies.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, LimboIDFile), []byte("\n  \n\t\n"), 0644); err != nil {
		t.Fatal(err)
	}

	id, err := ResolveProjectID(root, fakeGit(fakeSHA+"\n", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != fakeSHA {
		t.Fatalf("id = %q, want git SHA %q", id, fakeSHA)
	}
}

func TestResolveProjectID_GitFirstCommit(t *testing.T) {
	// A12: 40-char lowercase hex SHA from rev-list --max-parents=0 HEAD.
	root := t.TempDir()
	id, err := ResolveProjectID(root, fakeGit(fakeSHA+"\n", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != fakeSHA {
		t.Fatalf("id = %q, want %q", id, fakeSHA)
	}
}

func TestResolveProjectID_GitMultipleRootsTakesLast(t *testing.T) {
	// Equivalent of `... | tail -1`: with multiple root commits the last line
	// is selected.
	root := t.TempDir()
	other := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	out := other + "\n" + fakeSHA + "\n"
	id, err := ResolveProjectID(root, fakeGit(out, nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != fakeSHA {
		t.Fatalf("id = %q, want last line %q", id, fakeSHA)
	}
}

func TestResolveProjectID_NoGitNoID(t *testing.T) {
	// A13 precursor: no .limbo-id and git fails (not a repo / no commits) ->
	// distinct ErrNoProjectID signal for the UUID-fallback case.
	root := t.TempDir()
	_, err := ResolveProjectID(root, fakeGit("", errors.New("fatal: not a git repository")))
	if !errors.Is(err, ErrNoProjectID) {
		t.Fatalf("err = %v, want ErrNoProjectID", err)
	}
}

func TestResolveProjectID_GitEmptyOutputIsNoID(t *testing.T) {
	// A repo with no commits returns empty rev-list output -> no id.
	root := t.TempDir()
	_, err := ResolveProjectID(root, fakeGit("\n", nil))
	if !errors.Is(err, ErrNoProjectID) {
		t.Fatalf("err = %v, want ErrNoProjectID", err)
	}
}

func TestResolveProjectID_GitMalformedOutputIsNoID(t *testing.T) {
	// Defensive: non-SHA git output must not be accepted as an ID.
	root := t.TempDir()
	_, err := ResolveProjectID(root, fakeGit("not-a-sha\n", nil))
	if !errors.Is(err, ErrNoProjectID) {
		t.Fatalf("err = %v, want ErrNoProjectID for malformed git output", err)
	}
}

func TestResolveProjectID_NilRunnerUsesRealGit(t *testing.T) {
	// With a nil runner and a non-git tempdir, the real git binary fails and
	// the resolver returns ErrNoProjectID. This exercises the gitRunner path.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	root := t.TempDir()
	_, err := ResolveProjectID(root, nil)
	if !errors.Is(err, ErrNoProjectID) {
		t.Fatalf("err = %v, want ErrNoProjectID in non-git tempdir", err)
	}
}
