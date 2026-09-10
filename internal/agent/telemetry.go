package agent

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
)

// logPromptComposition logs the byte size of each static request
// component. It runs once per Run, where the system prompt, tool list,
// and MCP instructions are assembled. systemPromptBytes measures the
// rendered prompt before MCP instructions are appended.
func logPromptComposition(sessionID string, systemPromptBytes int, mcpInstructions string, agentTools []fantasy.AgentTool, sections []prompt.PromptSection) {
	builtinSchemaBytes, mcpSchemaBytes := toolSchemaBytes(agentTools)
	attrs := []any{
		"session_id", sessionID,
		"system_prompt_bytes", systemPromptBytes,
		"mcp_instructions_bytes", len(mcpInstructions),
		"builtin_tool_schema_bytes", builtinSchemaBytes,
		"mcp_tool_schema_bytes", mcpSchemaBytes,
		"tool_count", len(agentTools),
	}
	for _, s := range sections {
		attrs = append(attrs,
			"section_"+s.Name+"_bytes", s.Bytes,
			"section_"+s.Name+"_est_tokens", s.EstTokens,
		)
	}
	slog.Debug("Prompt composition", attrs...)
}

// logStepComposition logs the byte size of each per-step request
// component. It runs inside PrepareStep, where the message list and
// tool list for the step are finalized. System-role messages are
// bucketed separately from conversation history: the main system
// prompt (and any system prompt prefix) counts toward system_bytes,
// while notebook recall blobs count toward notebook_bytes.
func logStepComposition(sessionID string, messages []fantasy.Message, agentTools []fantasy.AgentTool) {
	var historyBytes, notebookBytes, systemBytes int
	for _, msg := range messages {
		n := messageContentBytes(msg)
		if msg.Role == fantasy.MessageRoleSystem {
			if len(msg.Content) > 0 {
				var text string
				switch tp := msg.Content[0].(type) {
				case fantasy.TextPart:
					text = tp.Text
				case *fantasy.TextPart:
					text = tp.Text
				}
				// Matches both the assembled <notebook> block and
				// maybeAutoInject's <notebook_auto_inject> recall blob.
				if strings.HasPrefix(text, "<notebook") {
					notebookBytes += n
					continue
				}
			}
			systemBytes += n
			continue
		}
		historyBytes += n
	}
	builtinSchemaBytes, mcpSchemaBytes := toolSchemaBytes(agentTools)
	slog.Debug("Step request composition",
		"session_id", sessionID,
		"system_bytes", systemBytes,
		"raw_history_bytes", historyBytes,
		"notebook_bytes", notebookBytes,
		"builtin_tool_schema_bytes", builtinSchemaBytes,
		"mcp_tool_schema_bytes", mcpSchemaBytes,
		"tool_count", len(agentTools),
		"message_count", len(messages),
	)
}

// isMCPTool reports whether t is an MCP tool, looking through the hook
// decorator. Origin is decided at call time; do not infer it elsewhere
// from the flattened tool list without unwrapping first.
func isMCPTool(t fantasy.AgentTool) bool {
	if h, ok := t.(*hookedTool); ok {
		t = h.Unwrap()
	}
	_, ok := t.(*tools.Tool)
	return ok
}

// toolSchemaBytes returns the serialized schema size of the tool list,
// split into built-in and MCP partitions.
func toolSchemaBytes(agentTools []fantasy.AgentTool) (builtin, mcp int64) {
	for _, t := range agentTools {
		data, err := json.Marshal(t.Info())
		if err != nil {
			continue
		}
		if isMCPTool(t) {
			mcp += int64(len(data))
		} else {
			builtin += int64(len(data))
		}
	}
	return builtin, mcp
}

// messageContentBytes returns the content-byte size of a fantasy
// message: role plus the size of each content part. It measures logical
// content, not transport bytes — see the transport wrapper for the
// wire-level measurement.
func messageContentBytes(msg fantasy.Message) int {
	n := len(msg.Role)
	for _, part := range msg.Content {
		n += messagePartBytes(part)
	}
	return n
}

func messagePartBytes(part fantasy.MessagePart) int {
	switch p := part.(type) {
	case fantasy.TextPart:
		return len(p.Text)
	case *fantasy.TextPart:
		return len(p.Text)
	case fantasy.ReasoningPart:
		return len(p.Text)
	case *fantasy.ReasoningPart:
		return len(p.Text)
	case fantasy.FilePart:
		return len(p.Data) + len(p.Filename) + len(p.MediaType)
	case *fantasy.FilePart:
		return len(p.Data) + len(p.Filename) + len(p.MediaType)
	case fantasy.ToolCallPart:
		return len(p.ToolName) + len(p.Input)
	case *fantasy.ToolCallPart:
		return len(p.ToolName) + len(p.Input)
	case fantasy.ToolResultPart:
		return toolResultOutputBytes(p.Output)
	case *fantasy.ToolResultPart:
		return toolResultOutputBytes(p.Output)
	default:
		return 0
	}
}

func toolResultOutputBytes(output fantasy.ToolResultOutputContent) int {
	switch o := output.(type) {
	case fantasy.ToolResultOutputContentText:
		return len(o.Text)
	case *fantasy.ToolResultOutputContentText:
		return len(o.Text)
	case fantasy.ToolResultOutputContentError:
		if o.Error != nil {
			return len(o.Error.Error())
		}
	case *fantasy.ToolResultOutputContentError:
		if o.Error != nil {
			return len(o.Error.Error())
		}
	case fantasy.ToolResultOutputContentMedia:
		return len(o.Data) + len(o.MediaType) + len(o.Text)
	case *fantasy.ToolResultOutputContentMedia:
		return len(o.Data) + len(o.MediaType) + len(o.Text)
	}
	return 0
}

