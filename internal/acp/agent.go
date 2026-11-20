package acp

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/skills"
	acp "github.com/coder/acp-go-sdk"
)

// crushAgent implements acp.Agent using Crush's internal services.
type crushAgent struct {
	app  *app.App
	log  *slog.Logger
	conn *acp.AgentSideConnection
	// bridge streams prompt-turn events as session/update notifications; used
	// to propagate request _meta onto outgoing notifications (S7).
	bridge *eventBridge

	// clientCaps holds the capabilities the connected client advertised during
	// initialize. Used to gate optional server track features (fs).
	clientCaps acp.ClientCapabilities

	// activeSessions holds the sessions created, loaded or resumed on this
	// connection. error.md:56 makes a session active only then, so a
	// session-scoped request naming any other id is -32002 even when the store
	// knows it.
	activeSessions sync.Map

	// sessionRoots holds each session's effective root set: the workspace cwd
	// followed by the additionalDirectories its setup request supplied
	// (session-setup.md:437-452).
	sessionRoots sync.Map
}

// newCrushAgent creates an ACP agent backed by the given Crush app.
func newCrushAgent(a *app.App, log *slog.Logger) *crushAgent {
	return &crushAgent{app: a, log: log}
}

// Initialize implements acp.Agent.
func (a *crushAgent) Initialize(_ context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error) {
	// Record what the client can do so we can gate optional server track
	// features (e.g. fs delegation) on it later.
	a.clientCaps = req.ClientCapabilities

	resp := acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentInfo: &acp.Implementation{
			Name:    "Crush",
			Title:   acp.Ptr("Crush"),
			Version: "dev",
		},
		AuthMethods: []acp.AuthMethod{},
		AgentCapabilities: acp.AgentCapabilities{
			Auth: acp.AgentAuthCapabilities{
				// Crush has no auth session, so logout is a no-op success.
				Logout: &acp.LogoutCapabilities{},
			},
			LoadSession: true,
			PromptCapabilities: acp.PromptCapabilities{
				Image:           true,
				Audio:           true,
				EmbeddedContext: true,
			},
			SessionCapabilities: acp.SessionCapabilities{
				AdditionalDirectories: &acp.SessionAdditionalDirectoriesCapabilities{},
				Close:                 &acp.SessionCloseCapabilities{},
				Delete:                &acp.SessionDeleteCapabilities{},
				List:                  &acp.SessionListCapabilities{},
				Resume:                &acp.SessionResumeCapabilities{},
			},
			// stdio is the baseline transport (session-setup.md:458); http and
			// sse are served because a Crush MCP server can use either.
			McpCapabilities: acp.McpCapabilities{Http: true, Sse: true},
		},
	}
	a.log.Info("ACP initialize", "protocol", resp.ProtocolVersion)
	return resp, nil
}

// NewSession implements acp.Agent.
func (a *crushAgent) NewSession(ctx context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	title := "ACP Session"

	if err := a.validateCwd(req.Cwd); err != nil {
		return acp.NewSessionResponse{}, err
	}

	session, err := a.app.Sessions.Create(context.Background(), title)
	if err != nil {
		return acp.NewSessionResponse{}, fmt.Errorf("create session: %w", err)
	}
	a.activeSessions.Store(string(session.ID), struct{}{})
	a.log.Info("ACP new session", "sessionId", session.ID, "cwd", req.Cwd)
	sid := acp.SessionId(session.ID)

	// The session's root set is cwd plus whatever additional roots the
	// client asked for, so paths under them do not need per-use consent
	// (session-setup.md:444-452).
	if err := a.setSessionRoots(sid, req.AdditionalDirectories); err != nil {
		return acp.NewSessionResponse{}, err
	}

	// Connect any MCP servers the client supplied. The spec makes stdio a
	// baseline every Agent must support (session-setup.md:458,466).
	if err := a.connectSessionMCPServers(ctx, session.ID, req.McpServers); err != nil {
		return acp.NewSessionResponse{}, err
	}

	// Advertise setup state the client needs before the first prompt. These
	// notifications are sent before the response returns, so the client's
	// request barrier (which waits for prior notifications) satisfies them
	// inline, exactly like streaming during a prompt.
	a.pushUpdate(ctx, sid, a.availableCommandsUpdate())
	a.pushUpdate(ctx, sid, a.sessionInfoUpdate(session.Title))

	return acp.NewSessionResponse{
		SessionId:     sid,
		Modes:         a.modeState(sid),
		ConfigOptions: a.configOptions(sid),
	}, nil
}

// validateCwd rejects a cwd naming a workspace other than the one Crush was
// started in, and any cwd that is not absolute: the spec makes cwd part of
// the session's root set and requires every protocol path to be absolute
// (session-setup.md:437-444, overview.md:160). Crush serves exactly one
// workspace per process, so a mismatch cannot be honoured.
func (a *crushAgent) validateCwd(cwd string) error {
	if cwd == "" {
		return nil
	}
	if !isAbsPath(cwd) {
		return invalidParam("cwd", cwd)
	}
	local := a.workingDir()
	if local == "" || samePath(local, cwd) {
		return nil
	}
	return fmt.Errorf(
		"session cwd %q does not match crush working directory %q; "+
			"start crush with -c %s to use that workspace",
		cwd, local, cwd)
}

// isAbsPath reports whether p is absolute in either form the protocol uses:
// the spec's POSIX-style leading slash, or a native drive-letter path.
func isAbsPath(p string) bool {
	return strings.HasPrefix(p, "/") || filepath.IsAbs(p)
}

