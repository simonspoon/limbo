package store

import (
	"path/filepath"
)

// Environment variable names that influence storage-root resolution.
const (
	// LimboHomeEnv, when set and non-empty, overrides the platform-default
	// central storage location. The storage root becomes
	// ${LIMBO_HOME}/projects/<id>/.
	LimboHomeEnv = "LIMBO_HOME"

	// XDGDataHomeEnv is the XDG base-directory variable consulted on linux. It
	// defaults to ${HOME}/.local/share when unset or empty.
	XDGDataHomeEnv = "XDG_DATA_HOME"
)

// projectsDir is the fixed subdirectory under the limbo home that holds one
// directory per project ID.
const projectsDir = "projects"

// limboDirName is the application directory created under the platform's
// data root.
const limboDirName = "limbo"

// Platform-conventional data-root suffixes, expressed with forward slashes so
// they read as the documented conventional paths regardless of host
// separator. filepath.FromSlash normalizes them for the running OS at join
// time.
const (
	// darwinDataSuffix is appended to ${HOME} on darwin:
	// ${HOME}/Library/Application Support (A8).
	darwinDataSuffix = "Library/Application Support"

	// linuxDataSuffix is the XDG_DATA_HOME default appended to ${HOME} on
	// linux when XDG_DATA_HOME is unset or empty: ${HOME}/.local/share (A9).
	linuxDataSuffix = ".local/share"
)

// StorageRoot resolves the central, platform-conventional storage root for the
// project identified by id. It is a pure function: all inputs are passed
// explicitly so the resolver is table-testable without touching the real
// process environment.
//
// Resolution order:
//
//   - If env[LIMBO_HOME] is set and non-empty, the root is
//     ${LIMBO_HOME}/projects/<id>/, overriding the platform default (A10).
//   - On goos == "darwin", the root is
//     ${HOME}/Library/Application Support/limbo/projects/<id>/ (A8).
//   - On any other goos (treated as linux/unix per A9), the root is
//     ${XDG_DATA_HOME}/limbo/projects/<id>/, defaulting XDG_DATA_HOME to
//     ${HOME}/.local/share when unset or empty (A9).
//
// home is the user's home directory (e.g. from os.UserHomeDir). goos is the
// target OS string (e.g. runtime.GOOS). env is a lookup function over the
// process environment (e.g. os.Getenv); a nil env is treated as "everything
// unset". The returned path is cleaned but not guaranteed to exist; creating
// it is the backend's responsibility.
func StorageRoot(home, goos string, env func(string) string, id string) string {
	if env == nil {
		env = func(string) string { return "" }
	}

	if limboHome := env(LimboHomeEnv); limboHome != "" {
		return filepath.Join(limboHome, projectsDir, id)
	}

	var dataRoot string
	switch goos {
	case "darwin":
		dataRoot = filepath.Join(home, filepath.FromSlash(darwinDataSuffix))
	default:
		// linux and other unix-likes follow the XDG base-directory spec.
		if xdg := env(XDGDataHomeEnv); xdg != "" {
			dataRoot = xdg
		} else {
			dataRoot = filepath.Join(home, filepath.FromSlash(linuxDataSuffix))
		}
	}

	return filepath.Join(dataRoot, limboDirName, projectsDir, id)
}
