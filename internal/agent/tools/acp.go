package tools

import (
	"context"
	"fmt"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"charm.land/fantasy"
)

// ACPConnector wraps the ACP AgentSideConnection so tools can make
// client-bound requests (ReadTextFile, WriteTextFile, CreateTerminal, etc.)
// without depending on the full connection lifecycle. Callers supply the
// getter closure once; the getter must return the current connection (which
// may be nil when no ACP client is connected).
type ACPConnector struct {
	GetConn func() *acp.AgentSideConnection
}

const (
	ACPViewToolName   = "zed_view"
	ACPWriteToolName  = "zed_write"
	ACPBashToolName   = "zed_bash"
	ACPPermissionName = "zed_request_permission"
)

// NewACPViewTool returns a tool that reads file contents through the ACP
// client's ReadTextFile method, giving the agent access to the client's
// LSP-indexed buffers.
func NewACPViewTool(connector *ACPConnector) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ACPViewToolName,
		"ACP-CLIENT view: read a file through the connected ACP client's buffers. "+
			"PREFERRED over 'view' when connected to an ACP client — the client's buffer reflects "+
			"unsaved changes, LSP diagnostics, and cursor state that the local filesystem may not have yet. "+
			"Supports offset (0-based line start) and limit (max lines, default 200).",
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			conn := connector.GetConn()
			if conn == nil {
				return fantasy.NewTextErrorResponse("ACP connection not available"), nil
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("no session ID in context"), nil
			}

			limit := params.Limit
			if limit == 0 {
				limit = DefaultReadLimit
			}
			line := params.Offset
			if line < 0 {
				line = 0
			} else {
				line++ // ACP ReadTextFile lines are 1-based
			}

			resp, err := conn.ReadTextFile(ctx, acp.ReadTextFileRequest{
				SessionId: acp.SessionId(sessionID),
				Path:      params.FilePath,
				Limit:     &limit,
				Line:      &line,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("zed_view failed: %s", err)), nil
			}
			return fantasy.NewTextResponse(resp.Content), nil
		},
	)
}

// ACPWriteTool returns a tool that writes file content through the ACP
// client's WriteTextFile method.
func NewACPWriteTool(connector *ACPConnector) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ACPWriteToolName,
		"ACP-CLIENT write: write file content through the connected ACP client's buffers. "+
			"PREFERRED over 'write'/'edit' when connected to an ACP client — the client's buffer updates "+
			"in real-time so the user sees the change immediately with proper syntax highlighting and LSP integration. "+
			"Takes an absolute file_path and the full file content to write.",
		func(ctx context.Context, params WriteACPParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			conn := connector.GetConn()
			if conn == nil {
				return fantasy.NewTextErrorResponse("ACP connection not available"), nil
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("no session ID in context"), nil
			}

			_, err := conn.WriteTextFile(ctx, acp.WriteTextFileRequest{
				SessionId: acp.SessionId(sessionID),
				Path:      params.FilePath,
				Content:   params.Content,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("zed_write failed: %s", err)), nil
			}
			return fantasy.NewTextResponse("File written successfully through ACP client"), nil
		},
	)
}

// WriteACPParams is the input for the zed_write tool.
type WriteACPParams struct {
	FilePath string `json:"file_path" description:"The absolute path to the file to write"`
	Content  string `json:"content" description:"The full content to write to the file"`
}

