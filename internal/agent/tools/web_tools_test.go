package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type recordingWebPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
	granted  bool
	requests []permission.CreatePermissionRequest
}

func (m *recordingWebPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	m.requests = append(m.requests, req)
	return m.granted, nil
}

func (m *recordingWebPermissionService) Grant(req permission.PermissionRequest) bool { return true }

func (m *recordingWebPermissionService) Deny(req permission.PermissionRequest) bool { return true }

func (m *recordingWebPermissionService) GrantPersistent(req permission.PermissionRequest) bool {
	return true
}

func (m *recordingWebPermissionService) AutoApproveSession(sessionID string) {}

func (m *recordingWebPermissionService) SetSkipRequests(skip bool) {}

func (m *recordingWebPermissionService) SkipRequests() bool { return false }

func (m *recordingWebPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(chan pubsub.Event[permission.PermissionNotification])
}

type countingTransport struct{ calls atomic.Int32 }

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return nil, errors.New("network should not be called")
}

func webToolContext() context.Context {
	return context.WithValue(context.Background(), SessionIDContextKey, "test-session")
}

func runWebTool(t *testing.T, tool fantasy.AgentTool, name string, params any) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	resp, err := tool.Run(webToolContext(), fantasy.ToolCall{
		ID:    "test-call",
		Name:  name,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}

func TestWebSearchDeniedDoesNotCallNetwork(t *testing.T) {
	transport := &countingTransport{}
	perms := &recordingWebPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), granted: false}
	tool := NewWebSearchTool(perms, t.TempDir(), &http.Client{Transport: transport})

	resp := runWebTool(t, tool, WebSearchToolName, WebSearchParams{Query: "latest go release 2026", MaxResults: 5})

	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "User denied permission")
	require.Equal(t, int32(0), transport.calls.Load())
	require.Len(t, perms.requests, 1)
	require.Equal(t, WebSearchToolName, perms.requests[0].ToolName)
	require.Equal(t, "search", perms.requests[0].Action)
	require.Equal(t, "latest go release 2026", perms.requests[0].Resource)
	params, ok := perms.requests[0].Params.(WebSearchPermissionsParams)
	require.True(t, ok)
	require.Equal(t, "latest go release 2026", params.Query)
	require.Equal(t, 5, params.MaxResults)
}

func TestWebFetchDeniedDoesNotCallNetwork(t *testing.T) {
	transport := &countingTransport{}
	perms := &recordingWebPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), granted: false}
	tool := NewWebFetchTool(perms, t.TempDir(), t.TempDir(), &http.Client{Transport: transport})

	resp := runWebTool(t, tool, WebFetchToolName, WebFetchParams{URL: "https://example.com/docs"})

	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "User denied permission")
	require.Equal(t, int32(0), transport.calls.Load())
	require.Len(t, perms.requests, 1)
	require.Equal(t, WebFetchToolName, perms.requests[0].ToolName)
	require.Equal(t, "fetch", perms.requests[0].Action)
	require.Equal(t, "https://example.com/docs", perms.requests[0].Resource)
	params, ok := perms.requests[0].Params.(WebFetchPermissionsParams)
	require.True(t, ok)
	require.Equal(t, "https://example.com/docs", params.URL)
}

func TestWebFetchLargeContentWritesToScratchDir(t *testing.T) {
	workingDir := t.TempDir()
	scratchDir := t.TempDir()
	body := "MCP server installation configuration command args\n" + strings.Repeat("a", LargeContentThreshold+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	perms := &recordingWebPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest](), granted: true}
	tool := NewWebFetchTool(perms, workingDir, scratchDir, server.Client())

	ctx := WithMCPSourceEvidence(webToolContext(), "install MCP")
	recordMCPSearchResults(ctx, []SearchResult{{Link: server.URL}})
	input, err := json.Marshal(WebFetchParams{URL: server.URL})
	require.NoError(t, err)
	resp, err := tool.Run(ctx, fantasy.ToolCall{Name: WebFetchToolName, Input: string(input)})
	require.NoError(t, err)

	require.False(t, resp.IsError)
	require.True(t, hasMCPSourceEvidence(ctx, server.URL))
	require.Contains(t, resp.Content, "Content saved to:")
	prefix := "Content saved to: "
	start := strings.Index(resp.Content, prefix)
	require.NotEqual(t, -1, start)
	savedPath := strings.TrimSpace(strings.Split(resp.Content[start+len(prefix):], "\n")[0])
	require.Contains(t, savedPath, scratchDir)
	require.FileExists(t, savedPath)

	savedContent, err := os.ReadFile(savedPath)
	require.NoError(t, err)
	require.Equal(t, body, string(savedContent))

	rootPages, err := filepath.Glob(filepath.Join(workingDir, "page-*.md"))
	require.NoError(t, err)
	require.Empty(t, rootPages)
}

func TestMCPSourceEvidenceRejectsGuessedURL(t *testing.T) {
	t.Parallel()
	ctx := WithMCPSourceEvidence(t.Context(), "install MCP")
	reason := recordMCPSourceEvidence(ctx, "https://guessed.example/mcp", "MCP installation configuration command args")

	require.Contains(t, reason, "not supplied by the user or returned by web_search")
	require.False(t, hasMCPSourceEvidence(ctx, "https://guessed.example/mcp"))
}

func TestMCPSourceEvidenceRejectsLoginInterstitial(t *testing.T) {
	t.Parallel()
	ctx := WithMCPSourceEvidence(t.Context(), "install MCP")
	serverURL := "https://example.com/mcp"
	recordMCPSearchResults(ctx, []SearchResult{{Link: serverURL}})
	reason := recordMCPSourceEvidence(ctx, serverURL, "# Sign in to GitHub\n{{ message }}\n### Uh oh!\nThere was an error while loading")

	require.Contains(t, reason, "login")
	require.False(t, hasMCPSourceEvidence(ctx, serverURL))
}

func TestMCPSourceEvidenceAcceptsUserProvidedURL(t *testing.T) {
	t.Parallel()
	serverURL := "https://example.com/mcp"
	ctx := WithMCPSourceEvidence(t.Context(), "Install this MCP: "+serverURL)
	reason := recordMCPSourceEvidence(ctx, serverURL+"/", "MCP server installation configuration command args")

	require.Empty(t, reason)
	require.True(t, hasMCPSourceEvidence(ctx, serverURL))
}
