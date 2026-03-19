package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
)

const (
	commandPluginTypeCommand       = "command"
	commandPluginHookMessages      = "chat_messages_transform"
	commandPluginHookSystem        = "chat_system_transform"
	commandPluginHookCompacting    = "session_compacting"
	commandPluginProtocolVersion   = 1
	commandPluginOutputMaxBytes    = 1 << 20
	commandPluginPartReasoning     = "reasoning"
	commandPluginPartText          = "text"
	commandPluginPartImageURL      = "image_url"
	commandPluginPartBinary        = "binary"
	commandPluginPartToolCall      = "tool_call"
	commandPluginPartToolResult    = "tool_result"
	commandPluginPartFinish        = "finish"
	commandPluginTruncatedSuffix   = "\n... [output truncated]"
	commandPluginTruncatedJSONStub = `{"error":"plugin output truncated"}`
)

var supportedCommandPluginHooks = []string{
	commandPluginHookMessages,
	commandPluginHookSystem,
	commandPluginHookCompacting,
}

type resolvedCommandPluginConfig struct {
	name    string
	command string
	args    []string
	env     []string
	cwd     string
	hooks   []string
	timeout int
}

type commandPlugin struct {
	cfg resolvedCommandPluginConfig
}

type commandPluginRequest struct {
	Version int             `json:"version"`
	Event   string          `json:"event"`
	Input   json.RawMessage `json:"input"`
	Output  json.RawMessage `json:"output"`
}

