package goal

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"log/slog"
	"sync"
	"text/template"
	"time"

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

	mu       sync.Mutex
	running  map[string]context.CancelFunc
	pending  map[string]bool
	turns    map[string]int
	sessions map[string]struct{}
	stopped  bool
}

func NewRuntime(store Service, agent AgentRunner, notify pubsub.Publisher[notify.Notification]) *Runtime {
	return &Runtime{
		store:    store,
		agent:    agent,
		notify:   notify,
		running:  make(map[string]context.CancelFunc),
		pending:  make(map[string]bool),
		turns:    make(map[string]int),
		sessions: make(map[string]struct{}),
	}
}

func (r *Runtime) OnTurnFinished(ctx context.Context, sessionID string) {
	err := r.MaybeContinue(ctx, sessionID)
	if err != nil {
		slog.Error("Goal runtime continuation failed", "session_id", sessionID, "error", err)
	}
}

// Pause persists the pause before cancelling a pending continuation. Completed
// goals keep their terminal status.
func (r *Runtime) Pause(ctx context.Context, sessionID string) (*Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pauseLocked(ctx, sessionID)
}

func (r *Runtime) pauseLocked(ctx context.Context, sessionID string) (*Goal, error) {
	g, err := r.store.Get(ctx, sessionID)
	if err == nil && g != nil && g.Status == GoalActive {
		g, err = r.store.UpdateStatus(ctx, sessionID, g.GoalID, GoalPaused)
	}
	if cancel := r.running[sessionID]; cancel != nil {
		cancel()
	}
	return g, err
}

// Clear removes the goal and cancels any admitted continuation.
func (r *Runtime) Clear(ctx context.Context, sessionID string) (*Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, err := r.store.Clear(ctx, sessionID)
	if cancel := r.running[sessionID]; cancel != nil {
		cancel()
	}
	return g, err
}

// Resume admits another continuation only after the paused turn has unwound.
func (r *Runtime) Resume(ctx context.Context, sessionID string) (*Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return nil, errors.New("goal runtime is shutting down")
	}
	if r.running[sessionID] != nil || r.turns[sessionID] > 0 {
		return nil, errors.New("goal turn is still stopping; try resume again shortly")
	}
	g, err := r.store.Get(ctx, sessionID)
	if err != nil || g == nil || g.Status != GoalPaused {
		return g, err
	}
	g, err = r.store.UpdateStatus(ctx, sessionID, g.GoalID, GoalActive)
	if err == nil {
		go r.OnTurnFinished(context.Background(), sessionID)
	}
	return g, err
}

// Stop prevents new continuations and pauses known goals before shutdown.
func (r *Runtime) Stop(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
	for sessionID := range r.sessions {
		if _, err := r.pauseLocked(ctx, sessionID); err != nil {
			slog.Error("Failed to pause goal during shutdown", "session_id", sessionID, "error", err)
		}
	}
}

// turnStopped reports a terminal failure or an intentional agent stop. A normal
// answer is the only successful finish that may start another goal turn.
func turnStopped(result *fantasy.AgentResult, err error) bool {
	if err != nil {
		return true
	}
	// A nil result without an error means the prompt was queued, not executed.
	if result == nil {
		return false
	}
	response := result.Response
	if len(result.Steps) > 0 {
		response = result.Steps[len(result.Steps)-1].Response
	}
	// FinishReasonStop is a normal answer.
	if response.FinishReason != fantasy.FinishReasonStop {
		return true
	}
	for _, tr := range response.Content.ToolResults() {
		if tr.StopTurn {
			return true
		}
	}
	return false
}

// BeginTurn keeps resume blocked through coordinator setup and finalization,
// including the interval after the session agent releases its busy flag.
func (r *Runtime) BeginTurn(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.turns[sessionID]++
	r.sessions[sessionID] = struct{}{}
}

