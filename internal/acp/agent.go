package acp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
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
	// activeMode records the mode each session selected, so mode state and the
	// "mode" config option agree. Nothing here switches the agent: see plan
	// O25/Q3, a switch is workspace-wide, not per session.
	activeMode sync.Map
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
				Close:  &acp.SessionCloseCapabilities{},
				Delete: &acp.SessionDeleteCapabilities{},
				List:   &acp.SessionListCapabilities{},
				Resume: &acp.SessionResumeCapabilities{},
			},
			McpCapabilities: acp.McpCapabilities{},
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
	a.log.Info("ACP new session", "sessionId", session.ID, "cwd", req.Cwd)
	sid := acp.SessionId(session.ID)

	// Additional workspace roots are not supported: the capability is not
	// advertised, so the request field is ignored rather than half-applied.
	// See plan O23 for the enforcement work this replaces.

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
// started in. The spec makes cwd part of the session's root set and the base
// for relative paths (session-setup.md:437-444), and Crush serves exactly one
// workspace per process, so a mismatch cannot be honoured.
func (a *crushAgent) validateCwd(cwd string) error {
	if cwd == "" {
		return nil
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

// ValidateCwd exposes validateCwd to the dispatcher, which owns session/load.
func (a *crushAgent) ValidateCwd(cwd string) error {
	return a.validateCwd(cwd)
}

// SessionState reports the mode and config-option state for a session, so
// session/load can return the same setup payload session/new and session/resume
// do (session-setup.md:200 allows it; the client needs it to render the
// session it just loaded).
func (a *crushAgent) SessionState(sessionID acp.SessionId) (*acp.SessionModeState, []acp.SessionConfigOption) {
	return a.modeState(sessionID), a.configOptions(sessionID)
}

// SessionExists reports whether the store knows the session. The dispatcher
// uses it for the -32002 check on session/load, which it serves itself.
func (a *crushAgent) SessionExists(ctx context.Context, sessionID acp.SessionId) bool {
	return a.requireSession(ctx, sessionID) == nil
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
			SessionId: acp.SessionId(s.ID),
			Title:     &t,
			Cwd:       cwd,
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
	a.app.AgentCoordinator.Cancel(id)
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
	if err := a.requireSession(ctx, req.SessionId); err != nil {
		return acp.ResumeSessionResponse{}, err
	}
	// The request's additional roots are not applied: the capability is not
	// advertised, so any list the client sends is left untouched.
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

// UnstableDeleteSession deletes a session from the store. The method name keeps
// the SDK's unstable prefix for the session/delete capability.
//
// Deleting a session that never existed is a success, not an error
// (session-delete.md:85). The store reports a missing row as sql.ErrNoRows,
// which is the only case swallowed here; a real failure still surfaces.
func (a *crushAgent) UnstableDeleteSession(ctx context.Context, req acp.UnstableDeleteSessionRequest) (acp.UnstableDeleteSessionResponse, error) {
	id := string(req.SessionId)
	a.log.Info("ACP delete session", "sessionId", id)
	if a.app.AgentCoordinator != nil {
		a.app.AgentCoordinator.Cancel(id)
	}
	if a.app.Sessions != nil {
		if err := a.app.Sessions.Delete(ctx, id); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return acp.UnstableDeleteSessionResponse{}, fmt.Errorf("delete session: %w", err)
		}
	}
	a.activeMode.Delete(id)
	return acp.UnstableDeleteSessionResponse{}, nil
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

	// Record any _meta the client sent so it is echoed on session/update
	// notifications streamed for this turn (S7).
	if a.bridge != nil {
		a.bridge.SetPromptMeta(sessionID, req.Meta)
	}

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
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled, Meta: req.Meta}, nil
		}
		return acp.PromptResponse{}, fmt.Errorf("run prompt: %w", err)
	}

	stopReason := mapFinishReason(result.Response.FinishReason)
	a.log.Info("ACP prompt done", "sessionId", sessionID, "stopReason", stopReason)
	return acp.PromptResponse{StopReason: stopReason, Meta: req.Meta}, nil
}

// requireSession reports -32002 when the named session does not exist. Callers
// treat a nil error as "go ahead"; a store error other than not-found is
// reported as-is rather than masked as a missing session.
func (a *crushAgent) requireSession(ctx context.Context, id acp.SessionId) error {
	if a.app == nil || a.app.Sessions == nil || id == "" {
		return nil
	}
	if _, err := a.app.Sessions.Get(ctx, string(id)); err != nil {
		// The SDK ships no constructor for -32002, so build the RequestError
		// directly.
		return &acp.RequestError{
			Code:    acpErrorResourceNotFound,
			Message: "Resource not found",
			Data:    map[string]any{"sessionId": string(id)},
		}
	}
	return nil
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
	if cfg := a.config(); cfg != nil {
		if !isSelectableMode(cfg, string(req.ModeId)) {
			return acp.SetSessionModeResponse{}, invalidParam("modeId", string(req.ModeId))
		}
	}
	a.setMode(ctx, req.SessionId, req.ModeId)
	return acp.SetSessionModeResponse{}, nil
}

