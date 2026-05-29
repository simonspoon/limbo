// Package jsonstore is the one concrete backend that satisfies the
// store.Store interface. It persists the task index as a JSON envelope
// (tasks.json) under a storage root, splits per-task structured content into
// markdown sidecars under <root>/context/<task-id>/context.md, serializes all
// access with an advisory gofrs/flock lock (exclusive for mutations, shared
// for reads), writes atomically via a temp-file + hardlink-backup + rename
// sequence, recovers from a corrupt tasks.json via the hardlinked .bak, and
// carries a monotonic revision counter that increments by exactly one on each
// successful mutation.
//
// The backend keeps no mutable in-memory state beyond the storage-root path:
// every operation reads current state from disk under the lock and writes it
// back under the same lock hold, so concurrent callers (even across processes)
// never lose updates.
package jsonstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/simonspoon/limbo/internal/store"
)

// SchemaVersion is the on-disk envelope version this backend reads and writes.
const SchemaVersion = "7.0.0"

// On-disk file and directory names, all resolved relative to the storage root.
const (
	tasksFileName   = "tasks.json"
	tmpFileName     = "tasks.json.tmp"
	bakFileName     = "tasks.json.bak"
	lockFileName    = "store.lock"
	contextDirName  = "context"
	contextFileName = "context.md"
)

// Syscall seams. Tests override these to fault-inject failures (e.g. a failing
// rename) without touching real disk semantics. Keep them package-level vars,
// not method fields, so the Store stays stateless.
var (
	osRename   = os.Rename
	osLink     = os.Link
	osWriteTmp = writeAndFsync
)

// envelope is the top-level tasks.json structure: a schema version, a
// monotonic revision counter, and the task index (with structured content
// fields stripped out into sidecars).
type envelope struct {
	Version  string        `json:"version"`
	Revision int           `json:"revision"`
	Tasks    []models.Task `json:"tasks"`
}

// Store is the JSON-backed implementation of store.Store. Its only field is
// the storage root; all state lives on disk.
type Store struct {
	root string
}

// compile-time assertion that *Store satisfies the seam.
var _ store.Store = (*Store)(nil)

// New constructs a JSON-backed Store rooted at the given storage path. The
// path need not exist yet; a fresh/missing store behaves as an empty store at
// revision 0.
func New(root string) *Store {
	return &Store{root: root}
}

func (s *Store) tasksPath() string { return filepath.Join(s.root, tasksFileName) }
func (s *Store) tmpPath() string   { return filepath.Join(s.root, tmpFileName) }
func (s *Store) bakPath() string   { return filepath.Join(s.root, bakFileName) }
func (s *Store) lockPath() string  { return filepath.Join(s.root, lockFileName) }

func (s *Store) contextDir(id string) string {
	return filepath.Join(s.root, contextDirName, id)
}

func (s *Store) contextFilePath(id string) string {
	return filepath.Join(s.contextDir(id), contextFileName)
}

// ensureRoot makes sure the storage root exists before we try to create the
// lock file inside it.
func (s *Store) ensureRoot() error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("create storage root: %w", err)
	}
	return nil
}

// withExclusiveLock runs fn while holding an advisory exclusive flock on
// store.lock. A fresh *flock.Flock (and therefore a fresh open file
// description) is created per call so that concurrent goroutines serialize via
// flock(2) per-fd semantics rather than sharing one descriptor. The lock is
// released on both success and error.
func (s *Store) withExclusiveLock(fn func() error) error {
	if err := s.ensureRoot(); err != nil {
		return err
	}
	lock := flock.New(s.lockPath())
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("acquire exclusive store.lock: %w", err)
	}
	defer func() {
		_ = lock.Unlock()
		_ = lock.Close()
	}()
	return fn()
}

// withSharedLock runs fn while holding an advisory shared flock on store.lock.
// Read-only operations use this so multiple readers can proceed concurrently
// while still excluding writers. The lock is released on success or error.
func (s *Store) withSharedLock(fn func() error) error {
	if err := s.ensureRoot(); err != nil {
		return err
	}
	lock := flock.New(s.lockPath())
	if err := lock.RLock(); err != nil {
		return fmt.Errorf("acquire shared store.lock: %w", err)
	}
	defer func() {
		_ = lock.Unlock()
		_ = lock.Close()
	}()
	return fn()
}

