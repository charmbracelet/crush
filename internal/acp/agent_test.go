package acp

import (
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
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

func TestExtractPromptContent_TextOnly(t *testing.T) {
	t.Parallel()
	blocks := []acp.ContentBlock{
		acp.TextBlock("hello "),
		acp.TextBlock("world"),
	}
	prompt, attachments := extractPromptContent(blocks)
	require.Equal(t, "hello world", prompt)
	require.Empty(t, attachments)
}

func TestExtractPromptContent_EmbeddedTextResource(t *testing.T) {
	t.Parallel()
	mimeType := "text/go"
	blocks := []acp.ContentBlock{
		acp.TextBlock("review this"),
		acp.ResourceBlock(acp.EmbeddedResourceResource{
			TextResourceContents: &acp.TextResourceContents{
				Uri:      "file:///home/user/main.go",
				Text:     "package main\n",
				MimeType: &mimeType,
			},
		}),
	}
	prompt, attachments := extractPromptContent(blocks)
	require.Equal(t, "review this", prompt)
	require.Len(t, attachments, 1)
	require.Equal(t, "/home/user/main.go", attachments[0].FilePath)
	require.Equal(t, "main.go", attachments[0].FileName)
	require.Equal(t, "text/go", attachments[0].MimeType)
	require.Equal(t, "package main\n", string(attachments[0].Content))
}

func TestExtractPromptContent_EmbeddedBlobResource(t *testing.T) {
	t.Parallel()
	imageData := []byte{0x89, 0x50, 0x4e, 0x47}
	encoded := base64.StdEncoding.EncodeToString(imageData)
	mimeType := "image/png"
	blocks := []acp.ContentBlock{
		acp.TextBlock("what is this?"),
		acp.ResourceBlock(acp.EmbeddedResourceResource{
			BlobResourceContents: &acp.BlobResourceContents{
				Uri:      "file:///tmp/screenshot.png",
				Blob:     encoded,
				MimeType: &mimeType,
			},
		}),
	}
	prompt, attachments := extractPromptContent(blocks)
	require.Equal(t, "what is this?", prompt)
	require.Len(t, attachments, 1)
	require.Equal(t, "/tmp/screenshot.png", attachments[0].FilePath)
	require.Equal(t, "screenshot.png", attachments[0].FileName)
	require.Equal(t, "image/png", attachments[0].MimeType)
	require.Equal(t, imageData, attachments[0].Content)
}

func TestExtractPromptContent_ResourceLink(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "data.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("file contents"), 0o644))

	mimeType := "text/plain"
	blocks := []acp.ContentBlock{
		acp.TextBlock("read this file"),
		{ResourceLink: &acp.ContentBlockResourceLink{
			Name:     "data.txt",
			Uri:      "file://" + filePath,
			MimeType: &mimeType,
		}},
	}
	prompt, attachments := extractPromptContent(blocks)
	require.Equal(t, "read this file", prompt)
	require.Len(t, attachments, 1)
	require.Equal(t, filePath, attachments[0].FilePath)
	require.Equal(t, "data.txt", attachments[0].FileName)
	require.Equal(t, "text/plain", attachments[0].MimeType)
	require.Equal(t, "file contents", string(attachments[0].Content))
}

func TestExtractPromptContent_ResourceLinkNonFileURI(t *testing.T) {
	t.Parallel()
	blocks := []acp.ContentBlock{
		acp.TextBlock("check this"),
		{ResourceLink: &acp.ContentBlockResourceLink{
			Name: "remote",
			Uri:  "https://example.com/file.txt",
		}},
	}
	prompt, attachments := extractPromptContent(blocks)
	require.Equal(t, "check this", prompt)
	require.Empty(t, attachments)
}

func TestExtractPromptContent_ImageBlock(t *testing.T) {
	t.Parallel()
	imageData := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	encoded := base64.StdEncoding.EncodeToString(imageData)
	uri := "file:///tmp/photo.jpg"
	blocks := []acp.ContentBlock{
		acp.TextBlock("describe this"),
		{Image: &acp.ContentBlockImage{
			Data:     encoded,
			MimeType: "image/jpeg",
			Uri:      &uri,
		}},
	}
	prompt, attachments := extractPromptContent(blocks)
	require.Equal(t, "describe this", prompt)
	require.Len(t, attachments, 1)
	require.Equal(t, "photo.jpg", attachments[0].FileName)
	require.Equal(t, "image/jpeg", attachments[0].MimeType)
	require.Equal(t, imageData, attachments[0].Content)
}

func TestExtractPromptContent_MixedBlocks(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(filePath, []byte("key: value"), 0o644))

	textMime := "text/yaml"
	imgData := []byte{0x89, 0x50}
	imgEncoded := base64.StdEncoding.EncodeToString(imgData)
	goMime := "text/go"

	blocks := []acp.ContentBlock{
		acp.TextBlock("analyze "),
		acp.TextBlock("these files"),
		acp.ResourceBlock(acp.EmbeddedResourceResource{
			TextResourceContents: &acp.TextResourceContents{
				Uri:      "file:///src/main.go",
				Text:     "package main",
				MimeType: &goMime,
			},
		}),
		{ResourceLink: &acp.ContentBlockResourceLink{
			Name:     "config.yaml",
			Uri:      "file://" + filePath,
			MimeType: &textMime,
		}},
		{Image: &acp.ContentBlockImage{
			Data:     imgEncoded,
			MimeType: "image/png",
		}},
	}

	prompt, attachments := extractPromptContent(blocks)
	require.Equal(t, "analyze these files", prompt)
	require.Len(t, attachments, 3)

	require.Equal(t, "text/go", attachments[0].MimeType)
	require.Equal(t, "package main", string(attachments[0].Content))

	require.Equal(t, "config.yaml", attachments[1].FileName)
	require.Equal(t, "key: value", string(attachments[1].Content))

	require.Equal(t, "image/png", attachments[2].MimeType)
	require.Equal(t, imgData, attachments[2].Content)
}

func TestExtractPromptContent_DefaultMimeTypes(t *testing.T) {
	t.Parallel()
	blocks := []acp.ContentBlock{
		acp.ResourceBlock(acp.EmbeddedResourceResource{
			TextResourceContents: &acp.TextResourceContents{
				Uri:  "file:///tmp/readme",
				Text: "hello",
			},
		}),
		acp.ResourceBlock(acp.EmbeddedResourceResource{
			BlobResourceContents: &acp.BlobResourceContents{
				Uri:  "file:///tmp/data.bin",
				Blob: base64.StdEncoding.EncodeToString([]byte{0x00}),
			},
		}),
	}
	_, attachments := extractPromptContent(blocks)
	require.Len(t, attachments, 2)
	require.Equal(t, "text/plain", attachments[0].MimeType)
	require.Equal(t, "application/octet-stream", attachments[1].MimeType)
}

func TestURIToPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		uri  string
		want string
	}{
		{"file:///home/user/file.go", "/home/user/file.go"},
		{"file:///tmp/test", "/tmp/test"},
		{"/absolute/path", "/absolute/path"},
		{"https://example.com/file", ""},
		{"relative/path", ""},
		{"", ""},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, uriToPath(tt.uri), "uriToPath(%q)", tt.uri)
	}
}

func TestFilenameFromURI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		uri  string
		want string
	}{
		{"file:///home/user/main.go", "main.go"},
		{"https://example.com/path/to/file.txt", "file.txt"},
		{"/just/a/path.rs", "path.rs"},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, filenameFromURI(tt.uri), "filenameFromURI(%q)", tt.uri)
	}
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
