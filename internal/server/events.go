package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/backend"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/skills"
)

// wrapEvent converts a raw tea.Msg (a pubsub.Event[T] from the app
// event fan-in) into a pubsub.Payload envelope with the correct
// PayloadType discriminator and a proto-typed inner payload that has
// proper JSON tags. Returns nil if the event type is unrecognized.
func wrapEvent(ev any) *pubsub.Payload {
	switch e := ev.(type) {
	case pubsub.Event[app.LSPEvent]:
		return envelope(pubsub.PayloadTypeLSPEvent, pubsub.Event[proto.LSPEvent]{
			Type: e.Type,
			Payload: proto.LSPEvent{
				Type:            proto.LSPEventType(e.Payload.Type),
				Name:            e.Payload.Name,
				State:           e.Payload.State,
				Error:           e.Payload.Error,
				DiagnosticCount: e.Payload.DiagnosticCount,
			},
		})
	case pubsub.Event[mcp.Event]:
		return envelope(pubsub.PayloadTypeMCPEvent, pubsub.Event[proto.MCPEvent]{
			Type: e.Type,
			Payload: proto.MCPEvent{
				Type:      mcpEventTypeToProto(e.Payload.Type),
				Name:      e.Payload.Name,
				State:     proto.MCPState(e.Payload.State),
				Error:     e.Payload.Error,
				ToolCount: e.Payload.Counts.Tools,
			},
		})
	case pubsub.Event[permission.PermissionRequest]:
		return envelope(pubsub.PayloadTypePermissionRequest, pubsub.Event[proto.PermissionRequest]{
			Type: e.Type,
			Payload: proto.PermissionRequest{
				ID:          e.Payload.ID,
				SessionID:   e.Payload.SessionID,
				ToolCallID:  e.Payload.ToolCallID,
				ToolName:    e.Payload.ToolName,
				Description: e.Payload.Description,
				Action:      e.Payload.Action,
				Path:        e.Payload.Path,
				Params:      e.Payload.Params,
			},
		})
	case pubsub.Event[permission.PermissionNotification]:
		return envelope(pubsub.PayloadTypePermissionNotification, pubsub.Event[proto.PermissionNotification]{
			Type: e.Type,
			Payload: proto.PermissionNotification{
				ToolCallID: e.Payload.ToolCallID,
				Granted:    e.Payload.Granted,
				Denied:     e.Payload.Denied,
			},
		})
	case pubsub.Event[question.Request]:
		slog.Info("Wrapping question batch event for SSE", "id", e.Payload.ID, "questions", len(e.Payload.Questions))
		return envelope(pubsub.PayloadTypeQuestionRequest, pubsub.Event[proto.QuestionRequest]{
			Type: e.Type,
			Payload: proto.QuestionRequest{
				ID:                 e.Payload.ID,
				SessionID:          e.Payload.SessionID,
				ToolCallID:         e.Payload.ToolCallID,
				Questions:          questionsToProto(e.Payload.Questions),
				ConfirmTitle:       e.Payload.ConfirmTitle,
				ConfirmDescription: e.Payload.ConfirmDescription,
			},
		})
	case pubsub.Event[question.Notification]:
		return envelope(pubsub.PayloadTypeQuestionNotification, pubsub.Event[proto.QuestionNotification]{
			Type: e.Type,
			Payload: proto.QuestionNotification{
				BatchID: e.Payload.BatchID,
			},
		})
	case pubsub.Event[message.Message]:
		return envelope(pubsub.PayloadTypeMessage, pubsub.Event[proto.Message]{
			Type:    e.Type,
			Payload: messageToProto(e.Payload),
		})
	case pubsub.Event[session.Session]:
		return envelope(pubsub.PayloadTypeSession, pubsub.Event[proto.Session]{
			Type:    e.Type,
			Payload: sessionToProto(e.Payload),
		})
	case pubsub.Event[history.File]:
		return envelope(pubsub.PayloadTypeFile, pubsub.Event[proto.File]{
			Type:    e.Type,
			Payload: fileToProto(e.Payload),
		})
	case pubsub.Event[notify.Notification]:
		payload := proto.AgentEvent{
			SessionID:    e.Payload.SessionID,
			SessionTitle: e.Payload.SessionTitle,
			RunID:        e.Payload.RunID,
			Type:         proto.AgentEventType(e.Payload.Type),
		}
		if e.Payload.Type == notify.TypeAgentError {
			payload.Type = proto.AgentEventTypeError
			payload.Error = errors.New(e.Payload.Message)
		}
		return envelope(pubsub.PayloadTypeAgentEvent, pubsub.Event[proto.AgentEvent]{
			Type:    e.Type,
			Payload: payload,
		})
	case pubsub.Event[notify.RunComplete]:
		return envelope(pubsub.PayloadTypeRunComplete, pubsub.Event[proto.RunComplete]{
			Type: e.Type,
			Payload: proto.RunComplete{
				SessionID: e.Payload.SessionID,
				RunID:     e.Payload.RunID,
				MessageID: e.Payload.MessageID,
				Text:      e.Payload.Text,
				Error:     e.Payload.Error,
				Cancelled: e.Payload.Cancelled,
			},
		})
	case pubsub.Event[proto.ConfigChanged]:
		return envelope(pubsub.PayloadTypeConfigChanged, e)
	case pubsub.Event[skills.Event]:
		return envelope(pubsub.PayloadTypeSkillsEvent, pubsub.Event[proto.SkillsEvent]{
			Type:    e.Type,
			Payload: skillsEventToProto(e.Payload),
		})
	default:
		slog.Warn("Unrecognized event type for SSE wrapping", "type", fmt.Sprintf("%T", ev))
		return nil
	}
}

