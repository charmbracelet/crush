package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/index"
	"github.com/stretchr/testify/require"
)

func runMapTool(t *testing.T, tool fantasy.AgentTool, params MapParams) fantasy.ToolResponse {
	t.Helper()

	input, err := json.Marshal(params)
	require.NoError(t, err)

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "test-call",
		Name:  MapToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}

func TestMapTool(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dataDir := t.TempDir()

	write := func(rel, content string) {
		abs := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
	}
	write("go.mod", "module example.com/proj\n")
	write("main.go", "package main\n\nimport \"example.com/proj/internal/api\"\n\nfunc main() { api.NewHandler() }\n")
	write("internal/api/handler.go", "package api\n\ntype Handler struct{}\n\nfunc NewHandler() *Handler { return nil }\n")

	cfg := config.NewTestStoreWithDir(&config.Config{
		Options: &config.Options{DataDirectory: dataDir},
	}, root)
	tool := NewMapTool(cfg)

	// Build finishes before queries — the tool path is non-blocking,
	// the test warms it via the shared service.
	svc := index.Shared(dataDir, root)
	require.NoError(t, svc.EnsureIndexed(context.Background()))

	skel := runMapTool(t, tool, MapParams{})
	require.False(t, skel.IsError)
	require.Contains(t, skel.Content, "internal/api")
	require.Contains(t, skel.Content, "main.go")

	sym := runMapTool(t, tool, MapParams{Symbol: "Handler"})
	require.Contains(t, sym.Content, "internal/api/handler.go")

	sub := runMapTool(t, tool, MapParams{Path: "internal/api"})
	require.Contains(t, sub.Content, "NewHandler")

	// Semantic mode is reserved — it returns guidance, not an error.
	sem := runMapTool(t, tool, MapParams{Semantic: "where is auth"})
	require.Contains(t, sem.Content, "not enabled")

	// A file written through the write path (NotifyWritten) becomes
	// visible without a re-walk — the mid-session new-file case.
	write("internal/api/middleware.go", "package api\n\nfunc Middleware() {}\n")
	index.NotifyWritten(filepath.Join(root, "internal/api/middleware.go"))
	sub = runMapTool(t, tool, MapParams{Path: "internal/api"})
	require.Contains(t, sub.Content, "Middleware")
}
