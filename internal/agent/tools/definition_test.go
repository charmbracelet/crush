package tools

import (
	"testing"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/stretchr/testify/require"
)

// formatDefinitions is the only piece of the lsp_definition tool that is
// pure (the rest needs a live LSP server). Table tests cover the shapes
// that come out of LSP for typical languages: single hit, cross-file,
// duplicates that collapse via cleanupLocations.
func TestFormatDefinitions(t *testing.T) {
	t.Parallel()

	loc := func(path string, line, col int) protocol.Location {
		return protocol.Location{
			URI: protocol.URIFromPath(path),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(line), Character: uint32(col)},
				End:   protocol.Position{Line: uint32(line), Character: uint32(col) + 1},
			},
		}
	}

	tests := []struct {
		name      string
		locs      []protocol.Location
		wantSubs  []string
	}{
		{
			name:     "single location includes path and 1-based position",
			locs:     []protocol.Location{loc("/tmp/foo.go", 9, 5)},
			wantSubs: []string{"Found 1 definition", "/tmp/foo.go:10:6"},
		},
		{
			name: "multiple files reported separately",
			locs: []protocol.Location{
				loc("/tmp/a.go", 0, 0),
				loc("/tmp/b.go", 4, 2),
			},
			wantSubs: []string{
				"Found 2 definition",
				"/tmp/a.go:1:1",
				"/tmp/b.go:5:3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := formatDefinitions(tt.locs)
			for _, sub := range tt.wantSubs {
				require.Contains(t, out, sub)
			}
		})
	}
}

// cleanupLocations is shared with lsp_references; verify it dedups
// identical hits so callers can pass raw LSP output through.
func TestCleanupLocations_DedupesAndSorts(t *testing.T) {
	t.Parallel()

	a := protocol.Location{URI: protocol.URIFromPath("/tmp/a.go"), Range: protocol.Range{Start: protocol.Position{Line: 0, Character: 0}}}
	b := protocol.Location{URI: protocol.URIFromPath("/tmp/b.go"), Range: protocol.Range{Start: protocol.Position{Line: 0, Character: 0}}}

	got := cleanupLocations([]protocol.Location{b, a, a, b})
	require.Len(t, got, 2)
	require.Equal(t, a.URI, got[0].URI, "expected sorted output to put /tmp/a.go first")
}
