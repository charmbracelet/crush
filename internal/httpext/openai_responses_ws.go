package httpext

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

func WrapOpenAIResponsesWebSocketHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		return &http.Client{Transport: openAIResponsesWebSocketTransport{base: http.DefaultTransport}}
	}

	clone := *client
	clone.Transport = openAIResponsesWebSocketTransport{base: client.Transport}
	return &clone
}

type openAIResponsesWebSocketTransport struct {
	base http.RoundTripper
}

func (t openAIResponsesWebSocketTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	if req == nil || req.URL == nil || req.Method != http.MethodPost || !isResponsesPath(req.URL.Path) {
		return base.RoundTrip(req)
	}

	body, payload, stream, err := readStreamingRequestBody(req)
	if err != nil {
		return nil, err
	}
	if !stream {
		restoreRequestBody(req, body)
		return base.RoundTrip(req)
	}

	wsURL := toWebSocketURL(*req.URL)
	headers := req.Header.Clone()
	if shouldSetOpenAIBetaHeader(wsURL, headers) {
		headers.Set("OpenAI-Beta", "responses-api=v1")
	}

	requestPayload := map[string]any{"type": "response.create"}
	for k, v := range payload {
		if k == "stream" {
			continue
		}
		requestPayload[k] = v
	}

	message, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal websocket request: %w", err)
	}

	dialer := websocket.Dialer{Proxy: http.ProxyFromEnvironment}
	conn, resp, err := dialer.DialContext(req.Context(), wsURL.String(), headers)
	if err != nil {
		return nil, formatWebSocketDialError(err, resp)
	}

	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(req.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer func() { _ = conn.Close() }()
		defer func() { _ = writer.Close() }()

		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			_ = writer.CloseWithError(fmt.Errorf("send websocket request: %w", err))
			return
		}

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || errors.Is(err, io.EOF) {
					return
				}
				_ = writer.CloseWithError(fmt.Errorf("read websocket event: %w", err))
				return
			}

			eventType := websocketEventType(data)
			if _, err := writer.Write([]byte("event: " + eventType + "\n")); err != nil {
				return
			}
			if _, err := writer.Write([]byte("data: ")); err != nil {
				return
			}
			if _, err := writer.Write(data); err != nil {
				return
			}
			if _, err := writer.Write([]byte("\n\n")); err != nil {
				return
			}
		}
	}()

	bodyCloser := &webSocketStreamBody{
		ReadCloser: reader,
		closeFn: func() error {
			cancel()
			_ = conn.Close()
			<-done
			return reader.Close()
		},
	}

	go func() {
		<-ctx.Done()
		_ = bodyCloser.Close()
	}()

	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Body:    bodyCloser,
		Request: req,
	}, nil
}

type webSocketStreamBody struct {
	io.ReadCloser
	once    sync.Once
	closeFn func() error
}

func (b *webSocketStreamBody) Close() error {
	var err error
	b.once.Do(func() {
		if b.closeFn != nil {
			err = b.closeFn()
			return
		}
		err = b.ReadCloser.Close()
	})
	return err
}

func isResponsesPath(path string) bool {
	if path == "/responses" || path == "/responses/" {
		return true
	}
	return strings.HasSuffix(path, "/responses")
}

func readStreamingRequestBody(req *http.Request) ([]byte, map[string]any, bool, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, nil, false, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, nil, false, fmt.Errorf("read request body: %w", err)
	}

	payload := make(map[string]any)
	if err := json.Unmarshal(body, &payload); err != nil {
		restoreRequestBody(req, body)
		return nil, nil, false, fmt.Errorf("decode request body: %w", err)
	}

	stream, _ := payload["stream"].(bool)
	return body, payload, stream, nil
}

func restoreRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
}

func toWebSocketURL(httpURL url.URL) url.URL {
	httpURL.Scheme = mapToWebSocketScheme(httpURL.Scheme)
	return httpURL
}

func mapToWebSocketScheme(scheme string) string {
	switch strings.ToLower(scheme) {
	case "http":
		return "ws"
	case "https":
		return "wss"
	default:
		return scheme
	}
}

func shouldSetOpenAIBetaHeader(wsURL url.URL, headers http.Header) bool {
	if headers.Get("OpenAI-Beta") != "" {
		return false
	}
	host := strings.ToLower(wsURL.Hostname())
	return host == "api.openai.com" || strings.HasSuffix(host, ".api.openai.com")
}

func websocketEventType(data []byte) string {
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &event); err != nil || event.Type == "" {
		return "message"
	}
	return event.Type
}

func formatWebSocketDialError(err error, resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return fmt.Errorf("dial websocket: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if readErr != nil || len(body) == 0 {
		return fmt.Errorf("dial websocket: %w", err)
	}
	return fmt.Errorf("dial websocket: %w: %s", err, strings.TrimSpace(string(body)))
}
