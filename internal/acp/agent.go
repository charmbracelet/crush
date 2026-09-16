package acp

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/commands"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	acp "github.com/coder/acp-go-sdk"
)

// crushAgent implements acp.Agent using Crush's internal services.
type crushAgent struct {
	app  *app.App
	log  *slog.Logger
	conn *acp.AgentSideConnection

	// clientCaps holds the capabilities the connected client advertised during
	// initialize. Used to gate optional server track features (fs).
	clientCaps acp.ClientCapabilities
	// additionalDirs records per-session additional roots accepted on
	// session/new so they can be echoed back via session/list.
	additionalDirs sync.Map
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
	sid := acp.SessionId(session.ID)

	// Record the additional roots so session/list can echo them back. Crush has
	// no multi-root model, so they are persisted for round-tripping only.
	if len(req.AdditionalDirectories) > 0 {
		dirs := make([]string, len(req.AdditionalDirectories))
		copy(dirs, req.AdditionalDirectories)
		a.additionalDirs.Store(session.ID, dirs)
	}

	// Advertise setup state the client needs before the first prompt. These
	// notifications are sent before the response returns, so the client's
	// request barrier (which waits for prior notifications) satisfies them
	// inline, exactly like streaming during a prompt.
	a.pushUpdate(ctx, sid, a.availableCommandsUpdate())
	a.pushUpdate(ctx, sid, a.sessionInfoUpdate(session.Title))

	return acp.NewSessionResponse{
		SessionId:     sid,
		Modes:         a.modeState(),
		ConfigOptions: a.configOptions(),
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
	cwd := a.workingDir()
	for _, s := range sessions {
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
		if dirs, ok := a.additionalDirs.Load(s.ID); ok {
			info.AdditionalDirectories = dirs.([]string)
		}
		out = append(out, info)
	}
	return acp.ListSessionsResponse{Sessions: out}, nil
}

// CloseSession implements acp.Agent.
func (a *crushAgent) CloseSession(_ context.Context, req acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	id := string(req.SessionId)
	a.app.AgentCoordinator.Cancel(id)
	return acp.CloseSessionResponse{}, nil
}

// ResumeSession is not supported. Per the spec a resume capability that is not
// advertised must return an error; Crush continues any session id via
// session/prompt, so resume is intentionally left unimplemented.
func (a *crushAgent) ResumeSession(_ context.Context, _ acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, fmt.Errorf("session/resume is not supported")
}

// Logout terminates the current authentication session. Crush has no auth
// session, so this is a successful no-op.
func (a *crushAgent) Logout(_ context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	a.log.Info("ACP logout")
	return acp.LogoutResponse{}, nil
}

// UnstableDeleteSession deletes a session from the store. The method name keeps
// the SDK's unstable prefix for the session/delete capability.
func (a *crushAgent) UnstableDeleteSession(ctx context.Context, req acp.UnstableDeleteSessionRequest) (acp.UnstableDeleteSessionResponse, error) {
	id := string(req.SessionId)
	a.log.Info("ACP delete session", "sessionId", id)
	if a.app.AgentCoordinator != nil {
		a.app.AgentCoordinator.Cancel(id)
	}
	if a.app.Sessions != nil {
		if err := a.app.Sessions.Delete(ctx, id); err != nil {
			return acp.UnstableDeleteSessionResponse{}, fmt.Errorf("delete session: %w", err)
		}
	}
	a.additionalDirs.Delete(id)
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

	prompt, attachments := buildPrompt(req.Prompt)
	a.log.Info("ACP prompt", "sessionId", sessionID, "promptLen", len(prompt))

	result, err := a.app.AgentCoordinator.Run(ctx, sessionID, prompt, attachments...)
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

// SetSessionMode acknowledges a mode change and notifies the client.
func (a *crushAgent) SetSessionMode(ctx context.Context, req acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	a.log.Info("ACP set session mode", "sessionId", req.SessionId, "modeId", req.ModeId)
	a.pushUpdate(ctx, req.SessionId, acp.SessionUpdate{
		CurrentModeUpdate: &acp.SessionCurrentModeUpdate{CurrentModeId: req.ModeId},
	})
	return acp.SetSessionModeResponse{}, nil
}

// SetSessionConfigOption acknowledges a configuration change and notifies the
// client of the resulting config options.
func (a *crushAgent) SetSessionConfigOption(ctx context.Context, req acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	sessionID := configOptionSessionID(req)
	if req.Boolean != nil && req.Boolean.ConfigId == "thinking" {
		if err := a.setThinking(ctx, req.Boolean.Value); err != nil {
			a.log.Warn("Failed to apply thinking config option", "error", err)
		}
	}
	a.log.Info("ACP set session config option", "sessionId", sessionID)
	a.pushUpdate(ctx, sessionID, a.configOptionsUpdate())
	return acp.SetSessionConfigOptionResponse{ConfigOptions: a.configOptions()}, nil
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

// availableCommandsUpdate advertises the slash commands Crush can run.
func (a *crushAgent) availableCommandsUpdate() acp.SessionUpdate {
	cmds := []acp.AvailableCommand{}
	if cfg := a.config(); cfg != nil {
		if custom, err := commands.LoadCustomCommands(cfg); err == nil {
			for _, c := range custom {
				cmds = append(cmds, acp.AvailableCommand{
					Name:        c.Name,
					Description: firstLine(c.Content),
				})
			}
		}
	}
	return acp.SessionUpdate{
		AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
			AvailableCommands: cmds,
		},
	}
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

// configOptions advertises the available session modes as a select option and a
// boolean "thinking" toggle so the client can render a mode switcher and a
// reasoning on/off control.
func (a *crushAgent) configOptions() []acp.SessionConfigOption {
	return configOptionsFor(a.config())
}

// configOptionsFor builds the config-option list from a config snapshot.
func configOptionsFor(cfg *config.Config) []acp.SessionConfigOption {
	opts := []acp.SessionConfigOption{}
	if cfg == nil {
		return opts
	}
	opts = append(opts, acp.SessionConfigOption{
		Boolean: &acp.SessionConfigOptionBoolean{
			Id:           "thinking",
			Name:         "Thinking",
			Type:         "boolean",
			CurrentValue: thinkingEnabled(cfg),
			Description:  acp.Ptr("Enable extended reasoning for supported models"),
		},
	})
	selectOpts := []acp.SessionConfigSelectOption{}
	for id, agent := range cfg.Agents {
		if agent.Disabled {
			continue
		}
		desc := agent.Description
		selectOpts = append(selectOpts, acp.SessionConfigSelectOption{
			Value:       acp.SessionConfigValueId(id),
			Name:        agent.Name,
			Description: &desc,
		})
	}
	ungrouped := acp.SessionConfigSelectOptionsUngrouped(selectOpts)
	opts = append(opts, acp.SessionConfigOption{
		Select: &acp.SessionConfigOptionSelect{
			Id:           "mode",
			Name:         "Mode",
			Type:         "select",
			CurrentValue: "coder",
			Options: acp.SessionConfigSelectOptions{
				Ungrouped: &ungrouped,
			},
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
				Options: acp.SessionConfigSelectOptions{
					Ungrouped: &ungroupedModels,
				},
			},
		})
	}
	return opts
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
func (a *crushAgent) configOptionsUpdate() acp.SessionUpdate {
	return acp.SessionUpdate{
		ConfigOptionUpdate: &acp.SessionConfigOptionUpdate{ConfigOptions: a.configOptions()},
	}
}

// modeState reports the available session modes and the active one.
func (a *crushAgent) modeState() *acp.SessionModeState {
	if cfg := a.config(); cfg != nil {
		modes := make([]acp.SessionMode, 0, len(cfg.Agents))
		for id, agent := range cfg.Agents {
			if agent.Disabled {
				continue
			}
			desc := agent.Description
			modes = append(modes, acp.SessionMode{
				Id:          acp.SessionModeId(id),
				Name:        agent.Name,
				Description: &desc,
			})
		}
		return &acp.SessionModeState{
			AvailableModes: modes,
			CurrentModeId:  "coder",
		}
	}
	return nil
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
		}
	}
	return sb.String(), attachments
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
