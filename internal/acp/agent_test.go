package acp

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
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
