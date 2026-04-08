package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/crushcl/internal/config"
	"github.com/charmbracelet/crushcl/internal/message"
	"github.com/charmbracelet/crushcl/internal/proto"
	"github.com/charmbracelet/crushcl/internal/pubsub"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// ListWorkspaces retrieves all workspaces from the server.
func (c *Client) ListWorkspaces(ctx context.Context) ([]proto.Workspace, error) {
	rsp, err := c.get(ctx, "/workspaces", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list workspaces: status code %d", rsp.StatusCode)
	}
	var workspaces []proto.Workspace
	if err := json.NewDecoder(rsp.Body).Decode(&workspaces); err != nil {
		return nil, fmt.Errorf("failed to decode workspaces: %w", err)
	}
	return workspaces, nil
}

// CreateWorkspace creates a new workspace on the server.
func (c *Client) CreateWorkspace(ctx context.Context, ws proto.Workspace) (*proto.Workspace, error) {
	rsp, err := c.post(ctx, "/workspaces", nil, jsonBody(ws), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create workspace: status code %d", rsp.StatusCode)
	}
	var created proto.Workspace
	if err := json.NewDecoder(rsp.Body).Decode(&created); err != nil {
		return nil, fmt.Errorf("failed to decode workspace: %w", err)
	}
	return &created, nil
}

// GetWorkspace retrieves a workspace from the server.
func (c *Client) GetWorkspace(ctx context.Context, id string) (*proto.Workspace, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s", id), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get workspace: status code %d", rsp.StatusCode)
	}
	var ws proto.Workspace
	if err := json.NewDecoder(rsp.Body).Decode(&ws); err != nil {
		return nil, fmt.Errorf("failed to decode workspace: %w", err)
	}
	return &ws, nil
}

// DeleteWorkspace deletes a workspace on the server.
func (c *Client) DeleteWorkspace(ctx context.Context, id string) error {
	rsp, err := c.delete(ctx, fmt.Sprintf("/workspaces/%s", id), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete workspace: status code %d", rsp.StatusCode)
	}
	return nil
}

// SubscribeEvents subscribes to server-sent events for a workspace.
func (c *Client) SubscribeEvents(ctx context.Context, id string) (<-chan any, error) {
	events := make(chan any, 100)
	//nolint:bodyclose
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/events", id), nil, http.Header{
		"Accept":        []string{"text/event-stream"},
		"Cache-Control": []string{"no-cache"},
		"Connection":    []string{"keep-alive"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	if rsp.StatusCode != http.StatusOK {
		rsp.Body.Close()
		return nil, fmt.Errorf("failed to subscribe to events: status code %d", rsp.StatusCode)
	}

	go func() {
		defer rsp.Body.Close()

		scr := bufio.NewReader(rsp.Body)
		for {
			line, err := scr.ReadBytes('\n')
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				slog.Error("Reading from events stream", "error", err)
				time.Sleep(time.Second * 2)
				continue
			}
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}

			data, ok := bytes.CutPrefix(line, []byte("data:"))
			if !ok {
				slog.Warn("Invalid event format", "line", string(line))
				continue
			}

			data = bytes.TrimSpace(data)

			var p pubsub.Payload
			if err := json.Unmarshal(data, &p); err != nil {
				slog.Error("Unmarshaling event envelope", "error", err)
				continue
			}

			switch p.Type {
			case pubsub.PayloadTypeLSPEvent:
				var e pubsub.Event[proto.LSPEvent]
				_ = json.Unmarshal(p.Payload, &e)
				sendEvent(ctx, events, e)
			case pubsub.PayloadTypeMCPEvent:
				var e pubsub.Event[proto.MCPEvent]
				_ = json.Unmarshal(p.Payload, &e)
				sendEvent(ctx, events, e)
			case pubsub.PayloadTypePermissionRequest:
				var e pubsub.Event[proto.PermissionRequest]
				_ = json.Unmarshal(p.Payload, &e)
				sendEvent(ctx, events, e)
			case pubsub.PayloadTypePermissionNotification:
				var e pubsub.Event[proto.PermissionNotification]
				_ = json.Unmarshal(p.Payload, &e)
				sendEvent(ctx, events, e)
			case pubsub.PayloadTypeMessage:
				var e pubsub.Event[proto.Message]
				_ = json.Unmarshal(p.Payload, &e)
				sendEvent(ctx, events, e)
			case pubsub.PayloadTypeSession:
				var e pubsub.Event[proto.Session]
				_ = json.Unmarshal(p.Payload, &e)
				sendEvent(ctx, events, e)
			case pubsub.PayloadTypeFile:
				var e pubsub.Event[proto.File]
				_ = json.Unmarshal(p.Payload, &e)
				sendEvent(ctx, events, e)
			case pubsub.PayloadTypeAgentEvent:
				var e pubsub.Event[proto.AgentEvent]
				_ = json.Unmarshal(p.Payload, &e)
				sendEvent(ctx, events, e)
			default:
				slog.Warn("Unknown event type", "type", p.Type)
				continue
			}
		}
	}()

	return events, nil
}

