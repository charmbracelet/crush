package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

// handleAutoMode implements the `auto_mode` builtin for the Bash-style
// config format.
//
// Usage:
//
//	auto_mode on|off
//	auto_mode model <provider>/<id>
//	auto_mode max-tokens N
//	auto_mode timeout N
//	auto_mode fail-open true|false
//	auto_mode environment "prose describing trusted repos/domains"
//	auto_mode prompt stage1|stage2 <file>
//	auto_mode transcript-max-chars N
//	auto_mode max-consecutive-denials N
//	auto_mode max-total-denials N
//
// It populates the auto_mode section of the JSON config, which the
// native auto mode reads (see config.AutoModeConfig).
func handleAutoMode(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: auto_mode on|off | auto_mode model <provider>/<id> | auto_mode <setting> <value>")
	}
	section := b.section("auto_mode")

	switch args[1] {
	case "on":
		section["enabled"] = true
		slog.Info("Auto mode enabled in shell config")
		return nil
	case "off":
		section["enabled"] = false
		slog.Info("Auto mode disabled in shell config")
		return nil
	case "model":
		if len(args) < 3 {
			return usage(stderr, "usage: auto_mode model <provider>/<id>")
		}
		provider, id, ok := splitProviderModel(args[2])
		if !ok {
			return usage(stderr, fmt.Sprintf("auto_mode model: expected <provider>/<id>, got %q", args[2]))
		}
		classifier := childMap(section, "classifier")
		classifier["provider"] = provider
		classifier["model"] = id
		slog.Info("Auto mode classifier selected in shell config", "provider", provider, "model", id)
		return nil
	case "environment":
		if len(args) < 3 {
			return usage(stderr, "usage: auto_mode environment \"prose\"")
		}
		section["environment"] = appendArr(section, "environment", strings.Join(args[2:], " "))
		slog.Info("Auto mode environment entry added in shell config")
		return nil
	case "prompt":
		if len(args) < 4 {
			return usage(stderr, "usage: auto_mode prompt stage1|stage2 <file>")
		}
		stage := args[2]
		if stage != "stage1" && stage != "stage2" {
			return usage(stderr, fmt.Sprintf("auto_mode prompt: expected stage1 or stage2, got %q", stage))
		}
		section["prompt_"+stage+"_file"] = args[3]
		slog.Info("Auto mode prompt override set in shell config", "stage", stage, "file", args[3])
		return nil
	case "max-tokens", "timeout", "fail-open", "transcript-max-chars", "max-consecutive-denials", "max-total-denials":
		return autoModeScalar(section, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("auto_mode: unknown setting %q (expected on, off, model, environment, prompt, max-tokens, timeout, fail-open, transcript-max-chars, max-consecutive-denials, max-total-denials)", args[1]))
	}
}

// autoModeScalar handles the numeric and boolean settings.
func autoModeScalar(section map[string]any, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, fmt.Sprintf("usage: auto_mode %s <value>", args[1]))
	}
	value := args[2]
	switch args[1] {
	case "fail-open":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return usage(stderr, fmt.Sprintf("auto_mode fail-open: expected true or false, got %q", value))
		}
		section["fail_open"] = b
	case "max-tokens":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return usage(stderr, fmt.Sprintf("auto_mode max-tokens: expected a positive integer, got %q", value))
		}
		section["max_tokens"] = n
	case "timeout":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return usage(stderr, fmt.Sprintf("auto_mode timeout: expected a positive integer (seconds), got %q", value))
		}
		section["timeout_seconds"] = n
	case "transcript-max-chars":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return usage(stderr, fmt.Sprintf("auto_mode transcript-max-chars: expected a positive integer, got %q", value))
		}
		section["transcript_max_chars"] = n
	case "max-consecutive-denials":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return usage(stderr, fmt.Sprintf("auto_mode max-consecutive-denials: expected a positive integer, got %q", value))
		}
		section["max_consecutive_denials"] = n
	case "max-total-denials":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return usage(stderr, fmt.Sprintf("auto_mode max-total-denials: expected a positive integer, got %q", value))
		}
		section["max_total_denials"] = n
	}
	slog.Info("Auto mode setting updated in shell config", "setting", args[1], "value", value)
	return nil
}