// readEnvelope reads and parses tasks.json. A missing file is an empty store
// at revision 0 (not an error). A parse failure falls back to the hardlinked
// tasks.json.bak: if .bak parses, it succeeds and warns to stderr naming the
// fallback path; if both fail, it returns a structured error. It never
// truncates or overwrites on a parse failure.
//
// Callers must already hold the appropriate lock.
func (s *Store) readEnvelope() (*envelope, error) {
	data, err := os.ReadFile(s.tasksPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &envelope{Version: SchemaVersion, Revision: 0, Tasks: []models.Task{}}, nil
		}
		return nil, fmt.Errorf("read tasks.json: %w", err)
	}

	var env envelope
	if perr := json.Unmarshal(data, &env); perr == nil {
		if env.Tasks == nil {
			env.Tasks = []models.Task{}
		}
		return &env, nil
	} else {
		// tasks.json is corrupt. Attempt recovery from the hardlinked backup.
		bakData, berr := os.ReadFile(s.bakPath())
		if berr != nil {
			return nil, fmt.Errorf("tasks.json failed to parse (%v) and backup %s is unavailable: %w", perr, s.bakPath(), berr)
		}
		var bakEnv envelope
		if jerr := json.Unmarshal(bakData, &bakEnv); jerr != nil {
			return nil, fmt.Errorf("tasks.json failed to parse (%v) and backup %s also failed to parse: %w", perr, s.bakPath(), jerr)
		}
		if bakEnv.Tasks == nil {
			bakEnv.Tasks = []models.Task{}
		}
		fmt.Fprintf(os.Stderr, "limbo: tasks.json failed to parse; recovered from backup %s\n", s.bakPath())
		return &bakEnv, nil
	}
}

// writeEnvelope persists env atomically. Callers must hold the exclusive lock.
// The sequence (A4) is: write+fsync the tmp file; remove any stale .bak;
// hardlink the current tasks.json to .bak (skipped on first-ever write);
// rename tmp over tasks.json; fsync the root directory. A crash at any step
// leaves tasks.json fully old or fully new.
func (s *Store) writeEnvelope(env *envelope) error {
	if env.Version == "" {
		env.Version = SchemaVersion
	}
	if env.Tasks == nil {
		env.Tasks = []models.Task{}
	}

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks envelope: %w", err)
	}
	data = append(data, '\n')

	// (a) write + fsync the temp file.
	if err := osWriteTmp(s.tmpPath(), data); err != nil {
		return fmt.Errorf("write tmp file: %w", err)
	}

	// (b) remove any stale backup, ignoring not-exist.
	if err := os.Remove(s.bakPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale backup: %w", err)
	}

	// (c) hardlink the current tasks.json to .bak — only if it already exists.
	if _, statErr := os.Stat(s.tasksPath()); statErr == nil {
		if err := osLink(s.tasksPath(), s.bakPath()); err != nil {
			return fmt.Errorf("hardlink backup: %w", err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat tasks.json: %w", statErr)
	}

	// (d) atomically move the new content into place.
	if err := osRename(s.tmpPath(), s.tasksPath()); err != nil {
		return fmt.Errorf("rename tmp over tasks.json: %w", err)
	}

	// (e) fsync the root directory so the rename is durable.
	if err := fsyncDir(s.root); err != nil {
		return fmt.Errorf("fsync root dir: %w", err)
	}

	return nil
}

// writeAndFsync writes data to path and fsyncs the file before returning.
func writeAndFsync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// fsyncDir opens dir and fsyncs it so that a rename within it is durable.
func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	// Directory fsync is best-effort on some platforms; ignore EINVAL-style
	// failures that simply mean the platform does not support it.
	if err := d.Sync(); err != nil {
		// Not fatal on platforms where directory sync is unsupported.
		return nil
	}
	return nil
}
