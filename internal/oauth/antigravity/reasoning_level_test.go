package antigravity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wrappedEnvelope asks the transport to wrap a Gemini-shaped request and hands
// back the model it will ask the gateway for, plus the inner request.
func wrappedEnvelope(t *testing.T, model, body string) (string, map[string]any) {
	t.Helper()

	transport := &Transport{}
	raw, err := transport.envelope(model, verbStream, []byte(body))
	require.NoError(t, err)

	var env struct {
		Model   string         `json:"model"`
		Request map[string]any `json:"request"`
	}
	require.NoError(t, json.Unmarshal(raw, &env))
	return env.Model, env.Request
}

// The gateway sells reasoning effort as separate models rather than as a
// parameter, so the level crush puts in the body has to come back out as an id
// suffix.
func TestEnvelopeTurnsThinkingLevelIntoModelSuffix(t *testing.T) {
	tests := []struct {
		name      string
		inModel   string
		body      string
		wantModel string
	}{
		{
			name:      "high",
			inModel:   "gemini-3.8-flash",
			body:      `{"contents":[],"generationConfig":{"thinkingConfig":{"thinkingLevel":"high","includeThoughts":true}}}`,
			wantModel: "gemini-3.8-flash-high",
		},
		{
			name:      "minimal becomes extra-low",
			inModel:   "gemini-3.8-flash",
			body:      `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"minimal","includeThoughts":true}}}`,
			wantModel: "gemini-3.8-flash-extra-low",
		},
		{
			name:      "level is case insensitive",
			inModel:   "gemini-3.8-flash",
			body:      `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"MEDIUM"}}}`,
			wantModel: "gemini-3.8-flash-medium",
		},
		{
			name:      "selector wins over a tier named in the id",
			inModel:   "gemini-3.8-flash-high",
			body:      `{"generationConfig":{"thinkingConfig":{"thinkingLevel":"low"}}}`,
			wantModel: "gemini-3.8-flash-low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, request := wrappedEnvelope(t, tt.inModel, tt.body)
			assert.Equal(t, tt.wantModel, got)

			// The level now travels in the model name; leaving it in the body
			// would be a second knob for one setting.
			if config := thinkingConfigOf(t, request); config != nil {
				_, present := config["thinkingLevel"]
				assert.False(t, present, "thinkingLevel left in the body: %v", config)
			}
		})
	}
}

func thinkingConfigOf(t *testing.T, request map[string]any) map[string]any {
	t.Helper()
	gen, ok := request["generationConfig"].(map[string]any)
	if !ok {
		return nil
	}
	config, _ := gen["thinkingConfig"].(map[string]any)
	return config
}

// includeThoughts is a different question from how hard to think, so it stays.
func TestEnvelopeKeepsIncludeThoughts(t *testing.T) {
	_, request := wrappedEnvelope(t, "gemini-3.8-flash",
		`{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high","includeThoughts":true}}}`)

	config := thinkingConfigOf(t, request)
	require.NotNil(t, config)
	assert.Equal(t, true, config["includeThoughts"])
}

// Without a level there is nothing to translate: crush asked for the model it
// named, and the request must arrive unchanged.
func TestEnvelopeWithoutThinkingLevel(t *testing.T) {
	bodies := []string{
		`{"contents":[]}`,
		`{"generationConfig":{"temperature":0.2}}`,
		`{"generationConfig":{"thinkingConfig":{"includeThoughts":true}}}`,
		`{"generationConfig":{"thinkingConfig":{"thinkingLevel":""}}}`,
	}
	for _, body := range bodies {
		got, request := wrappedEnvelope(t, "gemini-3.1-pro", body)
		assert.Equalf(t, "gemini-3.1-pro", got, "body %s", body)

		if config := thinkingConfigOf(t, request); config != nil {
			level, present := config["thinkingLevel"]
			if present {
				assert.Emptyf(t, level, "body %s: only an empty level may remain", body)
			}
		}
	}
}

// A level the gateway does not sell is not ours to reinterpret: leave both the
// id and the body as crush wrote them so the backend answers instead.
func TestEnvelopeLeavesUnknownLevelAlone(t *testing.T) {
	got, request := wrappedEnvelope(t, "gemini-3.8-flash",
		`{"generationConfig":{"thinkingConfig":{"thinkingLevel":"turbo"}}}`)

	assert.Equal(t, "gemini-3.8-flash", got)
	config := thinkingConfigOf(t, request)
	require.NotNil(t, config)
	assert.Equal(t, "turbo", config["thinkingLevel"])
}

// Anthropic-family ids are mangled inside crush and unmangled for the gateway,
// and the level has to survive that trip.
func TestEnvelopeAppliesLevelToMangledID(t *testing.T) {
	got, _ := wrappedEnvelope(t, "cld-sonnet-4-6",
		`{"generationConfig":{"thinkingConfig":{"thinkingLevel":"high"}}}`)

	assert.Equal(t, "claude-sonnet-4-6-high", got)
}
