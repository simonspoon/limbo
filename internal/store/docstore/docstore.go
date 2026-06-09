// Package docstore is a parallel backend for project-scoped documents (PRD,
// architecture overview, ADRs, ...). It is rooted at the same central <root> as
// the task store but operates entirely under <root>/docs/: a metadata index at
// <root>/docs/index.json plus per-doc markdown sidecars at
// <root>/docs/<id>/body.md.
//
// It deliberately does NOT extend the store.Store seam — the docs/ tree is a
// separate on-disk namespace with its own identity, lifecycle, and link graph,
// so bolting doc methods onto store.Store would widen that seam needlessly. It
// shares the SAME <root>/store.lock as jsonstore (exclusive for mutations,
// shared for reads) so doc writes and task writes never interleave a partial
// index, and it ports jsonstore's atomic write discipline (tmp + fsync ->
// rename -> dir fsync) faithfully so index.json at rest is always complete,
// valid JSON.
//
// This package lives under internal/store, so its direct file IO is exempt from
// the "no direct store IO outside internal/store" constraint that applies to
// the command layer.
package docstore

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	"github.com/simonspoon/limbo/internal/models"
)

// SchemaVersion is the on-disk envelope version this backend reads and writes.
const SchemaVersion = "1.0.0"

// On-disk names. The lock file is the SAME store.lock the task store uses (at
// the central root); the index, tmp, and bak files all live under <root>/docs/
// with doc-specific names so they never collide with tasks.json.* at the root.
const (
	docsDirName  = "docs"
	indexName    = "index.json"
	tmpName      = "index.json.tmp"
	bakName      = "index.json.bak"
	lockFileName = "store.lock"
	bodyName     = "body.md"
)

// ErrDocNotFound is returned by Get/Update/UpdatePair/Delete when the target
// doc id is absent from the index. The command layer surfaces it (and the
// ADR-lifecycle rejections it constructs) as JSON errors on stderr.
var ErrDocNotFound = errors.New("doc not found")

// envelope is the top-level docs/index.json structure.
type envelope struct {
	Version string       `json:"version"`
	Docs    []models.Doc `json:"docs"`
}

// DocStore persists the doc index and bodies under <root>/docs/. Its only field
// is the central storage root; all state lives on disk.
type DocStore struct {
	root string
}

// New constructs a DocStore rooted at the given central storage path (the same
// <root> the task store uses). The path need not exist yet; a fresh/missing
// store behaves as an empty store.
func New(root string) *DocStore {
	return &DocStore{root: root}
}

func (s *DocStore) docsDir() string   { return filepath.Join(s.root, docsDirName) }
func (s *DocStore) indexPath() string { return filepath.Join(s.docsDir(), indexName) }
func (s *DocStore) tmpPath() string   { return filepath.Join(s.docsDir(), tmpName) }
func (s *DocStore) bakPath() string   { return filepath.Join(s.docsDir(), bakName) }
func (s *DocStore) lockPath() string  { return filepath.Join(s.root, lockFileName) }

func (s *DocStore) docDir(id string) string   { return filepath.Join(s.docsDir(), id) }
func (s *DocStore) bodyPath(id string) string { return filepath.Join(s.docDir(id), bodyName) }

// BodyPath exposes the on-disk path to a doc's body sidecar so callers (and
// tests) can assert on its presence. It does no validation; the command layer
// validates the id before any path construction.
func (s *DocStore) BodyPath(id string) string { return s.bodyPath(id) }

// IndexPath exposes the on-disk path to the doc index.
func (s *DocStore) IndexPath() string { return s.indexPath() }

// DocDir exposes the on-disk directory for a doc's sidecar subtree.
func (s *DocStore) DocDir(id string) string { return s.docDir(id) }

// ensureDocsDir makes sure both the central root and the docs/ subdir exist
// before we create the lock file or write the index.
func (s *DocStore) ensureDocsDir() error {
	if err := os.MkdirAll(s.docsDir(), 0o755); err != nil {
		return fmt.Errorf("create docs dir: %w", err)
	}
	return nil
}

