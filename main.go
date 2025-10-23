package main

import (
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/charmbracelet/crush/internal/cmd"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			slog.Info("Starting pprof server", "address", "http://localhost:6060/debug/pprof/")
			slog.Warn("pprof server is enabled - disable in production by unsetting CRUSH_PROFILE")

			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil {
				slog.Error("Failed to start pprof server", "error", httpErr)
			}
		}()
	}

	cmd.Execute()
}
