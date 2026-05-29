package tools

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/stretchr/testify/require"
)

func TestSymbolKindName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind protocol.SymbolKind
		want string
	}{
		{protocol.Function, "[func]"},
		{protocol.Method, "[method]"},
		{protocol.Class, "[class]"},
		{protocol.Struct, "[struct]"},
		{protocol.Interface, "[interface]"},
		{protocol.Variable, "[var]"},
		{protocol.Constant, "[const]"},
		{protocol.SymbolKind(999), "[kind:999]"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, symbolKindName(tt.kind))
	}
}

func TestFormatDocumentSymbols(t *testing.T) {
	t.Parallel()

	mkSym := func(name string, kind protocol.SymbolKind, line uint32, children ...protocol.DocumentSymbol) protocol.DocumentSymbol {
		return protocol.DocumentSymbol{
			Name:     name,
			Kind:     kind,
			Range:    protocol.Range{Start: protocol.Position{Line: line}},
			Children: children,
		}
	}

	tests := []struct {
		name     string
		results  []protocol.DocumentSymbolResult
		wantSubs []string
	}{
		{
			name: "flat list of top-level symbols",
			results: []protocol.DocumentSymbolResult{
				ptrSym(mkSym("Foo", protocol.Function, 9)),
				ptrSym(mkSym("Bar", protocol.Class, 19)),
			},
			wantSubs: []string{
				"2 top-level symbol",
				"[func] Foo (line 10)",
				"[class] Bar (line 20)",
			},
		},
		{
			name: "hierarchical class with methods indents children",
			results: []protocol.DocumentSymbolResult{
				ptrSym(mkSym("Service", protocol.Class, 4,
					mkSym("Run", protocol.Method, 5),
					mkSym("Stop", protocol.Method, 12),
				)),
			},
			wantSubs: []string{
				"[class] Service",
				"  [method] Run (line 6)",
				"  [method] Stop (line 13)",
			},
		},
		{
			name: "SymbolInformation flat fallback path",
			results: []protocol.DocumentSymbolResult{
				&protocol.SymbolInformation{
					Name: "globalThing",
					Kind: protocol.Variable,
					Location: protocol.Location{
						URI:   protocol.URIFromPath("/tmp/x.go"),
						Range: protocol.Range{Start: protocol.Position{Line: 41}},
					},
				},
			},
			wantSubs: []string{"[var] globalThing (line 42)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := formatDocumentSymbols("/tmp/file.go", tt.results)
			for _, sub := range tt.wantSubs {
				require.True(t, strings.Contains(out, sub), "want %q in:\n%s", sub, out)
			}
		})
	}
}

// ptrSym is a tiny helper that returns &v so we can place DocumentSymbols
// directly into a []DocumentSymbolResult literal.
func ptrSym(s protocol.DocumentSymbol) *protocol.DocumentSymbol { return &s }
