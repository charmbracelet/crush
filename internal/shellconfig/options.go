package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

// handleOption implements the `option` builtin.
//
// Usage: option <key> <value>
//
// Sets a single option field. The key is a kebab-case name; for list fields
// (context-path, disable-skill, etc.) each call appends to the list.
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
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: option <key> [value]")
	}

	key := args[1]
	o := b.section("options")

	if key == "ui" {
		return optionUI(o, args, stderr)
	}

	// "option reset <key>" wipes a list back to empty. Because the builder
	// applies operations in execution order, this is just an assignment:
	// values added after the reset are kept, earlier ones are dropped.
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
		o[jsonKey] = []any{}
		slog.Info("Option list reset in shell config", "key", target)
		return nil
	}

	// Determine the value.
	var val string
	if len(args) >= 3 {
		val = args[2]
	}

	if key == "attribution-trailer-style" {
		if val == "" {
			return usage(stderr, "option: attribution-trailer-style requires a value")
		}
		switch val {
		case "none", "co-authored-by", "assisted-by":
		default:
			return usage(stderr, fmt.Sprintf("option: attribution-trailer-style expects none, co-authored-by, or assisted-by, got %q", val))
		}
		attribution := childMap(o, "attribution")
		if _, ok := attribution["generated_with"]; !ok {
			attribution["generated_with"] = true
		}
		attribution["trailer_style"] = val
		slog.Info("Option set in shell config", "key", key, "value", val)
		return nil
	}

	if key == "attribution-generated-with" {
		bv := true
		if val != "" {
			parsed, err := parseBool(val)
			if err != nil {
				return usage(stderr, fmt.Sprintf("option: attribution-generated-with expects true/false, got %q", val))
			}
			bv = parsed
		}
		childMap(o, "attribution")["generated_with"] = bv
		slog.Info("Option set in shell config", "key", key, "value", bv)
		return nil
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
		return nil
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
		return nil
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
		return nil
	}

	// String fields
	if val == "" {
		return usage(stderr, fmt.Sprintf("option: %s requires a value", key))
	}
	o[jsonKey] = val
	slog.Info("Option set in shell config", "key", key, "value", val)
	return nil
}

// optionKeyMap maps user-facing kebab-case keys to JSON field names. The
// second return value reports whether the key's boolean value must be
// inverted before storing: several config fields are phrased negatively
// (disable_metrics), but users set the positive sense (metrics false).
// Returns an empty jsonKey for unknown keys.
func optionUI(options map[string]any, args []string, stderr io.Writer) error {
	if len(args) != 4 {
		return usage(stderr, "usage: option ui <compact|diff|transparent|scrollbar|completions-max-depth|completions-max-items> <value>")
	}

	key := args[2]
	value := args[3]
	ui := childMap(options, "tui")

	switch key {
	case "compact", "transparent":
		parsed, err := parseBool(value)
		if err != nil {
			return usage(stderr, fmt.Sprintf("option ui %s expects true/false, got %q", key, value))
		}
		jsonKey := "compact_mode"
		if key == "transparent" {
			jsonKey = "transparent"
		}
		ui[jsonKey] = parsed
	case "diff":
		if value != "unified" && value != "split" {
			return usage(stderr, fmt.Sprintf("option ui diff expects unified or split, got %q", value))
		}
		ui["diff_mode"] = value
	case "scrollbar":
		if value != "default" && value != "always" && value != "never" {
			return usage(stderr, fmt.Sprintf("option ui scrollbar expects default, always, or never, got %q", value))
		}
		ui["scrollbar"] = value
	case "completions-max-depth", "completions-max-items":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			return usage(stderr, fmt.Sprintf("option ui %s expects a non-negative integer, got %q", key, value))
		}
		jsonKey := "max_depth"
		if key == "completions-max-items" {
			jsonKey = "max_items"
		}
		childMap(ui, "completions")[jsonKey] = parsed
	default:
		return usage(stderr, fmt.Sprintf("option ui: unknown key %q", key))
	}

	slog.Info("UI option set in shell config", "key", key, "value", value)
	return nil
}

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
	case "context_paths", "global_context_paths", "skills_paths", "disabled_skills":
		return true
	default:
		return false
	}
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", s)
	}
}
