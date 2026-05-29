package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/simonspoon/limbo/internal/store"
	"github.com/simonspoon/limbo/internal/store/taskstore"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate a legacy in-tree .limbo store to the central location",
	Long: `Migrate a legacy in-tree .limbo/ store to the central, platform-conventional
storage root.

migrate scans the resolved project root for a legacy .limbo/ directory. If one
exists, it copies tasks.json and the context/ subtree to the central root,
rewrites the schema version to the current version, increments the revision by
one, and renames the source aside to .limbo.migrated-<ISO-8601-UTC>/ (it never
deletes the source). If the central root already holds a tasks.json, migrate
refuses and exits non-zero so the operator can reconcile manually. Running
migrate when no legacy store is present prints "no legacy store found" and exits
0 (idempotent).`,
	Args: cobra.NoArgs,
	RunE: runMigrate,
}

func runMigrate(cmd *cobra.Command, args []string) error {
	// Resolve the project root, honoring no-climb. A missing project anchor is
	// treated the same as "no legacy store found": migrate is a no-op.
	root, err := resolveProjectRoot()
	if err != nil {
		fmt.Println("no legacy store found")
		return nil
	}

	legacyDir := filepath.Join(root, ".limbo")
	if info, statErr := os.Stat(legacyDir); statErr != nil || !info.IsDir() {
		fmt.Println("no legacy store found")
		return nil
	}

	// Derive the project ID and the central storage root.
	id, err := store.ResolveProjectID(root, nil)
	if err != nil {
		return fmt.Errorf("resolve project id: %w", err)
	}
	centralRoot, err := centralStorageRoot(id)
	if err != nil {
		return err
	}

	// (A17) Refuse if the central root already holds a tasks.json. Do not copy
	// or rename anything; the operator must reconcile the two stores.
	if fileExists(filepath.Join(centralRoot, "tasks.json")) {
		return fmt.Errorf(
			"refusing to migrate: a central store already exists at %s.\n"+
				"Reconcile the legacy store at %s with the central store manually, "+
				"then delete or rename the legacy dir.",
			centralRoot, legacyDir)
	}

	// Copy the legacy store to the central root (transcoded to the current
	// schema version with revision incremented). The actual envelope read,
	// transcode, and context copy live in internal/store (A1).
	if err := taskstore.MigrateLegacy(legacyDir, centralRoot); err != nil {
		return fmt.Errorf("migrate legacy store: %w", err)
	}

	// Rename the source aside so it is no longer treated as a legacy store, but
	// is preserved for safety (never deleted).
	migratedDir := filepath.Join(root, ".limbo.migrated-"+time.Now().UTC().Format("20060102T150405Z"))
	if err := os.Rename(legacyDir, migratedDir); err != nil {
		return fmt.Errorf("rename legacy store aside: %w", err)
	}

	fmt.Printf("migrated legacy store\n  from: %s\n  to:   %s\n", migratedDir, centralRoot)
	return nil
}
