package agent

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/notebook"
)

// checkpointRunTag is the dedup tag a run's checkpoint carries — the
// durable form of the per-run claim, checked inside the commit
// transaction and by the run-end pass.
func checkpointRunTag(stamp uint64) string {
	return fmt.Sprintf("run:%d", stamp)
}

// runStartIndex returns the index just past the last user message —
// this run's calls start there. The run stamp identifies one user
// turn (repair retries share it, a folded prompt starts a new turn
// with a new stamp), so a previous run's writes can never trip this
// run's boundary pre-scan.
func runStartIndex(msgs []message.Message) int {
	start := 0
	for i, m := range msgs {
		if m.Role == message.User {
			start = i + 1
		}
	}
	return start
}

// claimCheckpoint takes the run's checkpoint slot for the mid-run
// boundary trigger — the inflight claim that dedups detection passes
// while generation is async. False when the stamp already claimed or
// a generation is still in flight.
func (t *segmentTracker) claimCheckpoint(stamp uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.checkpointStamp == stamp || t.checkpointInFlight {
		return false
	}
	t.checkpointStamp = stamp
	t.checkpointInFlight = true
	return true
}

// retryCheckpoint re-claims the slot for the run-end pass. A mid-run
// claim that completed without committing (below the boundary
// threshold) must not block the run-end floor — but a genuinely
// in-flight generation does.
func (t *segmentTracker) retryCheckpoint(stamp uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.checkpointInFlight {
		return false
	}
	t.checkpointStamp = stamp
	t.checkpointInFlight = true
	return true
}

// finishCheckpoint resolves the claim. A committed checkpoint or a
// clean not-due outcome keeps the slot claimed for the rest of the
// run — the boundary was evaluated once. A failure releases it so the
// run-end pass can retry.
func (t *segmentTracker) finishCheckpoint(stamp uint64, failed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.checkpointInFlight = false
	if failed {
		t.checkpointStamp = 0
	}
}

// firstMutatingResult is the cheap pre-scan: reports whether msgs
// contains a finished write-class call whose result landed without
// error — the investigation→execution boundary's trigger condition.
// It runs on every step before the expensive path (GetEntries +
// classifyEvents inside GenerateCheckpoint) is reached.
func firstMutatingResult(msgs []message.Message) bool {
	succeeded := make(map[string]bool)
	for _, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for _, tr := range m.ToolResults() {
			if !tr.IsError {
				succeeded[tr.ToolCallID] = true
			}
		}
	}
	for _, m := range msgs {
		if m.Role != message.Assistant {
			continue
		}
		for _, tc := range m.ToolCalls() {
			if tc.Finished && tools.IsMutatingCall(tc.Name, tc.Input) && succeeded[tc.ID] {
				return true
			}
		}
	}
	return false
}

// checkpointSegmentKey picks the coverage key for a new checkpoint:
// the session's last closed segment, possibly from an earlier turn.
// An open-segment key can never render while the run lives (the
// boundary must pass the key first), so the closed segment is
// preferred; with no closed segment the open tail's key still serves
// the next run, whose boundary will pass it once the tail's own
// coverage commits.
func checkpointSegmentKey(segs []segment) (segmentKey, bool) {
	for i := len(segs) - 1; i >= 0; i-- {
		if !segs[i].open {
			return segs[i].key(), true
		}
	}
	if len(segs) > 0 {
		return segs[len(segs)-1].key(), true
	}
	return segmentKey{}, false
}

// uncoveredTail returns the messages past the last segment with
// committed coverage — the raw slice whose classified events join the
// checkpoint's input.
func uncoveredTail(msgs []message.Message, segs []segment, processed map[segmentKey]bool) []message.Message {
	tailStart := 0
	for _, s := range segs {
		if !s.open && processed[s.key()] {
			tailStart = s.end
		}
	}
	if tailStart > len(msgs) {
		tailStart = len(msgs)
	}
	return msgs[tailStart:]
}

// maybeCheckpointBoundary is the mid-run trigger, called from
// detectSegments so it runs on every per-step rebuild regardless of
// whether the scope gate wraps the toolset. The pre-scan keeps it
// cheap: a run that never lands a successful write never pays for the
// service call. Detection is keyed to the run stamp, which only exists
// under PrepareStep's callContext — the Run-start and summarize
// preparePrompt paths carry no stamp and correctly skip.
func (a *sessionAgent) maybeCheckpointBoundary(ctx context.Context, sessionID string, msgs []message.Message, segs []segment, processed map[segmentKey]bool) {
	if !a.notebookCheckpoint {
		return
	}
	stamp := tools.GetRunStampFromContext(ctx)
	if stamp == 0 || sessionID == "" {
		return
	}
	tracker := a.segmentTracker(sessionID)
	if !firstMutatingResult(msgs[runStartIndex(msgs):]) {
		return
	}
	if !tracker.claimCheckpoint(stamp) {
		return
	}
	key, ok := checkpointSegmentKey(segs)
	if !ok {
		tracker.finishCheckpoint(stamp, true)
		return
	}
	a.spawnCheckpoint(ctx, sessionID, tracker, stamp, notebook.CheckpointRequest{
		TurnNumber:     key.turn,
		SegmentNumber:  key.segment,
		Granularity:    notebook.GranularityBoundary,
		RunTag:         checkpointRunTag(stamp),
		MinExploration: scopeGateMinExploration,
		Msgs:           uncoveredTail(msgs, segs, processed),
	})
}

