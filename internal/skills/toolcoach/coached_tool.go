package toolcoach

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/message"
)

// coachedTool wraps a fantasy.AgentTool to run heuristic coaching before
// delegating to the inner tool. It records timing and injects coaching tips
// into the tool result when an anti-pattern is detected.
type coachedTool struct {
	inner     fantasy.AgentTool
	coach     *sessionState
	sessionID string
	messages  message.Service
	cfg       ToolcoachConfig
	intensity CoachingIntensity
}

// newCoachedTool wraps the provided tool with coaching logic.
func newCoachedTool(
	inner fantasy.AgentTool,
	coach *sessionState,
	sessionID string,
	messages message.Service,
	cfg ToolcoachConfig,
	intensity CoachingIntensity,
) fantasy.AgentTool {
	return &coachedTool{
		inner:     inner,
		coach:     coach,
		sessionID: sessionID,
		messages:  messages,
		cfg:       cfg,
		intensity: intensity,
	}
}

func (c *coachedTool) Info() fantasy.ToolInfo {
	return c.inner.Info()
}

func (c *coachedTool) ProviderOptions() fantasy.ProviderOptions {
	return c.inner.ProviderOptions()
}

func (c *coachedTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	c.inner.SetProviderOptions(opts)
}

func (c *coachedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	// Resolve session ID from context if not already set.
	sessionID := c.sessionID
	if sid := tools.GetSessionFromContext(ctx); sid != "" {
		sessionID = sid
	}

	// Record that we analyzed this tool call.
	c.coach.metrics.recordToolCall()

	// Run the heuristic coach.
	result := c.coach.runCoach(c.cfg, c.intensity, call.Name, call.Input)

	if result != nil {
		c.coach.addCoachTime(time.Duration(result.DelayMicros) * time.Microsecond)
		c.coach.metrics.recordDelay(result.DelayMicros)
		c.coach.metrics.recordCoachTime(float64(result.DelayMicros) / 1000.0)
		c.coach.metrics.recordPatternFire(result.PatternID, call.Name, call.Input)
		c.showCoachIndicator(sessionID, result)
		event.TrackToolcoachPattern(sessionID, result.PatternID, result.Severity)
		event.TrackToolcoachTime(sessionID, result.DelayMicros, float64(c.coach.totalTime().Microseconds())/1000.0)
	}

	// Check if any previous pending tips were resolved by this tool call.
	// Use pattern-specific validators when available.
	validator := func(toolName, input string, tip pendingTip) bool {
		pat := patternByID(tip.patternID)
		if pat != nil && pat.Validate != nil {
			return pat.Validate(c.coach, toolName, input, tip)
		}
		// Default heuristic: expected tool + expected file.
		if toolName == tip.expectedTool {
			if tip.expectedFile == "" {
				return true
			}
			var filePath string
			switch toolName {
			case "view", "edit", "write", "multiedit":
				filePath, _ = jsonpeek(input, "file_path")
			}
			return tip.expectedFile == filePath
		}
		return false
	}
	c.coach.metrics.checkPendingTips(call.Name, call.Input, validator)

	// Guided retry: if a pattern fired with a suggested fix and auto-retry is
	// enabled, try the improved input once before returning the original result.
	var resp fantasy.ToolResponse
	var err error
	var didRetry bool
	if result != nil && result.SuggestedInput != "" && result.SuggestedInput != call.Input &&
		c.cfg.AutoRetry && !c.coach.hasRetriedThisTurn() {
		c.coach.markRetriedThisTurn()
		modifiedCall := call
		modifiedCall.Input = result.SuggestedInput
		resp, err = c.inner.Run(ctx, modifiedCall)
		if err == nil {
			didRetry = true
		}
	}

	// If no retry happened, delegate to the actual tool with original input.
	if !didRetry {
		resp, err = c.inner.Run(ctx, call)
		if err != nil {
			return resp, err
		}
	}

	// Cache view results for semantic edit validation.
	if call.Name == "view" && resp.Content != "" {
		if filePath, ok := jsonpeek(call.Input, "file_path"); ok {
			c.coach.cacheFileContent(filePath, resp.Content)
		}
	}

	// Append the coaching tip to the tool result so the LLM sees it.
	if result != nil {
		if resp.Content != "" {
			resp.Content += "\n\n"
		}
		resp.Content += fmt.Sprintf(
			"[Coach %s] %s (coach delay: %dµs, spent: %s)",
			result.Severity,
			result.Tip,
			result.DelayMicros,
			c.coach.totalTime().Round(time.Microsecond),
		)
	}

	return resp, nil
}

// showCoachIndicator creates a brief ephemeral spinner message that displays
// the coach timing to the user. It runs fire-and-forget so that a slow
// database write can never block the tool execution path.
//
// Defensive behaviours:
//   - Only one indicator per session at a time (deduplication).
//   - Panic recovery in the goroutine.
//   - Context-aware sleep so shutdown does not orphan spinners.
//   - Retry delete once on failure.
func (c *coachedTool) showCoachIndicator(sessionID string, result *coachResult) {
	if c.messages == nil {
		return
	}

	label := fmt.Sprintf(
		"Coach tip: %s (delay: %dµs, spent: %s)",
		result.PatternID,
		result.DelayMicros,
		c.coach.totalTime().Round(time.Microsecond),
	)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Toolcoach indicator goroutine panicked", "session_id", sessionID, "recover", r)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		msg, err := c.messages.Create(ctx, sessionID, message.CreateMessageParams{
			Role:         message.Assistant,
			SpinnerLabel: label,
		})
		if err != nil {
			slog.Warn("Toolcoach failed to create indicator", "session_id", sessionID, "error", err)
			return
		}

		// Deduplication: if there was a previous indicator for this session,
		// delete it immediately so only the latest one remains visible.
		if prevID := c.coach.swapActiveIndicator(msg.ID); prevID != "" {
			delCtx, delCancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = c.messages.Delete(delCtx, prevID)
			delCancel()
		}

		// Brief display so the user sees the coach fired.
		// Use a context-aware sleep so shutdown does not leave orphaned spinners.
		timer := time.NewTimer(800 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			c.coach.clearActiveIndicator(msg.ID)
			return
		}

		// Delete the indicator. Retry once on failure.
		for attempt := 0; attempt < 2; attempt++ {
			delCtx, delCancel := context.WithTimeout(context.Background(), 2*time.Second)
			delErr := c.messages.Delete(delCtx, msg.ID)
			delCancel()
			if delErr == nil {
				c.coach.clearActiveIndicator(msg.ID)
				return
			}
			if attempt == 0 {
				slog.Warn("Toolcoach failed to delete indicator, retrying", "session_id", sessionID, "error", delErr)
				time.Sleep(50 * time.Millisecond)
			}
		}
		slog.Warn("Toolcoach failed to delete indicator after retry", "session_id", sessionID)
		c.coach.clearActiveIndicator(msg.ID)
	}()
}
