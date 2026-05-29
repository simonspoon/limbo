package commands

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/simonspoon/limbo/internal/store"
)

// noClimbEnv is the environment variable that, when truthy, confines project
// root resolution to the current working directory (A14).
const noClimbEnv = "LIMBO_NO_CLIMB"

// wantNoClimb reports whether parent-directory climbing is disabled, either via
// the --no-climb persistent flag or a truthy LIMBO_NO_CLIMB.
func wantNoClimb() bool {
	if noClimbFlag {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv(noClimbEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// resolveProjectRoot finds the project root by climbing from cwd for an anchor
// (.git or .limbo-id), honoring no-climb. A *store.ProjectRootNotFoundError is
// surfaced as a "run 'limbo init'" error.
func resolveProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	root, err := store.FindProjectRoot(cwd, wantNoClimb())
	if err != nil {
		var nf *store.ProjectRootNotFoundError
		if errors.As(err, &nf) {
			return "", fmt.Errorf("not in a limbo project: %s", nf.Error())
		}
		return "", err
	}
	return root, nil
}

// resolveStorageRoot resolves the central storage root for a non-init command:
// it finds the project root, derives the project ID (erroring with a
// run-init message when none is available, A13), and computes the
// platform-conventional storage path (LIMBO_HOME honored, A8-A10).
func resolveStorageRoot() (string, error) {
	_, central, err := resolveRoots()
	return central, err
}

// resolveRoots resolves BOTH the project root and the central storage root for
// a non-init command. The project root is needed by callers (getStorage,
// migrate) that must inspect the legacy in-tree .limbo/ directory alongside the
// central store. It finds the project root, derives the project ID (erroring
// with a run-init message when none is available, A13), and computes the
// platform-conventional central storage path (LIMBO_HOME honored, A8-A10).
func resolveRoots() (projectRoot, centralRoot string, err error) {
	root, err := resolveProjectRoot()
	if err != nil {
		return "", "", err
	}

	id, err := store.ResolveProjectID(root, nil)
	if err != nil {
		if errors.Is(err, store.ErrNoProjectID) {
			return "", "", fmt.Errorf("no limbo project id for %s. Run 'limbo init' first", root)
		}
		return "", "", err
	}

	central, err := centralStorageRoot(id)
	if err != nil {
		return "", "", err
	}
	return root, central, nil
}

// centralStorageRoot computes the platform-conventional storage root for the
// given project ID, honoring LIMBO_HOME.
func centralStorageRoot(id string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		// On platforms / environments where the home dir is unavailable, a
		// LIMBO_HOME override still works (StorageRoot consults env first).
		home = ""
	}
	return store.StorageRoot(home, runtime.GOOS, os.Getenv, id), nil
}

// generateUUIDv4 returns a random RFC-4122 version-4 UUID string. limbo has no
// uuid dependency, so this is built directly from crypto/rand: 16 random bytes
// with the version (0x40) and variant (0x80) bits forced.
func generateUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