// withExclusiveLock runs fn while holding an advisory exclusive flock on the
// SHARED store.lock. A fresh *flock.Flock (fresh open file description) is
// created per call so concurrent goroutines serialize via flock(2) per-fd
// semantics. The lock is released on success and error.
//
// IMPORTANT (R2): flock(2) is per-fd and non-reentrant. Never invoke a
// doc-store mutation from inside another store's lock callback, or a fresh-fd
// LOCK_EX against an already-held lock self-deadlocks.
func (s *DocStore) withExclusiveLock(fn func() error) error {
	if err := s.ensureDocsDir(); err != nil {
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

// withSharedLock runs fn while holding an advisory shared flock on the shared
// store.lock. The lock is released on success and error.
func (s *DocStore) withSharedLock(fn func() error) error {
	if err := s.ensureDocsDir(); err != nil {
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

// readEnvelope reads and parses docs/index.json. A missing file is an empty
// store (not an error). A parse failure falls back to the index.json.bak; if
// both fail, it returns a structured error. Callers must already hold the
// appropriate lock.
func (s *DocStore) readEnvelope() (*envelope, error) {
	data, err := os.ReadFile(s.indexPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &envelope{Version: SchemaVersion, Docs: []models.Doc{}}, nil
		}
		return nil, fmt.Errorf("read docs index: %w", err)
	}
	var env envelope
	if perr := json.Unmarshal(data, &env); perr == nil {
		if env.Docs == nil {
			env.Docs = []models.Doc{}
		}
		return &env, nil
	} else {
		bakData, berr := os.ReadFile(s.bakPath())
		if berr != nil {
			return nil, fmt.Errorf("docs index failed to parse (%v) and backup %s is unavailable: %w", perr, s.bakPath(), berr)
		}
		var bakEnv envelope
		if jerr := json.Unmarshal(bakData, &bakEnv); jerr != nil {
			return nil, fmt.Errorf("docs index failed to parse (%v) and backup %s also failed to parse: %w", perr, s.bakPath(), jerr)
		}
		if bakEnv.Docs == nil {
			bakEnv.Docs = []models.Doc{}
		}
		fmt.Fprintf(os.Stderr, "limbo: docs index failed to parse; recovered from backup %s\n", s.bakPath())
		return &bakEnv, nil
	}
}

// writeEnvelope persists env atomically. Callers must hold the exclusive lock.
// The sequence faithfully mirrors jsonstore.writeEnvelope: write+fsync the tmp
// file; remove stale .bak; hardlink the current index to .bak (skipped on the
// first-ever write); rename tmp over index.json; fsync the docs dir. A crash at
// any step leaves index.json fully old or fully new.
func (s *DocStore) writeEnvelope(env *envelope) error {
	if env.Version == "" {
		env.Version = SchemaVersion
	}
	if env.Docs == nil {
		env.Docs = []models.Doc{}
	}

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal docs envelope: %w", err)
	}
	data = append(data, '\n')

	// (a) write + fsync the temp file.
	if err := writeAndFsync(s.tmpPath(), data); err != nil {
		return fmt.Errorf("write tmp file: %w", err)
	}

	// (b) remove any stale backup, ignoring not-exist.
	if err := os.Remove(s.bakPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale backup: %w", err)
	}

	// (c) hardlink the current index to .bak — only if it already exists.
	if _, statErr := os.Stat(s.indexPath()); statErr == nil {
		if err := os.Link(s.indexPath(), s.bakPath()); err != nil {
			return fmt.Errorf("hardlink backup: %w", err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat docs index: %w", statErr)
	}

	// (d) atomically move the new content into place.
	if err := os.Rename(s.tmpPath(), s.indexPath()); err != nil {
		return fmt.Errorf("rename tmp over docs index: %w", err)
	}

	// (e) fsync the docs dir so the rename is durable.
	if err := fsyncDir(s.docsDir()); err != nil {
		return fmt.Errorf("fsync docs dir: %w", err)
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

// fsyncDir opens dir and fsyncs it so a rename within it is durable. Directory
// fsync is best-effort on some platforms.
func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return nil
	}
	return nil
}

// ---- public API ----

// List returns all docs from the index under a shared lock. A fresh/missing
// store returns an empty slice and a nil error.
func (s *DocStore) List() ([]models.Doc, error) {
	var docs []models.Doc
	err := s.withSharedLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		out := make([]models.Doc, len(env.Docs))
		copy(out, env.Docs)
		docs = out
		return nil
	})
	if err != nil {
		return nil, err
	}
	if docs == nil {
		docs = []models.Doc{}
	}
	return docs, nil
}

// Get returns a single doc by ID, or ErrDocNotFound. Read-only (shared lock).
func (s *DocStore) Get(id string) (models.Doc, error) {
	var found *models.Doc
	err := s.withSharedLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		for i := range env.Docs {
			if env.Docs[i].ID == id {
				d := env.Docs[i]
				found = &d
				return nil
			}
		}
		return fmt.Errorf("%w: %s", ErrDocNotFound, id)
	})
	if err != nil {
		return models.Doc{}, err
	}
	return *found, nil
}

// ReadBody returns the raw markdown body sidecar for a doc, or an empty string
// (nil error) when no sidecar exists. Read-only (shared lock).
func (s *DocStore) ReadBody(id string) (string, error) {
	var body string
	err := s.withSharedLock(func() error {
		data, err := os.ReadFile(s.bodyPath(id))
		if err != nil {
			if os.IsNotExist(err) {
				body = ""
				return nil
			}
			return fmt.Errorf("read doc body: %w", err)
		}
		body = string(data)
		return nil
	})
	return body, err
}

