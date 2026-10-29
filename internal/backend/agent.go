package backend

import (
	"context"
	"errors"
	"os"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/notify"
	mcptools "github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
)

// SendMessage validates and accepts a prompt for the workspace's agent,
// then dispatches the run on a goroutine bound to the workspace context
// and returns immediately. It does not wait for the LLM turn to
// complete: the run's lifetime is owned by the workspace, not by the
// caller. Errors from the dispatched run reach observers through the
// agent event channels (a notify.TypeAgentError notification), not
// through this return value.
//
// SendMessage returns synchronously when the request cannot be accepted:
// ErrWorkspaceNotFound if the workspace is missing, ErrAgentNotInitialized
// if its coordinator is nil, the structural validation errors from
// agent.ValidateCall (ErrEmptyPrompt, ErrSessionMissing) when the prompt
// or session is missing, and ErrWorkspaceClosing if the workspace is
// being torn down.
func (b *Backend) SendMessage(workspaceID string, msg proto.AgentMessage) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	coord := ws.Coordinator()
	if coord == nil {
		return ErrAgentNotInitialized
	}

	if err := agent.ValidateCall(agent.SessionAgentCall{
		SessionID:   msg.SessionID,
		Prompt:      msg.Prompt,
		Attachments: proto.AttachmentsToMessage(msg.Attachments),
	}); err != nil {
		return err
	}

	accept := coord.BeginAccepted(msg.SessionID)

	ws.runMu.Lock()
	if ws.closing {
		ws.runMu.Unlock()
		accept.Close()
		return ErrWorkspaceClosing
	}
	ws.runWG.Add(1)
	ws.runMu.Unlock()

	go b.runAgent(ws, coord, msg, accept)
	return nil
}

// runAgent executes an accepted agent run for the workspace on coord, the
// coordinator that accepted it. It owns the accept reservation (releasing
// it on return) and the runWG ticket added by SendMessage. The run is bound
// to the workspace context so its lifetime is independent of any client's
// HTTP request.
//
// On a non-cancel error it surfaces the failure to observers via a
// notify.TypeAgentError notification (lossy, best-effort). That alone is
// not a reliable terminal signal: the agent-event fan-in uses lossy
// subscribers, so a `crush run` caller blocking on its RunID could hang
// if the event is dropped. To guarantee termination, when msg.RunID is
// non-empty and the coordinator did not already publish the run's
// authoritative terminal RunComplete (e.g. the error was returned before
// sessionAgent.Run executed, such as a readyWg or UpdateModels failure),
// runAgent emits an errored RunComplete on the must-deliver
// runCompletions broker so the waiter observes a deterministic terminal
// event. context.Canceled is expected (sessionAgent.Run already
// publishes the cancelled terminal marker) and produces no error
// terminal event.
//
// When msg.RunID is non-empty it is attached to the context via
// agent.WithRunID so the coordinator can stamp the terminal
// notify.RunComplete event with that correlator. A run-complete marker
// is also attached so the coordinator can report whether it published
// the terminal event, letting runAgent avoid a duplicate fallback.
func (b *Backend) runAgent(ws *Workspace, coord agent.Coordinator, msg proto.AgentMessage, accept *agent.AcceptedRun) {
	defer ws.runWG.Done()
	defer accept.Close()

	ctx := ws.ctx
	if msg.HiddenUserMessage {
		ctx = message.WithHiddenUserMessage(ctx)
	}
	if msg.RunID != "" {
		ctx = agent.WithRunID(ctx, msg.RunID)
	}
	ctx = agent.WithRunCompleteMarker(ctx)

	if msg.Channel != "" {
		ctx = agent.WithChannel(ctx, msg.Channel)
	}
	_, err := coord.RunAccepted(ctx, accept, msg.SessionID, msg.Prompt, proto.AttachmentsToMessage(msg.Attachments)...)
	if err == nil || errors.Is(err, context.Canceled) {
		return
	}

	ws.AgentNotifications().Publish(pubsub.CreatedEvent, notify.Notification{
		SessionID: msg.SessionID,
		RunID:     msg.RunID,
		Type:      notify.TypeAgentError,
		Message:   err.Error(),
	})

	// Reliable terminal fallback. Only needed when a RunID waiter
	// exists and the coordinator has not already emitted the run's
	// terminal RunComplete; otherwise this would be a duplicate.
	if msg.RunID == "" || agent.RunCompletePublished(ctx) {
		return
	}
	if rc := ws.RunCompletions(); rc != nil {
		rc.PublishMustDeliver(ctx, pubsub.UpdatedEvent, notify.RunComplete{
			SessionID: msg.SessionID,
			RunID:     msg.RunID,
			Error:     err.Error(),
		})
	}
}

// GetAgentInfo returns the agent's model and busy status.
func (b *Backend) GetAgentInfo(workspaceID string) (proto.AgentInfo, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.AgentInfo{}, err
	}

	var agentInfo proto.AgentInfo
	if coord := ws.Coordinator(); coord != nil {
		m := coord.Model()
		agentInfo = proto.AgentInfo{
			Model:    m.CatwalkCfg,
			ModelCfg: m.ModelCfg,
			IsBusy:   coord.IsBusy(),
			IsReady:  true,
		}
	}
	return agentInfo, nil
}

// Coordinator returns the workspace's agent coordinator, or nil when it
// has not been created yet.
func (w *Workspace) Coordinator() agent.Coordinator {
	if w.App == nil {
		return nil
	}
	w.agentMu.RLock()
	defer w.agentMu.RUnlock()
	return w.AgentCoordinator
}

