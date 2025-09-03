package title

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/llm/conversation"
	"github.com/charmbracelet/crush/internal/llm/provider"
	"github.com/charmbracelet/crush/internal/llm/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestTitleGenerator(t *testing.T) {
	t.Parallel()

	t.Run("it should generate a title", func(t *testing.T) {
		t.Parallel()

		prov := &mockProvider{
			response: "Test title",
		}
		gen := NewGenerator(prov)
		conv := conversation.New(
			message.Message{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Hi"}}},
			message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Hello"}}},
		)

		title, err := gen.Generate(context.Background(), conv)
		require.NoError(t, err)
		require.Equal(t, "Test title", title)
	})

	t.Run("it should return an empty title if no user messages are found", func(t *testing.T) {
		t.Parallel()

		prov := &mockProvider{}
		gen := NewGenerator(prov)
		conv := conversation.New(
			message.Message{Role: message.Assistant, Parts: []message.ContentPart{message.TextContent{Text: "Hello"}}},
		)

		title, err := gen.Generate(context.Background(), conv)
		require.NoError(t, err)
		require.Equal(t, "", title)
	})

	t.Run("it should return an error if the provider returns an error", func(t *testing.T) {
		t.Parallel()

		prov := &mockProvider{
			err: errors.New("provider error"),
		}
		gen := NewGenerator(prov)
		conv := conversation.New(
			message.Message{Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "Hello"}}},
		)

		_, err := gen.Generate(context.Background(), conv)
		require.Error(t, err)
		require.Equal(t, "provider error", err.Error())
	})
}

type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) StreamResponse(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan provider.ProviderEvent {
	ch := make(chan provider.ProviderEvent, 1)
	if m.err != nil {
		ch <- provider.ProviderEvent{Error: m.err}
	} else {
		ch <- provider.ProviderEvent{Response: &provider.ProviderResponse{Content: m.response}}
	}
	close(ch)
	return ch
}

func (m *mockProvider) SendMessages(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (*provider.ProviderResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &provider.ProviderResponse{Content: m.response}, nil
}

func (m *mockProvider) Model() catwalk.Model {
	return catwalk.Model{}
}
