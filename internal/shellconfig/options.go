package shellconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"

	"github.com/charmbracelet/crush/internal/shell"
)

// resetSentinel marks a "reset" point inside a list option. Because the
// config merge concatenates lists, `option reset <key>` appends this marker
// instead of trying to clear the slice in place. resolveResetSentinels
// truncates each list at its last sentinel after merging, so values added
// after a reset survive while earlier ones are dropped. The NUL prefix makes
// accidental collision with a real config value effectively impossible.
const resetSentinel = "\x00__crush_reset__"

// handleOption implements the `option` builtin.
//
// Usage: option <key> <value>
//
// Sets a single option field. The key is a kebab-case name; for list fields
// (context-path, disable-tool, etc.) each call appends to the list.
//
// "option reset <list-key>" wipes a list back to empty, dropping values set
// earlier in the script or via source. Values added after the reset are kept.
//
// Some config fields are phrased negatively (disable_metrics). Those are
// exposed positively — the user sets "metrics false" and it is stored as
// "disable_metrics true".
//
// Examples:
//
//	option data-directory .crush
//	option context-path .cursorrules
//	option reset skill-path
//	option metrics false
//	option debug true
//	option auto-lsp false
//
// Boolean shortcuts: for boolean fields, omitting the value sets it to true.
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

	// "option reset <key>" wipes a list back to empty. It emits a sentinel
	// element that rides the concatenating merge and is resolved (everything
	// up to and including the last sentinel is dropped) after the script's
	// fragments are merged. This keeps "reset then re-add" order-correct.
	if key == "reset" {
		if len(args) < 3 {
			return usage(stderr, "usage: option reset <list-key>")
		}
		target := args[2]
		jsonKey, _ := optionKeyMap(target)
		if jsonKey == "" {
			return usage(stderr, fmt.Sprintf("option: unknown key %q", target))
		}
		if !isListOption(jsonKey) {
			return usage(stderr, fmt.Sprintf("option: reset only applies to list options, %q is not one", target))
		}
		o[jsonKey] = []any{resetSentinel}
		slog.Info("Option list reset in shell config", "key", target)
		return f.append(b)
	}

	// Determine the value.
	var val string
	if len(args) >= 3 {
		val = args[2]
	}

	jsonKey, inverted := optionKeyMap(key)
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

	// Boolean fields: if no value, default to true. Inverted keys store the
	// negation, so a positive key like "metrics" maps onto "disable_metrics".
	if isBoolOption(jsonKey) {
		bv := true
		if val != "" {
			parsed, err := parseBool(val)
			if err != nil {
				return usage(stderr, fmt.Sprintf("option: %s expects true/false, got %q", key, val))
			}
			bv = parsed
		}
		if inverted {
			bv = !bv
		}
		o[jsonKey] = bv
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

// optionKeyMap maps user-facing kebab-case keys to JSON field names. The
// second return value reports whether the key's boolean value must be
// inverted before storing: several config fields are phrased negatively
// (disable_metrics), but users set the positive sense (metrics false).
// Returns an empty jsonKey for unknown keys.
func optionKeyMap(key string) (jsonKey string, inverted bool) {
	switch key {
	// Boolean fields (stored as-is).
	case "debug":
		return "debug", false
	case "debug-lsp":
		return "debug_lsp", false
	case "auto-lsp":
		return "auto_lsp", false
	case "progress":
		return "progress", false

	// Boolean fields exposed positively but stored as their negation.
	case "metrics":
		return "disable_metrics", true
	case "notifications":
		return "disable_notifications", true
	case "auto-summarize":
		return "disable_auto_summarize", true
	case "provider-auto-update":
		return "disable_provider_auto_update", true
	case "default-providers":
		return "disable_default_providers", true

	// String fields
	case "data-directory":
		return "data_directory", false
	case "initialize-as":
		return "initialize_as", false
	case "notification-style":
		return "notification_style", false

	// List fields. Keys are singular because each call appends one value.
	case "context-path":
		return "context_paths", false
	case "global-context-path":
		return "global_context_paths", false
	case "skill-path":
		return "skills_paths", false
	case "disable-tool":
		return "disabled_tools", false
	case "disable-skill":
		return "disabled_skills", false

	default:
		return "", false
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

func isIntOption(_ string) bool {
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

// resolveResetSentinels applies `option reset` markers to the merged config.
// For every list under "options", it keeps only the elements that follow the
// last resetSentinel (dropping the sentinel and everything before it), so a
// reset wipes inherited values while later additions survive. Lists without a
// sentinel are left untouched. It returns the (possibly rewritten) JSON.
func resolveResetSentinels(data []byte) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		// Not a JSON object; nothing to resolve.
		return data, nil
	}
	opts, ok := root["options"].(map[string]any)
	if !ok {
		return data, nil
	}

	changed := false
	for key, v := range opts {
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		last := -1
		for i, item := range arr {
			if s, ok := item.(string); ok && s == resetSentinel {
				last = i
			}
		}
		if last < 0 {
			continue
		}
		kept := make([]any, 0, len(arr)-last-1)
		for _, item := range arr[last+1:] {
			if s, ok := item.(string); ok && s == resetSentinel {
				continue
			}
			kept = append(kept, item)
		}
		opts[key] = kept
		changed = true
	}

	if !changed {
		return data, nil
	}
	return json.Marshal(root)
}