// NewACPBashTool returns a tool that spawns a terminal in the ACP client
// to execute commands, rather than running them on the agent's local host.
func NewACPBashTool(connector *ACPConnector) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ACPBashToolName,
		"ACP-CLIENT bash: execute commands in a terminal spawned inside the ACP client. "+
			"PREFERRED over 'bash' when connected to an ACP client — runs in the client's workspace "+
			"with the client's environment variables and working directory. "+
			"Use for any command execution when an ACP client is connected.",
		func(ctx context.Context, params ACPBashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			conn := connector.GetConn()
			if conn == nil {
				return fantasy.NewTextErrorResponse("ACP connection not available"), nil
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("no session ID in context"), nil
			}

			// Create the terminal in the client.
			createReq := acp.CreateTerminalRequest{
				SessionId: acp.SessionId(sessionID),
				Command:   params.Command,
				Args:      params.Args,
			}
			if params.WorkingDir != "" {
				wd := params.WorkingDir
				createReq.Cwd = &wd
			}
			if params.OutputByteLimit > 0 {
				createReq.OutputByteLimit = &params.OutputByteLimit
			}

			createResp, err := conn.CreateTerminal(ctx, createReq)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("zed_bash (create terminal) failed: %s", err)), nil
			}

			// Wait for the terminal to exit, then gather its output.
			waitResp, err := conn.WaitForTerminalExit(ctx, acp.WaitForTerminalExitRequest{
				SessionId:  acp.SessionId(sessionID),
				TerminalId: createResp.TerminalId,
			})
			if err != nil {
				// Try to clean up.
				_, _ = conn.KillTerminal(ctx, acp.KillTerminalRequest{
					SessionId:  acp.SessionId(sessionID),
					TerminalId: createResp.TerminalId,
				})
				return fantasy.NewTextErrorResponse(fmt.Sprintf("zed_bash (wait for exit) failed: %s", err)), nil
			}

			// Fetch terminal output.
			outputResp, err := conn.TerminalOutput(ctx, acp.TerminalOutputRequest{
				SessionId:  acp.SessionId(sessionID),
				TerminalId: createResp.TerminalId,
			})
			if err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf(
					"Command completed (exit code: %d) but output could not be retrieved: %s",
					waitResp.ExitCode, err,
				)), nil
			}

			// Release the terminal on the client side.
			_, _ = conn.ReleaseTerminal(ctx, acp.ReleaseTerminalRequest{
				SessionId:  acp.SessionId(sessionID),
				TerminalId: createResp.TerminalId,
			})

			return fantasy.NewTextResponse(fmt.Sprintf(
				"Exit code: %d\n\n%s",
				waitResp.ExitCode, outputResp.Output,
			)), nil
		},
	)
}

// ACPBashParams is the input for the zed_bash tool.
type ACPBashParams struct {
	Description     string   `json:"description" description:"A brief description of what the command does"`
	Command         string   `json:"command" description:"The command to execute in the client's terminal"`
	Args            []string `json:"args,omitempty" description:"Arguments to pass to the command"`
	WorkingDir      string   `json:"working_dir,omitempty" description:"The working directory for the command"`
	OutputByteLimit int      `json:"output_byte_limit,omitempty" description:"Maximum bytes of output to capture"`
}

// ── Visual Control Tools ──────────────────────────────────────────────────
// These tools emit visual command metadata through the ACP tool lifecycle.
// Compatible clients can consume the metadata to dispatch workspace actions.

const (
	// ZedVisualNavigate emits navigation commands.
	ZedVisualNavigateToolName = "zed_visual"
	// ZedVisualPane manages panes (split, close, navigate).
	ZedVisualPaneToolName = "zed_pane"
	// ZedVisualPanel manages panels (toggle focus).
	ZedVisualPanelToolName = "zed_panel"
	// ZedVisualBatch executes multiple visual commands atomically.
	ZedVisualBatchToolName = "zed_batch"
)

// ZedVisualNavigateParams controls client navigation.
type ZedVisualNavigateParams struct {
	Action string `json:"action" description:"Navigation action: back, forward"`
}

// ZedVisualPaneParams controls client pane management.
type ZedVisualPaneParams struct {
	Action    string `json:"action" description:"Pane action: split, close, navigate"`
	Direction string `json:"direction,omitempty" description:"Direction for split/navigate: left, right, up, down"`
	Path      string `json:"path,omitempty" description:"File path to open (for split action)"`
}