func sendEvent(ctx context.Context, evc chan any, ev any) {
	slog.Info("Event received", "event", fmt.Sprintf("%T %+v", ev, ev))
	select {
	case evc <- ev:
	case <-ctx.Done():
		close(evc)
		return
	}
}

// GetLSPDiagnostics retrieves LSP diagnostics for a specific LSP client.
func (c *Client) GetLSPDiagnostics(ctx context.Context, id string, lspName string) (map[protocol.DocumentURI][]protocol.Diagnostic, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/lsps/%s/diagnostics", id, lspName), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get LSP diagnostics: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get LSP diagnostics: status code %d", rsp.StatusCode)
	}
	var diagnostics map[protocol.DocumentURI][]protocol.Diagnostic
	if err := json.NewDecoder(rsp.Body).Decode(&diagnostics); err != nil {
		return nil, fmt.Errorf("failed to decode LSP diagnostics: %w", err)
	}
	return diagnostics, nil
}

// GetLSPs retrieves the LSP client states for a workspace.
func (c *Client) GetLSPs(ctx context.Context, id string) (map[string]proto.LSPClientInfo, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/lsps", id), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get LSPs: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get LSPs: status code %d", rsp.StatusCode)
	}
	var lsps map[string]proto.LSPClientInfo
	if err := json.NewDecoder(rsp.Body).Decode(&lsps); err != nil {
		return nil, fmt.Errorf("failed to decode LSPs: %w", err)
	}
	return lsps, nil
}

// MCPGetStates retrieves the MCP client states for a workspace.
func (c *Client) MCPGetStates(ctx context.Context, id string) (map[string]proto.MCPClientInfo, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/mcp/states", id), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP states: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get MCP states: status code %d", rsp.StatusCode)
	}
	var states map[string]proto.MCPClientInfo
	if err := json.NewDecoder(rsp.Body).Decode(&states); err != nil {
		return nil, fmt.Errorf("failed to decode MCP states: %w", err)
	}
	return states, nil
}

// MCPRefreshPrompts refreshes prompts for a named MCP client.
func (c *Client) MCPRefreshPrompts(ctx context.Context, id, name string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/mcp/refresh-prompts", id), nil,
		jsonBody(struct {
			Name string `json:"name"`
		}{Name: name}),
		http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("failed to refresh MCP prompts: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to refresh MCP prompts: status code %d", rsp.StatusCode)
	}
	return nil
}

// MCPRefreshResources refreshes resources for a named MCP client.
func (c *Client) MCPRefreshResources(ctx context.Context, id, name string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/mcp/refresh-resources", id), nil,
		jsonBody(struct {
			Name string `json:"name"`
		}{Name: name}),
		http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("failed to refresh MCP resources: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to refresh MCP resources: status code %d", rsp.StatusCode)
	}
	return nil
}

