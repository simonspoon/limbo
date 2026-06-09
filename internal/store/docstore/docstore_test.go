package docstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/simonspoon/limbo/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocStore_AddListGet(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	d1, err := s.Add(models.Doc{Title: "My PRD", Type: "prd"}, "body one")
	require.NoError(t, err)
	assert.True(t, models.IsValidDocID(d1.ID))
	assert.Equal(t, "my-prd", d1.Slug)

	d2, err := s.Add(models.Doc{Title: "Architecture", Type: "arch"}, "body two")
	require.NoError(t, err)
	assert.NotEqual(t, d1.ID, d2.ID)

	docs, err := s.List()
	require.NoError(t, err)
	assert.Len(t, docs, 2)

	got, err := s.Get(d1.ID)
	require.NoError(t, err)
	assert.Equal(t, "My PRD", got.Title)

	body, err := s.ReadBody(d1.ID)
	require.NoError(t, err)
	assert.Equal(t, "body one", body)
}

func TestDocStore_GetNotFound(t *testing.T) {
	s := New(t.TempDir())
	_, err := s.Get("zzzz")
	assert.ErrorIs(t, err, ErrDocNotFound)
}

func TestDocStore_EmptyList(t *testing.T) {
	s := New(t.TempDir())
	docs, err := s.List()
	require.NoError(t, err)
	assert.NotNil(t, docs)
	assert.Len(t, docs, 0)
}

func TestDocStore_SlugCollisionDisambiguation(t *testing.T) {
	s := New(t.TempDir())
	a, err := s.Add(models.Doc{Title: "Same Title", Type: "prd"}, "")
	require.NoError(t, err)
	b, err := s.Add(models.Doc{Title: "Same Title", Type: "prd"}, "")
	require.NoError(t, err)
	c, err := s.Add(models.Doc{Title: "Same Title", Type: "prd"}, "")
	require.NoError(t, err)
	assert.Equal(t, "same-title", a.Slug)
	assert.Equal(t, "same-title-2", b.Slug)
	assert.Equal(t, "same-title-3", c.Slug)
}

func TestDocStore_ADRDefaultsToProposed(t *testing.T) {
	s := New(t.TempDir())
	d, err := s.Add(models.Doc{Title: "Use X", Type: models.DocTypeADR}, "")
	require.NoError(t, err)
	assert.Equal(t, models.ADRStatusProposed, d.Status)

	// reload from disk
	got, err := New(s.root).Get(d.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ADRStatusProposed, got.Status)
}

func TestDocStore_NonADRHasNoStatus(t *testing.T) {
	s := New(t.TempDir())
	d, err := s.Add(models.Doc{Title: "Plain", Type: "prd"}, "")
	require.NoError(t, err)
	assert.Empty(t, d.Status)
}

func TestDocStore_DeleteRemovesEntryAndDir(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	d, err := s.Add(models.Doc{Title: "Doomed", Type: "prd"}, "bye")
	require.NoError(t, err)

	require.NoError(t, s.Delete(d.ID))

	_, err = s.Get(d.ID)
	assert.ErrorIs(t, err, ErrDocNotFound)
	_, statErr := os.Stat(s.DocDir(d.ID))
	assert.True(t, os.IsNotExist(statErr))
}

func TestDocStore_DeleteNonexistent(t *testing.T) {
	s := New(t.TempDir())
	err := s.Delete("zzzz")
	assert.ErrorIs(t, err, ErrDocNotFound)
}

func TestDocStore_UpdateNotFound(t *testing.T) {
	s := New(t.TempDir())
	err := s.Update(models.Doc{ID: "zzzz", Title: "ghost"})
	assert.ErrorIs(t, err, ErrDocNotFound)
}

func TestDocStore_UpdatePairAtomic(t *testing.T) {
	s := New(t.TempDir())
	a, _ := s.Add(models.Doc{Title: "A", Type: models.DocTypeADR}, "")
	b, _ := s.Add(models.Doc{Title: "B", Type: models.DocTypeADR}, "")

	a.SupersededBy = b.ID
	a.Status = models.ADRStatusSuperseded
	b.Supersedes = a.ID
	require.NoError(t, s.UpdatePair(a, b))

	// reload from disk
	fresh := New(s.root)
	ga, _ := fresh.Get(a.ID)
	gb, _ := fresh.Get(b.ID)
	assert.Equal(t, b.ID, ga.SupersededBy)
	assert.Equal(t, models.ADRStatusSuperseded, ga.Status)
	assert.Equal(t, a.ID, gb.Supersedes)
}

