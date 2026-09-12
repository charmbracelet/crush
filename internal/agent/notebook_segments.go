package agent

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/notebook"
)

const (
	// segmentTokenThreshold closes a segment once it accumulates this
	// many estimated tokens. It is the effective granularity of the
	// raw window: DefaultRawTokenBudget becomes a soft ceiling over
	// whole segments rather than a hard cut.
	segmentTokenThreshold = 15_000
	// segmentMaxStepCount closes a segment after this many assistant
	// steps even when the token threshold has not tripped.
	segmentMaxStepCount = 10
	// stubRecentSegmentGuard is the recency guard for stub promotion
	// in segment terms: results in the last two segments (the open
	// segment and the most recent closed one) are never stubbed.
	stubRecentSegmentGuard = 2
	// segmentRetryBase/segmentRetryMax bound the exponential backoff
	// applied to failed segment generation attempts.
	segmentRetryBase = 30 * time.Second
	segmentRetryMax  = 10 * time.Minute
	// segmentGenBurstLimit caps how many segment generations one
	// detection pass may fire. A legacy session or crash-recovery
	// backlog would otherwise launch a small-model call per uncovered
	// segment at once; uncovered segments stay raw via pull-back and
	// are picked up by later passes.
	segmentGenBurstLimit = 4
)

// segmentKey identifies a segment within a session: its turn number
// and its index within that turn.
type segmentKey struct {
	turn    int64
	segment int64
}

// segment is a group of completed steps inside a user turn — the
// coverage and recency unit for intra-turn boundaries. Start is
// inclusive, End exclusive; both are message indices into the slice
// the segment was computed on. Open marks the trailing segment still
// accumulating steps: it is always raw and never needs coverage.
type segment struct {
	turn   int64
	number int64
	start  int
	end    int
	open   bool
}

func (s segment) key() segmentKey {
	return segmentKey{turn: s.turn, segment: s.number}
}

// allCallsResolved is the reference predicate for segment safety.
// segmentBoundaries implements a strictly stricter variant inline via
// its pending map — it keeps a call pending until the LAST position
// of its results, while this predicate accepts the first — so tests
// asserting this on every closed boundary verify the minimum
// invariant the implementation must satisfy.
//
// allCallsResolved reports whether index i is a safe cut: every tool
// call in msgs[:i] has its result in msgs[:i], or has no result
// anywhere in msgs. A call whose result exists but lands after i makes
// the cut unsafe — it would split a call from its result. A call with
// no result anywhere (cancelled/interrupted calls never produce one;
// the render synthesizes a response instead) is an orphan and must not
// block, or one cancelled call would freeze the predicate and pin the
// entire post-cancel tail raw forever.
func allCallsResolved(msgs []message.Message, i int) bool {
	resolved := make(map[string]bool) // results at index < i
	known := make(map[string]bool)    // results anywhere in msgs
	for j, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for _, tr := range m.ToolResults() {
			known[tr.ToolCallID] = true
			if j < i {
				resolved[tr.ToolCallID] = true
			}
		}
	}
	for j := 0; j < i && j < len(msgs); j++ {
		if msgs[j].Role != message.Assistant {
			continue
		}
		for _, tc := range msgs[j].ToolCalls() {
			if known[tc.ID] && !resolved[tc.ID] {
				return false
			}
		}
	}
	return true
}