// GetAgentSessionQueuedPrompts retrieves the number of queued prompts for a
// session.
func (c *Client) GetAgentSessionQueuedPrompts(ctx context.Context, id string, sessionID string) (int, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/agent/sessions/%s/prompts/queued", id, sessionID), nil, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get session agent queued prompts: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get session agent queued prompts: status code %d", rsp.StatusCode)
	}
	var count int
	if err := json.NewDecoder(rsp.Body).Decode(&count); err != nil {
		return 0, fmt.Errorf("failed to decode session agent queued prompts: %w", err)
	}
	return count, nil
}

// ClearAgentSessionQueuedPrompts clears the queued prompts for a session.
func (c *Client) ClearAgentSessionQueuedPrompts(ctx context.Context, id string, sessionID string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/agent/sessions/%s/prompts/clear", id, sessionID), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to clear session agent queued prompts: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to clear session agent queued prompts: status code %d", rsp.StatusCode)
	}
	return nil
}

// GetAgentInfo retrieves the agent status for a workspace.
func (c *Client) GetAgentInfo(ctx context.Context, id string) (*proto.AgentInfo, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/agent", id), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent status: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get agent status: status code %d", rsp.StatusCode)
	}
	var info proto.AgentInfo
	if err := json.NewDecoder(rsp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode agent status: %w", err)
	}
	return &info, nil
}

// UpdateAgent triggers an agent model update on the server.
func (c *Client) UpdateAgent(ctx context.Context, id string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/agent/update", id), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update agent: status code %d", rsp.StatusCode)
	}
	return nil
}

// SendMessage sends a message to the agent for a workspace.
func (c *Client) SendMessage(ctx context.Context, id string, sessionID, prompt string, attachments ...message.Attachment) error {
	protoAttachments := make([]proto.Attachment, len(attachments))
	for i, a := range attachments {
		protoAttachments[i] = proto.Attachment{
			FilePath: a.FilePath,
			FileName: a.FileName,
			MimeType: a.MimeType,
			Content:  a.Content,
		}
	}
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/agent", id), nil, jsonBody(proto.AgentMessage{
		SessionID:   sessionID,
		Prompt:      prompt,
		Attachments: protoAttachments,
	}), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("failed to send message to agent: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send message to agent: status code %d", rsp.StatusCode)
	}
	return nil
}

// GetAgentSessionInfo retrieves the agent session info for a workspace.
func (c *Client) GetAgentSessionInfo(ctx context.Context, id string, sessionID string) (*proto.AgentSession, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/agent/sessions/%s", id, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get session agent info: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get session agent info: status code %d", rsp.StatusCode)
	}
	var info proto.AgentSession
	if err := json.NewDecoder(rsp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode session agent info: %w", err)
	}
	return &info, nil
}

// AgentSummarizeSession requests a session summarization.
func (c *Client) AgentSummarizeSession(ctx context.Context, id string, sessionID string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/agent/sessions/%s/summarize", id, sessionID), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to summarize session: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to summarize session: status code %d", rsp.StatusCode)
	}
	return nil
}

// InitiateAgentProcessing triggers agent initialization on the server.
func (c *Client) InitiateAgentProcessing(ctx context.Context, id string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/agent/init", id), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to initiate session agent processing: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to initiate session agent processing: status code %d", rsp.StatusCode)
	}
	return nil
}

// ListMessages retrieves all messages for a session as proto types.
func (c *Client) ListMessages(ctx context.Context, id string, sessionID string) ([]proto.Message, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s/messages", id, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get messages: status code %d", rsp.StatusCode)
	}
	var msgs []proto.Message
	if err := json.NewDecoder(rsp.Body).Decode(&msgs); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}
	return msgs, nil
}

// GetSession retrieves a specific session as a proto type.
func (c *Client) GetSession(ctx context.Context, id string, sessionID string) (*proto.Session, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s", id, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get session: status code %d", rsp.StatusCode)
	}
	var sess proto.Session
	if err := json.NewDecoder(rsp.Body).Decode(&sess); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}
	return &sess, nil
}

// ListSessionHistoryFiles retrieves history files for a session as proto types.
func (c *Client) ListSessionHistoryFiles(ctx context.Context, id string, sessionID string) ([]proto.File, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s/history", id, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get session history files: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get session history files: status code %d", rsp.StatusCode)
	}
	var files []proto.File
	if err := json.NewDecoder(rsp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode session history files: %w", err)
	}
	return files, nil
}

