package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	crushlog "github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var serverHost string

func init() {
	serverCmd.Flags().StringVarP(&serverHost, "host", "H", server.DefaultHost(), "Server host (TCP or Unix socket)")
	rootCmd.AddCommand(serverCmd)
}

// workspaceShutdownTimeout giới hạn việc dừng các workspace khi server nhận
// tín hiệu dừng.
const workspaceShutdownTimeout = 15 * time.Second

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the agent-cli server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		dataDir, err := cmd.Flags().GetString("data-dir")
		if err != nil {
			return fmt.Errorf("failed to get data directory: %v", err)
		}
		debug, err := cmd.Flags().GetBool("debug")
		if err != nil {
			return fmt.Errorf("failed to get debug flag: %v", err)
		}

		cfg, err := config.Load(config.GlobalWorkspaceDir(), dataDir, debug)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %v", err)
		}

		hostURL, err := server.ParseHostURL(serverHost)
		if err != nil {
			return fmt.Errorf("invalid server host: %v", err)
		}

		logFile := serverLogPath(hostURL)

		if term.IsTerminal(os.Stderr.Fd()) {
			crushlog.Setup(logFile, debug, os.Stderr)
		} else {
			crushlog.Setup(logFile, debug)
		}

		srv := server.NewServer(cfg, hostURL.Scheme, hostURL.Host)
		srv.SetLogger(slog.Default())
		slog.Info("Starting agent-cli server...", "addr", serverHost)

		errch := make(chan error, 1)
		sigch := make(chan os.Signal, 1)
		signal.Notify(sigch, withoutIgnoredHangup(terminationSignals())...)

		go func() {
			errch <- srv.ListenAndServe()
		}()

		select {
		case sig := <-sigch:
			slog.Info("Received termination signal, shutting down workspaces...", "signal", sig.String())
			// Tín hiệu thứ hai (Ctrl+C lần nữa) thoát ngay theo mặc định.
			signal.Stop(sigch)
			// Dừng từng workspace trước (huỷ run, ghi lại việc dở) để lần mở
			// sau không thấy tool call "đang chạy" giả; quá hạn thì bỏ, không
			// để một run lì treo việc dừng server.
			done := make(chan struct{})
			go func() {
				srv.ShutdownWorkspaces()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(workspaceShutdownTimeout):
				slog.Warn("Workspaces did not shut down in time; exiting anyway", "timeout", workspaceShutdownTimeout)
			}
		case err = <-errch:
			if err != nil && !errors.Is(err, server.ErrServerClosed) {
				_ = srv.Close()
				slog.Error("Server error", "error", err)
				return fmt.Errorf("server error: %v", err)
			}
		}

		if errors.Is(err, server.ErrServerClosed) {
			return nil
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()

		slog.Info("Shutting down...")

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown server", "error", err)
			return fmt.Errorf("failed to shutdown server: %v", err)
		}

		return nil
	},
}
