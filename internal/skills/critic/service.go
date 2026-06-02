package critic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// CriticVerdictEvent is published when a critic review completes.
type CriticVerdictEvent struct {
	SessionID    string
	Verdict      string
	Confidence   float64
	ConcernCount int
	LatencyMs    int64
}

// CriticService orchestrates the review loop between the primary agent and the
// critic agent.
type CriticService struct {
	cfg      CriticSkillConfig
	emitter  CheckpointEmitter
	cache    *FeedbackCache
	pub      pubsub.Publisher[any]
	breakers *breakerRegistry
}

// NewCriticService creates a new critic service with the given configuration.
func NewCriticService(cfg CriticSkillConfig, pub pubsub.Publisher[any]) *CriticService {
	var cache *FeedbackCache
	if cfg.CacheSize > 0 {
		var err error
		cache, err = NewFeedbackCache(cfg.CacheSize)
		if err != nil {
			slog.Warn("Failed to create critic feedback cache, caching disabled", "error", err)
			cache = nil
		}
	}
	return &CriticService{
		cfg:      cfg,
		cache:    cache,
		pub:      pub,
		breakers: newBreakerRegistry(),
	}
}

// SetCheckpointEmitter configures the emitter used to request reviews.
func (cs *CriticService) SetCheckpointEmitter(emitter CheckpointEmitter) {
	cs.emitter = emitter
}

// Review submits a checkpoint for critique and returns structured feedback.
// It manages timeouts, retries, caching, circuit breaker, and pub/sub emission.
func (cs *CriticService) Review(ctx context.Context, sessionID string, cp Checkpoint) (*CriticFeedback, error) {
	if cs.emitter == nil {
		return nil, fmt.Errorf("critic service has no checkpoint emitter configured")
	}

	start := time.Now()

	// Check cache.
	if cs.cache != nil {
		if fb, hit := cs.cache.Get(cp); hit {
			cs.publishVerdict(sessionID, fb, time.Since(start).Milliseconds())
			return fb, nil
		}
	}

	// Check circuit breaker before attempting the call.
	if breakerErr := cs.breakers.CanExecute(sessionID); breakerErr != nil {
		slog.Warn("Critic circuit breaker open", "session_id", sessionID)
		return nil, breakerErr
	}

	// Apply timeout.
	ctx, cancel := context.WithTimeout(ctx, cs.cfg.Timeout)
	defer cancel()

	var fb *CriticFeedback
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 500 * time.Millisecond
			slog.Warn("Critic review failed, retrying", "attempt", attempt, "backoff", backoff, "error", err)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, fmt.Errorf("critic review cancelled during backoff: %w", ctx.Err())
			}
		}
		fb, err = cs.emitter(ctx, cp)
		if breakerErr := cs.breakers.RecordResult(sessionID, err); breakerErr != nil {
			if errors.Is(breakerErr, ErrCircuitOpen) {
				slog.Warn("Critic circuit breaker open after attempt", "session_id", sessionID, "attempt", attempt)
			}
			// Stop retrying if the breaker opened.
			return nil, breakerErr
		}
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("critic review timed out: %w", err)
		}
	}

	if err != nil {
		return nil, err
	}

	latency := time.Since(start).Milliseconds()

	// Store in cache.
	if cs.cache != nil {
		cs.cache.Put(cp, fb)
	}

	cs.publishVerdict(sessionID, fb, latency)
	return fb, nil
}

// PublishLoopCompleted emits the loop-completed event.
func (cs *CriticService) PublishLoopCompleted(sessionID string, iterations int, finalVerdict string) {
	event.TrackCriticLoopCompleted(sessionID, iterations, finalVerdict)
	if cs.cache != nil {
		hits, misses := cs.cache.Stats()
		if hits+misses > 0 {
			slog.Debug("Critic cache stats", "session_id", sessionID, "hits", hits, "misses", misses, "hit_rate", float64(hits)/float64(hits+misses))
		}
	}
	if cs.pub == nil {
		return
	}
	cs.pub.Publish(pubsub.CriticLoopCompletedEvent, pubsub.CriticLoopEvent{
		SessionID:    sessionID,
		Iterations:   iterations,
		FinalVerdict: finalVerdict,
	})
}

// ShouldAutoApprove reports whether this feedback can proceed without user
// confirmation. When false, the caller should treat the revision as skipped.
func (cs *CriticService) ShouldAutoApprove(feedback *CriticFeedback) bool {
	if cs.cfg.AutoApprove {
		return true
	}
	if feedback.Confidence >= cs.cfg.Threshold {
		return true
	}
	for _, c := range feedback.Concerns {
		if c.Severity == "critical" {
			return false
		}
	}
	return false
}

func (cs *CriticService) publishVerdict(sessionID string, fb *CriticFeedback, latency int64) {
	event.TrackCriticVerdict(sessionID, fb.Verdict, fb.Confidence)
	if cs.pub == nil {
		return
	}
	cs.pub.Publish(pubsub.CriticVerdictRenderedEvent, CriticVerdictEvent{
		SessionID:    sessionID,
		Verdict:      fb.Verdict,
		Confidence:   fb.Confidence,
		ConcernCount: len(fb.Concerns),
		LatencyMs:    latency,
	})
}

// Enabled reports whether the critic service is configured to run.
func (cs *CriticService) Enabled() bool {
	return cs.cfg.Enabled
}
