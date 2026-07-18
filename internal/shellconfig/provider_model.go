package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// handleProviderModel implements the `provider-model` builtin.
//
// Usage: provider-model <provider-id> --id <model-id>
//
//	[--name NAME] [--context-window N] [--default-max-tokens N]
//	[--can-reason true|false] [--supports-images true|false]
//	[--cost-per-1m-in F] [--cost-per-1m-out F]
//	[--reasoning-effort low|medium|high]
//
// The first positional arg is the provider ID. --id (the model ID) is
// required. Each call appends a model to the provider's "models" array.
func handleProviderModel(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: provider-model <provider-id> --id <model-id> [--name NAME] [--context-window N] [--default-max-tokens N] [--can-reason true|false] [--supports-images true|false] [--cost-per-1m-in F] [--cost-per-1m-out F] [--reasoning-effort low|medium|high]")
	}

	providerID := args[1]
	if strings.HasPrefix(providerID, "--") {
		return usage(stderr, "provider-model: first arg must be the provider ID, not a flag")
	}

	var (
		modelID           string
		name              string
		contextWindow     int64
		defaultMaxTokens  int64
		canReason         bool
		supportsImages    bool
		costPer1MIn       float64
		costPer1MOut      float64
		reasoningEffort   string
		hasContextWindow  bool
		hasMaxTokens      bool
		hasCostIn         bool
		hasCostOut        bool
		hasCanReason      bool
		hasSupportsImages bool
	)

	i := 2
	for i < len(args) {
		switch args[i] {
		case "--id":
			v, err := flagStr(args, &i, "id")
			if err != nil {
				return usage(stderr, err.Error())
			}
			modelID = v
		case "--name":
			v, err := flagStr(args, &i, "name")
			if err != nil {
				return usage(stderr, err.Error())
			}
			name = v
		case "--context-window":
			v, err := flagInt(args, &i, "context-window")
			if err != nil {
				return usage(stderr, err.Error())
			}
			contextWindow = int64(v)
			hasContextWindow = true
		case "--default-max-tokens":
			v, err := flagInt(args, &i, "default-max-tokens")
			if err != nil {
				return usage(stderr, err.Error())
			}
			defaultMaxTokens = int64(v)
			hasMaxTokens = true
		case "--can-reason":
			v, err := flagBool(args, &i, "can-reason")
			if err != nil {
				return usage(stderr, err.Error())
			}
			canReason = v
			hasCanReason = true
		case "--supports-images":
			v, err := flagBool(args, &i, "supports-images")
			if err != nil {
				return usage(stderr, err.Error())
			}
			supportsImages = v
			hasSupportsImages = true
		case "--cost-per-1m-in":
			v, err := flagFloat64(args, &i, "cost-per-1m-in")
			if err != nil {
				return usage(stderr, err.Error())
			}
			costPer1MIn = v
			hasCostIn = true
		case "--cost-per-1m-out":
			v, err := flagFloat64(args, &i, "cost-per-1m-out")
			if err != nil {
				return usage(stderr, err.Error())
			}
			costPer1MOut = v
			hasCostOut = true
		case "--reasoning-effort":
			v, err := flagStr(args, &i, "reasoning-effort")
			if err != nil {
				return usage(stderr, err.Error())
			}
			reasoningEffort = v
		default:
			return usage(stderr, fmt.Sprintf("provider-model: unknown flag %s", args[i]))
		}
	}

	if modelID == "" {
		return usage(stderr, "provider-model: --id is required")
	}

	slog.Info("Provider model defined in shell config", "provider", providerID, "model", modelID)

	model := map[string]any{
		"id": modelID,
	}
	if name != "" {
		model["name"] = name
	}
	if hasContextWindow {
		model["context_window"] = contextWindow
	}
	if hasMaxTokens {
		model["default_max_tokens"] = defaultMaxTokens
	}
	if hasCanReason {
		model["can_reason"] = canReason
	}
	if hasSupportsImages {
		model["supports_attachments"] = supportsImages
	}
	if hasCostIn {
		model["cost_per_1m_in"] = costPer1MIn
	}
	if hasCostOut {
		model["cost_per_1m_out"] = costPer1MOut
	}
	if reasoningEffort != "" {
		model["default_reasoning_effort"] = reasoningEffort
	}

	p := childMap(b.section("providers"), providerID)
	modelsArr, _ := p["models"].([]any)
	p["models"] = append(modelsArr, model)

	slog.Debug("Provider model recorded", "provider", providerID, "model", modelID)
	return nil
}
