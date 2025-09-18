package server

import (
	"log/slog"
	"net/rpc"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
)

// ServerCodec is a wrapper around msgpackrpc.ServerCodec that adds logging
// functionality.
type ServerCodec struct {
	*msgpackrpc.MsgpackCodec
	logger *slog.Logger
}

var _ rpc.ServerCodec = (*ServerCodec)(nil)

// ReadRequestHeader reads the request header and logs it.
func (c *ServerCodec) ReadRequestHeader(r *rpc.Request) error {
	err := c.MsgpackCodec.ReadRequestHeader(r)
	if c.logger != nil {
		c.logger.Debug("rpc request",
			slog.String("service_method", r.ServiceMethod),
			slog.Int("seq", int(r.Seq)),
		)
	}
	return err
}

// WriteResponse writes the response and logs it.
func (c *ServerCodec) WriteResponse(r *rpc.Response, body any) error {
	err := c.MsgpackCodec.WriteResponse(r, body)
	if c.logger != nil {
		c.logger.Debug("rpc response",
			slog.String("service_method", r.ServiceMethod),
			slog.String("error", r.Error),
			slog.Int("seq", int(r.Seq)),
		)
	}
	return err
}
