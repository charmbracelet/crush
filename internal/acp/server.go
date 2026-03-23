package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
)

const (
	// writeSyncBufSize is the buffer size for high-priority synchronous writes
	// (responses and outgoing calls). Keep small since these are infrequent.
	writeSyncBufSize = 32
	// writeAsyncBufSize is the buffer size for low-priority async writes
	// (notifications). Large enough to absorb bursts of streaming deltas.
	writeAsyncBufSize = 4096
)

// writeRequest is a single serialized write to stdout.
type writeRequest struct {
	data []byte
	ack  chan error // nil = async (fire-and-forget)
}

// Server is the ACP JSON-RPC 2.0 server running over stdio.
type Server struct {
	handler *Handler

	in  io.Reader
	out io.Writer

	// writeSyncCh carries high-priority, synchronous writes (responses, outgoing
	// calls). The writer goroutine always drains this channel before writeAsyncCh.
	writeSyncCh chan writeRequest
	// writeAsyncCh carries low-priority, async writes (notifications).
	// Notifications are dropped when this channel is full to prevent the
	// event-processing loop from blocking and causing deadlocks with
	// permission-request writes.
	writeAsyncCh chan writeRequest

	nextID  atomic.Int64
	pending sync.Map // id -> chan *Response
}

// NewServer creates a new ACP server using stdin/stdout.
func NewServer(handler *Handler) *Server {
	return &Server{
		handler:      handler,
		in:           os.Stdin,
		out:          os.Stdout,
		writeSyncCh:  make(chan writeRequest, writeSyncBufSize),
		writeAsyncCh: make(chan writeRequest, writeAsyncBufSize),
	}
}

// NewServerWithIO creates a new ACP server with custom IO streams (for testing).
func NewServerWithIO(handler *Handler, in io.Reader, out io.Writer) *Server {
	return &Server{
		handler:      handler,
		in:           in,
		out:          out,
		writeSyncCh:  make(chan writeRequest, writeSyncBufSize),
		writeAsyncCh: make(chan writeRequest, writeAsyncBufSize),
	}
}

// runWriter is a dedicated goroutine that serializes all writes to out.
// It prioritizes writeSyncCh (responses, outgoing calls) over writeAsyncCh
// (notifications) to prevent slow notification writes from delaying critical
// messages such as permission requests.
func (s *Server) runWriter(ctx context.Context) {
	doWrite := func(req writeRequest) {
		_, err := fmt.Fprintf(s.out, "%s\n", req.data)
		if req.ack != nil {
			req.ack <- err
		}
	}

	for {
		// Drain the high-priority sync channel first.
		select {
		case req := <-s.writeSyncCh:
			doWrite(req)
			continue
		case <-ctx.Done():
			return
		default:
		}

		// Wait for either type of write.
		select {
		case req := <-s.writeSyncCh:
			doWrite(req)
		case req := <-s.writeAsyncCh:
			doWrite(req)
		case <-ctx.Done():
			return
		}
	}
}

