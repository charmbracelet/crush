package acp

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	acp "github.com/coder/acp-go-sdk"
	"github.com/stretchr/testify/require"
)

func TestBuildPromptTextAndAttachments(t *testing.T) {
	// "YWJj" is base64 for "abc".
	blocks := []acp.ContentBlock{
		{Text: acp.Ptr(acp.ContentBlockText{Text: "hello "})},
		{Image: &acp.ContentBlockImage{Data: "iVBORw0KGgo=", MimeType: "image/png"}},
		{ResourceLink: &acp.ContentBlockResourceLink{Name: "f", Uri: "file:///f"}},
	}
	text, atts := buildPrompt(blocks)
	require.Equal(t, "hello [resource: f: file:///f]", text)
	require.Len(t, atts, 1)
	require.Equal(t, "image/png", atts[0].MimeType)

	// Invalid base64 image is skipped, but valid audio is kept.
	blocks = []acp.ContentBlock{
		{Image: &acp.ContentBlockImage{Data: "not-base64!!!", MimeType: "image/png"}},
		{Audio: &acp.ContentBlockAudio{Data: "YWJj", MimeType: "audio/wav"}},
	}
	text, atts = buildPrompt(blocks)
	require.Empty(t, text)
	require.Len(t, atts, 1)
	require.Equal(t, []byte("abc"), atts[0].Content)
	require.Equal(t, "audio/wav", atts[0].MimeType)
}

func TestThinkingEnabled(t *testing.T) {
	cfg := &config.Config{
		Agents: map[string]config.Agent{"coder": {Model: config.SelectedModelTypeLarge}},
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Think: true},
		},
	}
	require.True(t, thinkingEnabled(cfg))

	cfg.Models[config.SelectedModelTypeLarge] = config.SelectedModel{Think: false}
	require.False(t, thinkingEnabled(cfg))

	// Unknown agent falls back to false.
	delete(cfg.Agents, "coder")
	require.False(t, thinkingEnabled(cfg))
}

func TestConfigOptionsIncludesThinking(t *testing.T) {
	cfg := &config.Config{
		Agents: map[string]config.Agent{
			"coder": {Name: "Coder", Model: config.SelectedModelTypeLarge},
		},
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Think: true},
		},
	}
	opts := configOptionsFor(cfg)
	require.NotEmpty(t, opts)

	var found bool
	for _, o := range opts {
		if o.Boolean != nil && o.Boolean.Id == "thinking" {
			found = true
			require.True(t, o.Boolean.CurrentValue)
		}
	}
	require.True(t, found, "expected a thinking boolean config option")
}

func TestParseModelValue(t *testing.T) {
	require.Equal(t, config.SelectedModel{Provider: "openai", Model: "gpt-4o"}, parseModelValue("openai/gpt-4o"))
	// A model id that itself contains a slash is preserved intact.
	require.Equal(t, config.SelectedModel{Provider: "9router", Model: "code/fast"}, parseModelValue("9router/code/fast"))
	// No slash means provider-less.
	require.Equal(t, config.SelectedModel{Model: "standalone"}, parseModelValue("standalone"))
}

func TestModelOptionsFor(t *testing.T) {
	cfg := &config.Config{
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			"openai": {
				Models: []catwalk.Model{
					{ID: "gpt-4o", Name: "GPT-4o"},
					{ID: "gpt-4o-mini"},
				},
			},
			"anthropic": {
				Models: []catwalk.Model{
					{ID: "claude-sonnet-4", Name: "Claude Sonnet 4"},
				},
			},
		}),
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "anthropic", Model: "claude-sonnet-4"},
		},
	}
	opts := modelOptionsFor(cfg)
	// Providers are sorted; catalog order preserved within a provider.
	require.Equal(t, "anthropic/claude-sonnet-4", string(opts[0].Value))
	require.Equal(t, "openai/gpt-4o", string(opts[1].Value))
	require.Equal(t, "GPT-4o", opts[1].Name)
	// An empty Name falls back to the model id.
	require.Equal(t, "openai/gpt-4o-mini", string(opts[2].Value))
	require.Equal(t, "gpt-4o-mini", opts[2].Name)
	require.Equal(t, "anthropic/claude-sonnet-4", modelSelectValue(cfg))
}

func TestModelOptionsFor_ExcludesDisabledAndIncludesCurrent(t *testing.T) {
	cfg := &config.Config{
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			"openai":   {Models: []catwalk.Model{{ID: "gpt-4o"}}},
			"disabled": {Disable: true, Models: []catwalk.Model{{ID: "nope"}}},
		}),
		Models: map[config.SelectedModelType]config.SelectedModel{
			// Current model is in no provider catalog; it must still be offered.
			config.SelectedModelTypeLarge: {Provider: "custom", Model: "my-model"},
		},
	}
	var values []string
	for _, o := range modelOptionsFor(cfg) {
		values = append(values, string(o.Value))
	}
	require.Contains(t, values, "custom/my-model")
	require.NotContains(t, values, "disabled/nope")
	require.Contains(t, values, "openai/gpt-4o")
}

func TestConfigOptionsIncludesModel(t *testing.T) {
	cfg := &config.Config{
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			"openai": {Models: []catwalk.Model{{ID: "gpt-4o", Name: "GPT-4o"}}},
		}),
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "openai", Model: "gpt-4o"},
		},
	}
	var modelOpt *acp.SessionConfigOptionSelect
	for _, o := range configOptionsFor(cfg) {
		if o.Select != nil && o.Select.Id == "model" {
			modelOpt = o.Select
		}
	}
	require.NotNil(t, modelOpt, "expected a model select config option")
	require.Equal(t, acp.SessionConfigValueId("openai/gpt-4o"), modelOpt.CurrentValue)
	require.NotNil(t, modelOpt.Options.Ungrouped)
	require.Len(t, *modelOpt.Options.Ungrouped, 1)
}
