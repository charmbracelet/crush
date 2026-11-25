package cmd

import (
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

var acpConnectSystemPrompt string

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
	acpConnectCmd.Flags().StringVar(&acpConnectSystemPrompt, "system-prompt", "", "append trusted instructions to the external agent's system prompt")
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

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	systemPrompt := acpConnectSystemPrompt
	if systemPrompt == "" {
		systemPrompt = os.Getenv("CRUSH_ACP_SYSTEM_PROMPT")
	}
	input := newLineInput(os.Stdin)

	cli, err := acp.NewClient(ctx, agentPath, agentArgs,
		acp.WithCWD(cwd),
		acp.WithPermissionHandler(newPermissionHandler(input)),
		acp.WithSystemPrompt(systemPrompt),
		acp.WithElicitationHandler(newElicitationHandler(input)),
	)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	defer cli.Close()

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
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		ClientCapabilities: acpsdk.ClientCapabilities{
			Fs:       acpsdk.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
			Terminal: true,
			Elicitation: &acpsdk.ElicitationCapabilities{
				Form: &acpsdk.ElicitationFormCapabilities{},
				Url:  &acpsdk.ElicitationUrlCapabilities{},
			},
			// Ask for boolean config options so the server offers
			// thinking as a toggle rather than a two-item select.
			Session: &acpsdk.ClientSessionCapabilities{
				ConfigOptions: &acpsdk.SessionConfigOptionsCapabilities{
					Boolean: &acpsdk.BooleanConfigOptionCapabilities{},
				},
				Notices:    &acpsdk.NoticeCapabilities{},
				Compaction: &acpsdk.CompactionCapabilities{},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Connected to %s v%s (protocol %v)\n\n",
		safeTerminalText(initResp.AgentInfo.Name), safeTerminalText(initResp.AgentInfo.Version), initResp.ProtocolVersion)

	// Create session.
	ns, err := cli.NewSession(ctx, acpsdk.NewSessionRequest{Cwd: cwd})
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer cli.CloseSession(ctx, acpsdk.CloseSessionRequest{SessionId: ns.SessionId})

	fmt.Fprintf(os.Stdout, "Session: %s\n\n", safeTerminalText(string(ns.SessionId)))

	// Drain session/update notifications (streaming chunks, tool calls) so the
	// user sees output as it arrives instead of only the final stop reason.
	sub, err := cli.Subscribe(ns.SessionId)
	if err != nil {
		return fmt.Errorf("subscribe session: %w", err)
	}
	go func() {
		for u := range sub {
			renderUpdate(u)
		}
	}()

	// Interactive prompt loop.
	for {
		prompt, ok := input.readLine("> ")
		if !ok {
			break
		}
		if prompt == "" || prompt == "exit" || prompt == "quit" {
			break
		}
		if prompt == "help" {
			fmt.Fprintln(os.Stdout, "Commands: exit, quit, help, cancel")
			continue
		}
		if prompt == "cancel" {
			if err := cli.Cancel(context.Background(), acpsdk.CancelNotification{SessionId: ns.SessionId}); err != nil {
				fmt.Fprintf(os.Stdout, "Cancel failed: %v\n\n", err)
			}
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
			fmt.Fprintf(os.Stdout, "Error: %s\n\n", safeTerminalText(err.Error()))
			continue
		}
		fmt.Fprintf(os.Stdout, "\n[stop: %s]\n\n", safeTerminalText(string(resp.StopReason)))
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

// renderUpdate writes a streaming session/update to the terminal. Agent message
// text goes to stdout (the answer); thoughts and tool calls go to stderr so they
// stay out of any piped stdout capture.
func renderUpdate(u acpsdk.SessionUpdate) {
	switch {
	case u.AgentMessageChunk != nil:
		if t := u.AgentMessageChunk.Content.Text; t != nil {
			fmt.Fprint(os.Stdout, safeTerminalText(t.Text))
		}
	case u.AgentThoughtChunk != nil:
		if t := u.AgentThoughtChunk.Content.Text; t != nil {
			fmt.Fprint(os.Stderr, safeTerminalText(t.Text))
		}
	case u.ToolCall != nil:
		title := u.ToolCall.Title
		if u.ToolCall.Name != nil && *u.ToolCall.Name != "" {
			title = *u.ToolCall.Name + ": " + title
		}
		fmt.Fprintf(os.Stderr, "\n[tool: %s]\n", safeTerminalText(title))
	case u.Notice != nil:
		description := ""
		if u.Notice.Description != nil {
			description = ": " + *u.Notice.Description
		}
		fmt.Fprintf(os.Stderr, "\n[notice: %s] %s%s\n", safeTerminalText(string(u.Notice.Severity)), safeTerminalText(u.Notice.Title), safeTerminalText(description))
	case u.CompactionUpdate != nil:
		fmt.Fprintf(os.Stderr, "\n[compaction: %s] %s\n", safeTerminalText(string(u.CompactionUpdate.CompactionId)), safeTerminalText(string(u.CompactionUpdate.Status)))
		if summary, ok := u.CompactionUpdate.Summary.Value(); ok {
			for _, block := range summary {
				if block.Text != nil {
					fmt.Fprintln(os.Stderr, safeTerminalText(block.Text.Text))
				}
			}
		}
	case u.CompactionSummaryChunk != nil:
		if text := u.CompactionSummaryChunk.Content.Text; text != nil {
			fmt.Fprint(os.Stderr, safeTerminalText(text.Text))
		}
	}
}