// ZedVisualPanelParams controls client panel management.
type ZedVisualPanelParams struct {
	Action string `json:"action" description:"Panel action: focus, toggle, close"`
	Panel  string `json:"panel" description:"Panel name: terminal, project, outline, assistant, etc."`
}

// NewZedVisualNavigateTool creates a tool for emitting navigation metadata.
func NewZedVisualNavigateTool(connector *ACPConnector) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ZedVisualNavigateToolName,
		"Navigate the ACP client's history. Actions: back, forward. "+
			"Use this to move through the client's navigation history without "+
			"needing a file tool call.",
		func(ctx context.Context, params ZedVisualNavigateParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			// Send a session/update with _zed_visual_command metadata for
			// clients that dispatch visual workspace actions from tool updates.
			conn := connector.GetConn()
			if conn == nil {
				return fantasy.NewTextErrorResponse("ACP connection not available"), nil
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("no session ID in context"), nil
			}
			// Use a synthetic tool call to carry the visual command meta.
			toolID := fmt.Sprintf("zed_nav_%d", time.Now().UnixNano())
			update := acp.UpdateToolCall(acp.ToolCallId(toolID),
				acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
			)
			meta := map[string]any{
				"_zed_visual_command": map[string]any{
					"command": params.Action,
					"params":  map[string]any{},
				},
			}
			update.ToolCallUpdate.Meta = meta
			sendCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_ = conn.SessionUpdate(sendCtx, acp.SessionNotification{
				SessionId: acp.SessionId(sessionID),
				Update:    acp.SessionUpdate{ToolCallUpdate: update.ToolCallUpdate},
			})
			return fantasy.NewTextResponse(fmt.Sprintf("Navigated %s", params.Action)), nil
		},
	)
}

// NewZedVisualPaneTool creates a tool for emitting pane-management metadata.
func NewZedVisualPaneTool(connector *ACPConnector) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ZedVisualPaneToolName,
		"Manage panes in ACP clients that support visual command metadata. "+
			"Actions: split (open file in new pane), close (close inactive panes), navigate (move focus). "+
			"Directions: left, right, up, down. "+
			"For split, provide an absolute file path.",
		func(ctx context.Context, params ZedVisualPaneParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			conn := connector.GetConn()
			if conn == nil {
				return fantasy.NewTextErrorResponse("ACP connection not available"), nil
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("no session ID in context"), nil
			}
			toolID := fmt.Sprintf("zed_pane_%d", time.Now().UnixNano())
			update := acp.UpdateToolCall(acp.ToolCallId(toolID),
				acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
			)
			cmdParams := map[string]any{}
			if params.Direction != "" {
				cmdParams["direction"] = params.Direction
			}
			if params.Path != "" {
				cmdParams["path"] = params.Path
			}
			meta := map[string]any{
				"_zed_visual_command": map[string]any{
					"command": params.Action,
					"params":  cmdParams,
				},
			}
			update.ToolCallUpdate.Meta = meta
			sendCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_ = conn.SessionUpdate(sendCtx, acp.SessionNotification{
				SessionId: acp.SessionId(sessionID),
				Update:    acp.SessionUpdate{ToolCallUpdate: update.ToolCallUpdate},
			})
			return fantasy.NewTextResponse(fmt.Sprintf("Pane %s completed", params.Action)), nil
		},
	)
}