// segmentBoundaries partitions msgs into segments. A segment closes at
// the first safe index after any of: the token threshold since the
// segment opened, the step cap, or a user message (a hard boundary —
// a folded mid-run prompt ends the segment and starts a new turn).
// The final segment is the open tail. Segmentation is deterministic
// and forward-only, and the token accumulator counts full stored
// content regardless of stub application, so a promotion inside a
// closed segment cannot drift its recomputed close point.
func segmentBoundaries(msgs []message.Message, tokenThreshold, maxSteps int) []segment {
	if len(msgs) == 0 {
		return nil
	}
	// Index the LAST position of each call's result so the walk keeps
	// a call pending until every stored result for it has been passed
	// — a call whose results span multiple tool messages must not be
	// cut between them.
	resultPos := make(map[string]int)
	for i, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for _, tr := range m.ToolResults() {
			resultPos[tr.ToolCallID] = i
		}
	}
	// pending holds calls whose result lands at or after the current
	// position. Orphaned calls (no result anywhere) are never added.
	pending := make(map[string]int)
	// resolvedBefore drops entries whose result precedes a cut at
	// index cut, leaving exactly the calls that would be split.
	resolvedBefore := func(cut int) {
		for id, pos := range pending {
			if pos < cut {
				delete(pending, id)
			}
		}
	}

	turns := messageTurns(msgs)
	var segs []segment
	start := 0
	acc := 0
	steps := 0
	curTurn := turns[0]
	var segNum int64
	for i := 0; i < len(msgs); i++ {
		m := msgs[i]
		if m.Role == message.Assistant {
			for _, tc := range m.ToolCalls() {
				if pos, ok := resultPos[tc.ID]; ok && pos > i {
					pending[tc.ID] = pos
				}
			}
			steps++
		}
		acc += estimateSegmentTokens(msgs[i : i+1])
		cut := i + 1
		if cut >= len(msgs) {
			break
		}
		resolvedBefore(cut)
		if msgs[cut].Role == message.User {
			// A user message is a hard boundary and wins over the
			// pending guard: a call whose results straddle the fold
			// cannot block it, or the segment would swallow the user
			// message and mislabel every following segment's turn.
			// The straggler result lands in the next segment as an
			// orphan, which the renderer drops gracefully.
			segs = append(segs, segment{turn: curTurn, number: segNum, start: start, end: cut})
			start = cut
			acc, steps = 0, 0
			curTurn = turns[cut]
			segNum = 0
			continue
		}
		if len(pending) > 0 {
			// Unsafe: a call opened before the cut resolves after
			// it. Keep accumulating until the run completes.
			continue
		}
		if acc >= tokenThreshold || steps >= maxSteps {
			segs = append(segs, segment{turn: curTurn, number: segNum, start: start, end: cut})
			start = cut
			acc, steps = 0, 0
			segNum++
		}
	}
	segs = append(segs, segment{turn: curTurn, number: segNum, start: start, end: len(msgs), open: true})
	return segs
}

// estimateSegmentTokens is the segmentation accumulator: unlike
// estimateRawMessageTokens it always counts full stored content,
// ignoring whether a superseded mark has been applied. Stability of
// the close point must not depend on prompt-assembly state.
func estimateSegmentTokens(msgs []message.Message) int {
	var totalChars int
	for _, m := range msgs {
		for _, part := range m.Parts {
			switch v := part.(type) {
			case message.TextContent:
				totalChars += len(v.Text)
			case message.ToolCall:
				totalChars += len(v.Input)
			case message.ToolResult:
				totalChars += len(v.Content)
			case message.ReasoningContent:
				totalChars += len(v.Thinking)
			}
		}
	}
	return totalChars / 4
}

// findSegmentBoundaryByTokenBudget returns the index where the raw
// window starts. The raw window is always whole segments: the open
// tail plus however many trailing closed segments fit the budget. A
// closed segment that lacks processed coverage stays raw — the stale
// pull-back moves the boundary to the oldest uncovered segment's
// start, so nothing drops without entries behind it.
func findSegmentBoundaryByTokenBudget(msgs []message.Message, tokenBudget int, segs []segment, processed map[segmentKey]bool) int {
	if tokenBudget <= 0 || len(segs) == 0 {
		return 0
	}
	boundary := 0
	accumulated := 0
	for i := len(segs) - 1; i >= 0; i-- {
		s := segs[i]
		segTokens := estimateRawMessageTokens(msgs[s.start:s.end])
		if !s.open && accumulated+segTokens > tokenBudget {
			boundary = s.end
			break
		}
		boundary = s.start
		accumulated += segTokens
	}
	// Stale-segment fallback: coverage is per segment, so a dropped
	// segment without a processed marker pulls the boundary back to
	// its start — exposing roughly one segment, not the whole turn.
	for _, s := range segs {
		if s.open || s.end > boundary {
			break
		}
		if !processed[s.key()] {
			boundary = s.start
			break
		}
	}
	return boundary
}

// boundarySegmentKey returns the coverage key of the segment starting
// at boundary — the (turn, segment) pair entries compare against.
// Boundaries always land on segment starts; the zero key is the
// fallback for an empty list.
func boundarySegmentKey(segs []segment, boundary int) segmentKey {
	for _, s := range segs {
		if s.start == boundary {
			return s.key()
		}
		if s.start > boundary {
			break
		}
	}
	return segmentKey{}
}

// segmentOrdinals maps each message index to its segment's position in
// segs, for the segment-grained recency guard.
func segmentOrdinals(segs []segment, n int) []int {
	ord := make([]int, n)
	for i, s := range segs {
		for j := s.start; j < s.end && j < n; j++ {
			ord[j] = i
		}
	}
	return ord
}

