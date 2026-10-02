package client

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

func TestSendMessagePermissionPolicyIntegration(t *testing.T) {
	t.Setenv("CRUSH_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("CRUSH_GLOBAL_DATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "proof.txt"), []byte("test"), 0o600))
	arguments, err := json.Marshal(struct {
		Path string `json:"path"`
	}{outside})
	require.NoError(t, err)
	encodedArguments, err := json.Marshal(string(arguments))
	require.NoError(t, err)
	var requests atomic.Int32
	var toolSent atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		n := requests.Add(1)
		if !toolSent.Swap(true) {
			fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"tool-%d\",\"type\":\"function\",\"function\":{\"name\":\"ls\",\"arguments\":%s}}]},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", n, encodedArguments)
		} else {
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"done\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		}
	}))
	defer provider.Close()
	workingDir := t.TempDir()
	cfg, err := config.Init(workingDir, "", false)
	require.NoError(t, err)
	cfg.Config().Providers.Set("test", config.ProviderConfig{
		ID: "test", Name: "Test", Type: openaicompat.Name, BaseURL: provider.URL + "/v1", APIKey: "test",
		Models: []catwalk.Model{{ID: "test-model", DefaultMaxTokens: 4096}},
	})
	selected := config.SelectedModel{Provider: "test", Model: "test-model"}
	cfg.OverridePreferredModel(config.SelectedModelTypeLarge, selected)
	cfg.OverridePreferredModel(config.SelectedModelTypeSmall, selected)
	cfg.SetupAgents()
	coder := cfg.Config().Agents[config.AgentCoder]
	coder.AllowedTools = []string{"ls"}
	cfg.Config().Agents[config.AgentCoder] = coder
	conn, err := db.Connect(ctx, t.TempDir())
	require.NoError(t, err)
	defer conn.Close()
	queries := db.New(conn)
	sessions := session.NewService(queries, conn)
	messages := message.NewService(queries)
	sess, err := sessions.Create(ctx, "session")
	require.NoError(t, err)
	_, err = messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User, Parts: []message.ContentPart{message.TextContent{Text: "existing"}},
	})
	require.NoError(t, err)
	a := app.NewForTest(ctx)
	defer a.ShutdownForTest()
	coord, err := agent.NewCoordinator(ctx, agent.CoordinatorOptions{
		Config: cfg, Sessions: sessions, Messages: messages, Permissions: a.Permissions,
		Notify: a.AgentNotifications(), RunComplete: a.RunCompletions(), Skills: skills.NewManager(nil, nil, nil),
	})
	require.NoError(t, err)
	a.AgentCoordinator = coord
	s := server.NewServer(nil, "tcp", "")
	ws := &backend.Workspace{ID: "workspace", Path: workingDir, App: a}
	backend.InsertWorkspaceForTest(s.Backend(), ws)
	backend.SetWorkspaceShutdownFnForTest(ws, func() {})
	httpServer := httptest.NewServer(s.Handler())
	defer httpServer.Close()
	c := captureClient(t, httpServer)
	info, err := c.VersionInfo(ctx)
	require.NoError(t, err)
	require.True(t, info.SupportsPerTurnPermissionPolicy)
	prompts := a.Permissions.Subscribe(ctx)
	decisions := a.Permissions.SubscribeNotifications(ctx)
	completions := a.RunCompletions().Subscribe(ctx)
	for index, policy := range []proto.PermissionRequestPolicy{
		proto.PermissionRequestPolicyPrompt, proto.PermissionRequestPolicyAutoApprove, proto.PermissionRequestPolicyPrompt,
	} {
		runID := fmt.Sprintf("run-%d", index)
		toolSent.Store(false)
		require.NoError(t, c.SendMessage(ctx, ws.ID, sess.ID, runID, "", "list outside", policy))
		prompted := 0
	wait:
		for {
			select {
			case request := <-prompts:
				prompted++
				require.Equal(t, outside, request.Payload.Path)
				a.Permissions.Deny(request.Payload)
			case completed := <-completions:
				require.Equal(t, runID, completed.Payload.RunID)
				require.Empty(t, completed.Payload.Error)
				if policy == proto.PermissionRequestPolicyAutoApprove {
					require.Equal(t, "done", completed.Payload.Text)
				} else {
					require.Empty(t, completed.Payload.Text, "permission denial stops before another model step")
				}
				break wait
			case <-ctx.Done():
				t.Fatal("permission turn did not finish")
			}
		}
		if policy == proto.PermissionRequestPolicyAutoApprove {
			require.Zero(t, prompted)
			require.Len(t, decisions, 1)
			require.True(t, (<-decisions).Payload.Granted)
		} else {
			require.Equal(t, 1, prompted, "a previous automatic approval must not grant this turn")
			for len(decisions) > 0 {
				require.False(t, (<-decisions).Payload.Granted)
			}
		}
	}
	require.EqualValues(t, 4, requests.Load())
}