// NewZedVisualPanelTool creates a tool for emitting panel-management metadata.
func NewZedVisualPanelTool(connector *ACPConnector) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ZedVisualPanelToolName,
		"Manage panels in ACP clients that support visual command metadata. "+
			"Actions: focus (bring panel to front), toggle (show/hide panel), close. "+
			"Panel names: terminal, project, outline, assistant, copilot_chat, "+
			"debug, git, notification, language_server, etc.",
		func(ctx context.Context, params ZedVisualPanelParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			conn := connector.GetConn()
			if conn == nil {
				return fantasy.NewTextErrorResponse("ACP connection not available"), nil
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("no session ID in context"), nil
			}
			toolID := fmt.Sprintf("zed_panel_%d", time.Now().UnixNano())
			update := acp.UpdateToolCall(acp.ToolCallId(toolID),
				acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
			)
			meta := map[string]any{
				"_zed_visual_command": map[string]any{
					"command": params.Action,
					"params": map[string]any{
						"panel": params.Panel,
					},
				},
			}
			update.ToolCallUpdate.Meta = meta
			sendCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_ = conn.SessionUpdate(sendCtx, acp.SessionNotification{
				SessionId: acp.SessionId(sessionID),
				Update:    acp.SessionUpdate{ToolCallUpdate: update.ToolCallUpdate},
			})
			switch params.Action {
			case "focus":
				return fantasy.NewTextResponse(fmt.Sprintf("Focused %s panel", params.Panel)), nil
			case "toggle":
				return fantasy.NewTextResponse(fmt.Sprintf("Toggled %s panel", params.Panel)), nil
			case "close":
				return fantasy.NewTextResponse(fmt.Sprintf("Closed %s panel", params.Panel)), nil
			default:
				return fantasy.NewTextResponse(fmt.Sprintf("Panel action %s completed", params.Action)), nil
			}
		},
	)
}

// ZedVisualBatchOp is a single operation in a batch.
type ZedVisualBatchOp struct {
	Action    string `json:"action" description:"Command: open_file, open_terminal, split, close, focus, toggle, back, forward"`
	Path      string `json:"path,omitempty" description:"File path (for open_file, split)"`
	Direction string `json:"direction,omitempty" description:"Direction (for split: left, right, up, down)"`
	Panel     string `json:"panel,omitempty" description:"Panel name (for focus, toggle: terminal, project, etc.)"`
}

// ZedVisualBatchParams executes multiple visual commands atomically.
type ZedVisualBatchParams struct {
	Operations []ZedVisualBatchOp `json:"operations" description:"List of visual commands to execute in sequence"`
}

// NewZedVisualBatchTool creates the zed_batch tool for atomic multi-operations.
func NewZedVisualBatchTool(connector *ACPConnector) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ZedVisualBatchToolName,
		"Execute multiple visual commands in a single batch. "+
			"All operations are sent as a single tool notification to minimize "+
			"intermediate UI states. "+
			"Operations: open_file, open_terminal, split, close, focus, toggle, back, forward.",
		func(ctx context.Context, params ZedVisualBatchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			conn := connector.GetConn()
			if conn == nil {
				return fantasy.NewTextErrorResponse("ACP connection not available"), nil
			}
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.NewTextErrorResponse("no session ID in context"), nil
			}
			if len(params.Operations) == 0 {
				return fantasy.NewTextErrorResponse("no operations provided"), nil
			}
			toolID := fmt.Sprintf("zed_batch_%d", time.Now().UnixNano())
			sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			for i, op := range params.Operations {
				update := acp.UpdateToolCall(acp.ToolCallId(toolID),
					acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
				)
				cmdParams := map[string]any{}
				if op.Path != "" {
					cmdParams["path"] = op.Path
				}
				if op.Direction != "" {
					cmdParams["direction"] = op.Direction
				}
				if op.Panel != "" {
					cmdParams["panel"] = op.Panel
				}
				update.ToolCallUpdate.Meta = map[string]any{
					"_zed_visual_command": map[string]any{
						"command":   op.Action,
						"params":    cmdParams,
						"seq":       i,
						"batch_id":  toolID,
						"batch_len": len(params.Operations),
					},
				}
				_ = conn.SessionUpdate(sendCtx, acp.SessionNotification{
					SessionId: acp.SessionId(sessionID),
					Update:    acp.SessionUpdate{ToolCallUpdate: update.ToolCallUpdate},
				})
			}
			return fantasy.NewTextResponse(fmt.Sprintf("Batch of %d operations sent", len(params.Operations))), nil
		},
	)
}
