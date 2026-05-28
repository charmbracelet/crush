package tools

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// resolvedSymbol holds the result of resolving a symbol name to an LSP position.
type resolvedSymbol struct {
	client *lsp.Client
	path   string
	line   int
	char   int
}

// resolveSymbol greps for a symbol name, triggers lazy LSP startup, and
// returns the first match position that a running client can serve.
func resolveSymbol(ctx context.Context, lspManager *lsp.Manager, symbol, workingDir string) (*resolvedSymbol, error) {
	results, err := resolveSymbolResults(ctx, lspManager, symbol, workingDir)
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// resolveSymbolResults greps for a symbol and returns all viable
// {client, path, position} tuples. Callers that need just one match
// (definition, rename, call hierarchy) use resolveSymbol; callers that
// want to iterate all matches (references) use this directly.
func resolveSymbolResults(ctx context.Context, lspManager *lsp.Manager, symbol, workingDir string) ([]*resolvedSymbol, error) {
	lspManager.Start(ctx, workingDir)

	matches, _, err := searchFiles(ctx, regexp.QuoteMeta(symbol), workingDir, "", 100)
	if err != nil {
		return nil, fmt.Errorf("failed to search for symbol: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("symbol '%s' not found in grep results", symbol)
	}

	var results []*resolvedSymbol
	for _, match := range matches {
		absPath, err := filepath.Abs(match.path)
		if err != nil {
			continue
		}

		client := findLSPClient(lspManager, absPath)
		if client == nil {
			continue
		}

		results = append(results, &resolvedSymbol{
			client: client,
			path:   absPath,
			line:   match.lineNum,
			char:   match.charNum + getSymbolOffset(symbol),
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no LSP client handles any file matching '%s'", symbol)
	}
	return results, nil
}

// findLSPClient returns the first LSP client that handles the given file path.
func findLSPClient(lspManager *lsp.Manager, filePath string) *lsp.Client {
	if abs, err := filepath.Abs(filePath); err == nil {
		filePath = abs
	}
	for c := range lspManager.Clients().Seq() {
		if c.HandlesFile(filePath) {
			return c
		}
	}
	return nil
}

// collectAffectedFiles extracts all unique file paths from a WorkspaceEdit.
func collectAffectedFiles(edit *protocol.WorkspaceEdit) []string {
	seen := make(map[string]struct{})
	var files []string

	for uri := range edit.Changes {
		path, err := uri.Path()
		if err != nil {
			continue
		}
		if _, ok := seen[path]; !ok {
			seen[path] = struct{}{}
			files = append(files, path)
		}
	}

	addURI := func(uri protocol.DocumentURI) {
		path, err := uri.Path()
		if err != nil {
			return
		}
		if _, ok := seen[path]; !ok {
			seen[path] = struct{}{}
			files = append(files, path)
		}
	}

	for _, change := range edit.DocumentChanges {
		switch {
		case change.TextDocumentEdit != nil:
			addURI(change.TextDocumentEdit.TextDocument.URI)
		case change.CreateFile != nil:
			addURI(change.CreateFile.URI)
		case change.RenameFile != nil:
			addURI(change.RenameFile.OldURI)
			addURI(change.RenameFile.NewURI)
		case change.DeleteFile != nil:
			addURI(change.DeleteFile.URI)
		}
	}

	return files
}

// isNoIdentifierError checks if an error indicates the grep match was not
// actually an identifier (e.g., matched inside a comment or string).
func isNoIdentifierError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no identifier found")
}

// getSymbolOffset returns the character offset to the actual symbol name
// in a qualified symbol (e.g., "Bar" in "foo.Bar" or "method" in "Class::method").
func getSymbolOffset(symbol string) int {
	if idx := strings.LastIndex(symbol, "::"); idx != -1 {
		return idx + 2
	}
	if idx := strings.LastIndex(symbol, "."); idx != -1 {
		return idx + 1
	}
	if idx := strings.LastIndex(symbol, "\\"); idx != -1 {
		return idx + 1
	}
	return 0
}

// cleanupLocations deduplicates and sorts a slice of LSP locations.
func cleanupLocations(locations []protocol.Location) []protocol.Location {
	slices.SortFunc(locations, func(a, b protocol.Location) int {
		if a.URI != b.URI {
			return strings.Compare(string(a.URI), string(b.URI))
		}
		if a.Range.Start.Line != b.Range.Start.Line {
			return cmp.Compare(a.Range.Start.Line, b.Range.Start.Line)
		}
		return cmp.Compare(a.Range.Start.Character, b.Range.Start.Character)
	})
	return slices.CompactFunc(locations, func(a, b protocol.Location) bool {
		return a.URI == b.URI &&
			a.Range.Start.Line == b.Range.Start.Line &&
			a.Range.Start.Character == b.Range.Start.Character
	})
}