// initAgent creates the workspace's coder agent coordinator when it does
// not exist yet and reports whether this call created it. An existing
// coordinator is never replaced: every client of the workspace shares it,
// and a new one would not know the runs, queues and cancels of the old one.
// Creating it is refused once the workspace is closing, so every write of
// the coordinator happens before [Workspace.Shutdown] reads it.
func (w *Workspace) initAgent(ctx context.Context, interactive bool) (bool, error) {
	if w.Coordinator() != nil {
		return false, nil
	}
	w.agentMu.Lock()
	defer w.agentMu.Unlock()
	if w.AgentCoordinator != nil {
		return false, nil
	}
	w.runMu.Lock()
	closing := w.closing
	w.runMu.Unlock()
	if closing {
		return false, ErrWorkspaceClosing
	}
	if interactive {
		return true, w.InitCoderAgent(ctx)
	}
	return true, w.InitCoderAgentNonInteractive(ctx)
}

// InitAgent makes the workspace's coder agent ready for a client that is
// attaching. The agent is created, with the interactivity the client asks
// for, only when the workspace has none yet. Otherwise the shared agent is
// kept as it is, interactivity included, and its models and tools are
// refreshed from the current config. A non-interactive client (`crush run`)
// gets one shot at the tool palette, so for it the refresh first waits,
// within the usual budget, for MCP initialization, as a non-interactive
// agent's run would.
func (b *Backend) InitAgent(ctx context.Context, workspaceID string, interactive bool) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	created, err := ws.initAgent(ctx, interactive)
	if err != nil || created {
		return err
	}
	if !interactive {
		if err := mcptools.WaitForInitBudget(ctx, mcptools.InitWaitBudget); err != nil {
			return err
		}
	}
	return ws.Coordinator().UpdateModels(ctx)
}

// UpdateAgent reloads the agent model configuration.
func (b *Backend) UpdateAgent(ctx context.Context, workspaceID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	coord := ws.Coordinator()
	if coord == nil {
		return errors.New("agent configuration is missing")
	}
	return coord.UpdateModels(ctx)
}

// SetMainAgent switches the workspace's active agent (coder or plan). It
// is rejected while the agent is running so a switch can never strand a
// run's queued prompts on the previous agent — the same protection the
// TUI gives itself with its busy check before toggling input mode.
func (b *Backend) SetMainAgent(workspaceID, agentID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	coord := ws.Coordinator()
	if coord == nil {
		return ErrAgentNotInitialized
	}

	if coord.IsBusy() {
		return ErrAgentBusy
	}

	return coord.SetMainAgent(agentID)
}

// CancelSession cancels an ongoing agent operation for the given
// session.
func (b *Backend) CancelSession(workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	if coord := ws.Coordinator(); coord != nil {
		coord.Cancel(sessionID)
	}
	return nil
}

// SummarizeSession triggers a session summarization.
func (b *Backend) SummarizeSession(ctx context.Context, workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	coord := ws.Coordinator()
	if coord == nil {
		return ErrAgentNotInitialized
	}

	return coord.Summarize(ctx, sessionID)
}

// QueuedPrompts returns the number of queued prompts for the session.
func (b *Backend) QueuedPrompts(workspaceID, sessionID string) (int, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return 0, err
	}

	coord := ws.Coordinator()
	if coord == nil {
		return 0, nil
	}

	return coord.QueuedPrompts(sessionID), nil
}

// ClearQueue clears the prompt queue for the session.
func (b *Backend) ClearQueue(workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	if coord := ws.Coordinator(); coord != nil {
		coord.ClearQueue(sessionID)
	}
	return nil
}

// QueuedPromptsList returns the list of queued prompt strings for a
// session.
func (b *Backend) QueuedPromptsList(workspaceID, sessionID string) ([]string, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	coord := ws.Coordinator()
	if coord == nil {
		return nil, nil
	}

	return coord.QueuedPromptsList(sessionID), nil
}

// GetDefaultSmallModel returns the default small model for a provider.
func (b *Backend) GetDefaultSmallModel(workspaceID, providerID string) (config.SelectedModel, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return config.SelectedModel{}, err
	}

	return ws.GetDefaultSmallModel(providerID), nil
}

// RunShellCommand runs a shell command in the workspace directory and
// persists the command + output as a user message in the session.
func (b *Backend) RunShellCommand(ctx context.Context, workspaceID string, req proto.ShellCommandRequest) (proto.ShellCommandResponse, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.ShellCommandResponse{}, err
	}

	// Oversized output spills into the workspace data directory. Tests
	// build workspaces without a config store; those fall back to the
	// system temp directory.
	var dataDir string
	if ws.Cfg != nil {
		dataDir = ws.Cfg.Config().Options.DataDirectory
	}

	var persist shell.PersistFunc
	if req.SessionID != "" {
		persist = func(cmd, output string, exitCode int) error {
			return shell.PersistOutput(ctx, ws.Messages, req.SessionID, cmd, output, exitCode, dataDir)
		}
	}

	result, err := shell.RunAndPersist(ctx, shell.RunOptions{
		Command:   req.Command,
		Cwd:       ws.Path,
		Env:       append(os.Environ(), ws.Env...),
		TermWidth: req.TermWidth,
	}, persist)
	if err != nil {
		return proto.ShellCommandResponse{}, err
	}

	return proto.ShellCommandResponse{
		Output:   result.Output,
		ExitCode: result.ExitCode,
	}, nil
}
