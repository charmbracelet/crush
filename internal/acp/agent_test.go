package acp

import (
	"encoding/base64"
	"io"
	"log/slog"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
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
	opts := configOptionsFor(cfg, "")
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
	for _, o := range configOptionsFor(cfg, "") {
		if o.Select != nil && o.Select.Id == "model" {
			modelOpt = o.Select
		}
	}
	require.NotNil(t, modelOpt, "expected a model select config option")
	require.Equal(t, acp.SessionConfigValueId("openai/gpt-4o"), modelOpt.CurrentValue)
	require.NotNil(t, modelOpt.Options.Ungrouped)
	require.Len(t, *modelOpt.Options.Ungrouped, 1)
}

func TestValidateConfigOption_AcceptsAdvertisedValues(t *testing.T) {
	cfg := &config.Config{
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			"openai": {Models: []catwalk.Model{{ID: "gpt-4o", Name: "GPT-4o"}}},
		}),
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "openai", Model: "gpt-4o"},
		},
	}

	// Both advertised option families validate.
	require.NoError(t, validateConfigOption(cfg, acp.SetSessionConfigOptionRequest{
		Boolean: &acp.SetSessionConfigOptionBoolean{ConfigId: "thinking", Value: true},
	}))
	require.NoError(t, validateConfigOption(cfg, acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{ConfigId: "model", Value: "openai/gpt-4o"},
	}))
}

func TestConfigOptionsIncludesMode(t *testing.T) {
	cfg := &config.Config{
		Agents: map[string]config.Agent{
			config.AgentCoder: {Name: "Coder"},
			config.AgentPlan:  {Name: "Plan"},
			config.AgentTask:  {Name: "Task"},
		},
	}

	var modeOpt *acp.SessionConfigOptionSelect
	for _, o := range configOptionsFor(cfg, "") {
		if o.Select != nil && o.Select.Id == "mode" {
			modeOpt = o.Select
		}
	}
	require.NotNil(t, modeOpt, "expected a mode select config option")
	require.NotNil(t, modeOpt.Category)
	require.Equal(t, acp.SessionConfigOptionCategoryMode, *modeOpt.Category)
	// coder is the default mode, and task is never offered as one.
	require.Equal(t, acp.SessionConfigValueId(config.AgentCoder), modeOpt.CurrentValue)
	require.NotNil(t, modeOpt.Options.Ungrouped)
	require.Len(t, *modeOpt.Options.Ungrouped, 2)
	for _, o := range *modeOpt.Options.Ungrouped {
		require.NotEqual(t, config.AgentTask, string(o.Value), "task is a subagent profile, not a session mode")
	}
}

func TestSelectableModesExcludesTaskAndDisabled(t *testing.T) {
	cfg := &config.Config{
		Agents: map[string]config.Agent{
			config.AgentCoder: {Name: "Coder"},
			config.AgentPlan:  {Name: "Plan"},
			config.AgentTask:  {Name: "Task"},
			"disabled-agent":  {Name: "Off", Disabled: true},
			"zzz-custom":      {Name: "Custom"},
		},
	}
	require.Equal(t, []string{config.AgentCoder, config.AgentPlan, "zzz-custom"}, selectableModes(cfg))
	require.True(t, isSelectableMode(cfg, config.AgentPlan))
	require.False(t, isSelectableMode(cfg, config.AgentTask))
	require.False(t, isSelectableMode(cfg, "disabled-agent"))
	require.False(t, isSelectableMode(cfg, ""))
}

func TestDefaultModeFallsBackWhenCoderMissing(t *testing.T) {
	// coder disabled: the default must still be a mode that is advertised.
	cfg := &config.Config{
		Agents: map[string]config.Agent{
			config.AgentCoder: {Disabled: true},
			config.AgentPlan:  {Name: "Plan"},
		},
	}
	require.Equal(t, acp.SessionModeId(config.AgentPlan), defaultMode(cfg))

	// No selectable agents at all: no mode claim is made.
	require.Empty(t, defaultMode(&config.Config{Agents: map[string]config.Agent{
		config.AgentTask: {Name: "Task"},
	}}))
}

func TestModeStateReportsCurrentMode(t *testing.T) {
	cfg := &config.Config{
		Agents: map[string]config.Agent{
			config.AgentCoder: {Name: "Coder"},
			config.AgentPlan:  {Name: "Plan"},
		},
	}
	a := &crushAgent{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	a.activeMode.Store("sess", acp.SessionModeId(config.AgentPlan))

	// currentMode is the resolver both modeState and the config option use, so
	// assert on it directly: the agent carries no config store in this test.
	require.Equal(t, acp.SessionModeId(config.AgentPlan), a.currentMode(cfg, "sess"))
	// An unknown session falls back to the default rather than reporting empty.
	require.Equal(t, acp.SessionModeId(config.AgentCoder), a.currentMode(cfg, "other"))
	// A stored mode that is no longer selectable is ignored.
	a.activeMode.Store("stale", acp.SessionModeId(config.AgentTask))
	require.Equal(t, acp.SessionModeId(config.AgentCoder), a.currentMode(cfg, "stale"))
}

func TestBuildPromptBlobResource(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	blob := acp.ContentBlockResource{
		Resource: acp.EmbeddedResourceResource{
			BlobResourceContents: &acp.BlobResourceContents{
				Uri:  "file:///img.png",
				Blob: base64.StdEncoding.EncodeToString(png),
			},
		},
	}
	prompt, attachments := buildPrompt([]acp.ContentBlock{{Resource: &blob}})
	// The MIME type is undeclared, so it is sniffed: a PNG must attach as an
	// image rather than degrade to a text placeholder.
	require.Len(t, attachments, 1)
	require.Equal(t, "image/png", attachments[0].MimeType)
	require.Equal(t, png, attachments[0].Content)
	require.Empty(t, prompt)

	// A non-media blob is described in the prompt so nothing is silently lost.
	text := acp.ContentBlockResource{
		Resource: acp.EmbeddedResourceResource{
			BlobResourceContents: &acp.BlobResourceContents{
				Uri:      "file:///data.bin",
				Blob:     base64.StdEncoding.EncodeToString([]byte("payload")),
				MimeType: acp.Ptr("application/octet-stream"),
			},
		},
	}
	prompt, attachments = buildPrompt([]acp.ContentBlock{{Resource: &text}})
	require.Empty(t, attachments)
	require.Contains(t, prompt, "file:///data.bin")
	require.Contains(t, prompt, "application/octet-stream")
}

func TestValidateConfigOption_RejectsUnknown(t *testing.T) {
	cfg := &config.Config{
		Providers: csync.NewMapFrom(map[string]config.ProviderConfig{
			"openai": {Models: []catwalk.Model{{ID: "gpt-4o"}}},
		}),
		Models: map[config.SelectedModelType]config.SelectedModel{
			config.SelectedModelTypeLarge: {Provider: "openai", Model: "gpt-4o"},
		},
	}

	// Unknown configId, unknown value, and an empty request all fail.
	require.Error(t, validateConfigOption(cfg, acp.SetSessionConfigOptionRequest{
		Boolean: &acp.SetSessionConfigOptionBoolean{ConfigId: "yolo", Value: true},
	}))
	require.Error(t, validateConfigOption(cfg, acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{ConfigId: "model", Value: "openai/nonexistent"},
	}))
	require.Error(t, validateConfigOption(cfg, acp.SetSessionConfigOptionRequest{}))
}
