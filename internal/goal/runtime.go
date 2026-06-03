package goal

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"text/template"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
)

//go:embed continuation_prompt.md.tpl
var continuationPromptTmpl []byte

var continuationTpl = template.Must(
	template.New("continuation").Parse(string(continuationPromptTmpl)),
)

type AgentRunner interface {
	Run(ctx context.Context, sessionID string, prompt string, attachments ...message.Attachment) (*fantasy.AgentResult, error)
	IsSessionBusy(sessionID string) bool
	QueuedPrompts(sessionID string) int
}

type Runtime struct {
	store  Service
	agent  AgentRunner
	notify pubsub.Publisher[notify.Notification]
}

func NewRuntime(store Service, agent AgentRunner, notify pubsub.Publisher[notify.Notification]) *Runtime {
	return &Runtime{
		store:  store,
		agent:  agent,
		notify: notify,
	}
}

func (r *Runtime) OnTurnFinished(ctx context.Context, sessionID string) {
	err := r.MaybeContinue(ctx, sessionID)
	if err != nil {
		slog.Error("Goal runtime continuation failed", "session_id", sessionID, "error", err)
	}
}

func (r *Runtime) MaybeContinue(ctx context.Context, sessionID string) error {
	if r.agent.IsSessionBusy(sessionID) {
		return nil
	}

	if r.agent.QueuedPrompts(sessionID) > 0 {
		return nil
	}

	goal, err := r.store.Get(ctx, sessionID)
	if err != nil || goal == nil {
		return err
	}

	if goal.Status != GoalActive {
		return nil
	}

	prompt, err := r.RenderContinuationPrompt(goal)
	if err != nil {
		return fmt.Errorf("rendering continuation prompt: %w", err)
	}

	slog.Info("Starting synthetic continuation turn", "session_id", sessionID, "goal_id", goal.GoalID)

	if r.notify != nil {
		r.notify.Publish(pubsub.CreatedEvent, notify.Notification{
			SessionID: sessionID,
			Type:      notify.TypeGoalContinue,
		})
	}

	// Inject the current GoalID into the context for stale update protection.
	goalCtx := context.WithValue(ctx, GoalIDContextKey, goal.GoalID)

	_, err = r.agent.Run(goalCtx, sessionID, prompt)
	return err
}

func (r *Runtime) RenderContinuationPrompt(g *Goal) (string, error) {
	var buf bytes.Buffer
	if err := continuationTpl.Execute(&buf, g); err != nil {
		return "", err
	}
	return buf.String(), nil
}
