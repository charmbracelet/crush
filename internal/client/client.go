package client

import (
	"net/rpc"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/server"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc/v2"
)

// Client represents an RPC client connected to a Crush server.
type Client struct {
	rpc *rpc.Client
}

// DefaultClient creates a new [Client] connected to the default server address.
func DefaultClient() (*Client, error) {
	return NewClient("unix", server.DefaultAddr())
}

// NewClient creates a new [Client] connected to the server at the given
// network and address.
func NewClient(network, address string) (*Client, error) {
	rpc, err := msgpackrpc.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return &Client{rpc: rpc}, nil
}

// GetConfig retrieves the server's configuration via RPC.
func (c *Client) GetConfig() (*config.Config, error) {
	var cfg config.Config
	var args proto.Args
	err := c.rpc.Call("ServerProto.GetConfig", &args, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