// CreateSession creates a new session in a workspace as a proto type.
func (c *Client) CreateSession(ctx context.Context, id string, title string) (*proto.Session, error) {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/sessions", id), nil, jsonBody(proto.Session{Title: title}), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create session: status code %d", rsp.StatusCode)
	}
	var sess proto.Session
	if err := json.NewDecoder(rsp.Body).Decode(&sess); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}
	return &sess, nil
}

// ListSessions lists all sessions in a workspace as proto types.
func (c *Client) ListSessions(ctx context.Context, id string) ([]proto.Session, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/sessions", id), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get sessions: status code %d", rsp.StatusCode)
	}
	var sessions []proto.Session
	if err := json.NewDecoder(rsp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("failed to decode sessions: %w", err)
	}
	return sessions, nil
}

// GrantPermission grants a permission on a workspace.
func (c *Client) GrantPermission(ctx context.Context, id string, req proto.PermissionGrant) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/permissions/grant", id), nil, jsonBody(req), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("failed to grant permission: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to grant permission: status code %d", rsp.StatusCode)
	}
	return nil
}

// SetPermissionsSkipRequests sets the skip-requests flag for a workspace.
func (c *Client) SetPermissionsSkipRequests(ctx context.Context, id string, skip bool) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/permissions/skip", id), nil, jsonBody(proto.PermissionSkipRequest{Skip: skip}), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("failed to set permissions skip requests: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set permissions skip requests: status code %d", rsp.StatusCode)
	}
	return nil
}

// GetPermissionsSkipRequests retrieves the skip-requests flag for a workspace.
func (c *Client) GetPermissionsSkipRequests(ctx context.Context, id string) (bool, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/permissions/skip", id), nil, nil)
	if err != nil {
		return false, fmt.Errorf("failed to get permissions skip requests: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to get permissions skip requests: status code %d", rsp.StatusCode)
	}
	var skip proto.PermissionSkipRequest
	if err := json.NewDecoder(rsp.Body).Decode(&skip); err != nil {
		return false, fmt.Errorf("failed to decode permissions skip requests: %w", err)
	}
	return skip.Skip, nil
}

// GetConfig retrieves the workspace-specific configuration.
func (c *Client) GetConfig(ctx context.Context, id string) (*config.Config, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/config", id), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get config: status code %d", rsp.StatusCode)
	}
	var cfg config.Config
	if err := json.NewDecoder(rsp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}
	return &cfg, nil
}

func jsonBody(v any) *bytes.Buffer {
	b := new(bytes.Buffer)
	m, _ := json.Marshal(v)
	b.Write(m)
	return b
}

// SaveSession updates a session in a workspace, returning a proto type.
func (c *Client) SaveSession(ctx context.Context, id string, sess proto.Session) (*proto.Session, error) {
	rsp, err := c.put(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s", id, sess.ID), nil, jsonBody(sess), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to save session: status code %d", rsp.StatusCode)
	}
	var saved proto.Session
	if err := json.NewDecoder(rsp.Body).Decode(&saved); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}
	return &saved, nil
}

// DeleteSession deletes a session from a workspace.
func (c *Client) DeleteSession(ctx context.Context, id string, sessionID string) error {
	rsp, err := c.delete(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s", id, sessionID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete session: status code %d", rsp.StatusCode)
	}
	return nil
}

// ListUserMessages retrieves user-role messages for a session as proto types.
func (c *Client) ListUserMessages(ctx context.Context, id string, sessionID string) ([]proto.Message, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s/messages/user", id, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get user messages: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user messages: status code %d", rsp.StatusCode)
	}
	var msgs []proto.Message
	if err := json.NewDecoder(rsp.Body).Decode(&msgs); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to decode user messages: %w", err)
	}
	return msgs, nil
}

