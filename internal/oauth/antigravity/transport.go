package antigravity

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/crush/internal/oauth"
)

// Transport teaches the Gemini SDK the subscription backend's dialect.
//
// Requests arrive as ordinary Gemini API calls —
// `POST {base}/v1beta/models/{model}:streamGenerateContent?alt=sse` with a bare
// GenerateContentRequest — and are rewritten into the Cloud Code Assist
// envelope the subscription accepts. Replies are converted on the way back.
//
// The conversions are not cosmetic; each one prevents a specific failure:
//
//   - the `{"response": …}` wrapper must be removed, or the SDK parses an empty
//     reply and reports no candidates;
//   - cumulative usage repeats on every streamed chunk while crush *sums*
//     output tokens across chunks, so usage is attached only to the chunk that
//     ends the turn;
//   - a stream with no finishReason is treated as truncated and the answer is
//     discarded, so one is synthesised;
//   - a unary reply without usageMetadata panics crush's response mapper,
//     which dereferences it unguarded;
//   - tool calls echoed back in history lose empty argument maps, which the
//     Anthropic-family models on this gateway reject outright.
type Transport struct {
	// Base is the transport requests are forwarded to. Defaults to
	// http.DefaultTransport.
	Base http.RoundTripper
	// Token is the subscription OAuth token. A nil token passes requests
	// through untouched.
	Token *oauth.Token
	// Project pins the Cloud Code Assist project id. When empty the
	// backend derives it from the credential, which works for consumer
	// accounts.
	Project string
	// Endpoint is the subscription gateway. Defaults to APIBase; tests
	// point it at a stub server.
	Endpoint string
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if t.Token == nil {
		return base.RoundTrip(req)
	}

	model, verb, ok := parseGeminiPath(req.URL.Path)
	if !ok {
		// Not a generation call (model listing, token refresh). The
		// subscription bearer is valid for one gateway only, so it never
		// leaves this transport any other way.
		clone := req.Clone(req.Context())
		clone.Header.Del("Authorization")
		clone.Header.Del("x-goog-api-key")
		return base.RoundTrip(clone)
	}

	body, err := io.ReadAll(io.LimitReader(req.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("read gemini request body: %w", err)
	}
	_ = req.Body.Close()

	envelope, err := t.envelope(model, verb, body)
	if err != nil {
		return nil, err
	}

	clone := req.Clone(req.Context())
	clone.Method = http.MethodPost
	// Rewriting the URL is not enough: Clone keeps the original Host header,
	// and Google's front end routes on it, so the request would arrive at the
	// gateway addressed to the Gemini API and be answered with a bare 404.
	clone.Host = ""
	endpoint := t.Endpoint
	if endpoint == "" {
		endpoint = APIBase
	}
	target := endpoint + ":" + verb
	if verb == verbStream {
		// The SDK always asks for SSE; the backend needs it spelled out.
		target += "?alt=sse"
	}
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("build subscription URL: %w", err)
	}
	clone.URL = targetURL

	payload := bytes.NewReader(envelope)
	clone.Body = io.NopCloser(payload)
	clone.ContentLength = int64(len(envelope))

	// The SDK authenticates with an API key header; the subscription
	// backend wants an OAuth bearer instead.
	clone.Header.Del("x-goog-api-key")
	clone.Header.Set("Authorization", "Bearer "+t.Token.AccessToken)
	clone.Header.Set("Content-Type", "application/json")
	clone.Header.Set("User-Agent", UserAgent)
	if verb == verbStream {
		clone.Header.Set("Accept", "text/event-stream")
	}

	resp, err := base.RoundTrip(clone)
	if err != nil {
		return nil, err
	}
	return convertResponse(req, resp, verb), nil
}

const (
	verbStream = "streamGenerateContent"
	verbUnary  = "generateContent"
)

