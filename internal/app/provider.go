package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	xstrings "github.com/charmbracelet/x/exp/strings"
)

// parseModelStr parses a model string into provider filter and model ID.
// Format: "model-name" or "provider/model-name" or "synthetic/moonshot/kimi-k2".
// This function only checks if the first component is a valid provider name; if not,
// it treats the entire string as a model ID (which may contain slashes).
func parseModelStr(providers map[string]config.ProviderConfig, modelStr string) (providerFilter, modelID string) {
	// TODO: simplify this
	parts := strings.Split(strings.ToLower(modelStr), "/")
	if len(parts) == 1 {
		return "", parts[0]
	}
	// Check if the first part is a valid provider name
	if _, ok := providers[parts[0]]; ok {
		return parts[0], strings.Join(parts[1:], "/")
	}

	// First part is not a valid provider, treat entire string as model ID
	return "", strings.ToLower(modelStr)
}

// modelMatch represents a found model.
type modelMatch struct {
	provider string
	modelID  string
}

func findModels(providers map[string]config.ProviderConfig, largeModel, smallModel string) ([]modelMatch, []modelMatch, error) {
	largeProviderFilter, largeModelID := parseModelStr(providers, largeModel)
	smallProviderFilter, smallModelID := parseModelStr(providers, smallModel)

	// Validate provider filters exist.
	for _, pf := range []struct {
		filter, label string
	}{
		{largeProviderFilter, "large"},
		{smallProviderFilter, "small"},
	} {
		if pf.filter != "" {
			if _, ok := providers[pf.filter]; !ok {
				return nil, nil, fmt.Errorf("%s model: provider %q not found in configuration", pf.label, pf.filter)
			}
		}
	}

	// Find matching models in a single pass.
	var largeMatches, smallMatches []modelMatch
	for name, provider := range providers {
		if provider.Disable {
			continue
		}
		for _, m := range provider.Models {
			modelIDLower := strings.ToLower(m.ID)
			if largeModelID != "" && modelIDLower == largeModelID &&
				(largeProviderFilter == "" || name == largeProviderFilter) {
				largeMatches = append(largeMatches, modelMatch{provider: name, modelID: m.ID})
			}
			if smallModelID != "" && modelIDLower == smallModelID &&
				(smallProviderFilter == "" || name == smallProviderFilter) {
				smallMatches = append(smallMatches, modelMatch{provider: name, modelID: m.ID})
			}
		}
	}

	return largeMatches, smallMatches, nil
}

// Validate and return a single match.
func validateMatches(matches []modelMatch, modelID, label string) (modelMatch, error) {
	switch {
	case len(matches) == 0:
		return modelMatch{}, fmt.Errorf("%s model %q not found", label, modelID)
	case len(matches) > 1:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.provider
		}
		return modelMatch{}, fmt.Errorf(
			"%s model: model %q found in multiple providers: %s. Please specify provider using 'provider/model' format",
			label,
			modelID,
			xstrings.EnglishJoin(names, true),
		)
	}
	return matches[0], nil
}
