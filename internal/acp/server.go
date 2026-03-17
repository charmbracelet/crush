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

// Server is the ACP JSON-RPC 2.0 server running over stdio.
type Server struct {
	handler *Handler

	in  io.Reader
	out io.Writer

	mu      sync.Mutex // protects writes to out
	nextID  atomic.Int64
	pending sync.Map // id -> chan *Response
}

// NewServer creates a new ACP server using stdin/stdout.
func NewServer(handler *Handler) *Server {
	return &Server{
		handler: handler,
		in:      os.Stdin,
		out:     os.Stdout,
	}
}

// NewServerWithIO creates a new ACP server with custom IO streams (for testing).
func NewServerWithIO(handler *Handler, in io.Reader, out io.Writer) *Server {
	return &Server{
		handler: handler,
		in:      in,
		out:     out,
	}
}

// Serve reads JSON-RPC messages from stdin and dispatches them until ctx is
// cancelled or the input stream is closed.
func (s *Server) Serve(ctx context.Context) error {
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
	s.writeResponse(&resp)
}

// writeResponse encodes and writes a response to stdout.
func (s *Server) writeResponse(resp *Response) {
	b, err := json.Marshal(resp)
	if err != nil {
		slog.Error("ACP: failed to marshal response", "err", err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.out, "%s\n", b)
}

// Notify sends a notification (no id, no response expected) to the client.
func (s *Server) Notify(ctx context.Context, method string, params any) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.out, "%s\n", raw)
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

	s.mu.Lock()
	_, writeErr := fmt.Fprintf(s.out, "%s\n", raw)
	s.mu.Unlock()
	if writeErr != nil {
		return nil, fmt.Errorf("acp: write request: %w", writeErr)
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
