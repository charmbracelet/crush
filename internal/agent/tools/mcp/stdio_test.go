package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// stallingStdioServer makes the test binary serve MCP over stdio but stop
// reading its stdin once a single message grows past stallAfter bytes, like a
// server that wedges on a large tool call and leaves the pipe full.
const (
	stallingStdioServer = "stall"
	stallAfter          = 64 << 10
)

func serveStallingStdio() {
	server := mcp.NewServer(&mcp.Implementation{Name: "stdio-stall"}, nil)
	mcp.AddTool(
		server,
		&mcp.Tool{Name: "write_file"},
		func(context.Context, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
		},
	)
	in := &stallingReader{r: os.Stdin}
	if err := server.Run(context.Background(), &mcp.IOTransport{Reader: in, Writer: os.Stdout}); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// stallingReader passes reads through until the current line is longer than
// stallAfter, then never returns.
type stallingReader struct {
	r    io.ReadCloser
	line int
}

func (s *stallingReader) Read(p []byte) (int, error) {
	for s.line > stallAfter {
		time.Sleep(time.Hour)
	}
	n, err := s.r.Read(p)
	if i := bytes.LastIndexByte(p[:n], '\n'); i >= 0 {
		s.line = n - i - 1
	} else {
		s.line += n
	}
	return n, err
}

func (s *stallingReader) Close() error { return s.r.Close() }

// TestRunTool_StalledStdioWriteHonorsContext guards #2854. A tool call whose
// request is bigger than the pipe buffer blocks in the write until the server
// reads it, and the SDK does not watch the context there. Against a server
// that stopped reading, a cancelled call never returned, and every later call
// to that server queued behind it.
func TestRunTool_StalledStdioWriteHonorsContext(t *testing.T) {
	t.Parallel()

	const name = "stdio-stalled-write"
	exe, err := os.Executable()
	require.NoError(t, err)
	m := config.MCPConfig{
		Type:    config.MCPStdio,
		Command: exe,
		Env:     map[string]string{stdioServerEnv: stallingStdioServer},
	}
	cfg := config.NewTestStore(&config.Config{
		MCP:     config.MCPs{name: m},
		Options: &config.Options{DataDirectory: t.TempDir()},
	})

	sess, err := createSession(t.Context(), cfg, name, m, shellResolverWithPath(t, nil), false)
	require.NoError(t, err)
	sessions.Set(name, sess)
	t.Cleanup(func() {
		if s, ok := sessions.Take(name); ok {
			closeSession(name, s)
		}
		allTools.Del(name)
		allPrompts.Del(name)
		allResources.Del(name)
		states.Del(name)
	})

	run := func(ctx context.Context, content string) error {
		input, err := json.Marshal(map[string]string{"content": content})
		require.NoError(t, err)
		done := make(chan error, 1)
		go func() {
			_, err := RunTool(ctx, cfg, name, "write_file", string(input))
			done <- err
		}()
		select {
		case err := <-done:
			return err
		case <-time.After(10 * time.Second):
			t.Fatal("RunTool is still blocked writing to a server that stopped reading")
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.ErrorIs(t, run(ctx, strings.Repeat("x", 4<<20)), context.DeadlineExceeded)

	require.NoError(t, run(t.Context(), "small"),
		"the next call must get a working session, not wait behind the stalled write")
	renewed, ok := sessions.Get(name)
	require.True(t, ok)
	require.NotSame(t, sess, renewed, "the stalled session must be replaced")
}
