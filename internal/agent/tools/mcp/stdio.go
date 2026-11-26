package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// stdioTransport wraps a stdio server's transport so that a write which
// outlives its context closes the connection instead of hanging.
//
// The SDK checks the context once and then blocks in the pipe write until the
// server reads. A request bigger than the pipe buffer, sent to a server that
// has stopped reading, never returns: cancelling the tool call does nothing,
// and every later request to that server waits behind it for the connection's
// write lock. A message cut off midway cannot be resumed, so closing the
// connection is the only way out. That unblocks the write, and the next call
// finds the session dead and starts a new one. See
// modelcontextprotocol/go-sdk#1498.
type stdioTransport struct {
	inner mcp.Transport
}

// unwrapTransport implements [transportWrapper].
func (t *stdioTransport) unwrapTransport() mcp.Transport { return t.inner }

// Connect implements mcp.Transport.
func (t *stdioTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	conn, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &stdioConn{Connection: conn}, nil
}

type stdioConn struct {
	mcp.Connection
}

// Write closes the connection if ctx ends before msg is fully written.
func (c *stdioConn) Write(ctx context.Context, msg jsonrpc.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	err := c.Connection.Write(ctx, msg)
	if !stop() {
		return ctx.Err()
	}
	return err
}
