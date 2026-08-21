package cmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	tea "charm.land/bubbletea/v2"
	acp "github.com/coder/acp-go-sdk"
	"github.com/spf13/cobra"

	acpadapter "github.com/charmbracelet/crush/internal/acp"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/skills"
	uicommon "github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/model"
	"github.com/charmbracelet/crush/internal/workspace"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Run as an ACP agent",
	Long: "Run as an ACP agent over stdio (for ACP client subprocess spawning), " +
		"or with --tui for interactive TUI mode, " +
		"or with --tui-acp for TUI + ACP server on a unix socket.",
	RunE: func(cmd *cobra.Command, args []string) error {
		tuiFlag, _ := cmd.Flags().GetBool("tui")
		tuiACPFlag, _ := cmd.Flags().GetBool("tui-acp")
		dataDirFlag, _ := cmd.Flags().GetString("data-dir")
		socketPathFlag, _ := cmd.Flags().GetString("socket-path")
		return runACPCommand(cmd.Context(), tuiFlag, tuiACPFlag, dataDirFlag, socketPathFlag)
	},
}

func init() {
	rootCmd.AddCommand(acpCmd)
	acpCmd.Flags().Bool("tui", false, "Run in TUI-only mode (no ACP server)")
	acpCmd.Flags().Bool("tui-acp", false, "Run TUI with ACP server on unix socket")
	acpCmd.Flags().StringP("data-dir", "D", "", "Custom crush data directory")
	acpCmd.Flags().String("socket-path", "", "Override ACP unix socket path (default: <data-dir>/crush-acp.sock)")
}

func runACPCommand(ctx context.Context, tui bool, tuiACP bool, dataDirArg string, socketPathArg string) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Determine data directory: explicit flag > env var > auto-detect > default
	dataDir := dataDirArg
	if dataDir == "" {
		dataDir = os.Getenv("CRUSH_DATA_DIR")
	}
	if dataDir == "" {
		// Auto-detect: if ~/TUIs/.crush/crush.db exists, use it
		home, _ := os.UserHomeDir()
		existingDB := filepath.Join(home, "TUIs", ".crush", "crush.db")
		if _, err := os.Stat(existingDB); err == nil {
			dataDir = filepath.Join(home, "TUIs", ".crush")
		}
	}
	store, err := config.Init(cwd, dataDir, false)
	if err != nil {
		return err
	}
	dataDir = store.Config().Options.DataDirectory

	conn, err := db.Connect(ctx, dataDir)
	if err != nil {
		return err
	}
	defer conn.Close()

	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return err
	}
	logFile := filepath.Join(logDir, "crush-acp.log")
	if tui && !tuiACP {
		log.Setup(logFile, false)
	} else {
		log.Setup(logFile, false, os.Stderr)
	}

	all, active, states := skills.DiscoverFromConfig(skills.DiscoveryConfig{
		SkillsPaths:    store.Config().Options.SkillsPaths,
		DisabledSkills: store.Config().Options.DisabledSkills,
		Resolver:       store.Resolver().ResolveValue,
	})
	skillsMgr := skills.NewManager(all, active, states, skills.WithGlobalMirror())

	appInstance, err := app.New(ctx, conn, store, skillsMgr)
	if err != nil {
		return err
	}
	defer appInstance.Shutdown()

	// TUI+ACP: run TUI with ACP server on unix socket (for external ACP clients to connect).
	if tuiACP {
		return runTUIAndACPInCmd(ctx, appInstance, store, dataDir, socketPathArg)
	}

	// TUI-only: run interactive TUI without ACP server.
	if tui {
		return runTUIOnly(ctx, appInstance, store)
	}

	// Default: ACP agent over stdio (spawned by ACP client as subprocess).
	return runACPInCmd(ctx, appInstance, os.Stdin, os.Stdout)
}