// normalizedPromptTokens is the canonical prompt-token count for a
// provider usage report. The session counters and the title/summarize
// path previously disagreed on which cache tokens count toward prompt
// tokens; this normalizes both to input + cache write + cache read.
func normalizedPromptTokens(usage fantasy.Usage) int64 {
	return usage.InputTokens + usage.CacheCreationTokens + usage.CacheReadTokens
}

// logStepUsage logs the raw provider token counters alongside the
// normalized prompt-token count so the two can be reconciled.
func logStepUsage(sessionID string, usage fantasy.Usage, estimated bool) {
	slog.Debug("Step token usage",
		"session_id", sessionID,
		"input_tokens", usage.InputTokens,
		"output_tokens", usage.OutputTokens,
		"cache_creation_tokens", usage.CacheCreationTokens,
		"cache_read_tokens", usage.CacheReadTokens,
		"prompt_tokens_normalized", normalizedPromptTokens(usage),
		"estimated", estimated,
	)
}

// byteCountingTransport is an http.RoundTripper that measures the
// serialized request and response bodies crossing the transport. It
// reports per-request totals tagged with the x-session-id header when
// present, giving transport-level payload telemetry that content
// measurements cannot capture (JSON field names, escaping, tool
// schemas, provider options, base64 attachments, headers).
//
// Streaming (SSE) responses are counted incrementally and reported when
// the body reaches EOF or is closed. The request-byte count is read at
// report time rather than when RoundTrip returns: with HTTP/2 or
// "Expect: 100-continue", response headers can arrive while the request
// body is still streaming, so an early sample can under-report. For
// early error responses the count may still be partial — provider
// payloads are buffered JSON, so this is a known but minor limitation.
type byteCountingTransport struct {
	next http.RoundTripper
}

func (t *byteCountingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var reqCounter *countingReadCloser
	if req.Body != nil && req.Body != http.NoBody {
		reqCounter = &countingReadCloser{inner: req.Body}
		req.Body = reqCounter
	}
	resp, err := t.next.RoundTrip(req)
	sessionHash := req.Header.Get("x-session-id")
	if err != nil {
		var reqBytes int64
		if reqCounter != nil {
			reqBytes = reqCounter.n.Load()
		}
		slog.Debug("LLM transport",
			"session_hash", sessionHash,
			"host", req.URL.Host,
			"request_bytes", reqBytes,
			"error", err,
		)
		return nil, err
	}
	if resp.Body != nil && resp.Body != http.NoBody {
		resp.Body = &reportingReadCloser{
			inner:      resp.Body,
			sessionID:  sessionHash,
			host:       req.URL.Host,
			statusCode: resp.StatusCode,
			reqCounter: reqCounter,
		}
	} else {
		var reqBytes int64
		if reqCounter != nil {
			reqBytes = reqCounter.n.Load()
		}
		slog.Debug("LLM transport",
			"session_hash", sessionHash,
			"host", req.URL.Host,
			"status", resp.StatusCode,
			"request_bytes", reqBytes,
			"response_bytes", 0,
		)
	}
	return resp, nil
}

// countingReadCloser counts bytes read from an underlying body.
type countingReadCloser struct {
	inner io.ReadCloser
	n     atomic.Int64
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.inner.Read(p)
	c.n.Add(int64(n))
	return n, err
}

func (c *countingReadCloser) Close() error { return c.inner.Close() }

// reportingReadCloser counts response-body bytes and logs the
// request/response pair once the body is fully consumed or closed. The
// request counter is read at report time so the request_bytes figure
// reflects as much of the streamed upload as completed.
type reportingReadCloser struct {
	inner      io.ReadCloser
	sessionID  string
	host       string
	statusCode int
	reqCounter *countingReadCloser
	respBytes  int64
	reported   bool
}

func (r *reportingReadCloser) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	r.respBytes += int64(n)
	if err == io.EOF {
		r.report()
	}
	return n, err
}

func (r *reportingReadCloser) Close() error {
	err := r.inner.Close()
	r.report()
	return err
}

func (r *reportingReadCloser) report() {
	if r.reported {
		return
	}
	r.reported = true
	var reqBytes int64
	if r.reqCounter != nil {
		reqBytes = r.reqCounter.n.Load()
	}
	slog.Debug("LLM transport",
		"session_hash", r.sessionID,
		"host", r.host,
		"status", r.statusCode,
		"request_bytes", reqBytes,
		"response_bytes", r.respBytes,
	)
}

// instrumentedHTTPClient wraps client's transport with the
// byte-counting transport, preserving any existing transport (e.g. the
// debug round-trip logger) underneath. A nil client yields a client
// measuring http.DefaultTransport.
//
// Coverage is asymmetric by necessity: REST providers (Anthropic,
// OpenAI, OpenRouter, Vercel, Azure, OpenAI-compatible) always get an
// instrumented client since they accept WithHTTPClient, while
// Bedrock/Vertex/Google SDK-managed transports are measured only when
// the caller injects a client (debug mode); otherwise transport
// measurement is unavailable for them.
func instrumentedHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	instrumented := *client
	instrumented.Transport = &byteCountingTransport{next: base}
	return &instrumented
}
