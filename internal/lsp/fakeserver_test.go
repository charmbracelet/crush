package lsp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

// fakeServerEnv makes the test binary run as a minimal LSP server instead of
// running the tests. The server publishes one error diagnostic for every
// document it is told was opened.
const fakeServerEnv = "CRUSH_TEST_FAKE_LSP_SERVER"

func TestMain(m *testing.M) {
	if os.Getenv(fakeServerEnv) == "1" {
		runFakeServer()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error                { return nil }

func runFakeServer() {
	ctx := context.Background()
	stream := jsonrpc2.NewBufferedStream(stdio{}, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(ctx, stream, jsonrpc2.HandlerWithError(
		func(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (any, error) {
			switch req.Method {
			case "initialize":
				return map[string]any{"capabilities": map[string]any{}}, nil
			case "shutdown":
				return nil, nil
			case "exit":
				os.Exit(0)
			case "textDocument/didOpen":
				var params protocol.DidOpenTextDocumentParams
				if req.Params != nil {
					_ = json.Unmarshal(*req.Params, &params)
				}
				_ = conn.Notify(ctx, "textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
					URI: params.TextDocument.URI,
					Diagnostics: []protocol.Diagnostic{{
						Severity: protocol.SeverityError,
						Message:  "fake",
					}},
				})
			}
			if req.Notif {
				return nil, nil
			}
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound, Message: req.Method}
		},
	))
	<-conn.DisconnectNotify()
}