func runACPInCmd(ctx context.Context, appInstance *app.App, r io.Reader, w io.Writer) error {
	coord := appInstance.AgentCoordinator
	if coord == nil {
		return fmt.Errorf("agent coordinator not initialized")
	}

	adapter := acpadapter.NewAdapter(coord, appInstance.Sessions, appInstance.Messages)
	adapter.SetLogger(slog.Default())
	adapter.SetObserver()
	acpConn := acp.NewAgentSideConnection(adapter, w, r)
	adapter.SetConnection(acpConn)

	coord.SetACPConnector(&tools.ACPConnector{
		GetConn: func() *acp.AgentSideConnection { return acpConn },
	})

	// Wire the update_plan tool to notify the ACP adapter on plan changes.
	tools.SetPlanObserver(func(sessionID string, entries []tools.PlanEntry) {
		agentEntries := make([]agent.PlanEntry, len(entries))
		for i, e := range entries {
			agentEntries[i] = agent.PlanEntry{
				Content:  e.Content,
				Priority: agent.PlanPriority(e.Priority),
				Status:   agent.PlanStatus(e.Status),
			}
		}
		adapter.OnPlanUpdate(sessionID, agentEntries)
	})

	// Wire session title changes to the ACP adapter.
	coord.SetOnTitleChange(func(sessionID, title string) {
		adapter.UpdateSessionInfo(sessionID, title)
	})

	slog.Info("crush acp: server ready")
	<-acpConn.Done()
	return nil
}

func runTUIAndACPInCmd(ctx context.Context, appInstance *app.App, store *config.ConfigStore, dataDir string, socketPathArg string) error {
	// Default to /tmp/crush-acp.sock for external ACP clients,
	// fall back to <data-dir>/crush-acp.sock for backward compatibility.
	socketPath := "/tmp/crush-acp.sock"
	if socketPathArg != "" {
		socketPath = socketPathArg
	} else if os.Getenv("CRUSH_ACP_SOCKET") != "" {
		socketPath = os.Getenv("CRUSH_ACP_SOCKET")
	}
	if runtime.GOOS == "windows" {
		socketPath = `\\.\pipe\crush-acp`
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		// If the socket file already exists (e.g. from a previous
		// crashed instance), remove it and retry once.
		if _, statErr := os.Stat(socketPath); statErr == nil {
			slog.Warn("Removing stale ACP socket", "path", socketPath)
			if rmErr := os.Remove(socketPath); rmErr != nil {
				return fmt.Errorf("failed to remove stale socket %s: %w", socketPath, rmErr)
			}
			ln, err = net.Listen("unix", socketPath)
		}
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", socketPath, err)
		}
	}
	defer ln.Close()
	defer os.Remove(socketPath)

	acpErr := make(chan error, 1)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case acpErr <- err:
				default:
				}
				return
			}
			go func(c net.Conn) {
				acpErr <- runACPInCmd(ctx, appInstance, c, c)
			}(conn)
		}
	}()

	ws := workspace.NewAppWorkspace(appInstance, store)
	com := uicommon.DefaultCommon(ws)
	m := model.New(com, "", false)

	inputFilter := model.NewFilter()
	program := tea.NewProgram(
		m,
		tea.WithContext(ctx),
		tea.WithFilter(inputFilter.Filter),
	)
	go ws.Subscribe(program)

	slog.Info("crush acp: TUI starting", "acp_socket", socketPath)
	_, err = program.Run()
	return err
}

// runTUIOnly runs the interactive TUI without any ACP server.
func runTUIOnly(ctx context.Context, appInstance *app.App, store *config.ConfigStore) error {
	ws := workspace.NewAppWorkspace(appInstance, store)
	com := uicommon.DefaultCommon(ws)
	m := model.New(com, "", false)

	inputFilter := model.NewFilter()
	program := tea.NewProgram(
		m,
		tea.WithContext(ctx),
		tea.WithFilter(inputFilter.Filter),
	)
	go ws.Subscribe(program)

	slog.Info("crush acp: TUI-only mode starting")
	_, err := program.Run()
	return err
}
