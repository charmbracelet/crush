package acp

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/app"
	acp "github.com/coder/acp-go-sdk"
)

// crushAgent implements acp.Agent using Crush's internal services.
type crushAgent struct {
	app *app.App
	log *slog.Logger
}

// newCrushAgent creates an ACP agent backed by the given Crush app.
func newCrushAgent(a *app.App, log *slog.Logger) Agent {
	return &crushAgent{app: a, log: log}
}

// Initialize implements acp.Agent.
func (a *crushAgent) Initialize(_ context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error) {
	resp := acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentInfo:       &acp.Implementation{Name: "Crush", Version: "dev"},
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: true,
			PromptCapabilities: acp.PromptCapabilities{
				Image: true,
				Audio: true,
				EmbeddedContext: true,
			},
			SessionCapabilities: acp.SessionCapabilities{
				Close: &acp.SessionCloseCapabilities{},
			},
			McpCapabilities: acp.McpCapabilities{},
		},
	}
	a.log.Info("ACP initialize", "protocol", resp.ProtocolVersion)
	return resp, nil
}

// NewSession implements acp.Agent.
func (a *crushAgent) NewSession(_ context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	title := "ACP Session"

	// Crush resolves its working directory once at startup, so it cannot move
	// on a per-session basis. A client asking for a different directory would
	// otherwise get a session silently operating on the wrong tree, so surface
	// the disagreement instead of dropping it.
	if req.Cwd != "" {
		if local := a.workingDir(); local != "" && !samePath(local, req.Cwd) {
			return acp.NewSessionResponse{}, fmt.Errorf(
				"session cwd %q does not match crush working directory %q; "+
					"start crush with -c %s to use that workspace",
				req.Cwd, local, req.Cwd)
		}
	}

	session, err := a.app.Sessions.Create(context.Background(), title)
	if err != nil {
		return acp.NewSessionResponse{}, fmt.Errorf("create session: %w", err)
	}
	a.log.Info("ACP new session", "sessionId", session.ID, "cwd", req.Cwd)
	return acp.NewSessionResponse{
		SessionId: acp.SessionId(session.ID),
	}, nil
}

// workingDir reports the directory Crush was started in, or "" when the app
// carries no config store (as in tests).
func (a *crushAgent) workingDir() string {
	if a.app == nil {
		return ""
	}
	store := a.app.Store()
	if store == nil {
		return ""
	}
	// A typed-nil *ConfigStore passes the comparison above, so guard the
	// dereference too: tests construct an App without a store.
	defer func() { _ = recover() }()
	return store.WorkingDir()
}

// samePath compares two filesystem paths, tolerating a trailing separator and
// relative forms.
func samePath(a, b string) bool {
	cleanA, errA := filepath.Abs(filepath.Clean(a))
	cleanB, errB := filepath.Abs(filepath.Clean(b))
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return cleanA == cleanB
}

// ListSessions implements acp.Agent.
func (a *crushAgent) ListSessions(_ context.Context, _ acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	sessions, err := a.app.Sessions.List(context.Background())
	if err != nil {
		return acp.ListSessionsResponse{}, fmt.Errorf("list sessions: %w", err)
	}
	out := make([]acp.SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		t := s.Title
		out = append(out, acp.SessionInfo{
			SessionId: acp.SessionId(s.ID),
			Title:     &t,
		})
	}
	return acp.ListSessionsResponse{Sessions: out}, nil
}

// CloseSession implements acp.Agent.
func (a *crushAgent) CloseSession(_ context.Context, req acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	id := string(req.SessionId)
	a.app.AgentCoordinator.Cancel(id)
	return acp.CloseSessionResponse{}, nil
}

// ResumeSession is not supported.
func (a *crushAgent) ResumeSession(_ context.Context, _ acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, fmt.Errorf("session/resume is not supported")
}

// Cancel implements acp.Agent.
func (a *crushAgent) Cancel(_ context.Context, req acp.CancelNotification) error {
	id := string(req.SessionId)
	a.log.Info("ACP cancel", "sessionId", id)
	if a.app.AgentCoordinator != nil {
		a.app.AgentCoordinator.Cancel(id)
	}
	return nil
}

// Prompt implements acp.Agent.
func (a *crushAgent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	sessionID := string(req.SessionId)

	// Build prompt text from content blocks.
	prompt := extractPromptText(req.Prompt)
	a.log.Info("ACP prompt", "sessionId", sessionID, "promptLen", len(prompt))

	result, err := a.app.AgentCoordinator.Run(ctx, sessionID, prompt)
	if err != nil {
		if ctx.Err() != nil {
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
		}
		return acp.PromptResponse{}, fmt.Errorf("run prompt: %w", err)
	}

	stopReason := mapFinishReason(result.Response.FinishReason)
	a.log.Info("ACP prompt done", "sessionId", sessionID, "stopReason", stopReason)
	return acp.PromptResponse{StopReason: stopReason}, nil
}

// SetSessionMode acknowledges a mode change.
//
// Crush has no mode concept, so there is nothing to change, but the method must
// still succeed: editors send it during session setup and an error can abort
// the session before the first prompt.
func (a *crushAgent) SetSessionMode(_ context.Context, req acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	a.log.Info("ACP set session mode", "sessionId", req.SessionId, "modeId", req.ModeId)
	return acp.SetSessionModeResponse{}, nil
}

// SetSessionConfigOption acknowledges a configuration change and reports the
// options Crush supports.
//
// The response type requires a non-nil configOptions list, so an empty slice is
// returned rather than nil.
func (a *crushAgent) SetSessionConfigOption(_ context.Context, req acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	var sessionID string
	if req.ValueId != nil {
		sessionID = string(req.ValueId.SessionId)
	} else if req.Boolean != nil {
		sessionID = string(req.Boolean.SessionId)
	}
	a.log.Info("ACP set session config option", "sessionId", sessionID)
	return acp.SetSessionConfigOptionResponse{ConfigOptions: []acp.SessionConfigOption{}}, nil
}

// extractPromptText converts ACP content blocks to a plain-text prompt.
func extractPromptText(blocks []acp.ContentBlock) string {
	var sb string
	for _, b := range blocks {
		if b.Text != nil {
			sb += b.Text.Text
		}
		if b.ResourceLink != nil {
			sb += "[resource: " + b.ResourceLink.Name + ": " + b.ResourceLink.Uri + "]"
		}
	}
	return sb
}

// mapFinishReason converts fantasy finish reasons to ACP stop reasons.
func mapFinishReason(reason fantasy.FinishReason) acp.StopReason {
	switch reason {
	case fantasy.FinishReasonStop:
		return acp.StopReasonEndTurn
	case fantasy.FinishReasonLength:
		return acp.StopReasonMaxTokens
	case fantasy.FinishReasonContentFilter:
		return acp.StopReasonRefusal
	default:
		return acp.StopReasonEndTurn
	}
}