func TestDocStore_UpdatePairMissing(t *testing.T) {
	s := New(t.TempDir())
	a, _ := s.Add(models.Doc{Title: "A", Type: models.DocTypeADR}, "")
	err := s.UpdatePair(a, models.Doc{ID: "zzzz"})
	assert.ErrorIs(t, err, ErrDocNotFound)
}

// AC-6: only the shared store.lock should exist; no docs.lock. Doc writes must
// never create tasks.json.tmp/.bak at the root (R3).
func TestDocStore_UsesStoreLock(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	_, err := s.Add(models.Doc{Title: "X", Type: "prd"}, "body")
	require.NoError(t, err)

	_, statErr := os.Stat(filepath.Join(root, "store.lock"))
	assert.NoError(t, statErr, "expected shared store.lock at root")

	_, docsLockErr := os.Stat(filepath.Join(root, "docs.lock"))
	assert.True(t, os.IsNotExist(docsLockErr), "must NOT create a separate docs.lock")
	_, docsLockErr2 := os.Stat(filepath.Join(root, "docs", "docs.lock"))
	assert.True(t, os.IsNotExist(docsLockErr2), "must NOT create docs/docs.lock")

	// R3: no tasks.json.* collision at the root.
	for _, name := range []string{"tasks.json.tmp", "tasks.json.bak"} {
		_, e := os.Stat(filepath.Join(root, name))
		assert.Truef(t, os.IsNotExist(e), "doc write must not create %s", name)
	}
}

// AC-3 / AC-6: index.json at rest is valid JSON readable without the CLI.
func TestDocStore_IndexAtRestIsValidJSON(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	_, err := s.Add(models.Doc{Title: "One", Type: "prd"}, "a")
	require.NoError(t, err)
	d2, err := s.Add(models.Doc{Title: "Two", Type: models.DocTypeADR}, "b")
	require.NoError(t, err)
	require.NoError(t, s.Delete(d2.ID))

	data, err := os.ReadFile(s.IndexPath())
	require.NoError(t, err)
	var env struct {
		Version string       `json:"version"`
		Docs    []models.Doc `json:"docs"`
	}
	require.NoError(t, json.Unmarshal(data, &env), "index.json must be valid JSON at rest")
	assert.Len(t, env.Docs, 1)
}

// AC-2 round-trip fidelity including embedded quotes/backslashes/unicode (R8).
func TestDocStore_BodyRoundTripFidelity(t *testing.T) {
	s := New(t.TempDir())
	body := "line one\n\"quoted\" and \\backslash\\ and unicode: café ☕\nfinal line\n"
	d, err := s.Add(models.Doc{Title: "Body", Type: "prd"}, body)
	require.NoError(t, err)

	got, err := New(s.root).ReadBody(d.ID)
	require.NoError(t, err)
	assert.Equal(t, body, got)
}

// AC-6: concurrent readers and writers never observe partial JSON, and the
// index parses cleanly after the storm. Run with -race.
func TestDocStoreConcurrent(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	// Seed a few docs so readers have something to read.
	for i := 0; i < 3; i++ {
		_, err := s.Add(models.Doc{Title: "seed", Type: "prd"}, "seed body")
		require.NoError(t, err)
	}

	const workers = 8
	const iterations = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers*iterations)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		writer := w%2 == 0
		go func(writer bool) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if writer {
					if _, err := s.Add(models.Doc{Title: "concurrent", Type: "prd"}, "b"); err != nil {
						errCh <- err
						return
					}
				} else {
					docs, err := s.List()
					if err != nil {
						errCh <- err
						return
					}
					// every successful read returns fully-parseable docs
					_ = docs
				}
			}
		}(writer)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	// index.json parses cleanly after the storm.
	data, err := os.ReadFile(s.IndexPath())
	require.NoError(t, err)
	var env envelope
	require.NoError(t, json.Unmarshal(data, &env))

	// All ids remain unique (no collision under concurrent Add).
	seen := make(map[string]bool)
	for _, d := range env.Docs {
		assert.Falsef(t, seen[d.ID], "duplicate doc id %s minted under concurrency", d.ID)
		seen[d.ID] = true
	}
}
