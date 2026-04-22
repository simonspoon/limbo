package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests here verify that read-style commands accept --json as a no-op flag.
// Motivation: scripts and aspects frequently pass --json defensively because
// most tools use that convention. Before this change, show/tree/search/list
// would fail with "unknown flag: --json" even though JSON is already the
// default output format for show/search/list. See repeated gotcha in simaris
// lessons 019d8d21, 019d8ddc, 019da9ca.

// jsonFlagCommands lists the commands that must accept --json as a no-op
// alongside a human-readable label used in failure messages.
var jsonFlagCommands = []struct {
	name string
	cmd  *cobra.Command
}{
	{"show", showCmd},
	{"tree", treeCmd},
	{"search", searchCmd},
	{"list", listCmd},
}

func TestJSONFlagRegistered(t *testing.T) {
	for _, tc := range jsonFlagCommands {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			flag := tc.cmd.Flags().Lookup("json")
			require.NotNil(t, flag, "%s must register a --json flag", tc.name)
			assert.Equal(t, "bool", flag.Value.Type(), "%s --json must be a bool flag", tc.name)
			assert.Equal(t, "false", flag.DefValue, "%s --json must default to false", tc.name)
			assert.NotEmpty(t, flag.Usage, "%s --json must document its usage", tc.name)
		})
	}
}

func TestJSONFlagDocumentedInLongHelp(t *testing.T) {
	for _, tc := range jsonFlagCommands {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			long := strings.ToLower(tc.cmd.Long)
			assert.Contains(t, long, "json", "%s long help must mention JSON output behaviour", tc.name)
		})
	}
}

func TestJSONFlagDoesNotConflictWithPretty(t *testing.T) {
	// Only show and tree expose --pretty that could conflict; list and search
	// also have it, but tree is the interesting case because its default is
	// pretty=true. We assert both flags coexist on every command that defines
	// --pretty.
	for _, tc := range jsonFlagCommands {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pretty := tc.cmd.Flags().Lookup("pretty")
			if pretty == nil {
				t.Skipf("%s does not define --pretty", tc.name)
			}
			jsonFlag := tc.cmd.Flags().Lookup("json")
			require.NotNil(t, jsonFlag, "%s must register a --json flag", tc.name)
			assert.NotEqual(t, pretty, jsonFlag, "--pretty and --json must be distinct flags on %s", tc.name)
		})
	}
}