// segmentRetryDue reports whether an unprocessed segment's next
// generation attempt is due — exponential backoff on retry_count,
// capped, so a persistently failing segment does not re-fire an LLM
// call on every detection pass.
func segmentRetryDue(seg notebook.ProcessedSegment, now time.Time) bool {
	if seg.LastAttemptAt == 0 {
		return true
	}
	backoff := segmentRetryBase << min(seg.RetryCount, 6)
	if backoff > segmentRetryMax {
		backoff = segmentRetryMax
	}
	return now.Sub(time.Unix(seg.LastAttemptAt, 0)) >= backoff
}

// segmentTracker holds the per-session in-memory segment state: which
// segments have a generation goroutine in flight (intra-process dedup
// — detection re-runs every step while generation is async), whether
// the pre-upgrade backfill has been attempted, and which drifted
// processed rows have already been logged.
type segmentTracker struct {
	mu                sync.Mutex
	inflight          map[segmentKey]bool
	backfillAttempted bool
	driftLogged       map[segmentKey]bool
}

func newSegmentTracker() *segmentTracker {
	return &segmentTracker{inflight: make(map[segmentKey]bool), driftLogged: make(map[segmentKey]bool)}
}

// markInflight atomically claims a segment for generation. The mark is
// taken in the same critical section as the decision to generate so a
// detection pass running while a completion commits cannot re-fire.
func (t *segmentTracker) markInflight(key segmentKey) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.inflight[key] {
		return false
	}
	t.inflight[key] = true
	return true
}

// clearInflight releases a segment's generation claim. Called only
// after the entries+marker commit has landed — clearing earlier would
// let a detection pass in the gap re-fire.
func (t *segmentTracker) clearInflight(key segmentKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.inflight, key)
}

func (t *segmentTracker) claimBackfill() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.backfillAttempted {
		return false
	}
	t.backfillAttempted = true
	return true
}

func (t *segmentTracker) resetBackfill() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.backfillAttempted = false
}

// claimDriftLog returns true the first time a drifted processed row is
// reported — the condition never self-resolves, so without the claim
// every detection pass would emit the same warning.
func (t *segmentTracker) claimDriftLog(key segmentKey) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.driftLogged[key] {
		return false
	}
	t.driftLogged[key] = true
	return true
}

// cachedPrefix is the rendered notebook prefix (notebook system
// message plus optional auto-inject message) cached on the boundary
// value and a fingerprint of everything the render reads: entries and
// relevance refs. A boundary that does not move reuses the cached
// prefix — the splice still runs every step, but the re-render is
// gated on actual input change, which makes the byte-identical-no-op
// criterion trivially satisfiable.
type cachedPrefix struct {
	boundary    int
	fingerprint uint64
	msgs        []fantasy.Message
}

func (a *sessionAgent) segTokenBudget() int {
	if a.segmentTokenBudget > 0 {
		return a.segmentTokenBudget
	}
	return segmentTokenThreshold
}

func (a *sessionAgent) segMaxSteps() int {
	if a.segmentMaxSteps > 0 {
		return a.segmentMaxSteps
	}
	return segmentMaxStepCount
}

func (a *sessionAgent) segmentTracker(sessionID string) *segmentTracker {
	return a.segmentTrackers.GetOrSet(sessionID, newSegmentTracker)
}

