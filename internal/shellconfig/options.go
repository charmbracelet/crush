package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/charmbracelet/crush/internal/shell"
)

// handleOption implements the `option` builtin.
//
// Usage: option <key> <value>
//
// Sets a single option field. The key is the JSON field name (snake_case).
// The value is parsed as a boolean, integer, float, or string depending on
// the field. For list fields (context_paths, disabled_tools, etc.), each
// call appends to the list.
//
// Examples:
//
//	option no-progress true
//	option data-directory .crush
//	option context-paths .cursorrules
//	option disable-metrics true
//	option debug true
//	option auto-lsp false
//
// Boolean shortcuts: for boolean fields, omitting the value sets it to true.
// Negated keys (no-progress, no-auto-lsp) set the corresponding field to false.
func handleOption(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := shell.ConfigBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: option <key> [value]")
	}

	key := args[1]
	f := newFragmentBuilder()
	o := f.rootMap("options")

	// Determine the value.
	var val string
	if len(args) >= 3 {
		val = args[2]
	}

	// Negated boolean keys: no-foo sets the corresponding field to false
	if strings.HasPrefix(key, "no-") {
		realKey := key[3:]
		jsonKey := negatedOptionKeyMap(realKey)
		if jsonKey == "" {
			return usage(stderr, fmt.Sprintf("option: unknown key %q", key))
		}
		o[jsonKey] = false
		slog.Info("Option set in shell config", "key", key, "value", false)
		return f.append(b)
	}

	jsonKey := optionKeyMap(key)
	if jsonKey == "" {
		return usage(stderr, fmt.Sprintf("option: unknown key %q", key))
	}

	// List fields: append to array
	if isListOption(jsonKey) {
		if val == "" {
			return usage(stderr, fmt.Sprintf("option: %s requires a value", key))
		}
		o[jsonKey] = appendArr(o, jsonKey, val)
		slog.Info("Option set in shell config", "key", key, "value", val)
		return f.append(b)
	}

	// Boolean fields: if no value, default to true
	if isBoolOption(jsonKey) {
		if val == "" {
			o[jsonKey] = true
		} else {
			bv, err := parseBool(val)
			if err != nil {
				return usage(stderr, fmt.Sprintf("option: %s expects true/false, got %q", key, val))
			}
			o[jsonKey] = bv
		}
		slog.Info("Option set in shell config", "key", key, "value", o[jsonKey])
		return f.append(b)
	}

	// Integer fields
	if isIntOption(jsonKey) {
		if val == "" {
			return usage(stderr, fmt.Sprintf("option: %s requires a value", key))
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return usage(stderr, fmt.Sprintf("option: %s expects an integer, got %q", key, val))
		}
		o[jsonKey] = n
		slog.Info("Option set in shell config", "key", key, "value", n)
		return f.append(b)
	}

	// String fields
	if val == "" {
		return usage(stderr, fmt.Sprintf("option: %s requires a value", key))
	}
	o[jsonKey] = val
	slog.Info("Option set in shell config", "key", key, "value", val)
	return f.append(b)
}

// negatedOptionKeyMap maps the part after "no-" to the JSON field name
// that should be set to false. Returns empty string for unknown keys.
func negatedOptionKeyMap(key string) string {
	switch key {
	case "progress":
		return "progress"
	case "auto-lsp":
		return "auto_lsp"
	case "metrics":
		return "disable_metrics"
	case "notifications":
		return "disable_notifications"
	case "auto-summarize":
		return "disable_auto_summarize"
	case "provider-auto-update":
		return "disable_provider_auto_update"
	case "default-providers":
		return "disable_default_providers"
	default:
		return ""
	}
}

// optionKeyMap maps user-facing kebab-case keys to JSON field names.
// Returns empty string for unknown keys.
func optionKeyMap(key string) string {
	switch key {
	// Boolean fields
	case "debug":
		return "debug"
	case "debug-lsp":
		return "debug_lsp"
	case "disable-auto-summarize":
		return "disable_auto_summarize"
	case "disable-provider-auto-update":
		return "disable_provider_auto_update"
	case "disable-default-providers":
		return "disable_default_providers"
	case "disable-metrics":
		return "disable_metrics"
	case "disable-notifications":
		return "disable_notifications"
	case "auto-lsp":
		return "auto_lsp"
	case "progress":
		return "progress"

	// String fields
	case "data-directory":
		return "data_directory"
	case "initialize-as":
		return "initialize_as"
	case "notification-style":
		return "notification_style"

	// List fields
	case "context-paths":
		return "context_paths"
	case "global-context-paths":
		return "global_context_paths"
	case "skills-paths":
		return "skills_paths"
	case "disabled-tools":
		return "disabled_tools"
	case "disabled-skills":
		return "disabled_skills"

	default:
		return ""
	}
}

func isBoolOption(jsonKey string) bool {
	switch jsonKey {
	case "debug", "debug_lsp", "disable_auto_summarize",
		"disable_provider_auto_update", "disable_default_providers",
		"disable_metrics", "disable_notifications", "auto_lsp", "progress":
		return true
	default:
		return false
	}
}

func isIntOption(jsonKey string) bool {
	return false
}

func isListOption(jsonKey string) bool {
	switch jsonKey {
	case "context_paths", "global_context_paths", "skills_paths",
		"disabled_tools", "disabled_skills":
		return true
	default:
		return false
	}
}

func parseBool(s string) (bool, error) {
	switch s {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", s)
	}
}