// ValidateCwd exposes validateCwd to the dispatcher, which owns session/load.
func (a *crushAgent) ValidateCwd(cwd string) error {
	return a.validateCwd(cwd)
}

// SetSessionRoots exposes setSessionRoots to the dispatcher, which owns
// session/load and carries the same additionalDirectories field session/new
// and session/resume do (session-setup.md:437-444).
func (a *crushAgent) SetSessionRoots(sessionID acp.SessionId, dirs []string) error {
	return a.setSessionRoots(sessionID, dirs)
}

// SessionState reports the mode and config-option state for a session, so
// session/load can return the same setup payload session/new and session/resume
// do (session-setup.md:200 allows it; the client needs it to render the
// session it just loaded).
func (a *crushAgent) SessionState(sessionID acp.SessionId) (*acp.SessionModeState, []acp.SessionConfigOption) {
	return a.modeState(sessionID), a.configOptions(sessionID)
}

// SessionExists reports whether the store knows the session. It deliberately
// ignores the active set: session/load is itself one of the ways a session
// becomes active, so the dispatcher calls it before MarkSessionActive.
func (a *crushAgent) SessionExists(ctx context.Context, sessionID acp.SessionId) bool {
	if a.app == nil || a.app.Sessions == nil || sessionID == "" {
		return true
	}
	_, err := a.app.Sessions.Get(ctx, string(sessionID))
	return err == nil
}

// MarkSessionActive records that a session was created, loaded or resumed on
// this connection. Until then a session-scoped request naming it answers
// -32002 even when the store knows the id (error.md:56).
func (a *crushAgent) MarkSessionActive(sessionID acp.SessionId) {
	a.activeSessions.Store(string(sessionID), struct{}{})
}

// setSessionRoots records the additional roots a setup request asked for.
// Each path must be absolute, since it becomes part of the session's root
// set, and protocol paths are always absolute (overview.md:160).
func (a *crushAgent) setSessionRoots(sessionID acp.SessionId, dirs []string) error {
	for _, dir := range dirs {
		if !isAbsPath(dir) {
			return invalidParam("additionalDirectories", dir)
		}
	}
	if len(dirs) == 0 {
		a.sessionRoots.Delete(string(sessionID))
		return nil
	}
	a.sessionRoots.Store(string(sessionID), dirs)
	return nil
}

// extraRoots returns the additional roots the session's setup request asked
// for, so a turn's tools treat them as in scope (session-setup.md:444-452).
func (a *crushAgent) extraRoots(sessionID acp.SessionId) []string {
	if v, ok := a.sessionRoots.Load(string(sessionID)); ok {
		return v.([]string)
	}
	return nil
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

// listPageSize bounds how many sessions one session/list response carries. The
// spec lets Agents enforce a reasonable page size and requires only that a
// missing nextCursor marks the end of the results.
const listPageSize = 50

// ListSessions implements acp.Agent. It supports the spec's cursor-based
// pagination and cwd filter: an opaque offset cursor comes back as
// nextCursor while more sessions remain, and an unparseable cursor is
// rejected with -32602 rather than silently restarting from the first page.
func (a *crushAgent) ListSessions(_ context.Context, req acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	cwd := a.workingDir()
	// The filter is itself a protocol path, so it must be absolute
	// (session-list.md:85).
	if req.Cwd != nil && *req.Cwd != "" && !isAbsPath(*req.Cwd) {
		return acp.ListSessionsResponse{}, invalidParam("cwd", *req.Cwd)
	}
	// Crush has a single workspace root, so a filter naming any other
	// directory matches nothing.
	if req.Cwd != nil && *req.Cwd != "" && cwd != "" && !samePath(cwd, *req.Cwd) {
		return acp.ListSessionsResponse{Sessions: []acp.SessionInfo{}}, nil
	}

	sessions, err := a.app.Sessions.List(context.Background())
	if err != nil {
		return acp.ListSessionsResponse{}, fmt.Errorf("list sessions: %w", err)
	}

	start := 0
	if req.Cursor != nil && *req.Cursor != "" {
		offset, err := decodeListCursor(*req.Cursor)
		if err != nil {
			return acp.ListSessionsResponse{}, acp.NewInvalidParams(map[string]any{
				"cursor": *req.Cursor,
				"error":  err.Error(),
			})
		}
		start = offset
	}
	if start >= len(sessions) {
		return acp.ListSessionsResponse{Sessions: []acp.SessionInfo{}}, nil
	}
	end := min(start+listPageSize, len(sessions))

	out := make([]acp.SessionInfo, 0, end-start)
	for _, s := range sessions[start:end] {
		t := s.Title
		info := acp.SessionInfo{
			SessionId:             acp.SessionId(s.ID),
			Title:                 &t,
			Cwd:                   cwd,
			AdditionalDirectories: a.extraRoots(acp.SessionId(s.ID)),
		}
		if s.UpdatedAt > 0 {
			ts := time.Unix(s.UpdatedAt, 0).UTC().Format(time.RFC3339)
			info.UpdatedAt = &ts
		}
		out = append(out, info)
	}

	resp := acp.ListSessionsResponse{Sessions: out}
	if end < len(sessions) {
		next := encodeListCursor(end)
		resp.NextCursor = &next
	}
	return resp, nil
}

// encodeListCursor renders an offset as the opaque cursor token handed to
// clients. Clients MUST treat it as opaque, so the encoding is an
// implementation detail.
func encodeListCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

// decodeListCursor parses a cursor produced by encodeListCursor.
func decodeListCursor(cursor string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("malformed cursor")
	}
	offset, err := strconv.Atoi(string(raw))
	if err != nil {
		return 0, fmt.Errorf("malformed cursor")
	}
	if offset < 0 {
		return 0, fmt.Errorf("malformed cursor")
	}
	return offset, nil
}