// envelope marshals the inner event and wraps it in a pubsub.Payload.
func envelope(payloadType pubsub.PayloadType, inner any) *pubsub.Payload {
	raw, err := json.Marshal(inner)
	if err != nil {
		slog.Error("Failed to marshal event payload", "error", err)
		return nil
	}
	return &pubsub.Payload{
		Type:    payloadType,
		Payload: raw,
	}
}

func mcpEventTypeToProto(t mcp.EventType) proto.MCPEventType {
	switch t {
	case mcp.EventStateChanged:
		return proto.MCPEventStateChanged
	case mcp.EventToolsListChanged:
		return proto.MCPEventToolsListChanged
	case mcp.EventPromptsListChanged:
		return proto.MCPEventPromptsListChanged
	case mcp.EventResourcesListChanged:
		return proto.MCPEventResourcesListChanged
	default:
		return proto.MCPEventStateChanged
	}
}

func sessionToProto(s session.Session) proto.Session {
	return proto.Session{
		ID:               s.ID,
		ParentSessionID:  s.ParentSessionID,
		Title:            s.Title,
		SummaryMessageID: s.SummaryMessageID,
		MessageCount:     s.MessageCount,
		PromptTokens:     s.PromptTokens,
		CompletionTokens: s.CompletionTokens,
		Cost:             s.Cost,
		Todos:            todosToProto(s.Todos),
		CreatedAt:        s.CreatedAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

// isSessionBusy reports whether the given workspace has an in-flight
// agent run for sessionID. It tolerates a nil workspace (treating it as
// "not busy") so REST handlers can pass GetWorkspace's result through
// unconditionally — the workspace lookup error is already surfaced by
// the prior ListSessions/GetSession call when relevant.
func isSessionBusy(ws *backend.Workspace, sessionID string) bool {
	if ws == nil || ws.App == nil || ws.AgentCoordinator == nil {
		return false
	}
	return ws.AgentCoordinator.IsSessionBusy(sessionID)
}

// attachedClients returns the number of clients currently viewing
// sessionID in ws. Hold-only clients (streams == 0) do not contribute.
// A nil workspace is treated as zero so handlers can pass GetWorkspace's
// result through without an extra guard.
func attachedClients(ws *backend.Workspace, sessionID string) int {
	if ws == nil {
		return 0
	}
	return ws.AttachedClientsForSession(sessionID)
}

func todosToProto(todos []session.Todo) []proto.Todo {
	if len(todos) == 0 {
		return nil
	}
	out := make([]proto.Todo, len(todos))
	for i, t := range todos {
		out[i] = proto.Todo{
			Content:    t.Content,
			Status:     string(t.Status),
			ActiveForm: t.ActiveForm,
		}
	}
	return out
}

func fileToProto(f history.File) proto.File {
	return proto.File{
		ID:        f.ID,
		SessionID: f.SessionID,
		Path:      f.Path,
		Content:   f.Content,
		Version:   f.Version,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
	}
}

func messageToProto(m message.Message) proto.Message {
	msg := proto.Message{
		ID:        m.ID,
		SessionID: m.SessionID,
		Role:      proto.MessageRole(m.Role),
		Model:     m.Model,
		Provider:  m.Provider,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}

	for _, p := range m.Parts {
		switch v := p.(type) {
		case message.TextContent:
			msg.Parts = append(msg.Parts, proto.TextContent{Text: v.Text})
		case message.ReasoningContent:
			msg.Parts = append(msg.Parts, proto.ReasoningContent{
				Thinking:   v.Thinking,
				Signature:  v.Signature,
				StartedAt:  v.StartedAt,
				FinishedAt: v.FinishedAt,
			})
		case message.ToolCall:
			msg.Parts = append(msg.Parts, proto.ToolCall{
				ID:       v.ID,
				Name:     v.Name,
				Input:    v.Input,
				Finished: v.Finished,
			})
		case message.ToolResult:
			msg.Parts = append(msg.Parts, proto.ToolResult{
				ToolCallID: v.ToolCallID,
				Name:       v.Name,
				Content:    v.Content,
				Data:       v.Data,
				MIMEType:   v.MIMEType,
				Metadata:   v.Metadata,
				IsError:    v.IsError,
			})
		case message.Finish:
			msg.Parts = append(msg.Parts, proto.Finish{
				Reason:  proto.FinishReason(v.Reason),
				Time:    v.Time,
				Message: v.Message,
				Details: v.Details,
			})
		case message.ImageURLContent:
			msg.Parts = append(msg.Parts, proto.ImageURLContent{URL: v.URL, Detail: v.Detail})
		case message.BinaryContent:
			msg.Parts = append(msg.Parts, proto.BinaryContent{Path: v.Path, MIMEType: v.MIMEType, Data: v.Data})
		}
	}

	return msg
}

// skillsEventToProto converts a skills.Event into its wire form. Errors
// are flattened to strings because error does not round-trip over JSON.
func skillsEventToProto(e skills.Event) proto.SkillsEvent {
	if len(e.States) == 0 {
		return proto.SkillsEvent{}
	}
	out := proto.SkillsEvent{States: make([]proto.SkillState, len(e.States))}
	for i, s := range e.States {
		entry := proto.SkillState{
			Name:  s.Name,
			Path:  s.Path,
			State: proto.SkillDiscoveryState(s.State),
		}
		if s.Err != nil {
			entry.Error = s.Err.Error()
		}
		out.States[i] = entry
	}
	return out
}

func messagesToProto(msgs []message.Message) []proto.Message {
	out := make([]proto.Message, len(msgs))
	for i, m := range msgs {
		out[i] = messageToProto(m)
	}
	return out
}

func questionsToProto(qs []question.Question) []proto.QuestionItem {
	if len(qs) == 0 {
		return nil
	}
	out := make([]proto.QuestionItem, len(qs))
	for i, q := range qs {
		choices := make([]proto.QuestionChoice, len(q.Choices))
		for j, c := range q.Choices {
			choices[j] = proto.QuestionChoice{
				ID:          c.ID,
				Label:       c.Label,
				Description: c.Description,
			}
		}
		out[i] = proto.QuestionItem{
			ID:          q.ID,
			Type:        string(q.Type),
			Label:       q.Label,
			Question:    q.Text,
			Description: q.Description,
			Choices:     choices,
		}
	}
	return out
}
