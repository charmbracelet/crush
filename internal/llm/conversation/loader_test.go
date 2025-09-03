package conversation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

type mockMessageService struct {
	message.Service
	listFunc func(ctx context.Context, sessionID string) ([]message.Message, error)
}

func (m *mockMessageService) List(ctx context.Context, sessionID string) ([]message.Message, error) {
	return m.listFunc(ctx, sessionID)
}

func TestLoadFromSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := session.Session{ID: "test-session"}

	t.Run("should return a conversation with the correct messages", func(t *testing.T) {
		t.Parallel()
		service := getService(t)

		conversation, err := LoadFromSession(ctx, s, service)
		require.NoError(t, err)
		require.Len(t, conversation.turns, 2)
		require.Equal(t, message.User, conversation.turns[0].Role)
		require.Equal(t, "hello", conversation.turns[0].Parts[0].(message.TextContent).Text)
		require.Equal(t, message.Assistant, conversation.turns[1].Role)
		require.Equal(t, "world", conversation.turns[1].Parts[0].(message.TextContent).Text)
	})

	t.Run("should return an error if the service fails to list messages", func(t *testing.T) {
		t.Parallel()
		service := &mockMessageService{
			listFunc: func(ctx context.Context, sessionID string) ([]message.Message, error) {
				return nil, errors.New("bork")
			},
		}

		_, err := LoadFromSession(ctx, s, service)
		require.Error(t, err)
		require.Equal(t, "bork", err.Error())
	})

	t.Run("should ignore messages before the summary", func(t *testing.T) {
		t.Parallel()
		service := getService(t)
		s = session.Session{ID: "test-session", SummaryMessageID: "1"}

		conversation, err := LoadFromSession(ctx, s, service)
		fmt.Println(conversation)
		require.NoError(t, err)
		require.Len(t, conversation.turns, 1)
		require.Equal(t, message.User, conversation.turns[0].Role)
		require.Equal(t, "hello", conversation.turns[0].Parts[0].(message.TextContent).Text)
	})
}

func getService(t *testing.T) message.Service {
	t.Helper()
	service := &mockMessageService{
		listFunc: func(ctx context.Context, sessionID string) ([]message.Message, error) {
			return []message.Message{
				{
					ID:    "1",
					Role:  "user",
					Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
				},
				{
					ID:    "2",
					Role:  "assistant",
					Parts: []message.ContentPart{message.TextContent{Text: "world"}},
				},
			}, nil
		},
	}
	return service
}