// CloseSession implements acp.Agent.
func (a *crushAgent) CloseSession(_ context.Context, req acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	id := string(req.SessionId)
	if a.app.AgentCoordinator != nil {
		a.app.AgentCoordinator.Cancel(id)
	}
	// Closing frees the resources associated with the session, which includes
	// any MCP servers it brought with it (session-setup.md:313).
	mcp.TeardownSession(id)
	a.activeSessions.Delete(id)
	a.sessionRoots.Delete(id)
	return acp.CloseSessionResponse{}, nil
}

// ResumeSession restores a session without replaying history (spec: unlike
// session/load, resume MUST NOT stream session/update notifications). Crush
// continues any session id on the next session/prompt (same as `crush
// --continue`), so resume returns the current mode/config state and succeeds.
//
// A session the store does not know is -32002 (error.md:58), and the request
// cwd must match this workspace for the same reason session/new checks it.
func (a *crushAgent) ResumeSession(ctx context.Context, req acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	a.log.Info("ACP resume session", "sessionId", req.SessionId)
	if err := a.validateCwd(req.Cwd); err != nil {
		return acp.ResumeSessionResponse{}, err
	}
	if !a.SessionExists(ctx, req.SessionId) {
		return acp.ResumeSessionResponse{}, sessionNotFound(req.SessionId)
	}
	// Resuming is what makes a session active on this connection
	// (error.md:56), so the existence check above stays store-only.
	a.activeSessions.Store(string(req.SessionId), struct{}{})
	if err := a.setSessionRoots(req.SessionId, req.AdditionalDirectories); err != nil {
		return acp.ResumeSessionResponse{}, err
	}
	// The spec requires resume to reconnect the requested MCP servers
	// (session-setup.md:257). The request list is authoritative for this
	// session, so any servers it already had are replaced.
	mcp.TeardownSession(string(req.SessionId))
	if err := a.connectSessionMCPServers(ctx, string(req.SessionId), req.McpServers); err != nil {
		return acp.ResumeSessionResponse{}, err
	}
	// The request's additional roots replace the session's list; an empty
	// list clears it (session-setup.md:437-444).
	return acp.ResumeSessionResponse{
		Modes:         a.modeState(req.SessionId),
		ConfigOptions: a.configOptions(req.SessionId),
	}, nil
}

// Logout terminates the current authentication session. Crush has no auth
// session, so this is a successful no-op.
func (a *crushAgent) Logout(_ context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	a.log.Info("ACP logout")
	return acp.LogoutResponse{}, nil
}

// DeleteSession deletes a session from the store, serving the session/delete
// capability.
//
// Deleting a session that never existed is a success, not an error
// (session-delete.md:85). The store reports a missing row as sql.ErrNoRows,
// which is the only case swallowed here; a real failure still surfaces.
func (a *crushAgent) DeleteSession(ctx context.Context, req acp.DeleteSessionRequest) (acp.DeleteSessionResponse, error) {
	id := string(req.SessionId)
	a.log.Info("ACP delete session", "sessionId", id)
	if a.app.AgentCoordinator != nil {
		a.app.AgentCoordinator.Cancel(id)
	}
	if a.app.Sessions != nil {
		if err := a.app.Sessions.Delete(ctx, id); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return acp.DeleteSessionResponse{}, fmt.Errorf("delete session: %w", err)
		}
	}
	a.activeSessions.Delete(id)
	mcp.TeardownSession(id)
	return acp.DeleteSessionResponse{}, nil
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

	// The spec requires -32002 with data.sessionId for a session-scoped
	// request naming a session that does not exist or is not active.
	if err := a.requireSession(ctx, req.SessionId); err != nil {
		return acp.PromptResponse{}, err
	}

	// _meta is echoed on this turn's session/update notifications (S7), with
	// a W3C traceparent added when the client did not send one so the turn
	// is correlatable end to end (extensibility.md:40-44).
	meta := withTraceMeta(req.Meta)
	if a.bridge != nil {
		a.bridge.SetPromptMeta(sessionID, meta)
	}

	// Paths under the session's additional roots are in scope for this
	// turn's tools (session-setup.md:444-452).
	ctx = context.WithValue(ctx, tools.RootsContextKey, a.extraRoots(req.SessionId))

	prompt, attachments := buildPrompt(req.Prompt)
	// A leading /name is resolved against the commands advertised in
	// available_commands_update, as the spec says commands are "run as part of
	// regular session/prompt requests" (slash-commands.md:9,99).
	prompt = a.resolveCommand(prompt)
	a.log.Info("ACP prompt", "sessionId", sessionID, "promptLen", len(prompt))

	// Prompt result.
	result, err := a.app.AgentCoordinator.Run(ctx, sessionID, prompt, attachments...)
	if err != nil {
		if ctx.Err() != nil {
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled, Meta: meta}, nil
		}
		return acp.PromptResponse{}, fmt.Errorf("run prompt: %w", err)
	}

	stopReason := mapFinishReason(result.Response.FinishReason)
	a.log.Info("ACP prompt done", "sessionId", sessionID, "stopReason", stopReason)
	return acp.PromptResponse{StopReason: stopReason, Meta: meta}, nil
}