// detectSegments is the per-step segment pass: recompute segment
// boundaries, backfill the registry for pre-upgrade sessions, record
// freshly closed segments as unprocessed, and fire generation for any
// closed segment lacking coverage — including catch-up for past turns
// whose runs never generated entries. It returns the segments and the
// set of segment keys with committed coverage. All durable writes go
// through a bounded detached context so a stream cancellation cannot
// wedge the registry.
func (a *sessionAgent) detectSegments(ctx context.Context, sessionID string, msgs []message.Message) (segs []segment, processed map[segmentKey]bool) {
	segs = segmentBoundaries(msgs, a.segTokenBudget(), a.segMaxSteps())
	processed = make(map[segmentKey]bool)
	if sessionID == "" || a.notebook == nil {
		return segs, processed
	}
	detCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	tracker := a.segmentTracker(sessionID)

	registry, err := a.segmentRegistry(detCtx, sessionID)
	if err != nil {
		return segs, processed
	}

	// Migration backfill: a pre-upgrade session has entries but an
	// empty registry. Mark every closed segment of a turn that has
	// entries processed directly — no regeneration, or covered turns
	// would double into the notebook. Runs once per session; a failed
	// attempt resets so the next pass retries.
	if len(registry) == 0 && tracker.claimBackfill() {
		if a.backfillSegmentRegistry(detCtx, sessionID, segs) {
			if registry, err = a.segmentRegistry(detCtx, sessionID); err != nil {
				return segs, processed
			}
		} else {
			tracker.resetBackfill()
		}
	}

	genCtx := context.WithoutCancel(ctx)
	fired := 0
	closed := false
	for _, s := range segs {
		if s.open {
			continue
		}
		key := s.key()
		row, recorded := registry[key]
		if !recorded {
			// Record the close before the boundary walk so this same
			// render's pull-back already sees the segment.
			if err := a.notebook.RecordSegmentClose(detCtx, sessionID, s.turn, s.number, int64(s.start), int64(s.end)); err != nil {
				slog.Warn("Failed to record segment close", "session_id", sessionID, "turn", s.turn, "segment", s.number, "error", err)
				continue
			}
			closed = true
			row = notebook.ProcessedSegment{TurnNumber: s.turn, SegmentNumber: s.number, State: notebook.SegmentUnprocessed}
			registry[key] = row
		} else if row.State == notebook.SegmentUnprocessed &&
			(row.StartIndex != int64(s.start) || row.EndIndex != int64(s.end)) {
			// The recomputed extent drifted from what was recorded —
			// refresh the row (upsert applies while unprocessed) so
			// generation covers the current extent.
			if err := a.notebook.RecordSegmentClose(detCtx, sessionID, s.turn, s.number, int64(s.start), int64(s.end)); err == nil {
				row.StartIndex = int64(s.start)
				row.EndIndex = int64(s.end)
				registry[key] = row
			}
		}
		if row.State == notebook.SegmentProcessed {
			// Coverage claims are extent-checked: a processed row
			// whose recorded extent no longer matches the recomputed
			// segment fails closed and stays raw rather than dropping
			// messages it may not actually summarize.
			if row.StartIndex == int64(s.start) && row.EndIndex == int64(s.end) {
				processed[key] = true
			} else if tracker.claimDriftLog(key) {
				slog.Warn("Processed segment extent drifted; keeping raw",
					"session_id", sessionID, "turn", s.turn, "segment", s.number,
					"recorded", fmt.Sprintf("[%d,%d)", row.StartIndex, row.EndIndex),
					"recomputed", fmt.Sprintf("[%d,%d)", s.start, s.end))
			}
			continue
		}
		if fired >= segmentGenBurstLimit ||
			!segmentRetryDue(row, time.Now()) || !tracker.markInflight(key) {
			continue
		}
		if err := a.notebook.RecordSegmentAttempt(detCtx, sessionID, s.turn, s.number); err != nil {
			slog.Warn("Failed to record segment attempt", "session_id", sessionID, "error", err)
			tracker.clearInflight(key)
			continue
		}
		fired++
		// Each goroutine gets a private deep clone: flagging and
		// promotion mutate tool-result parts and mark pointers, so a
		// shared or shallow clone would race with sibling goroutines
		// and with stub promotion on this goroutine.
		genMsgs := cloneMessagesForGen(msgs)
		if a.syncSegmentGen {
			a.generateSegment(genCtx, sessionID, s, genMsgs[s.start:s.end], tracker)
		} else {
			go a.generateSegment(genCtx, sessionID, s, genMsgs[s.start:s.end], tracker)
		}
	}
	// Flag superseded tool results once per pass that fired generation
	// or recorded a close — the mid-run stub win ("a read superseded
	// two segments ago gets stubbed") needs a pass over the full
	// history; running it only on fires would delay supersession while
	// a slow generation sits in-flight.
	if (fired > 0 || closed) && a.stubSuperseded {
		flagMsgs := cloneMessagesForGen(msgs)
		if a.syncSegmentGen {
			a.flagPrunableToolResults(genCtx, flagMsgs)
		} else {
			go a.flagPrunableToolResults(genCtx, flagMsgs)
		}
	}
	return segs, processed
}

// cloneMessagesForGen deep-copies messages for a generation goroutine.
// Message.Clone copies the parts slice but shares pointer fields —
// ToolResult.Superseded is a *SupersededMark that stub flagging and
// promotion both mutate, so the mark must be copied too or a
// generation goroutine still aliases the live message's marks.
func cloneMessagesForGen(msgs []message.Message) []message.Message {
	out := make([]message.Message, len(msgs))
	for i := range msgs {
		out[i] = msgs[i].Clone()
		for j, p := range out[i].Parts {
			if tr, ok := p.(message.ToolResult); ok && tr.Superseded != nil {
				mark := *tr.Superseded
				tr.Superseded = &mark
				out[i].Parts[j] = tr
			}
		}
	}
	return out
}

