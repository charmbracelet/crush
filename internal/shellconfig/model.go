package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// handleModel implements the `model` builtin.
//
// Usage:
//
//	model add <provider>/<id> [--name NAME] [--context-window N]
//	    [--default-max-tokens N] [--can-reason true|false]
//	    [--supports-images true|false] [--price-input F]
//	    [--price-output F] [--price-cache-create F]
//	    [--price-cache-hit F] [--reasoning-effort low|medium|high]
//	model remove <provider>/<id>   (alias: rm)
//	model large [<provider>/<id>] [--think] [--reasoning-effort L]
//	    [--max-tokens N] [--temperature F]
//	model small [<provider>/<id>] [...]
//
// "add" registers a model on an existing provider (the provider must have
// been declared with `provider add` first). "remove" removes it. "large" and
// "small" set the selected model for that slot, or print the current
// selection as <provider>/<id> when given no argument.
func handleModel(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: model add|remove <provider>/<id> | model large|small [<provider>/<id>]")
	}

	switch args[1] {
	case "add":
		return modelAdd(b, args, stderr)
	case "remove", "rm":
		return modelRemove(b, args, stderr)
	case "large", "small":
		return modelSelect(b, args, stdout, stderr)
	default:
		return usage(stderr, fmt.Sprintf("model: unknown subcommand %q (expected add, remove, large, or small)", args[1]))
	}
}

// splitProviderModel splits "provider/id" on the first slash. Model ids may
// themselves contain slashes, so only the first separates provider from id.
func splitProviderModel(s string) (provider, id string, ok bool) {
	provider, id, found := strings.Cut(s, "/")
	if !found || provider == "" || id == "" {
		return "", "", false
	}
	return provider, id, true
}

func modelAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: model add <provider>/<id> [--name NAME] [--context-window N] [--default-max-tokens N] [--can-reason true|false] [--supports-images true|false] [--price-input F] [--price-output F] [--price-cache-create F] [--price-cache-hit F] [--reasoning-effort low|medium|high]")
	}
	provider, id, ok := splitProviderModel(args[2])
	if !ok {
		return usage(stderr, fmt.Sprintf("model add: expected <provider>/<id>, got %q", args[2]))
	}

	providers := b.section("providers")
	if _, exists := providers[provider]; !exists {
		return usage(stderr, fmt.Sprintf("model add: provider %q does not exist (declare it with `provider add %s` first)", provider, provider))
	}

	model := map[string]any{"id": id}

	i := 3
	for i < len(args) {
		switch args[i] {
		case "--name":
			v, err := flagStr(args, &i, "name")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["name"] = v
		case "--context-window":
			v, err := flagInt(args, &i, "context-window")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["context_window"] = int64(v)
		case "--default-max-tokens":
			v, err := flagInt(args, &i, "default-max-tokens")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["default_max_tokens"] = int64(v)
		case "--can-reason":
			v, err := flagBool(args, &i, "can-reason")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["can_reason"] = v
		case "--supports-images":
			v, err := flagBool(args, &i, "supports-images")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["supports_attachments"] = v
		case "--price-input":
			v, err := flagFloat64(args, &i, "price-input")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["cost_per_1m_in"] = v
		case "--price-output":
			v, err := flagFloat64(args, &i, "price-output")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["cost_per_1m_out"] = v
		case "--price-cache-create":
			v, err := flagFloat64(args, &i, "price-cache-create")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["cost_per_1m_out_cached"] = v
		case "--price-cache-hit":
			v, err := flagFloat64(args, &i, "price-cache-hit")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["cost_per_1m_in_cached"] = v
		case "--reasoning-effort":
			v, err := flagStr(args, &i, "reasoning-effort")
			if err != nil {
				return usage(stderr, err.Error())
			}
			model["default_reasoning_effort"] = v
		default:
			return usage(stderr, fmt.Sprintf("model add: unknown flag %s", args[i]))
		}
	}

	p := childMap(providers, provider)
	modelsArr, _ := p["models"].([]any)
	p["models"] = append(modelsArr, model)

	slog.Info("Model added in shell config", "provider", provider, "model", id)
	return nil
}

func modelRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: model remove <provider>/<id>")
	}
	provider, id, ok := splitProviderModel(args[2])
	if !ok {
		return usage(stderr, fmt.Sprintf("model remove: expected <provider>/<id>, got %q", args[2]))
	}

	providers := b.section("providers")
	p, exists := providers[provider].(map[string]any)
	if !exists {
		return nil
	}
	modelsArr, _ := p["models"].([]any)
	kept := make([]any, 0, len(modelsArr))
	for _, item := range modelsArr {
		m, ok := item.(map[string]any)
		if ok && m["id"] == id {
			continue
		}
		kept = append(kept, item)
	}
	p["models"] = kept

	slog.Info("Model removed in shell config", "provider", provider, "model", id)
	return nil
}

func modelSelect(b *ConfigBuilder, args []string, stdout, stderr io.Writer) error {
	slot := args[1]

	// No argument: print the current selection as <provider>/<id>.
	if len(args) == 2 {
		if models, ok := b.root["models"].(map[string]any); ok {
			if sel, ok := models[slot].(map[string]any); ok {
				provider, _ := sel["provider"].(string)
				id, _ := sel["model"].(string)
				if provider != "" && id != "" {
					fmt.Fprintln(stdout, provider+"/"+id)
				}
			}
		}
		return nil
	}

	provider, id, ok := splitProviderModel(args[2])
	if !ok {
		return usage(stderr, fmt.Sprintf("model %s: expected <provider>/<id>, got %q", slot, args[2]))
	}

	sel := childMap(b.section("models"), slot)
	sel["provider"] = provider
	sel["model"] = id

	i := 3
	for i < len(args) {
		switch args[i] {
		case "--think":
			sel["think"] = true
			i++
		case "--reasoning-effort":
			v, err := flagStr(args, &i, "reasoning-effort")
			if err != nil {
				return usage(stderr, err.Error())
			}
			sel["reasoning_effort"] = v
		case "--max-tokens":
			v, err := flagInt(args, &i, "max-tokens")
			if err != nil {
				return usage(stderr, err.Error())
			}
			sel["max_tokens"] = v
		case "--temperature":
			v, err := flagFloat64(args, &i, "temperature")
			if err != nil {
				return usage(stderr, err.Error())
			}
			sel["temperature"] = v
		default:
			return usage(stderr, fmt.Sprintf("model %s: unknown flag %s", slot, args[i]))
		}
	}

	slog.Info("Model selected in shell config", "slot", slot, "provider", provider, "model", id)
	return nil
}