// requireSession reports -32002 when the named session does not exist or is
// not active on this connection. Callers treat a nil error as "go ahead"; a
// store error other than not-found is reported as-is rather than masked as a
// missing session.
func (a *crushAgent) requireSession(ctx context.Context, id acp.SessionId) error {
	if a.app == nil || a.app.Sessions == nil || id == "" {
		return nil
	}
	if _, err := a.app.Sessions.Get(ctx, string(id)); err != nil {
		return sessionNotFound(id)
	}
	if _, ok := a.activeSessions.Load(string(id)); !ok {
		return sessionNotFound(id)
	}
	return nil
}

// sessionNotFound builds the -32002 the spec prescribes for a session that
// does not exist or is not active on the connection (error.md:56-58).
func sessionNotFound(id acp.SessionId) error {
	return acp.NewRequestError(
		acpErrorResourceNotFound,
		"Resource not found",
		map[string]any{"sessionId": string(id)},
	)
}

// SetSessionMode acknowledges a mode change and notifies the client. An
// unknown mode is rejected with -32602 as the spec prescribes; validation is
// skipped when no config is loaded, since there is nothing to compare against.
//
// The selection is recorded per session so modeState and the "mode" config
// option report it back. It does not switch Crush's agent: the coordinator's
// main agent is workspace-wide, not per session (see plan O25).
func (a *crushAgent) SetSessionMode(ctx context.Context, req acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	a.log.Info("ACP set session mode", "sessionId", req.SessionId, "modeId", req.ModeId)
	if err := a.requireSession(ctx, req.SessionId); err != nil {
		return acp.SetSessionModeResponse{}, err
	}
	mode, ok := resolveModeId(req.ModeId)
	if !ok {
		return acp.SetSessionModeResponse{}, invalidParam("modeId", string(req.ModeId))
	}
	if err := a.applyMode(ctx, req.SessionId, mode); err != nil {
		return acp.SetSessionModeResponse{}, err
	}
	return acp.SetSessionModeResponse{}, nil
}

// applyMode switches Crush into the mode and announces it. Both
// session/set_mode and the "mode" config option funnel through here so the two
// entry points can never disagree.
//
// The switch is real: SetMainAgent changes the agent for the whole workspace
// (the coordinator has no per-session agent), and SetPermissionMode changes the
// permission mode for the session. Both halves are needed because ACP's mode
// enum is a flattening of the TUI's (agent, permission) ring — see modes.go.
func (a *crushAgent) applyMode(ctx context.Context, sessionID acp.SessionId, mode acpMode) error {
	if a.app != nil {
		if a.app.AgentCoordinator != nil {
			if err := a.app.AgentCoordinator.SetMainAgent(mode.agent); err != nil {
				return fmt.Errorf("switch agent to %q: %w", mode.agent, err)
			}
		}
		if a.app.Permissions != nil {
			a.app.Permissions.SetPermissionMode(mode.perm)
		}
	}
	a.pushUpdate(ctx, sessionID, acp.SessionUpdate{
		CurrentModeUpdate: &acp.SessionCurrentModeUpdate{CurrentModeId: mode.id},
	})
	return nil
}

// SetSessionConfigOption acknowledges a configuration change and notifies the
// client of the resulting config options. Unknown ids and unsupported values
// are rejected with -32602, as the spec prescribes.
func (a *crushAgent) SetSessionConfigOption(ctx context.Context, req acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	sessionID := configOptionSessionID(req)
	if err := a.requireSession(ctx, sessionID); err != nil {
		return acp.SetSessionConfigOptionResponse{}, err
	}
	if cfg := a.config(); cfg != nil {
		if err := validateConfigOption(cfg, req); err != nil {
			return acp.SetSessionConfigOptionResponse{}, err
		}
	}
	if req.ValueId != nil && req.ValueId.ConfigId == "mode" {
		mode, ok := resolveModeId(acp.SessionModeId(req.ValueId.Value))
		if !ok {
			return acp.SetSessionConfigOptionResponse{}, invalidParam("value", string(req.ValueId.Value))
		}
		if err := a.applyMode(ctx, sessionID, mode); err != nil {
			return acp.SetSessionConfigOptionResponse{}, err
		}
	}
	if req.Boolean != nil && req.Boolean.ConfigId == "thinking" {
		if err := a.setThinking(ctx, req.Boolean.Value); err != nil {
			a.log.Warn("Failed to apply thinking config option", "error", err)
		}
	}
	if req.ValueId != nil && req.ValueId.ConfigId == "thinking" {
		if err := a.setThinking(ctx, req.ValueId.Value == "on"); err != nil {
			a.log.Warn("Failed to apply thinking config option", "error", err)
		}
	}
	if req.ValueId != nil && req.ValueId.ConfigId == "model" {
		if err := a.setModel(ctx, parseModelValue(string(req.ValueId.Value))); err != nil {
			a.log.Warn("Failed to apply model config option", "error", err)
		}
	}
	a.log.Info("ACP set session config option", "sessionId", sessionID)
	a.pushUpdate(ctx, sessionID, a.configOptionsUpdate(sessionID))
	return acp.SetSessionConfigOptionResponse{ConfigOptions: a.configOptions(sessionID)}, nil
}

// configOptionSessionID extracts the session id from either request variant.
func configOptionSessionID(req acp.SetSessionConfigOptionRequest) acp.SessionId {
	if req.ValueId != nil {
		return req.ValueId.SessionId
	}
	if req.Boolean != nil {
		return req.Boolean.SessionId
	}
	return ""
}

