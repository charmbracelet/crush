package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/honcho"
)

//go:embed honcho_status.md
var honchoStatusDescription string

type HonchoStatusParams struct{}

type HonchoStatusResponseMetadata struct {
	Workspace       string `json:"workspace"`
	Session         string `json:"session"`
	UserPeer        string `json:"user_peer"`
	AgentPeer       string `json:"agent_peer"`
	ObservationMode string `json:"observation_mode"`
	SessionStrategy string `json:"session_strategy"`
	RecallMode      string `json:"recall_mode"`
	BaseURL         string `json:"base_url"`
	QueuePending    int    `json:"queue_pending"`
	QueueInProgress int    `json:"queue_in_progress"`
	QueueEmpty      bool   `json:"queue_empty"`
}

// NewHonchoStatusTool builds the diagnostic tool. It answers the
// question a user actually asks when memory disappoints them: which
// workspace and session was this written to, and has the backend
// finished reasoning over it yet.
//
// It never reports the API key. A status dump lands in the transcript,
// and a transcript is not a secret store.
func NewHonchoStatusTool(svc *honcho.Service) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		HonchoStatusToolName,
		honchoStatusDescription,
		func(ctx context.Context, _ HonchoStatusParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if svc == nil {
				return memoryNotConfigured(), nil
			}

			cfg := svc.Config()
			id := svc.Identity()

			baseURL := svc.Client().BaseURL()
			if baseURL == "" {
				baseURL = honcho.DefaultBaseURL
			}

			metadata := HonchoStatusResponseMetadata{
				Workspace:       id.Workspace,
				Session:         id.SessionKey,
				UserPeer:        id.UserPeer,
				AgentPeer:       id.AgentPeer,
				ObservationMode: string(cfg.ObservationMode),
				SessionStrategy: string(cfg.SessionStrategy),
				RecallMode:      string(cfg.RecallMode),
				BaseURL:         baseURL,
			}

			var out strings.Builder
			out.WriteString("Memory is configured.\n\n")
			fmt.Fprintf(&out, "Deployment:       %s\n", baseURL)
			fmt.Fprintf(&out, "Workspace:        %s\n", id.Workspace)
			fmt.Fprintf(&out, "Session key:      %s\n", id.SessionKey)
			fmt.Fprintf(&out, "User peer:        %s\n", id.UserPeer)
			fmt.Fprintf(&out, "Agent peer:       %s\n", id.AgentPeer)
			fmt.Fprintf(&out, "Observation mode: %s\n", cfg.ObservationMode)
			fmt.Fprintf(&out, "Session strategy: %s\n", cfg.SessionStrategy)
			fmt.Fprintf(&out, "Recall mode:      %s\n", cfg.RecallMode)
			fmt.Fprintf(&out, "Observing agent:  %t\n", cfg.AgentObserveMe)
			fmt.Fprintf(&out, "Capturing tools:  %t\n", cfg.CaptureTools)

			// A queue lookup is the only network call here, so a
			// failure degrades the report rather than replacing it:
			// the identifiers above are exactly what the user needs
			// when the backend is unreachable.
			status, err := svc.Client().QueueStatus(ctx, id.SessionKey)
			switch {
			case err != nil:
				fmt.Fprintf(&out, "\nReasoning queue:  unavailable (%v)\n", err)
			case status.Empty || status.Pending+status.InProgress == 0:
				out.WriteString("\nReasoning queue:  idle. Everything written so far has been processed.\n")
				metadata.QueueEmpty = true
			default:
				metadata.QueuePending = status.Pending
				metadata.QueueInProgress = status.InProgress
				fmt.Fprintf(&out,
					"\nReasoning queue:  %d pending, %d in progress, %d of %d complete.\n"+
						"Recent turns have not yet influenced what memory knows.\n",
					status.Pending, status.InProgress, status.Completed, status.Total)
			}

			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(out.String()), metadata), nil
		},
	)
}
