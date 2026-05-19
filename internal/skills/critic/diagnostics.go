package critic

import (
	"context"
	"path/filepath"
	"time"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// DiagnosticSnapshot is a flattened, serializable view of an LSP diagnostic.
type DiagnosticSnapshot struct {
	Path     string `json:"path"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Line     int    `json:"line"`
}

// FetchLSPDiagnostics queries all LSP clients for diagnostics on the given
// paths. It waits up to waitDuration for diagnostic versions to change,
// mitigating the async publish race.
func FetchLSPDiagnostics(
	ctx context.Context,
	lspMgr *lsp.Manager,
	paths []string,
	waitDuration time.Duration,
) ([]DiagnosticSnapshot, error) {
	if lspMgr == nil || len(paths) == 0 {
		return nil, nil
	}

	clients := lspMgr.Clients()
	if clients == nil {
		return nil, nil
	}

	// Build a map of absolute path -> client.
	type clientEntry struct {
		client *lsp.Client
		uri    protocol.DocumentURI
	}
	entries := make(map[string]clientEntry)

	for name, client := range clients.Seq2() {
		_ = name
		for _, p := range paths {
			abs, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			if client.HandlesFile(abs) {
				uri := protocol.DocumentURI("file://" + abs)
				entries[abs] = clientEntry{client: client, uri: uri}
			}
		}
	}

	// Wait for diagnostics to settle.
	if waitDuration > 0 {
		for _, e := range entries {
			waitCtx, cancel := context.WithTimeout(ctx, waitDuration)
			e.client.WaitForDiagnostics(waitCtx, waitDuration)
			cancel()
		}
	}

	var snaps []DiagnosticSnapshot
	for p, e := range entries {
		diags := e.client.GetFileDiagnostics(e.uri)
		for _, d := range diags {
			snaps = append(snaps, DiagnosticSnapshot{
				Path:     p,
				Severity: severityString(d.Severity),
				Message:  d.Message,
				Line:     int(d.Range.Start.Line),
			})
		}
	}

	return snaps, nil
}

func severityString(s protocol.DiagnosticSeverity) string {
	switch s {
	case protocol.SeverityError:
		return "error"
	case protocol.SeverityWarning:
		return "warning"
	case protocol.SeverityInformation:
		return "information"
	case protocol.SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}
