// Package main is the entry point for the Crush CLI.
package main

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/charmbracelet/crush/internal/cmd"
	_ "github.com/charmbracelet/crush/internal/dns"
	_ "github.com/joho/godotenv/autoload"
	"github.com/pkg/browser"
)

func main() {
	// Send browser output directly to the null device to keep it off the TUI.
	browser.Stdout = nil
	browser.Stderr = nil

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