type commandPluginResponse struct {
	Output json.RawMessage `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
}

type commandPluginMessage struct {
	ID               string              `json:"id,omitempty"`
	Role             string              `json:"role"`
	SessionID        string              `json:"session_id,omitempty"`
	Model            string              `json:"model,omitempty"`
	Provider         string              `json:"provider,omitempty"`
	CreatedAt        int64               `json:"created_at,omitempty"`
	UpdatedAt        int64               `json:"updated_at,omitempty"`
	IsSummaryMessage bool                `json:"is_summary_message,omitempty"`
	Parts            []commandPluginPart `json:"parts,omitempty"`
}

type commandPluginPart struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type commandPluginChatMessagesTransformInput struct {
	SessionID string               `json:"session_id"`
	Agent     string               `json:"agent"`
	Model     ModelInfo            `json:"model"`
	Provider  ProviderContext      `json:"provider"`
	Purpose   string               `json:"purpose"`
	Message   commandPluginMessage `json:"message"`
}

type commandPluginChatMessagesTransformOutput struct {
	Messages []commandPluginMessage `json:"messages"`
}

type commandPluginChatSystemTransformInput struct {
	SessionID string               `json:"session_id"`
	Agent     string               `json:"agent"`
	Model     ModelInfo            `json:"model"`
	Provider  ProviderContext      `json:"provider"`
	Purpose   string               `json:"purpose"`
	Message   commandPluginMessage `json:"message"`
}

type commandPluginChatSystemTransformOutput struct {
	System []string `json:"system,omitempty"`
	Prefix string   `json:"prefix,omitempty"`
}

type commandPluginSessionCompactingInput struct {
	SessionID string    `json:"session_id"`
	Agent     string    `json:"agent"`
	Model     ModelInfo `json:"model"`
	Purpose   string    `json:"purpose"`
}

type commandPluginSessionCompactingOutput struct {
	Context []string `json:"context,omitempty"`
	Prompt  string   `json:"prompt,omitempty"`
}

type boundedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newBoundedBuffer(limit int) *boundedBuffer {
	if limit < 0 {
		limit = commandPluginOutputMaxBytes
	}
	return &boundedBuffer{limit: limit}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.limit < 0 {
		return b.buf.Write(p)
	}
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		toWrite := p
		if len(toWrite) > remaining {
			toWrite = toWrite[:remaining]
			b.truncated = true
		}
		if _, err := b.buf.Write(toWrite); err != nil {
			return 0, err
		}
	}
	if len(p) > remaining {
		b.truncated = true
	}
	return len(p), nil
}

func (b *boundedBuffer) Truncated() bool {
	return b.truncated
}

func (b *boundedBuffer) String() string {
	if b.buf.Len() == 0 {
		if b.truncated {
			return commandPluginTruncatedSuffix
		}
		return ""
	}
	s := b.buf.String()
	if b.truncated {
		return s + commandPluginTruncatedSuffix
	}
	return s
}

func (b *boundedBuffer) BytesForJSON() []byte {
	if !b.truncated {
		return b.buf.Bytes()
	}
	if b.buf.Len() == 0 {
		return []byte(commandPluginTruncatedJSONStub)
	}
	return b.buf.Bytes()
}

func newConfiguredPlugins(input PluginInput) ([]Plugin, error) {
	if input.Config == nil || input.Config.Config() == nil {
		return nil, nil
	}
	cfgs := input.Config.Config().Plugins
	if len(cfgs) == 0 {
		return nil, nil
	}
	plugins := make([]Plugin, 0, len(cfgs))
	for _, cfg := range cfgs {
		resolved, err := resolveCommandPluginConfig(input, cfg)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, &commandPlugin{cfg: resolved})
	}
	return plugins, nil
}

func resolveCommandPluginConfig(input PluginInput, cfg config.PluginConfig) (resolvedCommandPluginConfig, error) {
	if cfg.Name == "" {
		return resolvedCommandPluginConfig{}, fmt.Errorf("plugin name is required")
	}
	pluginType := cfg.Type
	if pluginType == "" {
		pluginType = commandPluginTypeCommand
	}
	if pluginType != commandPluginTypeCommand {
		return resolvedCommandPluginConfig{}, fmt.Errorf("plugin %q has unsupported type %q", cfg.Name, pluginType)
	}
	if cfg.Command == "" {
		return resolvedCommandPluginConfig{}, fmt.Errorf("plugin %q command is required", cfg.Name)
	}
	command, err := resolvePluginValue(input.Config, cfg.Command)
	if err != nil {
		return resolvedCommandPluginConfig{}, fmt.Errorf("plugin %q command: %w", cfg.Name, err)
	}
	args := make([]string, 0, len(cfg.Args))
	for _, arg := range cfg.Args {
		resolved, err := resolvePluginValue(input.Config, arg)
		if err != nil {
			return resolvedCommandPluginConfig{}, fmt.Errorf("plugin %q arg %q: %w", cfg.Name, arg, err)
		}
		args = append(args, resolved)
	}
	envPairs := make([]string, 0, len(cfg.Env)+3)
	for key, value := range cfg.Env {
		resolved, err := resolvePluginValue(input.Config, value)
		if err != nil {
			return resolvedCommandPluginConfig{}, fmt.Errorf("plugin %q env %q: %w", cfg.Name, key, err)
		}
		envPairs = append(envPairs, key+"="+resolved)
	}
	envPairs = append(envPairs,
		"CRUSH_PLUGIN_NAME="+cfg.Name,
		"CRUSH_WORKING_DIR="+input.WorkingDir,
	)
	cwd := input.WorkingDir
	if cfg.CWD != "" {
		resolved, err := resolvePluginValue(input.Config, cfg.CWD)
		if err != nil {
			return resolvedCommandPluginConfig{}, fmt.Errorf("plugin %q cwd: %w", cfg.Name, err)
		}
		if filepath.IsAbs(resolved) {
			cwd = resolved
		} else {
			cwd = filepath.Join(input.WorkingDir, resolved)
		}
	}
	hooks, err := normalizeCommandPluginHooks(cfg.Name, cfg.Hooks)
	if err != nil {
		return resolvedCommandPluginConfig{}, err
	}
	return resolvedCommandPluginConfig{
		name:    cfg.Name,
		command: command,
		args:    args,
		env:     envPairs,
		cwd:     cwd,
		hooks:   hooks,
		timeout: cfg.TimeoutMs,
	}, nil
}

func resolvePluginValue(store *config.ConfigStore, value string) (string, error) {
	if store == nil {
		return value, nil
	}
	return store.Resolver().ResolveValue(value)
}

func normalizeCommandPluginHooks(name string, hooks []string) ([]string, error) {
	if len(hooks) == 0 {
		return append([]string(nil), supportedCommandPluginHooks...), nil
	}
	normalized := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		if !slices.Contains(supportedCommandPluginHooks, hook) {
			return nil, fmt.Errorf("plugin %q hook %q is unsupported", name, hook)
		}
		if !slices.Contains(normalized, hook) {
			normalized = append(normalized, hook)
		}
	}
	return normalized, nil
}

func (p *commandPlugin) Name() string {
	return p.cfg.name
}

func (p *commandPlugin) Init(ctx context.Context, input PluginInput) (Hooks, error) {
	_ = ctx
	_ = input
	var hooks Hooks
	if slices.Contains(p.cfg.hooks, commandPluginHookMessages) {
		hooks.ChatMessagesTransform = p.chatMessagesTransform
	}
	if slices.Contains(p.cfg.hooks, commandPluginHookSystem) {
		hooks.ChatSystemTransform = p.chatSystemTransform
	}
	if slices.Contains(p.cfg.hooks, commandPluginHookCompacting) {
		hooks.SessionCompacting = p.sessionCompacting
	}
	return hooks, nil
}

func (p *commandPlugin) chatMessagesTransform(ctx context.Context, input ChatMessagesTransformInput, output *ChatMessagesTransformOutput) error {
	requestInput := commandPluginChatMessagesTransformInput{
		SessionID: input.SessionID,
		Agent:     input.Agent,
		Model:     input.Model,
		Provider:  input.Provider,
		Purpose:   string(input.Purpose),
		Message:   toCommandPluginMessage(input.Message),
	}
	requestOutput := commandPluginChatMessagesTransformOutput{
		Messages: toCommandPluginMessages(output.Messages),
	}
	var response commandPluginChatMessagesTransformOutput
	if err := p.invoke(ctx, commandPluginHookMessages, requestInput, requestOutput, &response); err != nil {
		return err
	}
	if response.Messages != nil {
		messages, err := fromCommandPluginMessages(response.Messages)
		if err != nil {
			return err
		}
		output.Messages = messages
	}
	return nil
}

func (p *commandPlugin) chatSystemTransform(ctx context.Context, input ChatSystemTransformInput, output *ChatSystemTransformOutput) error {
	requestInput := commandPluginChatSystemTransformInput{
		SessionID: input.SessionID,
		Agent:     input.Agent,
		Model:     input.Model,
		Provider:  input.Provider,
		Purpose:   string(input.Purpose),
		Message:   toCommandPluginMessage(input.Message),
	}
	requestOutput := commandPluginChatSystemTransformOutput{
		System: append([]string(nil), output.System...),
		Prefix: output.Prefix,
	}
	var response commandPluginChatSystemTransformOutput
	if err := p.invoke(ctx, commandPluginHookSystem, requestInput, requestOutput, &response); err != nil {
		return err
	}
	if response.System != nil {
		output.System = response.System
	}
	if response.Prefix != "" {
		output.Prefix = response.Prefix
	}
	return nil
}

func (p *commandPlugin) sessionCompacting(ctx context.Context, input SessionCompactingInput, output *SessionCompactingOutput) error {
	requestInput := commandPluginSessionCompactingInput{
		SessionID: input.SessionID,
		Agent:     input.Agent,
		Model:     input.Model,
		Purpose:   string(input.Purpose),
	}
	requestOutput := commandPluginSessionCompactingOutput{
		Context: append([]string(nil), output.Context...),
		Prompt:  output.Prompt,
	}
	var response commandPluginSessionCompactingOutput
	if err := p.invoke(ctx, commandPluginHookCompacting, requestInput, requestOutput, &response); err != nil {
		return err
	}
	if response.Context != nil {
		output.Context = response.Context
	}
	if response.Prompt != "" {
		output.Prompt = response.Prompt
	}
	return nil
}

func (p *commandPlugin) invoke(ctx context.Context, event string, input any, output any, responseOutput any) error {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("plugin %q marshal input: %w", p.cfg.name, err)
	}
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("plugin %q marshal output: %w", p.cfg.name, err)
	}
	requestJSON, err := json.Marshal(commandPluginRequest{
		Version: commandPluginProtocolVersion,
		Event:   event,
		Input:   inputJSON,
		Output:  outputJSON,
	})
	if err != nil {
		return fmt.Errorf("plugin %q marshal request: %w", p.cfg.name, err)
	}
	callCtx := ctx
	if p.cfg.timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, config.PluginConfig{TimeoutMs: p.cfg.timeout}.Timeout())
		defer cancel()
	}
	cmd := exec.CommandContext(callCtx, p.cfg.command, p.cfg.args...)
	cmd.Dir = p.cfg.cwd
	cmd.Env = append(os.Environ(), p.cfg.env...)
	cmd.Env = append(cmd.Env, "CRUSH_PLUGIN_EVENT="+event)
	cmd.Stdin = bytes.NewReader(requestJSON)
	stdout := newBoundedBuffer(commandPluginOutputMaxBytes)
	stderr := newBoundedBuffer(commandPluginOutputMaxBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message != "" {
			return fmt.Errorf("plugin %q event %q failed: %w: %s", p.cfg.name, event, err, message)
		}
		return fmt.Errorf("plugin %q event %q failed: %w", p.cfg.name, event, err)
	}
	stdoutJSON := stdout.BytesForJSON()
	var response commandPluginResponse
	if err := json.Unmarshal(stdoutJSON, &response); err != nil {
		if stdout.Truncated() {
			return fmt.Errorf("plugin %q event %q returned invalid json: output exceeded %d bytes and was truncated", p.cfg.name, event, commandPluginOutputMaxBytes)
		}
		return fmt.Errorf("plugin %q event %q returned invalid json: %w", p.cfg.name, event, err)
	}
	if response.Error != "" {
		return fmt.Errorf("plugin %q event %q failed: %s", p.cfg.name, event, response.Error)
	}
	if len(response.Output) == 0 {
		return nil
	}
	if err := json.Unmarshal(response.Output, responseOutput); err != nil {
		return fmt.Errorf("plugin %q event %q returned invalid output: %w", p.cfg.name, event, err)
	}
	return nil
}

func toCommandPluginMessages(messages []message.Message) []commandPluginMessage {
	converted := make([]commandPluginMessage, len(messages))
	for i := range messages {
		converted[i] = toCommandPluginMessage(messages[i])
	}
	return converted
}

func toCommandPluginMessage(msg message.Message) commandPluginMessage {
	parts := make([]commandPluginPart, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		converted, ok := toCommandPluginPart(part)
		if ok {
			parts = append(parts, converted)
		}
	}
	return commandPluginMessage{
		ID:               msg.ID,
		Role:             string(msg.Role),
		SessionID:        msg.SessionID,
		Model:            msg.Model,
		Provider:         msg.Provider,
		CreatedAt:        msg.CreatedAt,
		UpdatedAt:        msg.UpdatedAt,
		IsSummaryMessage: msg.IsSummaryMessage,
		Parts:            parts,
	}
}

func toCommandPluginPart(part message.ContentPart) (commandPluginPart, bool) {
	var typ string
	switch part.(type) {
	case message.ReasoningContent:
		typ = commandPluginPartReasoning
	case message.TextContent:
		typ = commandPluginPartText
	case message.ImageURLContent:
		typ = commandPluginPartImageURL
	case message.BinaryContent:
		typ = commandPluginPartBinary
	case message.ToolCall:
		typ = commandPluginPartToolCall
	case message.ToolResult:
		typ = commandPluginPartToolResult
	case message.Finish:
		typ = commandPluginPartFinish
	default:
		return commandPluginPart{}, false
	}
	data, err := json.Marshal(part)
	if err != nil {
		return commandPluginPart{}, false
	}
	return commandPluginPart{Type: typ, Data: data}, true
}

func fromCommandPluginMessages(messages []commandPluginMessage) ([]message.Message, error) {
	converted := make([]message.Message, len(messages))
	for i := range messages {
		parts, err := fromCommandPluginParts(messages[i].Parts)
		if err != nil {
			return nil, err
		}
		converted[i] = message.Message{
			ID:               messages[i].ID,
			Role:             message.MessageRole(messages[i].Role),
			SessionID:        messages[i].SessionID,
			Parts:            parts,
			Model:            messages[i].Model,
			Provider:         messages[i].Provider,
			CreatedAt:        messages[i].CreatedAt,
			UpdatedAt:        messages[i].UpdatedAt,
			IsSummaryMessage: messages[i].IsSummaryMessage,
		}
	}
	return converted, nil
}

func fromCommandPluginParts(parts []commandPluginPart) ([]message.ContentPart, error) {
	converted := make([]message.ContentPart, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case commandPluginPartReasoning:
			var data message.ReasoningContent
			if err := json.Unmarshal(part.Data, &data); err != nil {
				return nil, err
			}
			converted = append(converted, data)
		case commandPluginPartText:
			var data message.TextContent
			if err := json.Unmarshal(part.Data, &data); err != nil {
				return nil, err
			}
			converted = append(converted, data)
		case commandPluginPartImageURL:
			var data message.ImageURLContent
			if err := json.Unmarshal(part.Data, &data); err != nil {
				return nil, err
			}
			converted = append(converted, data)
		case commandPluginPartBinary:
			var data message.BinaryContent
			if err := json.Unmarshal(part.Data, &data); err != nil {
				return nil, err
			}
			converted = append(converted, data)
		case commandPluginPartToolCall:
			var data message.ToolCall
			if err := json.Unmarshal(part.Data, &data); err != nil {
				return nil, err
			}
			converted = append(converted, data)
		case commandPluginPartToolResult:
			var data message.ToolResult
			if err := json.Unmarshal(part.Data, &data); err != nil {
				return nil, err
			}
			converted = append(converted, data)
		case commandPluginPartFinish:
			var data message.Finish
			if err := json.Unmarshal(part.Data, &data); err != nil {
				return nil, err
			}
			converted = append(converted, data)
		default:
			return nil, fmt.Errorf("unsupported plugin part type %q", part.Type)
		}
	}
	return converted, nil
}