// AfterTurn synchronizes ordinary user turns with their active goal. Synthetic
// turns are owned by MaybeContinue, including their cancellation and retries.
func (r *Runtime) AfterTurn(ctx context.Context, sessionID string, result *fantasy.AgentResult, err error) {
	continueGoal := false
	defer func() {
		r.mu.Lock()
		if r.turns[sessionID] > 1 {
			r.turns[sessionID]--
		} else {
			delete(r.turns, sessionID)
		}
		r.mu.Unlock()
		if continueGoal {
			go r.OnTurnFinished(context.Background(), sessionID)
		}
	}()
	if _, synthetic := ctx.Value(GoalIDContextKey).(string); synthetic {
		return
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if turnStopped(result, err) {
		pauseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, pauseErr := r.Pause(pauseCtx, sessionID); pauseErr != nil {
			slog.Error("Failed to pause goal after agent stopped", "session_id", sessionID, "error", pauseErr)
		}
		return
	}
	continueGoal = result != nil
}

func (r *Runtime) MaybeContinue(ctx context.Context, sessionID string) error {
	r.mu.Lock()
	if r.stopped || r.turns[sessionID] > 0 {
		r.mu.Unlock()
		return nil
	}
	if r.running[sessionID] != nil {
		r.pending[sessionID] = true
		r.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.running[sessionID] = cancel
	r.sessions[sessionID] = struct{}{}
	r.mu.Unlock()
	defer func() {
		failure := recover()
		if failure != nil {
			pauseCtx, pauseCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			if _, err := r.Pause(pauseCtx, sessionID); err != nil {
				slog.Error("Failed to pause goal after panic", "session_id", sessionID, "error", err)
			}
			pauseCancel()
		}
		cancel()
		r.mu.Lock()
		delete(r.running, sessionID)
		pending := r.pending[sessionID] && !r.stopped
		delete(r.pending, sessionID)
		r.mu.Unlock()
		if pending && failure == nil {
			go r.OnTurnFinished(context.Background(), sessionID)
		}
		if failure != nil {
			panic(failure)
		}
	}()

	for {
		r.mu.Lock()
		turnInFlight := r.turns[sessionID] > 0
		r.mu.Unlock()
		if turnInFlight || r.agent.IsSessionBusy(sessionID) || r.agent.QueuedPrompts(sessionID) > 0 {
			return nil
		}
		g, err := r.store.Get(context.WithoutCancel(ctx), sessionID)
		if err != nil {
			return err
		}
		if g == nil || g.Status != GoalActive {
			return nil
		}
		prompt, err := r.RenderContinuationPrompt(g)
		if err == nil {
			err = runCtx.Err()
		}
		var result *fantasy.AgentResult
		if err == nil {
			slog.Info("Starting synthetic continuation turn", "session_id", sessionID, "goal_id", g.GoalID)
			if r.notify != nil {
				r.notify.Publish(pubsub.CreatedEvent, notify.Notification{SessionID: sessionID, Type: notify.TypeGoalContinue})
			}
			goalCtx := context.WithValue(runCtx, GoalIDContextKey, g.GoalID)
			result, err = r.agent.Run(goalCtx, sessionID, prompt)
		}
		if runCtx.Err() != nil {
			err = errors.Join(err, runCtx.Err())
		}
		if turnStopped(result, err) {
			pauseCtx, pauseCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			r.mu.Lock()
			current, pauseErr := r.store.Get(pauseCtx, sessionID)
			if pauseErr == nil && current != nil && current.GoalID == g.GoalID && current.Status == GoalActive {
				_, pauseErr = r.store.UpdateStatus(pauseCtx, sessionID, g.GoalID, GoalPaused)
			}
			r.mu.Unlock()
			pauseCancel()
			return errors.Join(err, pauseErr)
		}
		if result == nil {
			return nil
		}
	}
}

func (r *Runtime) RenderContinuationPrompt(g *Goal) (string, error) {
	var buf bytes.Buffer
	if err := continuationTpl.Execute(&buf, g); err != nil {
		return "", err
	}
	return buf.String(), nil
}
