package tools

import (
	"context"
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type historySearchStub struct {
	*pubsub.Broker[history.File]
	results []history.MessageSearchResult
	err     error
	params  history.SearchParams
}

func newHistorySearchStub() *historySearchStub {
	return &historySearchStub{Broker: pubsub.NewBroker[history.File]()}
}

func (s *historySearchStub) Create(context.Context, string, string, string) (history.File, error) {
	return history.File{}, nil
}

func (s *historySearchStub) CreateVersion(context.Context, string, string, string) (history.File, error) {
	return history.File{}, nil
}

func (s *historySearchStub) Get(context.Context, string) (history.File, error) {
	return history.File{}, nil
}

func (s *historySearchStub) GetByPathAndSession(context.Context, string, string) (history.File, error) {
	return history.File{}, nil
}

func (s *historySearchStub) ListBySession(context.Context, string) ([]history.File, error) {
	return nil, nil
}

func (s *historySearchStub) ListLatestSessionFiles(context.Context, string) ([]history.File, error) {
	return nil, nil
}

func (s *historySearchStub) SearchMessages(_ context.Context, params history.SearchParams) ([]history.MessageSearchResult, error) {
	s.params = params
	return s.results, s.err
}

func (s *historySearchStub) Delete(context.Context, string) error {
	return nil
}

func (s *historySearchStub) DeleteSessionFiles(context.Context, string) error {
	return nil
}

func runHistorySearchTool(t *testing.T, tool fantasy.AgentTool, params HistorySearchParams) (fantasy.ToolResponse, error) {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	return tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  HistorySearchToolName,
		Input: string(input),
	})
}

func TestHistorySearchTool(t *testing.T) {
	t.Parallel()

	stub := newHistorySearchStub()
	stub.results = []history.MessageSearchResult{
		{SessionID: "session-1", Role: "user", Text: "Need help with search", CreatedAt: 1710000000},
	}
	tool := NewHistorySearchTool(stub)

	resp, err := runHistorySearchTool(t, tool, HistorySearchParams{Query: "search", SessionID: "session-1", Limit: 5})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, "Found 1 matching messages")
	require.Contains(t, resp.Content, "session=session-1")
	require.Equal(t, "search", stub.params.Query)
	require.Equal(t, "session-1", stub.params.SessionID)
	require.Equal(t, 5, stub.params.Limit)
}

func TestHistorySearchToolEmptyQuery(t *testing.T) {
	t.Parallel()

	tool := NewHistorySearchTool(newHistorySearchStub())
	resp, err := runHistorySearchTool(t, tool, HistorySearchParams{Query: "  "})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "query is required")
}

func TestHistorySearchToolNoResults(t *testing.T) {
	t.Parallel()

	tool := NewHistorySearchTool(newHistorySearchStub())
	resp, err := runHistorySearchTool(t, tool, HistorySearchParams{Query: "missing"})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Equal(t, "No matching messages found.", resp.Content)
}
