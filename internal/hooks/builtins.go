package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"mvdan.cc/sh/v3/interp"
)

// crushGetInput reads a field from the hook context JSON.
// Usage: VALUE=$(crush_get_input "field_name")
func crushGetInput(ctx context.Context, args []string) error {
	hc := interp.HandlerCtx(ctx)

	if len(args) != 2 {
		fmt.Fprintln(hc.Stderr, "Usage: crush_get_input <field_name>")
		return interp.ExitStatus(1)
	}

	fieldName := args[1]
	stdin := hc.Env.Get("_CRUSH_STDIN").Str

	var data map[string]any
	if err := json.Unmarshal([]byte(stdin), &data); err != nil {
		fmt.Fprintf(hc.Stderr, "crush_get_input: failed to parse JSON: %v\n", err)
		return interp.ExitStatus(1)
	}

	if value, ok := data[fieldName]; ok && value != nil {
		fmt.Fprint(hc.Stdout, formatJSONValue(value))
	}

	return nil
}

// crushGetToolInput reads a tool input parameter from the hook context JSON.
// Usage: COMMAND=$(crush_get_tool_input "command")
func crushGetToolInput(ctx context.Context, args []string) error {
	hc := interp.HandlerCtx(ctx)
	if len(args) != 2 {
		fmt.Fprintln(hc.Stderr, "Usage: crush_get_tool_input <param_name>")
		return interp.ExitStatus(1)
	}

	paramName := args[1]
	stdin := hc.Env.Get("_CRUSH_STDIN").Str

	var data map[string]any
	if err := json.Unmarshal([]byte(stdin), &data); err != nil {
		fmt.Fprintf(hc.Stderr, "crush_get_tool_input: failed to parse JSON: %v\n", err)
		return interp.ExitStatus(1)
	}

	toolInput, ok := data["tool_input"].(map[string]any)
	if !ok {
		return nil
	}

	if value, ok := toolInput[paramName]; ok && value != nil {
		fmt.Fprint(hc.Stdout, formatJSONValue(value))
	}

	return nil
}

// crushGetPrompt reads the user prompt from the hook context JSON.
// Usage: PROMPT=$(crush_get_prompt)
func crushGetPrompt(ctx context.Context, args []string) error {
	hc := interp.HandlerCtx(ctx)

	stdin := hc.Env.Get("_CRUSH_STDIN").Str

	var data map[string]any
	if err := json.Unmarshal([]byte(stdin), &data); err != nil {
		fmt.Fprintf(hc.Stderr, "crush_get_prompt: failed to parse JSON: %v\n", err)
		return interp.ExitStatus(1)
	}

	if prompt, ok := data["prompt"]; ok && prompt != nil {
		fmt.Fprint(hc.Stdout, formatJSONValue(prompt))
	}

	return nil
}

// crushLog writes a log message using slog.Debug.
// Usage: crush_log "debug message"
func crushLog(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return nil
	}

	slog.Debug(joinArgs(args[1:]))
	return nil
}

// formatJSONValue converts a JSON value to a string suitable for shell output.
func formatJSONValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		// JSON numbers are float64 by default
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return ""
	default:
		// For complex types (arrays, objects), return JSON representation
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// joinArgs joins arguments with spaces.
func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	result := args[0]
	for _, arg := range args[1:] {
		result += " " + arg
	}
	return result
}

// RegisterBuiltins returns an ExecHandlerFunc that registers all Crush hook builtins.
func RegisterBuiltins(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	builtins := map[string]func(context.Context, []string) error{
		"crush_get_input":      crushGetInput,
		"crush_get_tool_input": crushGetToolInput,
		"crush_get_prompt":     crushGetPrompt,
		"crush_log":            crushLog,
	}

	return func(ctx context.Context, args []string) error {
		if len(args) == 0 {
			return next(ctx, args)
		}

		if fn, ok := builtins[args[0]]; ok {
			return fn(ctx, args)
		}

		return next(ctx, args)
	}
}