// acpErrorResourceNotFound is JSON-RPC "Resource not found" (-32002). The SDK
// defines no constructor for it, though it does for -32602 and friends.
const acpErrorResourceNotFound = -32002

// invalidParam builds the -32602 error the spec prescribes for malformed
// parameters and unsupported values (error.md:48-52).
func invalidParam(field, value string) error {
	return acp.NewInvalidParams(map[string]any{field: value})
}

// validateConfigOption rejects an unknown configId and any value outside the
// set the Agent advertises, so callers cannot silently no-op a bad request.
// Only the option families Crush actually implements are validated; anything
// else is unknown.
func validateConfigOption(cfg *config.Config, req acp.SetSessionConfigOptionRequest) error {
	switch {
	case req.Boolean != nil:
		if req.Boolean.ConfigId != "thinking" {
			return invalidParam("configId", string(req.Boolean.ConfigId))
		}
	case req.ValueId != nil:
		switch req.ValueId.ConfigId {
		case "model":
			value := string(req.ValueId.Value)
			for _, opt := range modelOptionsFor(cfg) {
				if string(opt.Value) == value {
					return nil
				}
			}
			return invalidParam("value", value)
		case "mode":
			value := string(req.ValueId.Value)
			if !isSelectableMode(value) {
				return invalidParam("value", value)
			}
			return nil
		case "thinking":
			value := string(req.ValueId.Value)
			if value != "on" && value != "off" {
				return invalidParam("value", value)
			}
			return nil
		default:
			return invalidParam("configId", string(req.ValueId.ConfigId))
		}
	default:
		return acp.NewInvalidParams(map[string]any{
			"error": "either valueId or boolean must be set",
		})
	}
	return nil
}

// setThinking toggles the current model's reasoning flag and persists it the
// same way the TUI's toggle-thinking command does.
func (a *crushAgent) setThinking(ctx context.Context, enabled bool) error {
	store := a.app.Store()
	if store == nil {
		return nil
	}
	cfg := store.Config()
	agent, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return nil
	}
	model := cfg.Models[agent.Model]
	if model.Think == enabled {
		return nil
	}
	model.Think = enabled
	if err := store.UpdatePreferredModel(config.ScopeGlobal, agent.Model, model); err != nil {
		return fmt.Errorf("update preferred model: %w", err)
	}
	return a.app.UpdateAgentModel(ctx)
}

// setModel applies a selected model (from the "model" select config option) to
// the large model slot, then refreshes the coordinator so the new model serves
// the next turn. The current thinking preference is preserved.
func (a *crushAgent) setModel(ctx context.Context, sel config.SelectedModel) error {
	store := a.app.Store()
	if store == nil {
		return fmt.Errorf("config store unavailable")
	}
	if cfg := a.config(); cfg != nil {
		if current, ok := cfg.Models[config.SelectedModelTypeLarge]; ok {
			sel.Think = current.Think
		}
	}
	if err := store.UpdatePreferredModel(config.ScopeGlobal, config.SelectedModelTypeLarge, sel); err != nil {
		return err
	}
	return a.app.UpdateAgentModel(ctx)
}

// parseModelValue splits a "provider/model" option value into a SelectedModel.
// The separator is the first slash so a model id that itself contains slashes
// is preserved intact.
func parseModelValue(value string) config.SelectedModel {
	if i := strings.Index(value, "/"); i > 0 {
		return config.SelectedModel{Provider: value[:i], Model: value[i+1:]}
	}
	return config.SelectedModel{Model: value}
}

