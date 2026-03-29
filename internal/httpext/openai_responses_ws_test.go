package httpext

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/openai/openai-go/v2/packages/ssestream"
	"github.com/openai/openai-go/v2/responses"
	"github.com/stretchr/testify/require"
)

func TestWrapOpenAIResponsesWebSocketHTTPClientStreamsResponseEvents(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{}
	var requestPayload map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/responses", r.URL.Path)
		require.Equal(t, "responses-api=v1", r.Header.Get("OpenAI-Beta"))

		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(msg, &requestPayload))

		created := map[string]any{
			"type": "response.created",
			"response": map[string]any{
				"id": "resp_123",
			},
		}
		completed := map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     "resp_123",
				"output": []any{},
				"usage": map[string]any{
					"input_tokens":          1,
					"output_tokens":         1,
					"total_tokens":          2,
					"input_tokens_details":  map[string]any{},
					"output_tokens_details": map[string]any{},
				},
			},
		}

		createdJSON, err := json.Marshal(created)
		require.NoError(t, err)
		completedJSON, err := json.Marshal(completed)
		require.NoError(t, err)

		require.NoError(t, conn.WriteMessage(websocket.TextMessage, createdJSON))
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, completedJSON))
		require.NoError(t, conn.Close())
	}))
	defer srv.Close()

	wrapped := WrapOpenAIResponsesWebSocketHTTPClient(srv.Client())
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/v1/responses", strings.NewReader(`{"model":"gpt-5","stream":true,"input":[]}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "responses-api=v1")

	resp, err := wrapped.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	stream := ssestream.NewStream[responses.ResponseStreamEventUnion](ssestream.NewDecoder(resp), nil)
	require.True(t, stream.Next())
	require.Equal(t, "response.created", stream.Current().Type)
	require.True(t, stream.Next())
	require.Equal(t, "response.completed", stream.Current().Type)
	require.NoError(t, stream.Err())

	require.Equal(t, "response.create", requestPayload["type"])
	_, hasStream := requestPayload["stream"]
	require.False(t, hasStream)
	require.Equal(t, "gpt-5", requestPayload["model"])
}

func TestWrapOpenAIResponsesWebSocketHTTPClientPassesThroughNonStreamingRequests(t *testing.T) {
	t.Parallel()

	sawRequest := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRequest = true
		require.Equal(t, "/v1/responses", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	wrapped := WrapOpenAIResponsesWebSocketHTTPClient(srv.Client())
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/v1/responses", strings.NewReader(`{"model":"gpt-5","stream":false,"input":[]}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := wrapped.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, `{"ok":true}`, string(body))
	require.True(t, sawRequest)
}