// parseGeminiPath splits `/v1beta/models/{model}:{verb}`.
func parseGeminiPath(path string) (model, verb string, ok bool) {
	if !strings.HasPrefix(path, "/v1beta/") && !strings.HasPrefix(path, "/v1/") {
		return "", "", false
	}
	index := strings.Index(path, "/models/")
	if index == -1 {
		return "", "", false
	}
	model, verb, ok = strings.Cut(path[index+len("/models/"):], ":")
	if !ok || model == "" {
		return "", "", false
	}
	switch verb {
	case verbStream, verbUnary:
		return model, verb, true
	}
	return "", "", false
}

// envelope wraps a GenerateContentRequest in the shape v1internal expects.
func (t *Transport) envelope(model, verb string, body []byte) ([]byte, error) {
	var in map[string]any
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, fmt.Errorf("gemini request body is not a JSON object: %w", err)
	}
	normalizeFunctionCalls(in)
	model = applyReasoningLevel(model, in)

	requestID, err := randomToken(12)
	if err != nil {
		return nil, err
	}
	env := map[string]any{
		// crush sees mangled ids for Anthropic-family models; the
		// backend only knows the real ones.
		"model":     RealModelID(model),
		"userAgent": "Antigravity/1.2.4",
		"requestId": requestID,
		"request":   in,
	}
	if t.Project != "" {
		env["project"] = t.Project
	}
	out, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("encode subscription envelope: %w", err)
	}
	return out, nil
}

// applyReasoningLevel moves the reasoning level crush selected out of the
// request body and into the model id, returning the id to ask the gateway for.
//
// The gateway has no thinking-level parameter: each effort is a different
// model, e.g. `gemini-3.8-flash-high`. crush drives effort through its
// reasoning selector, which the Gemini SDK serializes as
// generationConfig.thinkingConfig.thinkingLevel, so that has to be translated
// back here. The key is then dropped, leaving a request identical to one that
// named the suffixed model directly.
func applyReasoningLevel(model string, req map[string]any) string {
	config, level, ok := thinkingLevel(req)
	if !ok {
		return model
	}
	suffix, sellable := effortSuffix(level)
	if !sellable {
		// A level this gateway does not sell: leave the request as crush built
		// it and let the backend answer, rather than quietly choosing a
		// different tier.
		return model
	}
	// The level now travels in the model name; sending both would be two knobs
	// for one setting.
	delete(config, "thinkingLevel")

	// Tolerate a caller that still names a tier directly, so the selector wins.
	family, _ := SplitEffort(model)
	return family + "-" + suffix
}

// thinkingLevel locates generationConfig.thinkingConfig and the level it asks
// for. ok is false when there is no usable level.
func thinkingLevel(req map[string]any) (config map[string]any, level string, ok bool) {
	gen, valid := req["generationConfig"].(map[string]any)
	if !valid {
		return nil, "", false
	}
	config, valid = gen["thinkingConfig"].(map[string]any)
	if !valid {
		return nil, "", false
	}
	level, ok = config["thinkingLevel"].(string)
	if !ok || strings.TrimSpace(level) == "" {
		return config, "", false
	}
	return config, level, true
}

// normalizeFunctionCalls repairs tool calls replayed in conversation history.
//
// The Gemini SDK omits `args` when a call took no arguments, and the
// Anthropic-family models served by this gateway then fail the whole turn with
// `messages.N.content.M.tool_use.input: Field required`. Any zero-parameter
// tool would break on its second turn, which is exactly when history exists.
func normalizeFunctionCalls(req map[string]any) {
	turns, ok := req["contents"].([]any)
	if !ok {
		return
	}
	for _, turn := range turns {
		turnMap, ok := turn.(map[string]any)
		if !ok {
			continue
		}
		parts, ok := turnMap["parts"].([]any)
		if !ok {
			continue
		}
		for _, part := range parts {
			partMap, ok := part.(map[string]any)
			if !ok {
				continue
			}
			call, ok := partMap["functionCall"].(map[string]any)
			if !ok {
				continue
			}
			if args, isMap := call["args"].(map[string]any); !isMap || args == nil {
				call["args"] = map[string]any{}
			}
			// crush correlates a tool result with its call by id, and
			// some backends omit it for the first call in a turn.
			if id, _ := call["id"].(string); id == "" {
				call["id"] = "call_" + mustToken(8)
			}
		}
	}
}