// segmentRegistry loads the processed_segments table into a lookup
// keyed by (turn, segment).
func (a *sessionAgent) segmentRegistry(ctx context.Context, sessionID string) (map[segmentKey]notebook.ProcessedSegment, error) {
	rows, err := a.notebook.ProcessedSegments(ctx, sessionID)
	if err != nil {
		slog.Error("Failed to list processed segments", "session_id", sessionID, "error", err)
		return nil, err
	}
	registry := make(map[segmentKey]notebook.ProcessedSegment, len(rows))
	for _, r := range rows {
		registry[segmentKey{turn: r.TurnNumber, segment: r.SegmentNumber}] = r
	}
	return registry, nil
}

// backfillSegmentRegistry marks closed segments of turns that already
// have entries as processed. Returns false when the backfill could not
// complete so the caller retries on the next pass.
func (a *sessionAgent) backfillSegmentRegistry(ctx context.Context, sessionID string, segs []segment) bool {
	turns, err := a.notebook.TurnsWithEntries(ctx, sessionID)
	if err != nil || len(turns) == 0 {
		return err == nil
	}
	var covered []notebook.ProcessedSegment
	for _, s := range segs {
		if s.open || !turns[s.turn] {
			continue
		}
		covered = append(covered, notebook.ProcessedSegment{
			TurnNumber:    s.turn,
			SegmentNumber: s.number,
			StartIndex:    int64(s.start),
			EndIndex:      int64(s.end),
		})
	}
	if len(covered) == 0 {
		return true
	}
	if err := a.notebook.MarkSegmentsProcessed(ctx, sessionID, covered); err != nil {
		slog.Error("Failed to backfill processed segments", "session_id", sessionID, "error", err)
		return false
	}
	return true
}

// generateSegment runs entry generation for one closed segment.
// Completion — entries and the processed marker — lands in one
// transaction inside GenerateSegmentEntries; on failure the in-flight
// mark clears and the segment stays unprocessed, retryable under
// backoff.
func (a *sessionAgent) generateSegment(ctx context.Context, sessionID string, s segment, segMsgs []message.Message, tracker *segmentTracker) {
	defer tracker.clearInflight(s.key())
	if err := a.notebook.GenerateSegmentEntries(ctx, sessionID, s.turn, s.number, int64(s.start), int64(s.end), segMsgs); err != nil {
		slog.Error("Failed to generate segment entries", "session_id", sessionID, "turn", s.turn, "segment", s.number, "error", err)
		return
	}
	if a.notebookSyncMem0 && a.configStore != nil {
		entries, err := a.notebook.GetByTurnSegment(ctx, sessionID, s.turn, s.number)
		if err != nil {
			slog.Error("Failed to get segment entries for mem0 sync", "error", err)
		} else {
			mem0 := notebook.NewMem0Sync(a.configStore, a.notebookMemoryServer, sessionID)
			mem0.SyncEntries(ctx, entries)
		}
	}
}

// generateRunEndSegments is the post-run coverage pass: it fires
// generation for every segment of the turns this run produced that
// still lacks committed coverage — the uncovered tail plus any
// segments whose mid-run generation failed — and nothing before this
// run's first turn (those retry under mid-run catch-up backoff).
// The tail becomes a closed segment the moment the next user message
// lands; writing its marker in the same transaction as its entries is
// what keeps the next run's catch-up from re-firing it into duplicate
// entries. A user message created by a *later* run (visible past this
// run's final assistant message) means the open tail belongs to that
// run and is left for its own detection.
func (a *sessionAgent) generateRunEndSegments(ctx context.Context, sessionID string, msgs []message.Message, preTurnMsgCount int, lastAssistantID string) {
	if a.notebook == nil || sessionID == "" || preTurnMsgCount >= len(msgs) {
		return
	}
	turns := messageTurns(msgs)
	thisTurn := turns[preTurnMsgCount]
	segs := segmentBoundaries(msgs, a.segTokenBudget(), a.segMaxSteps())

	// The tail belongs to the turn containing this run's final
	// assistant message; a user message after it means a newer run
	// owns the tail now.
	tailTurn := thisTurn
	if lastAssistantID != "" {
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].ID == lastAssistantID {
				tailTurn = turns[i]
				break
			}
		}
	}

	registry, err := a.segmentRegistry(ctx, sessionID)
	if err != nil {
		return
	}
	tracker := a.segmentTracker(sessionID)
	for _, s := range segs {
		if s.turn < thisTurn || s.turn > tailTurn {
			continue
		}
		if s.open && s.turn != tailTurn {
			continue
		}
		key := s.key()
		row, recorded := registry[key]
		if !recorded {
			if err := a.notebook.RecordSegmentClose(ctx, sessionID, s.turn, s.number, int64(s.start), int64(s.end)); err != nil {
				slog.Warn("Failed to record segment close", "session_id", sessionID, "turn", s.turn, "segment", s.number, "error", err)
				continue
			}
			row = notebook.ProcessedSegment{State: notebook.SegmentUnprocessed}
			registry[key] = row
		}
		if row.State == notebook.SegmentProcessed || !tracker.markInflight(key) {
			continue
		}
		genMsgs := cloneMessagesForGen(msgs)
		if a.syncSegmentGen {
			a.generateSegment(ctx, sessionID, s, genMsgs[s.start:s.end], tracker)
		} else {
			go a.generateSegment(ctx, sessionID, s, genMsgs[s.start:s.end], tracker)
		}
	}
}

