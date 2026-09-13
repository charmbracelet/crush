package cmd

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/charmbracelet/crush/internal/acp"
	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/spf13/cobra"
)

var acpConnectCmd = &cobra.Command{
	Use:   "connect <agent-binary> [args...]",
	Short: "Connect to an external ACP agent over stdio",
	Long: `Connect to an external Agent Client Protocol agent and drive it interactively.

The agent binary is spawned as a subprocess and communicates via stdio JSON-RPC 2.0.
Prompts are read from stdin and responses are printed to stdout.

Example:
  crush acp connect my-agent -- --config path/to/config.json`,
	Args: cobra.MinimumNArgs(1),
	RunE: runACPConnect,
}

func init() {
	acpCmd.AddCommand(acpConnectCmd)
}

func runACPConnect(cmd *cobra.Command, args []string) error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return fmt.Errorf("resolve cwd: %w", err)
	}

	agentPath := args[0]
	agentArgs := args[1:]

	log := slog.Default()
	log.Info("starting ACP client", "agent", agentPath, "args", agentArgs)

	cli, err := acp.NewClient(agentPath, agentArgs,
		acp.WithCWD(cwd),
		acp.WithAutoAllowPermissions(),
	)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Start the connection loop in the background.
	connErr := make(chan error, 1)
	go func() {
		connErr <- cli.Start(ctx)
	}()

	// Initialize.
	initResp, err := cli.Initialize(ctx, acpsdk.InitializeRequest{
		ProtocolVersion:    acpsdk.ProtocolVersionNumber,
		ClientCapabilities: acpsdk.ClientCapabilities{Fs: acpsdk.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true}},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Connected to %s v%s (protocol %v)\n\n",
		initResp.AgentInfo.Name, initResp.AgentInfo.Version, initResp.ProtocolVersion)

	// Create session.
	ns, err := cli.NewSession(ctx, acpsdk.NewSessionRequest{Cwd: cwd})
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer cli.CloseSession(ctx, acpsdk.CloseSessionRequest{SessionId: ns.SessionId})

	fmt.Fprintf(os.Stdout, "Session: %s\n\n", ns.SessionId)

	// Interactive prompt loop.
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprintf(os.Stdout, "> ")
		if !scanner.Scan() {
			break
		}
		prompt := strings.TrimSpace(scanner.Text())
		if prompt == "" || prompt == "exit" || prompt == "quit" {
			break
		}
		if prompt == "help" {
			fmt.Fprintln(os.Stdout, "Commands: exit, quit, help")
			continue
		}

		resp, err := cli.Prompt(ctx, acpsdk.PromptRequest{
			SessionId: ns.SessionId,
			Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(prompt)},
		})
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			fmt.Fprintf(os.Stdout, "Error: %v\n\n", err)
			continue
		}
		fmt.Fprintf(os.Stdout, "\n[stop: %s]\n\n", resp.StopReason)
	}

	if err := scanner.Err(); err != nil {
		log.Warn("stdin read error", "err", err)
	}

	// Wait for connection to close.
	select {
	case err := <-connErr:
		if err != nil && !strings.Contains(err.Error(), "agent process exited") {
			log.Warn("connection error", "err", err)
		}
	case <-ctx.Done():
	}

	return nil
}