// convertResponse rewrites an upstream reply into a Gemini API reply. The
// returned response shares the upstream body through a converting reader so
// streaming stays incremental.
func convertResponse(req *http.Request, resp *http.Response, verb string) *http.Response {
	if resp.StatusCode != http.StatusOK {
		// Errors are already shaped as {"error": …}, which is what the
		// SDK expects; only the wrapper may differ.
		resp.Body = unwrapErrorBody(resp.Body)
		return resp
	}

	in := bufio.NewReader(resp.Body)
	var out io.ReadCloser
	if verb == verbStream {
		out = &streamConverter{src: in, raw: resp.Body, ctx: req.Context()}
	} else {
		out = &unaryConverter{raw: resp.Body, src: in}
	}

	resp.Body = out
	if verb == verbStream {
		resp.Header.Set("Content-Type", "text/event-stream")
	}
	// The converted body is a different length than the upstream one.
	resp.ContentLength = -1
	resp.Header.Del("Content-Length")
	return resp
}

// streamConverter turns `data: {"response": {…}}` events into
// `data: {…}` events, deferring usage to the terminating chunk.
type streamConverter struct {
	src       *bufio.Reader
	raw       io.ReadCloser
	ctx       context.Context
	pending   [][]byte
	sawFinish bool
	heldUse   any
	err       error
	done      bool
}

func (c *streamConverter) Read(p []byte) (int, error) {
	for len(c.pending) == 0 {
		if c.done {
			if c.err != nil {
				return 0, c.err
			}
			return 0, io.EOF
		}
		if cerr := c.ctx.Err(); cerr != nil {
			c.done = true
			c.err = cerr
			return 0, cerr
		}
		if err := c.next(); err != nil {
			c.done = true
			c.err = err
			if errors.Is(err, io.EOF) && !c.sawFinish {
				// Emit the closing chunk the backend omitted, but
				// only once, by suppressing the sentinel until the
				// synthesised event has been drained.
				c.pending = append(c.pending, c.syntheticFinish())
				c.err = nil
				continue
			}
			return 0, err
		}
	}
	chunk := c.pending[0]
	c.pending = c.pending[1:]
	n := copy(p, chunk)
	if n < len(chunk) {
		c.pending = append([][]byte{chunk[n:]}, c.pending...)
	}
	return n, nil
}

// next reads one upstream event and appends the converted bytes.
func (c *streamConverter) next() error {
	for {
		line, err := readEventLine(c.src)
		if err != nil {
			return err
		}
		if len(line) == 0 {
			// Blank separator: skip, the converter re-emits its own.
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			// Comments, keepalives, and raw error objects. Pass through
			// anything that is not a data line the SDK could choke on.
			if bytes.Contains(line, []byte(`"error"`)) {
				c.pending = append(c.pending, append(line, '\n', '\n'))
			}
			continue
		}
		payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		converted, err := c.convertChunk(payload)
		if err != nil {
			// An unparseable chunk must not end an otherwise good
			// stream; skip it.
			continue
		}
		if converted != nil {
			c.pending = append(c.pending, converted)
		}
		return nil
	}
}

// convertChunk unwraps one event and returns the bytes to emit, or nil when
// the event carried nothing the SDK can use.
func (c *streamConverter) convertChunk(payload []byte) ([]byte, error) {
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return nil, nil
	}
	obj, err := unwrapObject(payload)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}

	// Hold usage back until the turn ends.
	if usage, ok := obj["usageMetadata"]; ok {
		c.heldUse = usage
		delete(obj, "usageMetadata")
	}

	if hasFinishReason(obj) {
		c.sawFinish = true
		if c.heldUse != nil {
			obj["usageMetadata"] = c.heldUse
		}
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return append(append([]byte("data: "), out...), '\n', '\n'), nil
}

