package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	crushlog "github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var serverHost string

func init() {
	serverCmd.Flags().StringVarP(&serverHost, "host", "H", server.DefaultHost(), "Server host  (HTTP, TCP or Unix socket)")
	rootCmd.AddCommand(serverCmd)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Crush server",
	Long: `Start the Crush server.

Usage:

  # Serve over HTTP
  crush server --host http://127.0.0.1:8080

  # Serve over TCP
  crush server --host tcp://127.0.0.1:8080

  # Serve over a Unix socket (default)
  crush server --host unix:///tmp/crush.sock

While the server is running, browse the HTTP API docs at /v1/docs/
(e.g. http://127.0.0.1:8080/v1/docs/). The OpenAPI 3.1 spec is served
at /v1/docs/openapi.json.
`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		dataDir, err := cmd.Flags().GetString("data-dir")
		if err != nil {
			return fmt.Errorf("failed to get data directory: %v", err)
		}
		debug, err := cmd.Flags().GetBool("debug")
		if err != nil {
			return fmt.Errorf("failed to get debug flag: %v", err)
		}

		cfg, err := config.Load(cmd.Context(), config.GlobalWorkspaceDir(), dataDir, debug)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %v", err)
		}

		hostURL, err := server.ParseHostURL(serverHost)
		if err != nil {
			return fmt.Errorf("invalid server host: %v", err)
		}

		logFile := serverLogFile(hostURL)

		if term.IsTerminal(os.Stderr.Fd()) {
			crushlog.Setup(logFile, debug, os.Stderr)
		} else {
			crushlog.Setup(logFile, debug)
		}

		srv := server.NewServer(cfg, hostURL.Scheme, hostURL.Host)
		srv.SetLogger(slog.Default())
		slog.Info("Starting Crush server...", "addr", serverHost)

		errch := make(chan error, 1)
		go func() {
			errch <- srv.ListenAndServe()
		}()

		if err = awaitStop(cmd.Context(), srv, errch); err != nil {
			if !errors.Is(err, server.ErrServerClosed) {
				_ = srv.Close()
				slog.Error("Server error", "error", err)
				return fmt.Errorf("server error: %v", err)
			}
		}

		if errors.Is(err, server.ErrServerClosed) {
			return nil
		}

		// The signal already cancelled cmd.Context(); detach so the
		// shutdown gets its full budget instead of none.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(cmd.Context()), 15*time.Second)
		defer cancel()

		slog.Info("Shutting down...")

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown server", "error", err)
			return fmt.Errorf("failed to shutdown server: %v", err)
		}

		return nil
	},
}

// awaitStop blocks until the root context is cancelled by a termination
// signal, returning nil, or the server stops on its own, returning its
// error. A server has no terminal to lose, so SIGHUP reloads config here
// instead of stopping it.
func awaitStop(ctx context.Context, srv *server.Server, errch <-chan error) error {
	signal.Reset(syscall.SIGHUP)
	hupch := make(chan os.Signal, 1)
	signal.Notify(hupch, syscall.SIGHUP)
	defer signal.Stop(hupch)

	for {
		select {
		case <-hupch:
			slog.Info("Received SIGHUP, reloading config...")
			reloadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			if err := srv.ReloadConfig(reloadCtx); err != nil {
				slog.Error("Failed to reload config", "error", err)
			}
			cancel()
		case <-ctx.Done():
			slog.Info("Received termination signal...")
			return nil
		case err := <-errch:
			return err
		}
	}
}
