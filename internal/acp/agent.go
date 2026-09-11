package acp

import (
	"context"
	"fmt"
	"log/slog"

	acp "github.com/coder/acp-go-sdk"
	"github.com/charmbracelet/crush/internal/app"
	"charm.land/fantasy"
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
			LoadSession: false,
			PromptCapabilities: acp.PromptCapabilities{
				Image:           false,
				Audio:           false,
				EmbeddedContext: false,
			},
			McpCapabilities: acp.McpCapabilities{
				Http: false,
				Sse:  false,
			},
			SessionCapabilities: acp.SessionCapabilities{},
		},
	}
	a.log.Info("ACP initialize", "protocol", resp.ProtocolVersion)
	return resp, nil
}

// NewSession implements acp.Agent.
func (a *crushAgent) NewSession(_ context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	title := "ACP Session"
	session, err := a.app.Sessions.Create(context.Background(), title)
	if err != nil {
		return acp.NewSessionResponse{}, fmt.Errorf("create session: %w", err)
	}
	a.log.Info("ACP new session", "sessionId", session.ID, "cwd", req.Cwd)
	return acp.NewSessionResponse{
		SessionId: acp.SessionId(session.ID),
	}, nil
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

// SetSessionMode is not yet wired.
func (a *crushAgent) SetSessionMode(_ context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, fmt.Errorf("session/set_mode is not yet implemented")
}

// SetSessionConfigOption is not yet wired.
func (a *crushAgent) SetSessionConfigOption(_ context.Context, _ acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, fmt.Errorf("session/set_config_option is not yet implemented")
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