// writeLineSync enqueues data for a synchronous, high-priority write and
// blocks until the write completes or ctx is cancelled.
func (s *Server) writeLineSync(ctx context.Context, data []byte) error {
	ack := make(chan error, 1)
	select {
	case s.writeSyncCh <- writeRequest{data: data, ack: ack}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-ack:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// writeLineAsync enqueues data for an async, low-priority write and returns
// immediately. Returns false if the async buffer is full and the write is dropped.
func (s *Server) writeLineAsync(data []byte) bool {
	select {
	case s.writeAsyncCh <- writeRequest{data: data}:
		return true
	default:
		return false
	}
}

// Serve reads JSON-RPC messages from stdin and dispatches them until ctx is
// cancelled or the input stream is closed.
func (s *Server) Serve(ctx context.Context) error {
	go s.runWriter(ctx)

	scanner := bufio.NewScanner(s.in)
	// ACP messages can be large; increase the buffer.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	for scanner.Scan() {
		// scanner.Bytes() returns a slice backed by the scanner's internal buffer.
		// We must copy it before passing to a goroutine, because the next Scan()
		// call will overwrite the underlying memory.
		src := scanner.Bytes()
		if len(src) == 0 {
			continue
		}
		raw := make(json.RawMessage, len(src))
		copy(raw, src)
		go s.dispatch(ctx, raw)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("acp: scanner error: %w", err)
	}
	return nil
}

// dispatch determines the message kind and handles it.
func (s *Server) dispatch(ctx context.Context, raw json.RawMessage) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("ACP: panic in dispatch", "panic", r)
			// Best-effort: try to parse the request ID so we can send an
			// error response back to the client instead of silently
			// dropping the request.
			var peek struct {
				ID *int64 `json:"id"`
			}
			if json.Unmarshal(raw, &peek) == nil && peek.ID != nil {
				s.writeResponse(ctx, &Response{
					JSONRPC: "2.0",
					ID:      peek.ID,
					Error:   &RPCError{Code: CodeInternalError, Message: fmt.Sprintf("internal panic: %v", r)},
				})
			}
		}
	}()

	// Peek at the message to determine type.
	var peek struct {
		ID     *int64          `json:"id"`
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  *RPCError       `json:"error"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		slog.Warn("ACP: failed to parse message", "err", err)
		return
	}

	// If it has Result or Error and an ID, it's a response to our outgoing call.
	if peek.ID != nil && peek.Method == "" {
		var resp Response
		if err := json.Unmarshal(raw, &resp); err != nil {
			slog.Warn("ACP: failed to parse response", "err", err)
			return
		}
		if ch, ok := s.pending.Load(*resp.ID); ok {
			ch.(chan *Response) <- &resp
		}
		return
	}

	// Otherwise it's a request or notification from the client.
	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		slog.Warn("ACP: failed to parse request", "err", err)
		return
	}

	result, rpcErr := s.handler.Handle(ctx, &req)

	// Notifications have no ID and expect no response.
	if req.ID == nil {
		return
	}

	var resp Response
	resp.JSONRPC = "2.0"
	resp.ID = req.ID
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		encoded, err := json.Marshal(result)
		if err != nil {
			resp.Error = &RPCError{Code: CodeInternalError, Message: err.Error()}
		} else {
			resp.Result = encoded
		}
	}
	s.writeResponse(ctx, &resp)
}

// writeResponse encodes and writes a response synchronously via the writer goroutine.
func (s *Server) writeResponse(ctx context.Context, resp *Response) {
	b, err := json.Marshal(resp)
	if err != nil {
		slog.Error("ACP: failed to marshal response", "err", err)
		return
	}
	if err := s.writeLineSync(ctx, b); err != nil {
		slog.Error("ACP: failed to write response", "err", err)
	}
}

// Notify sends a notification (no id, no response expected) to the client.
// The write is asynchronous and non-blocking: if the output buffer is full
// the notification is dropped. This prevents slow client reads from stalling
// the agent event loop or blocking permission-request writes.
func (s *Server) Notify(_ context.Context, method string, params any) {
	b, err := json.Marshal(params)
	if err != nil {
		slog.Error("ACP: failed to marshal notification params", "method", method, "err", err)
		return
	}
	msg := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  b,
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		slog.Error("ACP: failed to marshal notification", "method", method, "err", err)
		return
	}
	if !s.writeLineAsync(raw) {
		slog.Warn("ACP: notification dropped, output buffer full", "method", method)
	}
}

// Call sends a request to the client and waits for its response.
// Returns the raw result JSON or an error.
func (s *Server) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := s.nextID.Add(1)

	b, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("acp: marshal params: %w", err)
	}
	req := Request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  b,
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("acp: marshal request: %w", err)
	}

	ch := make(chan *Response, 1)
	s.pending.Store(id, ch)
	defer s.pending.Delete(id)

	if err := s.writeLineSync(ctx, raw); err != nil {
		return nil, fmt.Errorf("acp: write request: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("acp: rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
