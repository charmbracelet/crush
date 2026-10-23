package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestFormatJSONPreservesNumbers(t *testing.T) {
	t.Parallel()

	for name, input := range map[string]string{
		"large integer":     `{"id":9007199254740993}`,
		"negative integer":  `{"id":-9007199254740993}`,
		"precise decimal":   `{"value":0.1234567890123456789}`,
		"large exponent":    `{"value":1e400}`,
		"nested numbers":    `{"values":[9007199254740993,{"value":0.1234567890123456789}]}`,
		"top-level integer": `9007199254740993`,
		"key order":         `{"z":1,"a":2}`,
		"duplicate keys":    `{"a":1,"a":2}`,
		"html characters":   `{"html":"<b>&</b>"}`,
		"string escapes":    `{"s":"\u00e9"}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			formatted := formatJSON(input)
			var compact bytes.Buffer
			require.NoError(t, json.Compact(&compact, []byte(formatted)))
			require.Equal(t, input, compact.String())
		})
	}
}

func TestFormatJSONKeepsInvalidInput(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", `{"id":}`, `{"id":1} {"id":2}`, `{"id":1} trailing`} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, input, formatJSON(input))
		})
	}
}

func TestFormatJSONIndentation(t *testing.T) {
	t.Parallel()

	formatted := formatJSON(" \n\t" + `{"values":[true,null,"text",1]}` + "\r\n ")
	require.Equal(t, "{\n  \"values\": [\n    true,\n    null,\n    \"text\",\n    1\n  ]\n}", formatted)
}

func TestWebFetchPreservesJSONNumbers(t *testing.T) {
	t.Parallel()

	const body = `{"id":9007199254740993,"value":0.1234567890123456789}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	tool := NewWebFetchTool(t.TempDir(), server.Client())
	input, err := json.Marshal(WebFetchParams{URL: server.URL})
	require.NoError(t, err)
	response, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "web-fetch-numbers",
		Name:  WebFetchToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Contains(t, response.Content, `"id": 9007199254740993`)
	require.Contains(t, response.Content, `"value": 0.1234567890123456789`)
}
