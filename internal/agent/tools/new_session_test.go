package tools

import (
	"context"
	"errors"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestNewSessionTool(t *testing.T) {
	t.Parallel()

	tool := NewNewSessionTool()
	info := tool.Info()

	require.Equal(t, NewSessionToolName, info.Name)
	require.NotEmpty(t, info.Description)
	require.Contains(t, info.Parameters, "summary")
}

func TestNewSessionToolReturnsError(t *testing.T) {
	t.Parallel()

	tool := NewNewSessionTool()
	summary := "Completed steps 1-3. Remaining: step 4 - write tests."

	_, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call_123",
		Name:  NewSessionToolName,
		Input: `{"summary":"` + summary + `"}`,
	})

	require.Error(t, err)

	var nse *NewSessionError
	require.True(t, errors.As(err, &nse))
	require.Equal(t, summary, nse.Summary)
}

func TestNewSessionToolEmptySummary(t *testing.T) {
	t.Parallel()

	tool := NewNewSessionTool()

	_, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call_456",
		Name:  NewSessionToolName,
		Input: `{"summary":""}`,
	})

	require.Error(t, err)

	var nse *NewSessionError
	require.True(t, errors.As(err, &nse))
	require.Empty(t, nse.Summary)
}

func TestNewSessionErrorMessage(t *testing.T) {
	t.Parallel()

	err := &NewSessionError{Summary: "test"}
	require.Equal(t, "new session requested", err.Error())
}
