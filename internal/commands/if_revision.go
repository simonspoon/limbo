package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/simonspoon/limbo/internal/store/taskstore"
	"github.com/spf13/cobra"
)

// revisionMismatch is the exact structured error emitted to stderr when an
// --if-revision guard fails (A7).
type revisionMismatch struct {
	Error    string `json:"error"`
	Expected int    `json:"expected"`
	Actual   int    `json:"actual"`
}

// errRevisionMismatch is the sentinel returned by checkIfRevision on a
// mismatch. The structured JSON is already written to stderr by the time it is
// returned; cobra must not print it again, so RunE callers return it directly
// and Execute()'s top-level handler is the only other printer (it prints to
// stderr, which is acceptable but redundant — see checkIfRevision).
var errRevisionMismatch = fmt.Errorf("revision mismatch")

// checkIfRevision enforces the global --if-revision N guard for mutating
// commands. When the flag is set and the store's current revision differs from
// N, it writes {"error":"revision mismatch","expected":N,"actual":M} to stderr
// and returns a non-nil error so the command aborts before mutating anything.
// When the flag is absent it is a no-op.
func checkIfRevision(cmd *cobra.Command, store *taskstore.Store) error {
	if cmd == nil {
		return nil
	}
	flag := cmd.Flags().Lookup("if-revision")
	if flag == nil || !flag.Changed {
		return nil
	}
	want, err := cmd.Flags().GetInt("if-revision")
	if err != nil {
		return err
	}
	actual, err := store.Revision()
	if err != nil {
		return err
	}
	if actual == want {
		return nil
	}
	payload, _ := json.Marshal(revisionMismatch{
		Error:    "revision mismatch",
		Expected: want,
		Actual:   actual,
	})
	fmt.Fprintln(os.Stderr, string(payload))
	return errRevisionMismatch
}
