package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
)

// handleHoncho implements the `honcho` builtin.
//
// Usage: honcho <key> [value]
//
// Configures the Honcho memory integration, which gives Crush recall across
// sessions. Any field left unset falls back to the shared cross-harness
// config file (~/.honcho/config.json) and then to built-in defaults, so a
// user who has already set Honcho up for another tool needs only:
//
//	honcho enabled
//
// Examples:
//
//	honcho enabled
//	honcho api-key "$HONCHO_API_KEY"
//	honcho base-url http://127.0.0.1:8000
//	honcho workspace my-project
//	honcho peer-name kieran
//	honcho session-strategy per-repo
//	honcho recall-mode tools
//	honcho capture-tools false
//
// Boolean shortcut: for boolean keys, omitting the value sets it to true.
func handleHoncho(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: honcho <key> [value]")
	}

	key := args[1]
	h := b.section("honcho")

	var val string
	if len(args) >= 3 {
		val = args[2]
	}

	spec, ok := honchoSpecs[key]
	if !ok {
		return usage(stderr, fmt.Sprintf("honcho: unknown key %q", key))
	}

	switch spec.kind {
	case optBool:
		bv := true
		if val != "" {
			parsed, err := parseBool(val)
			if err != nil {
				return usage(stderr, fmt.Sprintf("honcho: %s expects true/false, got %q", key, val))
			}
			bv = parsed
		}
		h[spec.jsonKey] = bv

	case optInt:
		if val == "" {
			return usage(stderr, fmt.Sprintf("honcho: %s requires a value", key))
		}
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			return usage(stderr, fmt.Sprintf("honcho: %s expects a non-negative number, got %q", key, val))
		}
		h[spec.jsonKey] = n

	default: // optString
		if val == "" {
			return usage(stderr, fmt.Sprintf("honcho: %s requires a value", key))
		}
		if len(spec.allowed) > 0 && !slices.Contains(spec.allowed, val) {
			return usage(stderr, fmt.Sprintf("honcho: %s expects one of %v, got %q", key, spec.allowed, val))
		}
		h[spec.jsonKey] = val
	}

	// Any honcho directive implies the user wants memory on, so enabling is
	// implicit. Writing the key explicitly to false still wins, because the
	// builder applies operations in execution order.
	if key != "enabled" {
		if _, set := h["enabled"]; !set {
			h["enabled"] = true
		}
	}

	// Never log the value of a credential.
	if spec.secret {
		slog.Info("Honcho option set in shell config", "key", key)
	} else {
		slog.Info("Honcho option set in shell config", "key", key, "value", h[spec.jsonKey])
	}
	return nil
}

// honchoSpec describes one user-facing honcho key: the JSON field it writes,
// its value type, the values it accepts, and whether it holds a credential
// that must stay out of logs.
type honchoSpec struct {
	jsonKey string
	kind    optionKind
	allowed []string
	secret  bool
}

// honchoSpecs maps user-facing kebab-case keys onto the Honcho config fields.
// Keys mirror the JSON field names so the two config formats stay legible as
// the same thing written two ways.
var honchoSpecs = map[string]honchoSpec{
	"enabled":          {jsonKey: "enabled", kind: optBool},
	"api-key":          {jsonKey: "api_key", kind: optString, secret: true},
	"base-url":         {jsonKey: "base_url", kind: optString},
	"workspace":        {jsonKey: "workspace", kind: optString},
	"peer-name":        {jsonKey: "peer_name", kind: optString},
	"agent-peer":       {jsonKey: "agent_peer", kind: optString},
	"agent-observe-me": {jsonKey: "agent_observe_me", kind: optBool},
	"capture-tools":    {jsonKey: "capture_tools", kind: optBool},
	"max-conclusions":  {jsonKey: "max_conclusions", kind: optInt},
	"context-tokens":   {jsonKey: "context_tokens", kind: optInt},

	"recall-mode": {
		jsonKey: "recall_mode", kind: optString,
		allowed: []string{"hybrid", "context", "tools"},
	},
	"observation-mode": {
		jsonKey: "observation_mode", kind: optString,
		allowed: []string{"unified", "directional"},
	},
	"session-strategy": {
		jsonKey: "session_strategy", kind: optString,
		allowed: []string{"per-directory", "per-repo", "git-branch", "per-session", "global"},
	},
}