// ListAllUserMessages retrieves all user-role messages across sessions as proto types.
func (c *Client) ListAllUserMessages(ctx context.Context, id string) ([]proto.Message, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/messages/user", id), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get all user messages: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get all user messages: status code %d", rsp.StatusCode)
	}
	var msgs []proto.Message
	if err := json.NewDecoder(rsp.Body).Decode(&msgs); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to decode all user messages: %w", err)
	}
	return msgs, nil
}

// CancelAgentSession cancels an ongoing agent operation for a session.
func (c *Client) CancelAgentSession(ctx context.Context, id string, sessionID string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/agent/sessions/%s/cancel", id, sessionID), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to cancel agent session: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to cancel agent session: status code %d", rsp.StatusCode)
	}
	return nil
}

// GetAgentSessionQueuedPromptsList retrieves the list of queued prompt
// strings for a session.
func (c *Client) GetAgentSessionQueuedPromptsList(ctx context.Context, id string, sessionID string) ([]string, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/agent/sessions/%s/prompts/list", id, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get queued prompts list: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get queued prompts list: status code %d", rsp.StatusCode)
	}
	var prompts []string
	if err := json.NewDecoder(rsp.Body).Decode(&prompts); err != nil {
		return nil, fmt.Errorf("failed to decode queued prompts list: %w", err)
	}
	return prompts, nil
}

// GetDefaultSmallModel retrieves the default small model for a provider.
func (c *Client) GetDefaultSmallModel(ctx context.Context, id string, providerID string) (*config.SelectedModel, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/agent/default-small-model", id), url.Values{"provider_id": []string{providerID}}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get default small model: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get default small model: status code %d", rsp.StatusCode)
	}
	var model config.SelectedModel
	if err := json.NewDecoder(rsp.Body).Decode(&model); err != nil {
		return nil, fmt.Errorf("failed to decode default small model: %w", err)
	}
	return &model, nil
}

// FileTrackerRecordRead records a file read for a session.
func (c *Client) FileTrackerRecordRead(ctx context.Context, id string, sessionID, path string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/filetracker/read", id), nil, jsonBody(struct {
		SessionID string `json:"session_id"`
		Path      string `json:"path"`
	}{SessionID: sessionID, Path: path}), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("failed to record file read: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to record file read: status code %d", rsp.StatusCode)
	}
	return nil
}

// FileTrackerLastReadTime returns the last read time for a file in a
// session.
func (c *Client) FileTrackerLastReadTime(ctx context.Context, id string, sessionID, path string) (time.Time, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/filetracker/lastread", id), url.Values{
		"session_id": []string{sessionID},
		"path":       []string{path},
	}, nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last read time: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("failed to get last read time: status code %d", rsp.StatusCode)
	}
	var t time.Time
	if err := json.NewDecoder(rsp.Body).Decode(&t); err != nil {
		return time.Time{}, fmt.Errorf("failed to decode last read time: %w", err)
	}
	return t, nil
}

// FileTrackerListReadFiles returns the list of read files for a session.
func (c *Client) FileTrackerListReadFiles(ctx context.Context, id string, sessionID string) ([]string, error) {
	rsp, err := c.get(ctx, fmt.Sprintf("/workspaces/%s/sessions/%s/filetracker/files", id, sessionID), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get read files: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get read files: status code %d", rsp.StatusCode)
	}
	var files []string
	if err := json.NewDecoder(rsp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode read files: %w", err)
	}
	return files, nil
}

// LSPStart starts an LSP server for a path.
func (c *Client) LSPStart(ctx context.Context, id string, path string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/lsps/start", id), nil, jsonBody(struct {
		Path string `json:"path"`
	}{Path: path}), http.Header{"Content-Type": []string{"application/json"}})
	if err != nil {
		return fmt.Errorf("failed to start LSP: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to start LSP: status code %d", rsp.StatusCode)
	}
	return nil
}

// LSPStopAll stops all LSP servers for a workspace.
func (c *Client) LSPStopAll(ctx context.Context, id string) error {
	rsp, err := c.post(ctx, fmt.Sprintf("/workspaces/%s/lsps/stop", id), nil, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to stop LSPs: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to stop LSPs: status code %d", rsp.StatusCode)
	}
	return nil
}
