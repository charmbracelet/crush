package model

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	uichat "github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestFormatModifiedFilePathUsesProjectRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	filePath := filepath.Join(root, "internal", "agent", "main.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(filePath), 0o755))
	require.NoError(t, os.WriteFile(filePath, []byte("package main"), 0o644))

	display := formatModifiedFilePath(filepath.Join(root, "dist"), filePath)
	require.Equal(t, filepath.Join("internal", "agent", "main.go"), display)
}

func TestCompactModifiedFilePathKeepsTail(t *testing.T) {
	t.Parallel()

	path := filepath.Join("internal", "verylongmodule", "subsystem", "main.go")
	compact := compactModifiedFilePath(path, 24)
	require.Contains(t, compact, filepath.Join("subsystem", "main.go"))
	require.NotContains(t, compact, "~")
}

func TestChildSessionsReturnsAllChildrenInStableOrder(t *testing.T) {
	t.Parallel()

	ui, parent, generalChild, firstChild, secondChild, fetchChild := testSessionUI(t)

	children, err := ui.childSessions(parent.ID)
	require.NoError(t, err)
	require.Len(t, children, 4)
	require.Equal(t, generalChild.ID, children[0].ID)
	require.Equal(t, firstChild.ID, children[1].ID)
	require.Equal(t, secondChild.ID, children[2].ID)
	require.Equal(t, fetchChild.ID, children[3].ID)
}

func TestSessionRoleLabelUsesSubagentType(t *testing.T) {
	t.Parallel()

	ui, parent, generalChild, _, _, fetchChild := testSessionUI(t)
	info := make(map[string]childSessionInfo)
	generalInfo, ok := fetchChildSessionMetadata(ui.com.App, generalChild.ID)
	require.True(t, ok)
	info[generalChild.ID] = generalInfo
	fetchInfo, ok := fetchChildSessionMetadata(ui.com.App, fetchChild.ID)
	require.True(t, ok)
	info[fetchChild.ID] = fetchInfo
	ui.childSessionInfoCache = info

	require.Equal(t, "Main", ui.sessionRoleLabel(parent))
	require.Equal(t, "General", ui.sessionRoleLabel(generalChild))
	require.Equal(t, "Fetch", ui.sessionRoleLabel(fetchChild))
}

func TestOpenSelectedChildSessionUsesCurrentSelection(t *testing.T) {
	t.Parallel()

	ui, parent, _, firstChild, _, _ := testSessionUI(t)
	ui.session = parent

	msgs, err := ui.com.App.Messages.List(context.Background(), parent.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)

	toolCalls := msgs[0].ToolCalls()
	require.Len(t, toolCalls, 4)
	ui.chat.SetMessages(
		uichat.NewToolMessageItem(ui.com.Styles, msgs[0].ID, toolCalls[0], nil, false),
		uichat.NewToolMessageItem(ui.com.Styles, msgs[0].ID, toolCalls[1], nil, false),
		uichat.NewToolMessageItem(ui.com.Styles, msgs[0].ID, toolCalls[2], nil, false),
		uichat.NewToolMessageItem(ui.com.Styles, msgs[0].ID, toolCalls[3], nil, false),
	)
	require.True(t, ui.chat.SelectMessage("call-explore-1"))

	cmd := ui.openSelectedChildSession()
	require.NotNil(t, cmd)

	msg, ok := cmd().(openChildSessionMsg)
	require.True(t, ok)
	require.Equal(t, firstChild.ID, msg.sessionID)
}

func TestOpenParentSessionSelectsOriginatingTool(t *testing.T) {
	t.Parallel()

	ui, _, generalChild, _, _, _ := testSessionUI(t)
	ui.session = generalChild

	cmd := ui.openParentSession()
	require.NotNil(t, cmd)

	msg, ok := cmd().(loadSessionMsg)
	require.True(t, ok)
	require.NotNil(t, msg.session)
	require.Equal(t, generalChild.ParentSessionID, msg.session.ID)
	require.Equal(t, "call-general", msg.selectedMessageID)
}

func TestCycleSiblingChildSessionStopsAtBounds(t *testing.T) {
	t.Parallel()

	ui, _, generalChild, _, _, fetchChild := testSessionUI(t)

	ui.session = generalChild
	cmd := ui.cycleSiblingChildSession(-1)
	require.NotNil(t, cmd)
	require.Nil(t, cmd())

	ui.session = fetchChild
	cmd = ui.cycleSiblingChildSession(1)
	require.NotNil(t, cmd)
	require.Nil(t, cmd())
}

func testSessionUI(t *testing.T) (*UI, *session.Session, *session.Session, *session.Session, *session.Session, *session.Session) {
	t.Helper()

	conn, err := db.Connect(context.Background(), t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)
	fileTracker := filetracker.NewService(q)
	historyService := history.NewService(q, conn)

	parent, err := sessions.Create(context.Background(), "Parent")
	require.NoError(t, err)

	assistantMsg, err := messages.Create(context.Background(), parent.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "call-general",
				Name:     agent.AgentToolName,
				Input:    `{"description":"Implement worker","prompt":"Do the work","subagent_type":"general"}`,
				Finished: true,
			},
			message.ToolCall{
				ID:       "call-explore-1",
				Name:     agent.AgentToolName,
				Input:    `{"description":"Search refs","prompt":"Search the repo","subagent_type":"explore"}`,
				Finished: true,
			},
			message.ToolCall{
				ID:       "call-explore-2",
				Name:     agent.AgentToolName,
				Input:    `{"description":"Search tests","prompt":"Search tests","subagent_type":"explore"}`,
				Finished: true,
			},
			message.ToolCall{
				ID:       "call-fetch",
				Name:     agenttools.AgenticFetchToolName,
				Input:    `{"prompt":"Research latest release notes"}`,
				Finished: true,
			},
			message.Finish{Reason: message.FinishReasonEndTurn},
		},
	})
	require.NoError(t, err)

	generalID := sessions.CreateAgentToolSessionID(assistantMsg.ID, "call-general")
	generalChild, err := sessions.CreateTaskSession(context.Background(), generalID, parent.ID, "Implement worker (@general subagent)")
	require.NoError(t, err)

	firstExploreID := sessions.CreateAgentToolSessionID(assistantMsg.ID, "call-explore-1")
	firstChild, err := sessions.CreateTaskSession(context.Background(), firstExploreID, parent.ID, "Search refs (@explore subagent)")
	require.NoError(t, err)

	secondExploreID := sessions.CreateAgentToolSessionID(assistantMsg.ID, "call-explore-2")
	secondChild, err := sessions.CreateTaskSession(context.Background(), secondExploreID, parent.ID, "Search tests (@explore subagent)")
	require.NoError(t, err)

	fetchID := sessions.CreateAgentToolSessionID(assistantMsg.ID, "call-fetch")
	fetchChild, err := sessions.CreateTaskSession(context.Background(), fetchID, parent.ID, "Fetch analysis")
	require.NoError(t, err)

	theme := styles.DefaultStyles()
	ui := &UI{
		com: &common.Common{
			App:    &app.App{Sessions: sessions, Messages: messages, History: historyService, FileTracker: fileTracker},
			Styles: &theme,
		},
		session: &generalChild,
	}
	ui.chat = NewChat(ui.com)

	return ui, &parent, &generalChild, &firstChild, &secondChild, &fetchChild
}