// generateRunEndCheckpoint is the post-run fallback: a run that
// gathered context but never crossed the write boundary — or whose
// mid-run checkpoint failed or fell under the boundary threshold —
// consolidates at run end. The run-tag existence check doubles as the
// durable dedup when the in-memory claim was lost to a rebuild.
func (a *sessionAgent) generateRunEndCheckpoint(ctx context.Context, sessionID string, msgs []message.Message, stamp uint64, registry map[segmentKey]notebook.ProcessedSegment) {
	if !a.notebookCheckpoint || a.notebook == nil || stamp == 0 || sessionID == "" {
		return
	}
	runTag := checkpointRunTag(stamp)
	existing, err := a.notebook.SearchByTag(ctx, sessionID, runTag)
	if err != nil {
		slog.Warn("Failed to check run checkpoint tag", "session_id", sessionID, "error", err)
		return
	}
	if len(existing) > 0 {
		return
	}
	tracker := a.segmentTracker(sessionID)
	if !tracker.retryCheckpoint(stamp) {
		return
	}
	segs := segmentBoundaries(msgs, a.segTokenBudget(), a.segMaxSteps())
	key, ok := checkpointSegmentKey(segs)
	if !ok {
		tracker.finishCheckpoint(stamp, true)
		return
	}
	processed := make(map[segmentKey]bool, len(registry))
	for k, row := range registry {
		if row.State == notebook.SegmentProcessed {
			processed[k] = true
		}
	}
	a.spawnCheckpoint(ctx, sessionID, tracker, stamp, notebook.CheckpointRequest{
		TurnNumber:    key.turn,
		SegmentNumber: key.segment,
		Granularity:   notebook.GranularityBoundary,
		RunTag:        runTag,
		// The run-end floor is one gathered event — "context was
		// gathered" — versus the boundary trigger's deeper
		// exploration floor.
		MinExploration: 1,
		Msgs:           uncoveredTail(msgs, segs, processed),
	})
}

// spawnCheckpoint runs checkpoint generation on the same async seam
// as segment generation — detached, bounded, and inline under
// syncSegmentGen so tests are deterministic.
func (a *sessionAgent) spawnCheckpoint(ctx context.Context, sessionID string, tracker *segmentTracker, stamp uint64, req notebook.CheckpointRequest) {
	genMsgs := cloneMessagesForGen(req.Msgs)
	req.Msgs = genMsgs
	genCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), segmentGenTimeout)
	if a.syncSegmentGen {
		a.runCheckpoint(genCtx, sessionID, req, tracker, stamp)
		cancel()
		return
	}
	go func() {
		defer cancel()
		a.runCheckpoint(genCtx, sessionID, req, tracker, stamp)
	}()
}

// runCheckpoint invokes the service and resolves the claim: keep the
// slot on success or a clean not-due (the boundary is evaluated once
// per run), release it on failure so the run-end pass retries.
func (a *sessionAgent) runCheckpoint(ctx context.Context, sessionID string, req notebook.CheckpointRequest, tracker *segmentTracker, stamp uint64) {
	committed, err := a.notebook.GenerateCheckpoint(ctx, sessionID, req)
	if err != nil {
		slog.Error("Failed to generate checkpoint", "session_id", sessionID, "error", err)
		tracker.finishCheckpoint(stamp, true)
		return
	}
	tracker.finishCheckpoint(stamp, false)
	if !committed {
		return
	}
	if a.nbStats != nil {
		stats, _ := a.nbStats.Get(sessionID)
		stats.CheckpointsWritten++
		a.nbStats.Set(sessionID, stats)
	}
	if a.notebookSyncMem0 && a.configStore != nil {
		entries, err := a.notebook.GetByTurnSegment(ctx, sessionID, req.TurnNumber, req.SegmentNumber)
		if err != nil {
			slog.Error("Failed to get checkpoint for mem0 sync", "error", err)
			return
		}
		var fresh []notebook.Entry
		for _, e := range entries {
			if e.EventType == notebook.EventCheckpoint && slices.Contains(e.Tags, req.RunTag) {
				fresh = append(fresh, e)
			}
		}
		// Checkpoint sync is explicit — segment generation is the
		// only other SyncEntries call site, and it never sees these
		// entries.
		notebook.NewMem0Sync(a.configStore, a.notebookMemoryServer).SyncEntries(ctx, fresh)
	}
}
