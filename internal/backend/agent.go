package backend

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
)

// SendMessage validates and accepts a prompt for the workspace's agent,
// then dispatches the run on a goroutine bound to a per-run context and
// returns immediately. It does not wait for the LLM turn to complete:
// the run's lifetime is owned by the workspace and by whoever asked for
// it, not by the HTTP request that delivered the prompt. Errors from the
// dispatched run reach observers through the agent event channels (a
// notify.TypeAgentError notification), not through this return value.
//
// The run context is a child of the workspace context, so workspace
// shutdown still ends it, and it is registered in ws.runs before the
// goroutine is scheduled so that a cancel arriving in that window is not
// lost. Three things end a run early: [Backend.CancelRun], the removal
// of msg.ClientID's claim on the workspace, and the maximum run
// duration.
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

	if ws.AgentCoordinator == nil {
		return ErrAgentNotInitialized
	}

	if err := agent.ValidateCall(agent.SessionAgentCall{
		SessionID:   msg.SessionID,
		Prompt:      msg.Prompt,
		Attachments: proto.AttachmentsToMessage(msg.Attachments),
	}); err != nil {
		return err
	}

	accept := ws.AgentCoordinator.BeginAccepted(msg.SessionID)

	ws.runMu.Lock()
	if ws.closing {
		ws.runMu.Unlock()
		accept.Close()
		return ErrWorkspaceClosing
	}
	ws.runWG.Add(1)
	ws.runMu.Unlock()

	go b.runAgent(ws, msg, accept, ws.newRun(msg, b.runDuration()))
	return nil
}

// runAgent executes an accepted agent run for the workspace. It owns the
// accept reservation (releasing it on return), the runWG ticket added by
// SendMessage, and the run handle SendMessage registered: the deferred
// end() stops the duration timer, cancels the run context and
// deregisters the handle.
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
//
// msg.AutoApprove, msg.NonInteractive and the requested models travel
// the same way. The agent takes a permission hold for the turn it
// actually runs, so the approval cannot outlive the run or be revoked
// out from under it by a client that exited; and the turn it runs, not
// the workspace, is what decides whether interactive tools and the
// MCP-initialization wait apply, and which model streams.
func (b *Backend) runAgent(ws *Workspace, msg proto.AgentMessage, accept *agent.AcceptedRun, run *runHandle) {
	defer ws.runWG.Done()
	defer accept.Close()
	defer ws.end(run)

	ctx := run.ctx
	if msg.RunID != "" {
		ctx = agent.WithRunID(ctx, msg.RunID)
	}
	if msg.AutoApprove {
		ctx = agent.WithAutoApprove(ctx)
	}
	if msg.NonInteractive {
		ctx = agent.WithNonInteractive(ctx)
	}
	ctx = agent.WithRequestedModels(ctx, msg.LargeModel, msg.SmallModel)
	ctx = agent.WithRunCompleteMarker(ctx)

	_, err := ws.AgentCoordinator.RunAccepted(ctx, accept, msg.SessionID, msg.Prompt, proto.AttachmentsToMessage(msg.Attachments)...)
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
		// Detached and bounded: the run context may already be
		// cancelled (the run was ended by a client loss or by the
		// duration bound), and a must-deliver publish on a cancelled
		// context delivers nothing. The event still has to reach a
		// waiter that has not gone away.
		pubCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		rc.PublishMustDeliver(pubCtx, pubsub.UpdatedEvent, notify.RunComplete{
			SessionID: msg.SessionID,
			RunID:     msg.RunID,
			Error:     err.Error(),
		})
	}
}

// runDuration reports the current ceiling on how long a dispatched run
// may live. Read under b.mu because SetMaxRunDuration can change it.
func (b *Backend) runDuration() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxRunDuration
}

// GetAgentInfo returns the agent's model and busy status.
func (b *Backend) GetAgentInfo(workspaceID string) (proto.AgentInfo, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.AgentInfo{}, err
	}

	var agentInfo proto.AgentInfo
	if ws.AgentCoordinator != nil {
		m := ws.AgentCoordinator.Model()
		agentInfo = proto.AgentInfo{
			Model:    m.CatwalkCfg,
			ModelCfg: m.ModelCfg,
			IsBusy:   ws.AgentCoordinator.IsBusy(),
			IsReady:  true,
		}
	}
	return agentInfo, nil
}

// InitAgent makes sure the workspace has a coder agent. A workspace
// keeps the coordinator it already has, so a second attach or a client
// reconnect cannot strand runs that are already in flight; see
// app.InitCoderAgent.
func (b *Backend) InitAgent(ctx context.Context, workspaceID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	return ws.InitCoderAgent(ctx)
}

// UpdateAgent reloads the agent model configuration.
func (b *Backend) UpdateAgent(ctx context.Context, workspaceID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	return ws.UpdateAgentModel(ctx)
}

// CancelSession cancels an ongoing agent operation for the given
// session.
func (b *Backend) CancelSession(workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	if ws.AgentCoordinator != nil {
		ws.AgentCoordinator.Cancel(sessionID)
	}
	return nil
}

// SummarizeSession triggers a session summarization.
func (b *Backend) SummarizeSession(ctx context.Context, workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	if ws.AgentCoordinator == nil {
		return ErrAgentNotInitialized
	}

	return ws.AgentCoordinator.Summarize(ctx, sessionID)
}

// QueuedPrompts returns the number of queued prompts for the session.
func (b *Backend) QueuedPrompts(workspaceID, sessionID string) (int, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return 0, err
	}

	if ws.AgentCoordinator == nil {
		return 0, nil
	}

	return ws.AgentCoordinator.QueuedPrompts(sessionID), nil
}

// ClearQueue clears the prompt queue for the session.
func (b *Backend) ClearQueue(workspaceID, sessionID string) error {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return err
	}

	if ws.AgentCoordinator != nil {
		ws.AgentCoordinator.ClearQueue(sessionID)
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

	if ws.AgentCoordinator == nil {
		return nil, nil
	}

	return ws.AgentCoordinator.QueuedPromptsList(sessionID), nil
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

	var persist shell.PersistFunc
	if req.SessionID != "" {
		persist = func(cmd, output string, exitCode int) error {
			return shell.PersistOutput(ctx, ws.Messages, req.SessionID, cmd, output, exitCode)
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
