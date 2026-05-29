package prompt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewLiteralPrompt_ReturnsContentUnchanged(t *testing.T) {
	t.Parallel()

	p := NewLiteralPrompt("hello world")
	result, err := p.Build(context.Background(), "", "", nil)
	require.NoError(t, err)
	require.Equal(t, "hello world", result)
}

func TestNewLiteralPrompt_TemplateMetacharsNotProcessed(t *testing.T) {
	t.Parallel()

	content := "this has {{.Provider}} in it"
	p := NewLiteralPrompt(content)
	result, err := p.Build(context.Background(), "", "", nil)
	require.NoError(t, err)
	require.Equal(t, content, result)
}

func TestNewLiteralPrompt_MultilineContent(t *testing.T) {
	t.Parallel()

	content := "# System Prompt\n\nYou are a specialist agent.\n\n## Instructions\n\nDo the thing."
	p := NewLiteralPrompt(content)
	result, err := p.Build(context.Background(), "", "", nil)
	require.NoError(t, err)
	require.Equal(t, content, result)
}
