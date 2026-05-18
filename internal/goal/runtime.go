package goal

import (
	"bytes"
	"context"
	_ "embed"
	"log/slog"
	"text/template"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
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
	store    Service
	sessions session.Service
	agent    AgentRunner
	notify   pubsub.Publisher[notify.Notification]
}

func NewRuntime(store Service, sessions session.Service, agent AgentRunner, notify pubsub.Publisher[notify.Notification]) *Runtime {
	return &Runtime{
		store:    store,
		sessions: sessions,
		agent:    agent,
		notify:   notify,
	}
}

func (r *Runtime) OnTurnFinished(ctx context.Context, scopeID string) {
	err := r.MaybeContinue(ctx, scopeID)
	if err != nil {
		slog.Error("Goal runtime continuation failed", "session_id", scopeID, "error", err)
	}
}

func (r *Runtime) MaybeContinue(ctx context.Context, scopeID string) error {
	if r.agent.IsSessionBusy(scopeID) {
		return nil
	}

	if r.agent.QueuedPrompts(scopeID) > 0 {
		return nil
	}

	goal, err := r.store.Get(ctx, scopeID)
	if err != nil || goal == nil {
		return err
	}

	if goal.Status != GoalActive {
		return nil
	}

	prompt := r.RenderContinuationPrompt(goal)

	slog.Info("Starting synthetic continuation turn", "session_id", scopeID, "goal_id", goal.GoalID)

	if r.notify != nil {
		r.notify.Publish(pubsub.CreatedEvent, notify.Notification{
			SessionID: scopeID,
			Type:      notify.TypeGoalContinue,
		})
	}

	// Inject the current GoalID into the context for stale update protection.
	goalCtx := context.WithValue(ctx, GoalIDContextKey, goal.GoalID)

	_, err = r.agent.Run(goalCtx, scopeID, prompt)
	return err
}

func (r *Runtime) RenderContinuationPrompt(g *Goal) string {
	var buf bytes.Buffer
	if err := continuationTpl.Execute(&buf, g); err != nil {
		panic(err)
	}
	return buf.String()
}
