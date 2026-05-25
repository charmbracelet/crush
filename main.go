// Package main is the entry point for the Crush CLI.
//
//	@title			Crush API
//	@version		1.0
//	@description	Crush is a terminal-based AI coding assistant. This API is served over a Unix socket (or Windows named pipe) and provides programmatic access to workspaces, sessions, agents, LSP, MCP, and more.
//	@contact.name	Charm
//	@contact.url	https://charm.sh
//	@license.name	MIT
//	@license.url	https://github.com/taigrr/crush/blob/main/LICENSE
//	@BasePath		/v1
package main

import (
	"cmp"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/taigrr/crush/internal/cmd"
	_ "github.com/taigrr/crush/internal/dns"
)

func main() {
	if os.Getenv("CRUSH_PROFILE") != "" {
		// Default to :6060 for the client; set CRUSH_PROFILE_PORT to use a
		// different port (the server subprocess should use 6061).
		addr := "localhost:" + cmp.Or(os.Getenv("CRUSH_PROFILE_PORT"), "6060")
		go func() {
			slog.Info("Serving pprof", "addr", addr)
			if httpErr := http.ListenAndServe(addr, nil); httpErr != nil {
				slog.Error("Failed to pprof listen", "error", httpErr)
			}
		}()
	}

	cmd.Execute()
}
