package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
	"github.com/charmbracelet/crush/internal/config"
)

func Test_getProviderOptions_reasoningEffort(t *testing.T) {
	cfg := config.ProviderConfig{
		Type: openaicompat.Name,
	}
	t.Run("model supporting reasoning effort gets it", func(t *testing.T) {
		model := Model{
			Model: fantasy.LanguageModel(nil),
			CatwalkCfg: catwalk.Model{
				ReasoningLevels: []string{"low", "medium", "high"},
			},
			ModelCfg: config.SelectedModel{
				ReasoningEffort: "medium",
			},
		}

		// Run
		opts := getProviderOptions(model, cfg)

		// Check
		subOpts := opts[openaicompat.Name].(*openaicompat.ProviderOptions)
		if subOpts.ReasoningEffort == nil || *subOpts.ReasoningEffort == "" {
			t.Error("Expected reasoning effort to be set")
		}
	})

	t.Run("model with catwalk not supporting reasoning effort levels does not get it (repro for issue 2078)", func(t *testing.T) {
		// Setup
		model := Model{
			Model: fantasy.LanguageModel(nil),
			CatwalkCfg: catwalk.Model{
				ReasoningLevels: nil,
			},
			ModelCfg: config.SelectedModel{
				ReasoningEffort: "medium",
			},
		}

		// Run
		opts := getProviderOptions(model, cfg)

		// Check
		subOpts := opts[openaicompat.Name].(*openaicompat.ProviderOptions)
		if subOpts.ReasoningEffort != nil {
			t.Error("Expected reasoning effort not to be set")
		}
	})
}
