package acp

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func TestAuthenticate_RejectsUnknownMethod(t *testing.T) {
	agent := NewAgent(nil)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: "unknown"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported auth method")
}

func TestNewSession_RequiresAuthentication(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/tmp"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Authentication required")
}

func TestAuthenticate_AllowsNewSession(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: authMethodLocal})
	require.NoError(t, err)

	resp, err := agent.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/tmp"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.SessionId)
	require.NotNil(t, resp.Modes)
	require.Equal(t, modeAsk, resp.Modes.CurrentModeId)
	require.Len(t, resp.Modes.AvailableModes, 2)
}

func TestNewSession_RequiresAbsoluteCwd(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: authMethodLocal})
	require.NoError(t, err)

	_, err = agent.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "relative/path"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cwd must be an absolute path")
}

func TestLoadSession_RequiresAuthentication(t *testing.T) {
	agent, application := setupAgentTestEnv(t)
	sess, err := application.Sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = agent.LoadSession(t.Context(), acp.LoadSessionRequest{SessionId: acp.SessionId(sess.ID), Cwd: "/tmp"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Authentication required")
}

func TestLoadSession_RequiresAbsoluteCwd(t *testing.T) {
	agent, application := setupAgentTestEnv(t)
	sess, err := application.Sessions.Create(t.Context(), "session")
	require.NoError(t, err)

	_, err = agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: authMethodLocal})
	require.NoError(t, err)

	_, err = agent.LoadSession(t.Context(), acp.LoadSessionRequest{SessionId: acp.SessionId(sess.ID), Cwd: "relative"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cwd must be an absolute path")
}

func TestSetSessionMode_ValidatesAndSwitches(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: authMethodLocal})
	require.NoError(t, err)

	sessResp, err := agent.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/tmp"})
	require.NoError(t, err)

	_, err = agent.SetSessionMode(t.Context(), acp.SetSessionModeRequest{
		SessionId: sessResp.SessionId,
		ModeId:    modeCode,
	})
	require.NoError(t, err)

	modeState := agent.buildSessionModeState(string(sessResp.SessionId))
	require.Equal(t, modeCode, modeState.CurrentModeId)
}

func TestSetSessionMode_RejectsUnsupportedMode(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: authMethodLocal})
	require.NoError(t, err)

	sessResp, err := agent.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/tmp"})
	require.NoError(t, err)

	_, err = agent.SetSessionMode(t.Context(), acp.SetSessionModeRequest{
		SessionId: sessResp.SessionId,
		ModeId:    "architect",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported mode")
}

func TestPrompt_RequiresAuthentication(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.Prompt(t.Context(), acp.PromptRequest{
		SessionId: "fake",
		Prompt:    []acp.ContentBlock{acp.TextBlock("hello")},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Authentication required")
}

func TestPrompt_RequiresValidSession(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: authMethodLocal})
	require.NoError(t, err)

	_, err = agent.Prompt(t.Context(), acp.PromptRequest{
		SessionId: "nonexistent",
		Prompt:    []acp.ContentBlock{acp.TextBlock("hello")},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown session")
}

func TestSetSessionModel_RequiresAuthentication(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.SetSessionModel(t.Context(), acp.SetSessionModelRequest{
		SessionId: "fake",
		ModelId:   "provider:model",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Authentication required")
}

func TestSetSessionModel_RequiresValidSession(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: authMethodLocal})
	require.NoError(t, err)

	_, err = agent.SetSessionModel(t.Context(), acp.SetSessionModelRequest{
		SessionId: "nonexistent",
		ModelId:   "provider:model",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown session")
}

func TestCancel_RequiresAuthentication(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	err := agent.Cancel(t.Context(), acp.CancelNotification{SessionId: "fake"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Authentication required")
}

func TestInitialize_ReturnsAgentInfo(t *testing.T) {
	agent := NewAgent(nil)

	resp, err := agent.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
	})
	require.NoError(t, err)
	require.Equal(t, acp.ProtocolVersion(acp.ProtocolVersionNumber), resp.ProtocolVersion)
	require.NotNil(t, resp.AgentInfo)
	require.Equal(t, "crush", resp.AgentInfo.Name)
	require.NotEmpty(t, resp.AgentInfo.Version)
	require.True(t, resp.AgentCapabilities.LoadSession)
	require.NotEmpty(t, resp.AuthMethods)
}

func TestInitialize_ResetsAuthState(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: authMethodLocal})
	require.NoError(t, err)

	_, err = agent.Initialize(t.Context(), acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
	})
	require.NoError(t, err)

	_, err = agent.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/tmp"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Authentication required")
}

func TestAuthenticate_RejectsEmptyMethodId(t *testing.T) {
	agent := NewAgent(nil)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: ""})
	require.Error(t, err)
	require.Contains(t, err.Error(), "methodId is required")
}

func TestShutdown_StopsSinks(t *testing.T) {
	agent, _ := setupAgentTestEnv(t)

	_, err := agent.Authenticate(t.Context(), acp.AuthenticateRequest{MethodId: authMethodLocal})
	require.NoError(t, err)

	_, err = agent.NewSession(t.Context(), acp.NewSessionRequest{Cwd: "/tmp"})
	require.NoError(t, err)

	require.Equal(t, 1, agent.sinks.Len())
	agent.Shutdown()
}

func setupAgentTestEnv(t *testing.T) (*Agent, *app.App) {
	t.Helper()

	workingDir := t.TempDir()
	dataDir := t.TempDir()

	cfgStore, err := config.Init(workingDir, dataDir, false)
	require.NoError(t, err)

	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	application, err := app.New(t.Context(), conn, cfgStore)
	require.NoError(t, err)
	t.Cleanup(application.Shutdown)

	agent := NewAgent(application)
	agent.SetAgentConnection(acp.NewAgentSideConnection(agent, io.Discard, strings.NewReader("")))
	return agent, application
}