// prefixFingerprint hashes every input the notebook prefix render
// reads: the boundary position, its coverage key, the entries
// (identity plus the fields compaction rewrites), and the relevance
// refs. Identical inputs must render byte-identical output, so the
// cache key is the input set itself.
func prefixFingerprint(boundary int, bKey segmentKey, entries []notebook.Entry, refs []string) uint64 {
	h := fnv.New64a()
	var scratch [8]byte
	write := func(v int64) {
		binary.LittleEndian.PutUint64(scratch[:], uint64(v))
		h.Write(scratch[:])
	}
	write(int64(boundary))
	write(bKey.turn)
	write(bKey.segment)
	for _, e := range entries {
		h.Write([]byte(e.ID))
		write(e.TurnNumber)
		write(e.SegmentNumber)
		write(e.EventNumber)
		write(e.CompressionLevel)
		write(e.TokenCount)
		// Hash the text itself, not just its length: compaction
		// rewrites entry text and only convention bumps the numeric
		// fields the fingerprint reads.
		h.Write([]byte(e.EntryText))
		write(int64(len(e.EntryText)))
		h.Write([]byte(e.EntryTextFull))
		write(int64(len(e.EntryTextFull)))
	}
	for _, r := range refs {
		h.Write([]byte(r))
		write(int64(len(r)))
	}
	return h.Sum64()
}

// notebookPrefix returns the prefix messages (notebook system message
// plus the optional auto-inject blob) for the given boundary. The
// render is cached on (boundary, fingerprint) so a step where nothing
// covered changed emits a byte-identical prefix without re-rendering.
func (a *sessionAgent) notebookPrefix(ctx context.Context, sessionID string, msgs []message.Message, boundary int, bKey segmentKey, segs []segment) []fantasy.Message {
	if a.notebook == nil || boundary <= 0 || sessionID == "" {
		return nil
	}
	detCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	entries, err := a.notebook.GetEntries(detCtx, sessionID)
	if err != nil {
		slog.Error("Failed to get notebook entries", "error", err)
		return nil
	}
	// Refs and auto-inject scan the FULL message list for the latest
	// user message: once a long turn's initiating prompt is covered by
	// the notebook, the raw window holds no user message at all, and
	// restricting the scan to it would silently drop file: refs for
	// the rest of the run.
	refs := notebookRelevanceRefs(detCtx, a.sessions, sessionID, msgs)
	fp := prefixFingerprint(boundary, bKey, entries, refs)
	if a.prefixCache != nil {
		if c, ok := a.prefixCache.Get(sessionID); ok && c.boundary == boundary && c.fingerprint == fp {
			return c.msgs
		}
	}
	prefix := a.renderNotebookPrefix(detCtx, sessionID, entries, msgs, bKey, coveredSegmentFloor(segs, boundary), refs)
	if a.prefixCache != nil {
		a.prefixCache.Set(sessionID, cachedPrefix{boundary: boundary, fingerprint: fp, msgs: prefix})
	}
	return prefix
}