// modelOptionsFor builds the ACP select-option values for the model picker.
// Values are encoded as "provider/model" so a selection round-trips to a
// SelectedModel unambiguously. Every configured provider's catalog is listed
// (deduped, provider-sorted), and the currently selected model is always
// included so the select's CurrentValue is never dangling.
func modelOptionsFor(cfg *config.Config) []acp.SessionConfigSelectOption {
	seen := map[string]bool{}
	opts := []acp.SessionConfigSelectOption{}
	add := func(provider, model, name string) {
		value := provider + "/" + model
		if seen[value] {
			return
		}
		seen[value] = true
		if name == "" {
			name = model
		}
		opts = append(opts, acp.SessionConfigSelectOption{
			Value: acp.SessionConfigValueId(value),
			Name:  name,
		})
	}
	if cfg != nil && cfg.Providers != nil {
		providers := cfg.Providers.Copy()
		ids := make([]string, 0, len(providers))
		for id := range providers {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, pid := range ids {
			p := providers[pid]
			if p.Disable {
				continue
			}
			for _, m := range p.Models {
				if m.ID == "" {
					continue
				}
				add(pid, m.ID, m.Name)
			}
		}
	}
	if cfg != nil {
		if current, ok := cfg.Models[config.SelectedModelTypeLarge]; ok && current.Model != "" {
			add(current.Provider, current.Model, current.Model)
		}
	}
	return opts
}

// modelSelectValue returns the currently selected large model as a
// "provider/model" value, or "" when no model is selected.
func modelSelectValue(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	current, ok := cfg.Models[config.SelectedModelTypeLarge]
	if !ok || current.Model == "" {
		return ""
	}
	return current.Provider + "/" + current.Model
}

// pushUpdate sends a session/update notification to the connected client. It is
// a no-op when no connection is established.
func (a *crushAgent) pushUpdate(ctx context.Context, sessionID acp.SessionId, update acp.SessionUpdate) {
	if a.conn == nil {
		return
	}
	note := acp.SessionNotification{SessionId: sessionID, Update: update}
	if err := a.conn.SessionUpdate(ctx, note); err != nil {
		a.log.Warn("Failed to send session update", "error", err)
	}
}

// config returns the active configuration, or nil when the app carries no
// config store (as in tests).
func (a *crushAgent) config() *config.Config {
	if a.app == nil {
		return nil
	}
	store := a.app.Store()
	if store == nil {
		return nil
	}
	return store.Config()
}

// store returns the configuration store, or nil when the app carries none (as
// in tests). MCP setup needs the store itself, not just a config snapshot.
func (a *crushAgent) store() *config.ConfigStore {
	if a.app == nil {
		return nil
	}
	return a.app.Store()
}

// availableCommandsUpdate advertises the slash commands Crush can run:
// user-invocable skills, custom markdown commands, and MCP prompts. When
// input is expected, an unstructured-input hint is included so the client can
// prompt the user for it. Shell builtins are intentionally omitted; they are
// crushrc config operations with no slash-command surface (see plan Q1).
func (a *crushAgent) availableCommandsUpdate() acp.SessionUpdate {
	var cmds []acp.AvailableCommand

	// User-invocable skills become commands the user can type directly.
	if a.app != nil && a.app.Skills != nil {
		entries := skills.Catalog(a.app.Skills.ActiveSkills(), a.app.Skills.ResolvedPaths(), a.workingDir())
		for _, c := range commands.FromSkillCatalog(entries) {
			desc := c.Name
			if c.Skill != nil && c.Skill.Description != "" {
				desc = c.Skill.Description
			}
			cmds = append(cmds, newAvailableCommand(c.Name, desc, c.Arguments))
		}
	}

	if cfg := a.config(); cfg != nil {
		if custom, err := commands.LoadCustomCommands(cfg); err == nil {
			for _, c := range custom {
				desc := firstLine(c.Content)
				cmds = append(cmds, newAvailableCommand(c.Name, desc, c.Arguments))
			}
		}
	}

	if prompts, err := commands.LoadMCPPrompts(); err == nil {
		for _, p := range prompts {
			cmds = append(cmds, newAvailableCommand(p.ID, p.Description, p.Arguments))
		}
	}

	return acp.SessionUpdate{
		AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
			AvailableCommands: cmds,
		},
	}
}

// newAvailableCommand builds one advertised command. The hint is left off
// when the command takes no arguments, since the spec requires hint to be
// non-empty whenever an input object is present.
func newAvailableCommand(name, description string, args []commands.Argument) acp.AvailableCommand {
	cmd := acp.AvailableCommand{Name: name, Description: description}
	if len(args) == 0 {
		return cmd
	}
	cmd.Input = &acp.AvailableCommandInput{
		Unstructured: &acp.UnstructuredCommandInput{Hint: commandHint(args)},
	}
	return cmd
}

// commandHint renders a short placeholder describing the arguments a command
// expects, e.g. "<name> [path]".
func commandHint(args []commands.Argument) string {
	var sb strings.Builder
	for i, a := range args {
		if i > 0 {
			sb.WriteByte(' ')
		}
		if !a.Required {
			sb.WriteByte('[')
		}
		label := a.Title
		if label == "" {
			label = a.ID
		}
		sb.WriteString("<" + label + ">")
		if !a.Required {
			sb.WriteByte(']')
		}
	}
	if sb.Len() == 0 {
		return "arguments"
	}
	return sb.String()
}

// sessionInfoUpdate reports the session's title and last-activity timestamp.
func (a *crushAgent) sessionInfoUpdate(title string) acp.SessionUpdate {
	now := time.Now().UTC().Format(time.RFC3339)
	return acp.SessionUpdate{
		SessionInfoUpdate: &acp.SessionSessionInfoUpdate{
			Title:     acp.PatchSet(title),
			UpdatedAt: acp.PatchSet(now),
		},
	}
}

// configOptions advertises the session's selectable modes, the available models
// and the thinking toggle, so the client can render a mode switcher, a model
// picker and a reasoning on/off control.
func (a *crushAgent) configOptions(sessionID acp.SessionId) []acp.SessionConfigOption {
	cfg := a.config()
	return configOptionsFor(cfg, a.currentMode(), a.booleanConfigOptions())
}

// booleanConfigOptions reports whether the client advertised support for
// boolean config options during initialize (session-config-options.md:207).
func (a *crushAgent) booleanConfigOptions() bool {
	caps := a.clientCaps.Session
	return caps != nil && caps.ConfigOptions != nil && caps.ConfigOptions.Boolean != nil
}

