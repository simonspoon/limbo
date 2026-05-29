package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// mkdirAll is a test helper that creates dir or fails the test.
func mkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
}

// touch creates an empty file or fails the test.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestFindProjectRoot_AnchorAtStart(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, gitDirName))

	got, err := FindProjectRoot(root, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("got %q, want %q", got, root)
	}
}

func TestFindProjectRoot_ClimbsToGit(t *testing.T) {
	// .git two levels up; start deep and climb to it (A14).
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, gitDirName))
	deep := filepath.Join(root, "a", "b", "c")
	mkdirAll(t, deep)

	got, err := FindProjectRoot(deep, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("got %q, want %q", got, root)
	}
}

func TestFindProjectRoot_ClimbsToLimboID(t *testing.T) {
	// .limbo-id is also a valid anchor (A14).
	root := t.TempDir()
	touch(t, filepath.Join(root, LimboIDFile))
	deep := filepath.Join(root, "x", "y")
	mkdirAll(t, deep)

	got, err := FindProjectRoot(deep, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("got %q, want %q", got, root)
	}
}

func TestFindProjectRoot_StopsAtFirstMatch(t *testing.T) {
	// An inner anchor must win over an outer one: the climb stops at the
	// first match encountered while ascending.
	outer := t.TempDir()
	mkdirAll(t, filepath.Join(outer, gitDirName))
	inner := filepath.Join(outer, "inner")
	mkdirAll(t, inner)
	touch(t, filepath.Join(inner, LimboIDFile))
	deep := filepath.Join(inner, "deeper")
	mkdirAll(t, deep)

	got, err := FindProjectRoot(deep, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Clean(inner) {
		t.Fatalf("got %q, want first match %q", got, inner)
	}
}

func TestFindProjectRoot_NoClimbWithAnchor(t *testing.T) {
	// no-climb returns startDir when it holds an anchor.
	root := t.TempDir()
	touch(t, filepath.Join(root, LimboIDFile))

	got, err := FindProjectRoot(root, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != filepath.Clean(root) {
		t.Fatalf("got %q, want %q", got, root)
	}
}

func TestFindProjectRoot_NoClimbWithoutAnchor(t *testing.T) {
	// no-climb must NOT ascend: even with a parent anchor it errors when
	// startDir has none.
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, gitDirName))
	child := filepath.Join(root, "child")
	mkdirAll(t, child)

	_, err := FindProjectRoot(child, true)
	var nf *ProjectRootNotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v, want *ProjectRootNotFoundError", err)
	}
	if !nf.NoClimb {
		t.Fatalf("expected NoClimb=true on error, got %+v", nf)
	}
	if nf.StartDir != child {
		t.Fatalf("StartDir = %q, want %q", nf.StartDir, child)
	}
}

func TestFindProjectRoot_NotFoundReachesRoot(t *testing.T) {
	// Climbing with no anchor anywhere up to the filesystem root returns a
	// structured error (A14).
	root := t.TempDir()
	deep := filepath.Join(root, "no", "anchor", "here")
	mkdirAll(t, deep)

	_, err := FindProjectRoot(deep, false)
	var nf *ProjectRootNotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %v, want *ProjectRootNotFoundError", err)
	}
	if nf.NoClimb {
		t.Fatalf("expected NoClimb=false on climbing error, got %+v", nf)
	}
	if nf.StartDir != deep {
		t.Fatalf("StartDir = %q, want %q", nf.StartDir, deep)
	}
	if nf.Error() == "" {
		t.Fatal("structured error must produce a non-empty message")
	}
}
