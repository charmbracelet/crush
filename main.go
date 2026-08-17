// Package main is the entry point for the Crush CLI.
//
//	@title			Crush API
//	@version		1.0
//	@description	Crush is a terminal-based AI coding assistant. This API is served over a Unix socket (or Windows named pipe) and provides programmatic access to workspaces, sessions, agents, LSP, MCP, and more.
//	@contact.name	Charm
//	@contact.url	https://charm.sh
//	@license.name	MIT
//	@license.url	https://github.com/charmbracelet/crush/blob/main/LICENSE
//	@BasePath		/v1
package main

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/cmd"
	_ "github.com/charmbracelet/crush/internal/dns"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	if len(os.Args) > 1 {
		exe, _ := os.Executable()

		if filepath.Clean(os.Args[1]) == filepath.Clean(exe) {
			os.Args = append(os.Args[:1], os.Args[2:]...)
		}
	}
	
	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			slog.Info("Serving pprof at localhost:6060")
			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil {
				slog.Error("Failed to pprof listen", "error", httpErr)
			}
		}()
	}

	cmd.Execute()
}