// setMode records the session's selected mode and announces it. Both
// session/set_mode and the "mode" config option funnel through here so the two
// entry points can never disagree.
func (a *crushAgent) setMode(ctx context.Context, sessionID acp.SessionId, modeID acp.SessionModeId) {
	a.activeMode.Store(string(sessionID), modeID)
	a.pushUpdate(ctx, sessionID, acp.SessionUpdate{
		CurrentModeUpdate: &acp.SessionCurrentModeUpdate{CurrentModeId: modeID},
	})
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
	if req.Boolean != nil && req.Boolean.ConfigId == "thinking" {
		if err := a.setThinking(ctx, req.Boolean.Value); err != nil {
			a.log.Warn("Failed to apply thinking config option", "error", err)
		}
	}
	if req.ValueId != nil && req.ValueId.ConfigId == "mode" {
		a.setMode(ctx, sessionID, acp.SessionModeId(req.ValueId.Value))
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
			if !isSelectableMode(cfg, value) {
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
			Title:     &title,
			UpdatedAt: &now,
		},
	}
}

// configOptions advertises the session's selectable modes, the available models
// and the thinking toggle, so the client can render a mode switcher, a model
// picker and a reasoning on/off control.
func (a *crushAgent) configOptions(sessionID acp.SessionId) []acp.SessionConfigOption {
	cfg := a.config()
	return configOptionsFor(cfg, a.currentMode(cfg, sessionID))
}

// configOptionsFor builds the config-option list from a config snapshot and the
// session's active mode.
//
// The "mode" select is emitted alongside `modes` because the spec makes config
// options the modern spelling of a mode switcher and asks Agents that offer
// mode-like configuration to send both during the transition
// (session-config-options.md:462-471).
func configOptionsFor(cfg *config.Config, currentMode acp.SessionModeId) []acp.SessionConfigOption {
	opts := []acp.SessionConfigOption{}
	if cfg == nil {
		return opts
	}
	if modeOpts := modeSelectOptions(cfg); len(modeOpts) > 0 {
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
				Description:  acp.Ptr("Switch between Crush's agents"),
				Options: acp.SessionConfigSelectOptions{
					Ungrouped: &ungroupedModes,
				},
			},
		})
	}
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

// modeSelectOptions renders the selectable agents as config-option values.
func modeSelectOptions(cfg *config.Config) []acp.SessionConfigSelectOption {
	names := selectableModes(cfg)
	opts := make([]acp.SessionConfigSelectOption, 0, len(names))
	for _, name := range names {
		agent := cfg.Agents[name]
		opt := acp.SessionConfigSelectOption{
			Value: acp.SessionConfigValueId(name),
			Name:  name,
		}
		if agent.Name != "" {
			opt.Name = agent.Name
		}
		if agent.Description != "" {
			desc := agent.Description
			opt.Description = &desc
		}
		opts = append(opts, opt)
	}
	return opts
}

// selectableModes lists the agent ids a client may switch the session to,
// sorted so the advertised order is stable across requests.
//
// config.AgentTask is excluded on purpose: it is the profile a subagent runs
// under (internal/agent/agent.go agentTool), with a reduced tool set and no
// MCP or LSP access. Offering it as a session mode would let a client degrade
// its own session by selecting it.
func selectableModes(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Agents))
	for id, agent := range cfg.Agents {
		if agent.Disabled || id == config.AgentTask {
			continue
		}
		names = append(names, id)
	}
	sort.Strings(names)
	return names
}

// isSelectableMode reports whether id names an agent a session may switch to.
func isSelectableMode(cfg *config.Config, id string) bool {
	if id == "" {
		return false
	}
	return slices.Contains(selectableModes(cfg), id)
}

// defaultMode picks the mode a session starts in: the coder agent when it is
// available, otherwise the first selectable agent. It never returns a mode that
// is absent from the advertised list (session-modes.md:58-64).
func defaultMode(cfg *config.Config) acp.SessionModeId {
	names := selectableModes(cfg)
	if len(names) == 0 {
		return ""
	}
	if slices.Contains(names, config.AgentCoder) {
		return acp.SessionModeId(config.AgentCoder)
	}
	return acp.SessionModeId(names[0])
}

// currentMode reports the mode the session selected, falling back to the
// default. A stored value that is no longer selectable is ignored, so a mode
// disabled in config cannot linger in the advertised state.
func (a *crushAgent) currentMode(cfg *config.Config, sessionID acp.SessionId) acp.SessionModeId {
	fallback := defaultMode(cfg)
	if sessionID == "" {
		return fallback
	}
	stored, ok := a.activeMode.Load(string(sessionID))
	if !ok {
		return fallback
	}
	mode, ok := stored.(acp.SessionModeId)
	if !ok || !isSelectableMode(cfg, string(mode)) {
		return fallback
	}
	return mode
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
func (a *crushAgent) modeState(sessionID acp.SessionId) *acp.SessionModeState {
	cfg := a.config()
	if cfg == nil {
		return nil
	}
	names := selectableModes(cfg)
	modes := make([]acp.SessionMode, 0, len(names))
	for _, id := range names {
		agent := cfg.Agents[id]
		mode := acp.SessionMode{Id: acp.SessionModeId(id), Name: id}
		if agent.Name != "" {
			mode.Name = agent.Name
		}
		if agent.Description != "" {
			desc := agent.Description
			mode.Description = &desc
		}
		modes = append(modes, mode)
	}
	return &acp.SessionModeState{
		AvailableModes: modes,
		CurrentModeId:  a.currentMode(cfg, sessionID),
	}
}

// firstLine returns the first non-empty line of s, trimmed.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// fsReadTextFile reads a text file. When the connected client advertises fs
// support the read is delegated to the client; otherwise Crush reads it from its
// own local filesystem. The line/limit windowing supported by the client path
// is applied only on delegation; local reads return the whole file.
func (a *crushAgent) fsReadTextFile(ctx context.Context, sessionID acp.SessionId, path string, line, limit *int) (string, error) {
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