// rebuildStepMessages recomputes the prompt from stored messages for
// the current step: flush pending debounced writes, re-list, run the
// shared preparePrompt path (segment detection, boundary, prefix), and
// re-attach the leading system message from the request's own message
// list. Returns false on any failure — the caller keeps Fantasy's
// accumulated messages, which is always a safe fallback.
func (a *sessionAgent) rebuildStepMessages(ctx context.Context, sessionID string, optionsMsgs []fantasy.Message, supportsImages bool) ([]fantasy.Message, bool) {
	// Update is debounced; List reads storage. Flush first so the
	// boundary never computes against a stale view.
	if err := a.messages.FlushAll(ctx); err != nil {
		slog.Warn("Failed to flush messages before step boundary recompute", "session_id", sessionID, "error", err)
		return nil, false
	}
	msgs, err := a.messages.List(ctx, sessionID)
	if err != nil || len(msgs) == 0 {
		return nil, false
	}
	history, _ := a.preparePrompt(ctx, msgs, supportsImages)
	out := history
	if a.systemPrompt.Get() != "" && len(optionsMsgs) > 0 && optionsMsgs[0].Role == fantasy.MessageRoleSystem {
		out = make([]fantasy.Message, 0, len(history)+1)
		out = append(out, optionsMsgs[0])
		out = append(out, history...)
	}
	if envEnabled(envNotebookShadow) {
		// Shadow mode: rebuild and diff every step but splice nothing —
		// the request keeps Fantasy's accumulated messages.
		diffFantasyMessages(sessionID, optionsMsgs, out)
		return nil, false
	}
	return out, true
}

// envNotebookShadow switches the splice into shadow mode: the rebuild
// still runs every step but is only diffed against Fantasy's
// accumulated messages and logged — the request keeps the accumulated
// tail. Tail fidelity is a release blocker, so the mode stays as an
// escape hatch for comparing rebuilt and accumulated message lists on
// a live provider.
const envNotebookShadow = "CRUSH_NOTEBOOK_SHADOW"

func envEnabled(name string) bool {
	return os.Getenv(name) == "1" || strings.EqualFold(os.Getenv(name), "true")
}

// diffFantasyMessages compares the rebuilt message list with what
// Fantasy accumulated and logs the first divergence. Part-level
// provider metadata (reasoning signatures, provider-executed calls)
// and rewritten tool-call inputs are the expected mismatch sources.
func diffFantasyMessages(sessionID string, want, got []fantasy.Message) {
	n := min(len(want), len(got))
	for i := 0; i < n; i++ {
		if !fantasyMessageEqual(want[i], got[i]) {
			slog.Info("Notebook shadow diff: rebuilt message diverges",
				"session_id", sessionID,
				"index", i,
				"want_role", want[i].Role,
				"got_role", got[i].Role,
				"want_parts", len(want[i].Content),
				"got_parts", len(got[i].Content),
			)
			return
		}
	}
	if len(want) != len(got) {
		slog.Info("Notebook shadow diff: rebuilt list length diverges",
			"session_id", sessionID,
			"want_len", len(want),
			"got_len", len(got),
		)
	}
}

