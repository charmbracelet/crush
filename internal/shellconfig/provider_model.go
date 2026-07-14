package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/charmbracelet/crush/internal/shell"
)

// handleProviderModel implements the `provider-model` builtin.
//
// Usage: provider-model --provider <id> --id <model-id>
//
//	[--name NAME] [--context-window N] [--default-max-tokens N]
//	[--can-reason true|false] [--supports-images true|false]
//	[--cost-per-1m-in F] [--cost-per-1m-out F]
//	[--reasoning-effort low|medium|high]
//
// Each call appends a model to the named provider's "models" array.
// Multiple calls for the same provider accumulate into the array via
// the deep-merge pipeline.
func handleProviderModel(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := shell.ConfigBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}

	var (
		providerID        string
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

	i := 1
	for i < len(args) {
		switch args[i] {
		case "--provider":
			v, err := flagStr(args, &i, "provider")
			if err != nil {
				return usage(stderr, err.Error())
			}
			providerID = v
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

	if providerID == "" {
		return usage(stderr, "provider-model: --provider is required")
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

	f := newFragmentBuilder()
	providers := f.rootMap("providers")
	p := f.nestedMap("providers", providerID)
	models := p["models"]
	if models == nil {
		models = make([]any, 0)
	}
	modelsArr, _ := models.([]any)
	p["models"] = append(modelsArr, model)
	// Ensure the providers root map references the nested map we just built.
	// nestedMap already set f.m["providers"]["providerID"] = p, so this is
	// just a safety no-op for the root map.
	_ = providers

	if err := f.append(b); err != nil {
		slog.Error("Failed to append provider-model fragment", "provider", providerID, "model", modelID, "error", err)
		return err
	}
	slog.Debug("Provider model fragment appended", "provider", providerID, "model", modelID)
	return nil
}
