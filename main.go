package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "net/http/pprof" // profiling

	_ "github.com/joho/godotenv/autoload" // automatically load .env files

	"github.com/charmbracelet/crush/cmd"
	"github.com/charmbracelet/crush/internal/log"
)

func main() {
	defer log.RecoverPanic("main", func() {
		slog.Error("Application terminated due to unhandled panic")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	// Start signal handler in a goroutine
	go func() {
		sig := <-sigChan
		slog.Info("Received signal, initiating graceful shutdown", "signal", sig)
		cancel()
	}()

	if os.Getenv("CRUSH_PROFILE") != "" {
		go func() {
			slog.Info("Serving pprof at localhost:6060")
			if httpErr := http.ListenAndServe("localhost:6060", nil); httpErr != nil {
				slog.Error(fmt.Sprintf("Failed to pprof listen: %v", httpErr))
			}
		}()
	}

	cmd.Execute(ctx)
}