// fantasyMessageEqual compares role plus each part's type and content
// — the wire-relevant payload — ignoring call-frame data like
// ProviderOptions, which is stripped and reapplied separately.
func fantasyMessageEqual(a, b fantasy.Message) bool {
	if a.Role != b.Role || len(a.Content) != len(b.Content) {
		return false
	}
	for i := range a.Content {
		if !fantasyPartEqual(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}

func fantasyPartEqual(a, b fantasy.MessagePart) bool {
	switch ap := a.(type) {
	case fantasy.TextPart:
		bp, ok := b.(fantasy.TextPart)
		return ok && ap.Text == bp.Text
	case *fantasy.TextPart:
		bp, ok := b.(*fantasy.TextPart)
		return ok && ap.Text == bp.Text
	case fantasy.ReasoningPart:
		bp, ok := b.(fantasy.ReasoningPart)
		return ok && ap.Text == bp.Text
	case *fantasy.ReasoningPart:
		bp, ok := b.(*fantasy.ReasoningPart)
		return ok && ap.Text == bp.Text
	case fantasy.FilePart:
		bp, ok := b.(fantasy.FilePart)
		return ok && ap.Filename == bp.Filename && ap.MediaType == bp.MediaType && string(ap.Data) == string(bp.Data)
	case *fantasy.FilePart:
		bp, ok := b.(*fantasy.FilePart)
		return ok && ap.Filename == bp.Filename && ap.MediaType == bp.MediaType && string(ap.Data) == string(bp.Data)
	case fantasy.ToolCallPart:
		bp, ok := b.(fantasy.ToolCallPart)
		return ok && ap.ToolCallID == bp.ToolCallID && ap.ToolName == bp.ToolName && ap.Input == bp.Input && ap.ProviderExecuted == bp.ProviderExecuted
	case *fantasy.ToolCallPart:
		bp, ok := b.(*fantasy.ToolCallPart)
		return ok && ap.ToolCallID == bp.ToolCallID && ap.ToolName == bp.ToolName && ap.Input == bp.Input && ap.ProviderExecuted == bp.ProviderExecuted
	case fantasy.ToolResultPart:
		bp, ok := b.(fantasy.ToolResultPart)
		return ok && ap.ToolCallID == bp.ToolCallID && fantasyToolResultOutputEqual(ap.Output, bp.Output)
	case *fantasy.ToolResultPart:
		bp, ok := b.(*fantasy.ToolResultPart)
		return ok && ap.ToolCallID == bp.ToolCallID && fantasyToolResultOutputEqual(ap.Output, bp.Output)
	}
	return false
}

func fantasyToolResultOutputEqual(a, b fantasy.ToolResultOutputContent) bool {
	switch ao := a.(type) {
	case fantasy.ToolResultOutputContentText:
		bo, ok := b.(fantasy.ToolResultOutputContentText)
		return ok && ao.Text == bo.Text
	case *fantasy.ToolResultOutputContentText:
		bo, ok := b.(*fantasy.ToolResultOutputContentText)
		return ok && ao.Text == bo.Text
	case fantasy.ToolResultOutputContentError:
		bo, ok := b.(fantasy.ToolResultOutputContentError)
		return ok && ao.Error != nil && bo.Error != nil && ao.Error.Error() == bo.Error.Error()
	case *fantasy.ToolResultOutputContentError:
		bo, ok := b.(*fantasy.ToolResultOutputContentError)
		return ok && ao.Error != nil && bo.Error != nil && ao.Error.Error() == bo.Error.Error()
	case fantasy.ToolResultOutputContentMedia:
		bo, ok := b.(fantasy.ToolResultOutputContentMedia)
		return ok && ao.Data == bo.Data && ao.MediaType == bo.MediaType && ao.Text == bo.Text
	case *fantasy.ToolResultOutputContentMedia:
		bo, ok := b.(*fantasy.ToolResultOutputContentMedia)
		return ok && ao.Data == bo.Data && ao.MediaType == bo.MediaType && ao.Text == bo.Text
	}
	return false
}

// renderNotebookPrefix filters entries to segments covered before the
// boundary, relevance-selects them, and renders the notebook system
// message plus the optional auto-inject blob. msgs is the full stored
// history — auto-inject scans it for the latest user message, which
// may itself sit inside the covered prefix of a long turn.
func (a *sessionAgent) renderNotebookPrefix(ctx context.Context, sessionID string, entries []notebook.Entry, msgs []message.Message, bKey segmentKey, floor segmentKey, refs []string) []fantasy.Message {
	var filtered []notebook.Entry
	for _, e := range entries {
		if e.TurnNumber < bKey.turn || (e.TurnNumber == bKey.turn && e.SegmentNumber < bKey.segment) {
			filtered = append(filtered, e)
		}
	}
	var out []fantasy.Message
	if len(filtered) > 0 {
		selected := selectNotebookEntries(filtered, refs, floor)
		if rendered := notebook.RenderEntries(selected); rendered != "" {
			// Turns whose every entry was budget-evicted have entries
			// but nothing rendered. Emit a breadcrumb so the omission
			// isn't silent — the raw window intentionally does not
			// extend to them.
			selectedTurns := make(map[int64]bool, len(selected))
			for _, e := range selected {
				selectedTurns[e.TurnNumber] = true
			}
			var omitted []int64
			for _, e := range filtered {
				if !selectedTurns[e.TurnNumber] && !slices.Contains(omitted, e.TurnNumber) {
					omitted = append(omitted, e.TurnNumber)
				}
			}
			if len(omitted) > 0 {
				slices.Sort(omitted)
				rendered += "\n\n[turns " + formatTurnRanges(omitted) + " have notebook entries not injected here — recallable via recall/notebook_search]"
			}
			msg := fantasy.NewSystemMessage("<notebook>\n" + rendered + "</notebook>")
			out = append(out, msg)
		}
	}
	if a.notebookAutoInject && len(msgs) > 0 {
		if injectMsg := a.maybeAutoInject(ctx, msgs, sessionID, bKey); injectMsg != nil {
			out = append(out, *injectMsg)
		}
	}
	return out
}
