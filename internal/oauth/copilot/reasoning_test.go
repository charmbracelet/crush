package copilot

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// makeTestResponse creates a minimal *http.Response for testing.
func makeTestResponse(contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestTransformReasoningLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "transforms reasoning_text to reasoning_content",
			input: `data: {"choices":[{"delta":{"content":"","reasoning_text":"hello","role":"assistant"}}]}`,
			want:  `data: {"choices":[{"delta":{"content":"","reasoning_content":"hello","role":"assistant"}}]}`,
		},
		{
			name:  "non-data line is unchanged",
			want:  `event: message`,
			input: `event: message`,
		},
		{
			name:  "data line without reasoning_text is unchanged",
			input: `data: {"choices":[{"delta":{"content":"hello"}}]}`,
			want:  `data: {"choices":[{"delta":{"content":"hello"}}]}`,
		},
		{
			name:  "DONE sentinel is unchanged",
			input: `data: [DONE]`,
			want:  `data: [DONE]`,
		},
		{
			name:  "empty reasoning_text is transformed",
			input: `data: {"choices":[{"delta":{"content":"","reasoning_text":"","role":"assistant"}}]}`,
			want:  `data: {"choices":[{"delta":{"content":"","reasoning_content":"","role":"assistant"}}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := transformReasoningLine(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestReasoningTransformReader(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"","reasoning_text":"thinking...","role":"assistant"}}]}`,
		`data: {"choices":[{"delta":{"content":"hello","role":"assistant"}}]}`,
		`data: [DONE]`,
		``,
	}, "\n")

	r := newReasoningTransformReader(io.NopCloser(strings.NewReader(input)))
	out, err := io.ReadAll(r)
	require.NoError(t, err)

	output := string(out)
	require.Contains(t, output, `"reasoning_content":"thinking..."`)
	require.NotContains(t, output, `"reasoning_text"`)
	require.Contains(t, output, `"content":"hello"`)
}

func TestWrapReasoningTransform_SSEResponse(t *testing.T) {
	t.Parallel()

	input := `data: {"choices":[{"delta":{"reasoning_text":"step 1"}}]}` + "\n"
	resp := makeTestResponse("text/event-stream", input)

	transformed := wrapReasoningTransform(resp)
	defer transformed.Body.Close()
	body, err := io.ReadAll(transformed.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), `"reasoning_content":"step 1"`)
	require.NotContains(t, string(body), `"reasoning_text"`)
}

func TestWrapReasoningTransform_NonSSEResponse(t *testing.T) {
	t.Parallel()

	input := `{"choices":[{"message":{"reasoning_text":"step 1"}}]}`
	resp := makeTestResponse("application/json", input)

	transformed := wrapReasoningTransform(resp)
	defer transformed.Body.Close()
	body, err := io.ReadAll(transformed.Body)
	require.NoError(t, err)
	// Non-SSE responses should not be transformed.
	require.Equal(t, input, string(body))
}
