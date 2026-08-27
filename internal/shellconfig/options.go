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
		spec, ok := optionSpecs[target]
		if !ok {
			return usage(stderr, fmt.Sprintf("option: unknown key %q", target))
		}
		if spec.kind != optList {
			return usage(stderr, fmt.Sprintf("option: reset only applies to list options, %q is not one", target))
		}
		o[spec.jsonKey] = []any{}
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

	spec, ok := optionSpecs[key]
	if !ok {
		return usage(stderr, fmt.Sprintf("option: unknown key %q", key))
	}

	switch spec.kind {
	case optList:
		if val == "" {
			return usage(stderr, fmt.Sprintf("option: %s requires a value", key))
		}
		o[spec.jsonKey] = appendArr(o, spec.jsonKey, val)
		slog.Info("Option set in shell config", "key", key, "value", val)
		return nil

	case optBool:
		// If no value, default to true. Inverted keys store the negation,
		// so a positive key like "metrics" maps onto "disable_metrics".
		bv := true
		if val != "" {
			parsed, err := parseBool(val)
			if err != nil {
				return usage(stderr, fmt.Sprintf("option: %s expects true/false, got %q", key, val))
			}
			bv = parsed
		}
		if spec.inverted {
			bv = !bv
		}
		o[spec.jsonKey] = bv
		slog.Info("Option set in shell config", "key", key, "value", o[spec.jsonKey])
		return nil

	default: // optString
		if val == "" {
			return usage(stderr, fmt.Sprintf("option: %s requires a value", key))
		}
		o[spec.jsonKey] = val
		slog.Info("Option set in shell config", "key", key, "value", val)
		return nil
	}
}

// optionKind is the value type of a user-facing option key.
type optionKind int

const (
	optString optionKind = iota
	optBool
	optList
)

// optionSpec describes one user-facing option key: the JSON field it writes,
// its value type, and (for booleans) whether the stored value is the inverse
// of what the user typed. Several config fields are phrased negatively
// (disable_metrics) but exposed positively (metrics), so "metrics false"
// stores "disable_metrics true".
type optionSpec struct {
	jsonKey  string
	kind     optionKind
	inverted bool
}

// optionSpecs maps user-facing kebab-case keys to their JSON field and type.
// This is the single source of truth for option key handling; the kind field
// drives parsing so there is no separate bool/list enumeration to drift out
// of sync.
//
// Not exhaustive by design: options with nested structure (option ui ...) or
// conditional logic (option attribution-...) are handled as special cases in
// handleOption above and do not appear here.
var optionSpecs = map[string]optionSpec{
	// Boolean fields (stored as-is).
	"debug":     {jsonKey: "debug", kind: optBool},
	"debug-lsp": {jsonKey: "debug_lsp", kind: optBool},
	"auto-lsp":  {jsonKey: "auto_lsp", kind: optBool},
	"progress":  {jsonKey: "progress", kind: optBool},

	// Boolean fields exposed positively but stored as their negation.
	"metrics":              {jsonKey: "disable_metrics", kind: optBool, inverted: true},
	"auto-summarize":       {jsonKey: "disable_auto_summarize", kind: optBool, inverted: true},
	"provider-auto-update": {jsonKey: "disable_provider_auto_update", kind: optBool, inverted: true},
	"default-providers":    {jsonKey: "disable_default_providers", kind: optBool, inverted: true},

	// String fields.
	"notifications":  {jsonKey: "notifications", kind: optString},
	"data-directory": {jsonKey: "data_directory", kind: optString},
	"initialize-as":  {jsonKey: "initialize_as", kind: optString},

	// List fields. Keys are singular because each call appends one value.
	"context-path":        {jsonKey: "context_paths", kind: optList},
	"global-context-path": {jsonKey: "global_context_paths", kind: optList},
	"skill-path":          {jsonKey: "skills_paths", kind: optList},
	"disable-skill":       {jsonKey: "disabled_skills", kind: optList},
}

// optionUI implements "option ui <key> <value>" for TUI-specific settings
// that live under options.tui rather than as top-level options.
func optionUI(options map[string]any, args []string, stderr io.Writer) error {
	if len(args) != 4 {
		return usage(stderr, "usage: option ui <compact|diff|transparent|scrollbar|completions-max-depth|completions-max-items|exit-banner> <value>")
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
	case "exit-banner":
		if value != "default" && value != "compact" && value != "none" {
			return usage(stderr, fmt.Sprintf("option ui exit-banner expects default, compact, or none, got %q", value))
		}
		ui["exit_banner"] = value
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
