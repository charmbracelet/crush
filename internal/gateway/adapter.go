// Package gateway runs a provider's traffic through the jq programs its
// configuration declares: requests and responses are rewritten in process,
// which lets a plugin adapt a provider whose wire format differs from what
// the SDK speaks without any provider-specific code in Crush.
package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/crush/internal/jq"
	"github.com/charmbracelet/crush/internal/oauth"
)

// maxBody bounds incoming request and unary response bodies.
const maxBody = 64 << 20

// Adapter rewrites HTTP traffic through the two jq programs of a [oauth.GatewaySpec].
type Adapter struct {
	Base     http.RoundTripper
	Token    string
	request  *jq.Program
	response *jq.Program
}

// New uses the default transport when base is nil.
func New(spec *oauth.GatewaySpec, token string, base http.RoundTripper) (*Adapter, error) {
	if !spec.Usable() {
		return nil, fmt.Errorf("gateway adapter needs both a request and a response program")
	}
	reqCode, err := jq.Compile(spec.Request)
	if err != nil {
		return nil, fmt.Errorf("gateway request program: %w", err)
	}
	respCode, err := jq.Compile(spec.Response)
	if err != nil {
		return nil, fmt.Errorf("gateway response program: %w", err)
	}
	if base == nil {
		base = http.DefaultTransport
	}
	return &Adapter{Base: base, Token: token, request: reqCode, response: respCode}, nil
}

// RoundTrip rewrites both halves of an HTTP exchange through jq.
func (a *Adapter) RoundTrip(req *http.Request) (*http.Response, error) {
	requestID := randomToken(12)
	rewritten, err := a.rewriteRequest(req, requestID)
	if err != nil {
		return nil, err
	}

	resp, err := a.Base.RoundTrip(rewritten)
	if err != nil {
		return nil, err
	}
	if err := a.rewriteResponse(req.Context(), resp, requestID); err != nil {
		return nil, err
	}
	return resp, nil
}

func (a *Adapter) rewriteRequest(req *http.Request, requestID string) (*http.Request, error) {
	raw, err := readAndCloseBody(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	// Non-JSON input is passed through unless the program supplies a body.
	body, _ := parseJSON(raw)

	input := map[string]any{
		"method":     req.Method,
		"url":        req.URL.String(),
		"headers":    firstHeaders(req.Header),
		"body":       body,
		"request_id": requestID,
		"token":      a.Token,
	}
	output, err := a.request.Run(req.Context(), input)
	if err != nil {
		return nil, fmt.Errorf("rewrite request: %w", err)
	}
	rules, ok := output.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("rewrite request: program output must be an object, got %T", output)
	}

	clone := req.Clone(req.Context())
	if url, ok := rules["url"].(string); ok && url != "" {
		parsed, err := parseAbsolute(url)
		if err != nil {
			return nil, fmt.Errorf("rewrite request url: %w", err)
		}
		// Clone keeps the original Host header, and some front ends route on
		// it: a rewritten URL must be addressed to the host it names.
		clone.Host = ""
		clone.URL = parsed
	}
	if headers, ok := rules["headers"].(map[string]any); ok {
		for key, value := range headers {
			if text, ok := value.(string); ok {
				clone.Header.Set(key, text)
			}
		}
	}
	if drops, ok := rules["drop_headers"].([]any); ok {
		for _, key := range drops {
			if text, ok := key.(string); ok {
				clone.Header.Del(text)
			}
		}
	}
	sent := raw
	if replacement := rules["body"]; replacement != nil {
		encoded, err := json.Marshal(replacement)
		if err != nil {
			return nil, fmt.Errorf("encode rewritten body: %w", err)
		}
		sent = encoded
		if clone.Header.Get("Content-Type") == "" {
			clone.Header.Set("Content-Type", "application/json")
		}
	}
	setRequestBody(clone, sent)
	return clone, nil
}

func setRequestBody(req *http.Request, data []byte) {
	body := func() io.ReadCloser {
		if len(data) == 0 {
			return http.NoBody
		}
		return io.NopCloser(bytes.NewReader(data))
	}
	req.Body = body()
	req.GetBody = func() (io.ReadCloser, error) { return body(), nil }
	req.ContentLength = int64(len(data))
}

func parseAbsolute(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw) //nolint:varnamelen // matches the surrounding style
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%q is not an http URL", raw)
	}
	return parsed, nil
}

func (a *Adapter) rewriteResponse(ctx context.Context, resp *http.Response, requestID string) error {
	base := map[string]any{
		"status":     resp.StatusCode,
		"headers":    firstHeaders(resp.Header),
		"request_id": requestID,
		"token":      a.Token,
	}

	if isEventStream(resp) {
		resp.Body = newEventConverter(ctx, a.response, base, resp.Body)
		// The upstream length does not apply to the rewritten stream.
		resp.ContentLength = -1
		resp.Header.Del("Content-Length")
		return nil
	}

	raw, err := readAndCloseBody(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	body, _ := parseJSON(raw)
	base["body"] = body
	output, err := a.response.Run(ctx, base)
	if err != nil {
		return fmt.Errorf("rewrite response: %w", err)
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("encode rewritten response: %w", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(encoded))
	resp.ContentLength = int64(len(encoded))
	resp.Header.Del("Content-Length")
	return nil
}

// firstHeaders flattens a header set to one value per key, the shape the
// programs work with. The values are boxed as any so a program can iterate
// the map: jq understands map[string]any, not the map[string]string Go
// headers natively are.
func firstHeaders(h http.Header) map[string]any {
	flat := make(map[string]any, len(h))
	for key, values := range h {
		if len(values) > 0 {
			flat[key] = values[0]
		}
	}
	return flat
}

func readAndCloseBody(body io.ReadCloser) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxBody {
		return nil, fmt.Errorf("body exceeds %d bytes", maxBody)
	}
	return raw, nil
}

// parseJSON reports whether raw parses as JSON, returning the value.
func parseJSON(raw []byte) (any, bool) {
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

func isEventStream(resp *http.Response) bool {
	return strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream")
}

func randomToken(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
