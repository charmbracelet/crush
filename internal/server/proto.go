package server

import (
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
)

// ServerProto defines the RPC methods exposed by the Crush server.
type ServerProto struct {
	*Server
}

// GetConfig is an RPC routine that returns the server's configuration.
func (s *ServerProto) GetConfig(args *proto.Args, reply *config.Config) error {
	*reply = *s.cfg
	return nil
}
