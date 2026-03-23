package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type commandHandler struct {
	cfg HookConfig
}

func newCommandHandler(cfg HookConfig) Handler {
	return &commandHandler{cfg: cfg}
}

func (h *commandHandler) Execute(ctx context.Context, input HookInput) (*HookOutput, error) {
	cmdCfg := h.cfg.Command

	if cmdCfg.Passthrough {
		return h.executePassthrough(ctx, input)
	}
	return h.executeJSON(ctx, input)
}

func (h *commandHandler) executePassthrough(ctx context.Context, input HookInput) (*HookOutput, error) {
	cmdCfg := h.cfg.Command
	field := cmdCfg.PassthroughField
	if field == "" {
		field = "command"
	}

	fieldValue, _ := input.ToolInput[field].(string)
	if fieldValue == "" {
		return &HookOutput{Decision: DecisionAllow}, nil
	}

	args := append(append([]string{}, cmdCfg.Args...), fieldValue)
	cmd := exec.CommandContext(ctx, cmdCfg.Command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("command execution failed: %w (stderr: %s)", err, stderr.String())
	}

	result := strings.TrimRight(stdout.String(), "\r\n")
	if result == "" {
		return &HookOutput{Decision: DecisionAllow}, nil
	}

	return &HookOutput{
		Decision: DecisionModify,
		ModifiedInput: map[string]any{
			field: result,
		},
		FallbackOnError: true,
		FallbackInput: map[string]any{
			field: fieldValue,
		},
	}, nil
}

func (h *commandHandler) executeJSON(ctx context.Context, input HookInput) (*HookOutput, error) {
	cmdCfg := h.cfg.Command

	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hook input: %w", err)
	}

	args := append([]string{}, cmdCfg.Args...)
	cmd := exec.CommandContext(ctx, cmdCfg.Command, args...)
	cmd.Stdin = bytes.NewReader(payload)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command execution failed: %w (stderr: %s)", err, stderr.String())
	}

	out := stdout.Bytes()
	if len(out) == 0 {
		return &HookOutput{Decision: DecisionAllow}, nil
	}

	var output HookOutput
	if err := json.Unmarshal(out, &output); err != nil {
		return nil, fmt.Errorf("failed to parse hook output: %w (output: %s)", err, string(out))
	}

	if output.Decision == "" {
		output.Decision = DecisionAllow
	}
	return &output, nil
}
