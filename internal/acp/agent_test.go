package acp

import (
	"encoding/base64"
	"io"
	"log/slog"
	"testing"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/permission"
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
	opts := configOptionsFor(cfg, "", false)
	require.NotEmpty(t, opts)

	var found bool
	for _, o := range opts {
		if o.Select != nil && o.Select.Id == "thinking" {
			found = true
			require.Equal(t, acp.SessionConfigValueId("on"), o.Select.CurrentValue)
			require.NotNil(t, o.Select.Options.Ungrouped)
			require.Len(t, *o.Select.Options.Ungrouped, 2)
		}
	}
	require.True(t, found, "expected a thinking config option")

	// Without the client advertising boolean support the spec forbids a
	// boolean option, so a select is the mandated fallback
	// (session-config-options.md:207-209).
	for _, o := range opts {
		require.Nil(t, o.Boolean, "no config option may be a boolean")
	}

	// With the capability advertised, thinking becomes a real toggle.
	boolOpts := configOptionsFor(cfg, "", true)
	found = false
	for _, o := range boolOpts {
		if o.Boolean != nil && o.Boolean.Id == "thinking" {
			found = true
			require.Equal(t, "boolean", o.Boolean.Type)
			require.True(t, o.Boolean.CurrentValue)
		}
		require.False(t, o.Select != nil && o.Select.Id == "thinking", "boolean-capable clients get no thinking select")
	}
	require.True(t, found, "expected a boolean thinking config option")
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
	for _, o := range configOptionsFor(cfg, "", false) {
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
	for _, o := range configOptionsFor(cfg, "", false) {
		if o.Select != nil && o.Select.Id == "mode" {
			modeOpt = o.Select
		}
	}
	require.NotNil(t, modeOpt, "expected a mode select config option")
	require.NotNil(t, modeOpt.Category)
	require.Equal(t, acp.SessionConfigOptionCategoryMode, *modeOpt.Category)
	// coder is the default mode, and task is never offered as one.
	require.Equal(t, acp.SessionConfigValueId("code"), modeOpt.CurrentValue)
	require.NotNil(t, modeOpt.Options.Ungrouped)
	require.Len(t, *modeOpt.Options.Ungrouped, len(acpModes))
	for _, o := range *modeOpt.Options.Ungrouped {
		require.NotEqual(t, config.AgentTask, string(o.Value), "task is a subagent profile, not a session mode")
	}
}

func TestAcpModeRing(t *testing.T) {
	// The ring flattens the TUI's four states onto Crush's two agents.
	want := []struct {
		id    acp.SessionModeId
		agent string
		perm  permission.PermissionMode
	}{
		{"code", config.AgentCoder, permission.PermissionModeNormal},
		{"plan", config.AgentPlan, permission.PermissionModeNormal},
		{"yolo", config.AgentCoder, permission.PermissionModeYolo},
		{"sysadmin", config.AgentCoder, permission.PermissionModeSysadmin},
	}
	require.Len(t, acpModes, len(want))
	for i, w := range want {
		require.Equal(t, w.id, acpModes[i].id)
		require.Equal(t, w.agent, acpModes[i].agent)
		require.Equal(t, w.perm, acpModes[i].perm)
		require.NotEmpty(t, acpModes[i].name)
		require.NotEmpty(t, acpModes[i].description)
	}
}

func TestResolveModeId(t *testing.T) {
	// Every ring id resolves, and "coder" is kept as a legacy alias.
	for _, m := range acpModes {
		got, ok := resolveModeId(m.id)
		require.True(t, ok, "mode %q must resolve", m.id)
		require.Equal(t, m.id, got.id)
	}
	got, ok := resolveModeId("coder")
	require.True(t, ok)
	require.Equal(t, acp.SessionModeId("code"), got.id)

	// Agent names that are not session modes, and nonsense, do not resolve.
	// task is a subagent profile with a reduced tool set.
	for _, bad := range []acp.SessionModeId{"", "task", "nonsense"} {
		_, ok := resolveModeId(bad)
		require.False(t, ok, "%q must not resolve", bad)
	}
}

func TestIsSelectableMode(t *testing.T) {
	require.True(t, isSelectableMode("code"))
	require.True(t, isSelectableMode("plan"))
	require.True(t, isSelectableMode("yolo"))
	require.True(t, isSelectableMode("sysadmin"))
	require.True(t, isSelectableMode("coder"), "legacy alias")
	require.False(t, isSelectableMode("task"))
	require.False(t, isSelectableMode(""))
}

func TestModeForState(t *testing.T) {
	// Exact agent+permission pairs win.
	require.Equal(t, acp.SessionModeId("code"), modeForState(config.AgentCoder, permission.PermissionModeNormal).id)
	require.Equal(t, acp.SessionModeId("plan"), modeForState(config.AgentPlan, permission.PermissionModeNormal).id)
	require.Equal(t, acp.SessionModeId("yolo"), modeForState(config.AgentCoder, permission.PermissionModeYolo).id)
	require.Equal(t, acp.SessionModeId("sysadmin"), modeForState(config.AgentCoder, permission.PermissionModeSysadmin).id)

	// plan + yolo is reachable in the TUI but has no ACP id, so it degrades to
	// plan rather than reporting a mode outside availableModes.
	require.Equal(t, acp.SessionModeId("plan"), modeForState(config.AgentPlan, permission.PermissionModeYolo).id)

	// An unknown agent still reports a mode that is in the list.
	require.Equal(t, acpModes[0].id, modeForState("mystery-agent", permission.PermissionModeNormal).id)
}

func TestCurrentModeDefaultsToCode(t *testing.T) {
	// No coordinator or permission service wired (as in unit tests).
	a := &crushAgent{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	require.Equal(t, acpModes[0].id, a.currentMode())
}

func TestModeStateAdvertisesTheRing(t *testing.T) {
	a := &crushAgent{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	// No config store means no session modes to report.
	require.Nil(t, a.modeState(""))

	modes := availableModes()
	require.Len(t, modes, len(acpModes))
	// currentModeId must always be one of availableModes
	// (session-modes.md:58-64).
	require.Equal(t, acpModes[0].id, a.currentMode())
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

func TestMapFinishReason(t *testing.T) {
	tests := []struct {
		reason fantasy.FinishReason
		want   acp.StopReason
	}{
		{fantasy.FinishReasonStop, acp.StopReasonEndTurn},
		{fantasy.FinishReasonToolCalls, acp.StopReasonEndTurn},
		{fantasy.FinishReasonOther, acp.StopReasonEndTurn},
		{fantasy.FinishReasonUnknown, acp.StopReasonEndTurn},
		{fantasy.FinishReasonLength, acp.StopReasonMaxTokens},
		{fantasy.FinishReasonContentFilter, acp.StopReasonRefusal},
		{fantasy.FinishReasonError, acp.StopReasonEndTurn},
	}
	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			require.Equal(t, tt.want, mapFinishReason(tt.reason))
		})
	}
}

func TestWithTraceMetaGeneratesTraceparent(t *testing.T) {
	// A prompt without _meta still gets a correlatable trace id
	// (extensibility.md:40-44).
	meta := withTraceMeta(nil)
	var tp string
	require.NoError(t, meta.Get("traceparent", &tp))
	require.NotEmpty(t, tp, "expected a generated traceparent")
	// W3C trace-context grammar: version-traceid-spanid-flags.
	require.Regexp(t, `^00-[0-9a-f]{32}-[0-9a-f]{16}-01$`, tp)

	// A client-supplied traceparent wins.
	in, err := acp.NewMeta(map[string]any{"traceparent": "00-abc-123-01", "custom": true})
	require.NoError(t, err)
	out := withTraceMeta(in)
	var got string
	require.NoError(t, out.Get("traceparent", &got))
	require.Equal(t, "00-abc-123-01", got)
	var custom bool
	require.NoError(t, out.Get("custom", &custom))
	require.Equal(t, true, custom)

	// The caller's map is not mutated.
	empty := acp.Meta{}
	var generated string
	require.NoError(t, withTraceMeta(empty).Get("traceparent", &generated))
	require.NotEmpty(t, generated)
	require.Empty(t, empty)
}
