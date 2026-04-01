package history

import (
	"testing"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestSearchMessages(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	q := db.New(conn)
	sessionSvc := session.NewService(q, conn)
	messageSvc := message.NewService(q)
	historySvc := NewService(q, conn)

	sessA, err := sessionSvc.Create(t.Context(), "A")
	require.NoError(t, err)
	sessB, err := sessionSvc.Create(t.Context(), "B")
	require.NoError(t, err)

	_, err = messageSvc.Create(t.Context(), sessA.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Need help with history search"}},
	})
	require.NoError(t, err)
	_, err = messageSvc.Create(t.Context(), sessB.ID, message.CreateMessageParams{
		Role:  message.Assistant,
		Parts: []message.ContentPart{message.TextContent{Text: "History search is now available"}},
	})
	require.NoError(t, err)
	_, err = messageSvc.Create(t.Context(), sessB.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "Other topic"}},
	})
	require.NoError(t, err)

	results, err := historySvc.SearchMessages(t.Context(), SearchParams{Query: "history"})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, sessB.ID, results[0].SessionID)
	require.Equal(t, sessA.ID, results[1].SessionID)

	sessionScoped, err := historySvc.SearchMessages(t.Context(), SearchParams{Query: "history", SessionID: sessA.ID})
	require.NoError(t, err)
	require.Len(t, sessionScoped, 1)
	require.Equal(t, sessA.ID, sessionScoped[0].SessionID)

	limited, err := historySvc.SearchMessages(t.Context(), SearchParams{Query: "history", Limit: 1})
	require.NoError(t, err)
	require.Len(t, limited, 1)
}

func TestSearchMessagesRejectsEmptyQuery(t *testing.T) {
	t.Parallel()

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	historySvc := NewService(db.New(conn), conn)
	_, err = historySvc.SearchMessages(t.Context(), SearchParams{Query: "   "})
	require.Error(t, err)
	require.Contains(t, err.Error(), "query is required")
}