// Add appends a new doc, generating a collision-free id and a slug derived from
// the title (disambiguated against the current index), defaulting an ADR's
// status to proposed, and writing the body sidecar. The whole read-modify-write
// (id/slug generation included) happens within ONE exclusive lock hold so
// concurrent adds never mint a duplicate id or slug. Returns the stored doc.
func (s *DocStore) Add(doc models.Doc, body string) (models.Doc, error) {
	var stored models.Doc
	err := s.withExclusiveLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}

		id, err := generateDocID(env.Docs)
		if err != nil {
			return err
		}
		doc.ID = id
		doc.Slug = uniqueSlug(models.SlugifyTitle(doc.Title), env.Docs)

		now := time.Now()
		if doc.Created.IsZero() {
			doc.Created = now
		}
		doc.Updated = now

		if models.IsADR(&doc) && doc.Status == "" {
			doc.Status = models.ADRStatusProposed
		}

		// Write the body sidecar first, then commit the index.
		if err := s.writeBodyLocked(id, body); err != nil {
			return err
		}

		env.Docs = append(env.Docs, doc)
		if err := s.writeEnvelope(env); err != nil {
			return err
		}
		stored = doc
		return nil
	})
	if err != nil {
		return models.Doc{}, err
	}
	return stored, nil
}

// Update replaces an existing doc (matched by ID) in the index. Errors with
// ErrDocNotFound if absent. Read-modify-write under one exclusive lock hold.
// The body sidecar is not touched.
func (s *DocStore) Update(doc models.Doc) error {
	return s.withExclusiveLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		doc.Updated = time.Now()
		found := false
		for i := range env.Docs {
			if env.Docs[i].ID == doc.ID {
				env.Docs[i] = doc
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: %s", ErrDocNotFound, doc.ID)
		}
		return s.writeEnvelope(env)
	})
}

// UpdatePair replaces two existing docs in the index within a SINGLE index
// persist, so a bidirectional relationship (e.g. supersede) is staged
// atomically and can never be left half-formed. Both ids must exist.
func (s *DocStore) UpdatePair(a, b models.Doc) error {
	return s.withExclusiveLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		now := time.Now()
		a.Updated = now
		b.Updated = now
		foundA, foundB := false, false
		for i := range env.Docs {
			switch env.Docs[i].ID {
			case a.ID:
				env.Docs[i] = a
				foundA = true
			case b.ID:
				env.Docs[i] = b
				foundB = true
			}
		}
		if !foundA {
			return fmt.Errorf("%w: %s", ErrDocNotFound, a.ID)
		}
		if !foundB {
			return fmt.Errorf("%w: %s", ErrDocNotFound, b.ID)
		}
		return s.writeEnvelope(env)
	})
}

// WriteBody overwrites the raw markdown body sidecar for a doc. Mutation
// (exclusive lock).
func (s *DocStore) WriteBody(id, body string) error {
	return s.withExclusiveLock(func() error {
		return s.writeBodyLocked(id, body)
	})
}

// writeBodyLocked writes the body sidecar. The caller must hold the exclusive
// lock.
func (s *DocStore) writeBodyLocked(id, body string) error {
	dir := s.docDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create doc dir: %w", err)
	}
	if err := os.WriteFile(s.bodyPath(id), []byte(body), 0o644); err != nil {
		return fmt.Errorf("write doc body: %w", err)
	}
	return nil
}

// Delete removes a doc's index entry and its sidecar subtree. Errors with
// ErrDocNotFound if absent. Read-modify-write under one exclusive lock hold.
func (s *DocStore) Delete(id string) error {
	return s.withExclusiveLock(func() error {
		env, err := s.readEnvelope()
		if err != nil {
			return err
		}
		out := make([]models.Doc, 0, len(env.Docs))
		found := false
		for i := range env.Docs {
			if env.Docs[i].ID == id {
				found = true
				continue
			}
			out = append(out, env.Docs[i])
		}
		if !found {
			return fmt.Errorf("%w: %s", ErrDocNotFound, id)
		}
		env.Docs = out
		if err := s.writeEnvelope(env); err != nil {
			return err
		}
		// Best-effort removal of the orphaned sidecar subtree.
		_ = os.RemoveAll(s.docDir(id))
		return nil
	})
}

// ---- id / slug generation (called under the exclusive lock) ----

// generateDocID returns a fresh 4-character lowercase alphabetic ID that
// collides with no doc in the given index.
func generateDocID(docs []models.Doc) (string, error) {
	existing := make(map[string]bool, len(docs))
	for i := range docs {
		existing[docs[i].ID] = true
	}
	for attempts := 0; attempts < 100; attempts++ {
		id := generateRandomAlphaID()
		if !existing[id] {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique doc ID after 100 attempts")
}

// uniqueSlug returns base, or base-2/base-3/... if base already exists in the
// index.
func uniqueSlug(base string, docs []models.Doc) string {
	existing := make(map[string]bool, len(docs))
	for i := range docs {
		existing[docs[i].Slug] = true
	}
	if !existing[base] {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !existing[candidate] {
			return candidate
		}
	}
}

// generateRandomAlphaID generates a random 4-character lowercase alphabetic
// string, falling back to a deterministic-but-valid sequence on a crypto/rand
// failure rather than erroring.
func generateRandomAlphaID() string {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = letters[i%26]
		}
		return string(b)
	}
	for i := range b {
		b[i] = letters[int(b[i])%26]
	}
	return string(b)
}