// syntheticFinish closes a stream that ended without a finishReason, so crush
// does not discard the completed answer as truncated.
func (c *streamConverter) syntheticFinish() []byte {
	chunk := map[string]any{
		"candidates": []any{map[string]any{
			"content":      map[string]any{"role": "model", "parts": []any{}},
			"finishReason": "STOP",
		}},
	}
	if c.heldUse != nil {
		chunk["usageMetadata"] = c.heldUse
	}
	out, err := json.Marshal(chunk)
	if err != nil {
		return nil
	}
	c.sawFinish = true
	return append(append([]byte("data: "), out...), '\n', '\n')
}

func (c *streamConverter) Close() error { return c.raw.Close() }

// unaryConverter reads a whole non-streamed reply and unwraps it.
type unaryConverter struct {
	raw  io.ReadCloser
	src  *bufio.Reader
	buf  []byte
	pos  int
	done bool
}

func (u *unaryConverter) Read(p []byte) (int, error) {
	if !u.done {
		u.done = true
		body, err := io.ReadAll(io.LimitReader(u.src, 64<<20))
		if err != nil {
			return 0, err
		}
		obj, err := unwrapObject(body)
		if err != nil || obj == nil {
			u.buf = body
		} else {
			// A unary reply without usageMetadata panics crush's
			// response mapper.
			if _, ok := obj["usageMetadata"]; !ok {
				obj["usageMetadata"] = map[string]any{
					"promptTokenCount":     0,
					"candidatesTokenCount": 0,
					"totalTokenCount":      0,
				}
			}
			if u.buf, err = json.Marshal(obj); err != nil {
				u.buf = body
			}
		}
	}
	if u.pos >= len(u.buf) {
		return 0, io.EOF
	}
	n := copy(p, u.buf[u.pos:])
	u.pos += n
	return n, nil
}

func (u *unaryConverter) Close() error { return u.raw.Close() }

// unwrapObject returns the response body with the `{"response": …}` wrapper
// removed, or nil when the payload holds only a wrapper and nothing inside.
func unwrapObject(payload []byte) (map[string]any, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, err
	}
	if inner, ok := obj["response"]; ok && len(obj) <= 3 {
		// v1internal wraps the reply and adds traceId/metadata.
		merged := map[string]any{}
		if err := json.Unmarshal(inner, &merged); err != nil {
			return nil, err
		}
		return merged, nil
	}
	out := make(map[string]any, len(obj))
	for k, v := range obj {
		out[k] = json.RawMessage(v)
	}
	return out, nil
}

func hasFinishReason(obj map[string]any) bool {
	cands, ok := obj["candidates"].([]any)
	if !ok {
		return false
	}
	for _, cand := range cands {
		m, ok := cand.(map[string]any)
		if !ok {
			continue
		}
		if reason, ok := m["finishReason"].(string); ok && reason != "" {
			return true
		}
	}
	return false
}

// unwrapErrorBody rewrites an error reply so the SDK can read it, keeping the
// body otherwise intact.
func unwrapErrorBody(body io.ReadCloser) io.ReadCloser {
	raw, err := io.ReadAll(io.LimitReader(body, 1<<20))
	_ = body.Close()
	if err != nil {
		return io.NopCloser(bytes.NewReader(nil))
	}
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	// Errors are already Gemini-shaped; hand them over untouched.
	if json.Unmarshal(raw, &envelope) != nil || len(envelope.Error) == 0 {
		return io.NopCloser(bytes.NewReader(raw))
	}
	return io.NopCloser(bytes.NewReader(raw))
}

// readEventLine returns one SSE line with its terminator stripped.
func readEventLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	return bytes.TrimRight(line, "\r\n"), nil
}