// configOptionsFor builds the config-option list from a config snapshot and the
// session's active mode.
//
// The "mode" select is emitted alongside `modes` because the spec makes config
// options the modern spelling of a mode switcher and asks Agents that offer
// mode-like configuration to send both during the transition
// (session-config-options.md:462-471).
func configOptionsFor(cfg *config.Config, currentMode acp.SessionModeId, booleanOK bool) []acp.SessionConfigOption {
	opts := []acp.SessionConfigOption{}
	if cfg == nil {
		return opts
	}
	modeOpts := modeSelectOptions()
	cur := acp.SessionConfigValueId(currentMode)
	if cur == "" {
		cur = modeOpts[0].Value
	}
	ungroupedModes := acp.SessionConfigSelectOptionsUngrouped(modeOpts)
	opts = append(opts, acp.SessionConfigOption{
		Select: &acp.SessionConfigOptionSelect{
			Id:           "mode",
			Name:         "Mode",
			Type:         "select",
			CurrentValue: cur,
			Category:     acp.Ptr(acp.SessionConfigOptionCategoryMode),
			Description:  acp.Ptr("Switch between Crush's modes"),
			Options: acp.SessionConfigSelectOptions{
				Ungrouped: &ungroupedModes,
			},
		},
	})
	// Thinking is a boolean only when the client advertised support for
	// boolean options; otherwise it falls back to the select the spec names
	// for exactly this case (session-config-options.md:207-209).
	if booleanOK {
		opts = append(opts, acp.SessionConfigOption{
			Boolean: &acp.SessionConfigOptionBoolean{
				Id:           "thinking",
				Name:         "Thinking",
				Type:         "boolean",
				CurrentValue: thinkingEnabled(cfg),
				Description:  acp.Ptr("Enable extended reasoning for supported models"),
				Category:     acp.Ptr(acp.SessionConfigOptionCategoryThoughtLevel),
			},
		})
	} else {
		thinkingOpts := []acp.SessionConfigSelectOption{
			{Value: "on", Name: "On"},
			{Value: "off", Name: "Off"},
		}
		thinkingCurrent := acp.SessionConfigValueId("off")
		if thinkingEnabled(cfg) {
			thinkingCurrent = "on"
		}
		ungroupedThinking := acp.SessionConfigSelectOptionsUngrouped(thinkingOpts)
		opts = append(opts, acp.SessionConfigOption{
			Select: &acp.SessionConfigOptionSelect{
				Id:           "thinking",
				Name:         "Thinking",
				Type:         "select",
				CurrentValue: thinkingCurrent,
				Description:  acp.Ptr("Enable extended reasoning for supported models"),
				Category:     acp.Ptr(acp.SessionConfigOptionCategoryThoughtLevel),
				Options: acp.SessionConfigSelectOptions{
					Ungrouped: &ungroupedThinking,
				},
			},
		})
	}
	modelOpts := modelOptionsFor(cfg)
	if len(modelOpts) > 0 {
		cur := acp.SessionConfigValueId(modelSelectValue(cfg))
		if cur == "" {
			cur = modelOpts[0].Value
		}
		ungroupedModels := acp.SessionConfigSelectOptionsUngrouped(modelOpts)
		opts = append(opts, acp.SessionConfigOption{
			Select: &acp.SessionConfigOptionSelect{
				Id:           "model",
				Name:         "Model",
				Type:         "select",
				CurrentValue: cur,
				Category:     acp.Ptr(acp.SessionConfigOptionCategoryModel),
				Options: acp.SessionConfigSelectOptions{
					Ungrouped: &ungroupedModels,
				},
			},
		})
	}
	return opts
}

// isSelectableMode reports whether id names a mode in the ACP ring, after
// alias resolution.
func isSelectableMode(id string) bool {
	_, ok := resolveModeId(acp.SessionModeId(id))
	return ok
}

// currentMode reports the mode the agent is actually in, read from live state
// rather than a private record.
//
// The spec requires an option's currentValue to be the value the Agent operates
// with (session-config-options.md:245), so this must reflect reality even when
// another ACP session, or the TUI, moved the workspace-wide agent. When no
// coordinator or permission service is wired (as in unit tests) it falls back
// to the ring's first entry.
func (a *crushAgent) currentMode() acp.SessionModeId {
	if a.app == nil {
		return acpModes[0].id
	}
	perm := permission.PermissionModeNormal
	if a.app.Permissions != nil {
		perm = a.app.Permissions.PermissionMode()
	}
	agent := config.AgentCoder
	if a.app.AgentCoordinator != nil {
		if name := a.app.AgentCoordinator.MainAgentName(); name != "" {
			agent = name
		}
	}
	return modeForState(agent, perm).id
}

// thinkingEnabled reports whether the current model has reasoning enabled.
func thinkingEnabled(cfg *config.Config) bool {
	agent, ok := cfg.Agents[config.AgentCoder]
	if !ok {
		return false
	}
	model, ok := cfg.Models[agent.Model]
	if !ok {
		return false
	}
	return model.Think
}

// configOptionsUpdate wraps configOptions in a session/update notification.
func (a *crushAgent) configOptionsUpdate(sessionID acp.SessionId) acp.SessionUpdate {
	return acp.SessionUpdate{
		ConfigOptionUpdate: &acp.SessionConfigOptionUpdate{ConfigOptions: a.configOptions(sessionID)},
	}
}

// modeState reports the available session modes and the active one.
func (a *crushAgent) modeState(_ acp.SessionId) *acp.SessionModeState {
	if a.config() == nil {
		return nil
	}
	return &acp.SessionModeState{
		AvailableModes: availableModes(),
		CurrentModeId:  a.currentMode(),
	}
}

// firstLine returns the first non-empty line of s, trimmed.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// clientFSAdapter lets the ACP connection serve as the tool layer's client
// file system. It reads the client's advertised capabilities at call time,
// because Initialize arrives after the tools are constructed.
type clientFSAdapter struct {
	agent *crushAgent
}

func (c *clientFSAdapter) ReadTextFile(ctx context.Context, path string) (string, error) {
	sessionID := acp.SessionId(tools.GetSessionFromContext(ctx))
	return c.agent.fsReadTextFile(ctx, sessionID, path, nil, nil)
}

