package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/crush/internal/acp"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/spf13/cobra"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Run Crush as an ACP server over stdio",
	Long:  `Run Crush as an Agent Client Protocol server, listening on stdin/stdout for JSON-RPC 2.0 messages from an IDE or editor.`,
	RunE:  runACP,
}

func runACP(cmd *cobra.Command, _ []string) error {
	// Redirect logs to stderr; stdout is reserved for JSON-RPC.
	level := slog.LevelInfo
	if debug, _ := cmd.Flags().GetBool("debug"); debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	ws, cleanup, err := setupLocalWorkspace(cmd)
	if err != nil {
		return fmt.Errorf("setup workspace: %w", err)
	}
	defer cleanup()

	// Extract the underlying app.App for direct service access.
	appWs, ok := ws.(*workspace.AppWorkspace)
	if !ok {
		return fmt.Errorf("expected AppWorkspace, got %T", ws)
	}
	app := appWs.App()
	log := slog.Default()
	log.Info("starting ACP server")

	srv := acp.NewServer(app, log, os.Stdin, os.Stdout)
	srv.SetLogger(log)

	// Wire the ACP permission service into the app so tools use it. The
	// coordinator captured the original TUI permission service during app
	// setup, so rebuild it to pick up the ACP bridge; without this, tool
	// permission requests would wait for a UI that never appears.
	app.Permissions = srv.PermissionService()
	if err := app.InitCoderAgentNonInteractive(cmd.Context()); err != nil {
		return fmt.Errorf("init coder agent: %w", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	go func() {
		select {
		case <-sigChan:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Start the event bridge to stream agent updates to the client.
	srv.StartEventBridge(ctx)

	if err := srv.Start(ctx); err != nil {
		if err.Error() == "client disconnected" {
			return nil
		}
		return fmt.Errorf("ACP server: %w", err)
	}
	return nil
}