func (c *clientFSAdapter) WriteTextFile(ctx context.Context, path, content string) error {
	sessionID := acp.SessionId(tools.GetSessionFromContext(ctx))
	return c.agent.fsWriteTextFile(ctx, sessionID, path, content)
}

// fsReadTextFile reads a text file. When the connected client advertises fs
// support the read is delegated to the client; otherwise Crush reads it from its
// own local filesystem. The line/limit windowing supported by the client path
// is applied only on delegation; local reads return the whole file.
func (a *crushAgent) fsReadTextFile(ctx context.Context, sessionID acp.SessionId, path string, line, limit *int64) (string, error) {
	if a.clientCaps.Fs.ReadTextFile && a.conn != nil {
		resp, err := a.conn.ReadTextFile(ctx, acp.ReadTextFileRequest{
			Path:      path,
			SessionId: sessionID,
			Line:      line,
			Limit:     limit,
		})
		if err != nil {
			return "", fmt.Errorf("client fs read: %w", err)
		}
		return resp.Content, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

// fsWriteTextFile writes a text file, mirroring fsReadTextFile's delegation
// policy. Local writes create parent directories as needed.
func (a *crushAgent) fsWriteTextFile(ctx context.Context, sessionID acp.SessionId, path, content string) error {
	if a.clientCaps.Fs.WriteTextFile && a.conn != nil {
		if _, err := a.conn.WriteTextFile(ctx, acp.WriteTextFileRequest{
			Path:      path,
			Content:   content,
			SessionId: sessionID,
		}); err != nil {
			return fmt.Errorf("client fs write: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// buildPrompt converts ACP content blocks into a plain-text prompt plus any
// binary attachments (images/audio) the model can consume.
func buildPrompt(blocks []acp.ContentBlock) (string, []message.Attachment) {
	var sb strings.Builder
	var attachments []message.Attachment
	for _, b := range blocks {
		switch {
		case b.Text != nil:
			sb.WriteString(b.Text.Text)
		case b.Image != nil:
			if data, err := base64.StdEncoding.DecodeString(b.Image.Data); err == nil {
				attachments = append(attachments, message.Attachment{
					Content:  data,
					MimeType: b.Image.MimeType,
					FileName: "image",
				})
			}
		case b.Audio != nil:
			if data, err := base64.StdEncoding.DecodeString(b.Audio.Data); err == nil {
				attachments = append(attachments, message.Attachment{
					Content:  data,
					MimeType: b.Audio.MimeType,
					FileName: "audio",
				})
			}
		case b.ResourceLink != nil:
			sb.WriteString("[resource: " + b.ResourceLink.Name + ": " + b.ResourceLink.Uri + "]")
		case b.Resource != nil && b.Resource.Resource.TextResourceContents != nil:
			sb.WriteString("[resource: " + b.Resource.Resource.TextResourceContents.Uri + ": " + b.Resource.Resource.TextResourceContents.Text + "]")
		case b.Resource != nil && b.Resource.Resource.BlobResourceContents != nil:
			blob := b.Resource.Resource.BlobResourceContents
			data, err := base64.StdEncoding.DecodeString(blob.Blob)
			if err != nil {
				sb.WriteString("[resource: " + blob.Uri + ": undecodable blob]")
				break
			}
			mimeType := ""
			if blob.MimeType != nil {
				mimeType = *blob.MimeType
			}
			if mediaType, ok := mediaAttachmentType(mimeType, data); ok {
				attachments = append(attachments, message.Attachment{
					Content:  data,
					MimeType: mediaType,
					FileName: "resource",
				})
				break
			}
			// Not media the model can consume, so describe it rather than
			// dropping the resource on the floor (content.md:148-160).
			label := mimeType
			if label == "" {
				label = "unknown type"
			}
			sb.WriteString(fmt.Sprintf("[resource: %s: %d bytes of %s]", blob.Uri, len(data), label))
		}
	}
	return sb.String(), attachments
}

// mediaAttachmentType reports the MIME type to attach a blob as, when the blob
// is media the model can consume. A blob with no declared type is sniffed, so
// an embedded image is not silently downgraded to a text placeholder.
func mediaAttachmentType(declared string, data []byte) (string, bool) {
	mimeType := declared
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	if strings.HasPrefix(mimeType, "image/") || strings.HasPrefix(mimeType, "audio/") {
		return mimeType, true
	}
	return "", false
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

// withTraceMeta returns a copy of the prompt's _meta carrying a W3C
// traceparent when the client did not send one, so the turn and every update
// of it share a correlatable id (extensibility.md:40-44 reserves the
// traceparent/tracestate/baggage keys for exactly this). Crush has no tracer
// backend; the value is a well-formed id a client can correlate against,
// not a span join.
func withTraceMeta(meta acp.Meta) acp.Meta {
	out := make(acp.Meta, len(meta)+1)
	for k, v := range meta {
		out[k] = v
	}
	if _, ok := out["traceparent"]; !ok {
		if tp := newTraceparent(); tp != "" {
			out.Set("traceparent", tp)
		}
	}
	return out
}

// newTraceparent renders a version-00 traceparent with a random trace and
// span id and the sampled flag set, per the W3C trace-context grammar.
func newTraceparent() string {
	var traceID [16]byte
	var spanID [8]byte
	if _, err := rand.Read(traceID[:]); err != nil {
		return ""
	}
	if _, err := rand.Read(spanID[:]); err != nil {
		return ""
	}
	return fmt.Sprintf("00-%s-%s-01", hex.EncodeToString(traceID[:]), hex.EncodeToString(spanID[:]))
}
