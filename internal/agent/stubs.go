package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

// stubMinContentBytes is the minimum result size worth stubbing — below
// it the stub text saves nothing.
const stubMinContentBytes = 200

// stubRecentTurnGuard is the recency guard for stub promotion: results
// from the last two completed turns are never stubbed, matching the
// recency rule in selectNotebookEntries. Turn-based, not count-based:
// a bash-spam turn must not push a still-active view past the guard.
const stubRecentTurnGuard = 2

// writeToolNames mutate files; a successful result supersedes earlier
// reads of the same path.
var writeToolNames = map[string]bool{"edit": true, "write": true, "multiedit": true}

// readToolNames capture file content; their results go stale on writes.
var readToolNames = map[string]bool{"view": true, "read": true}

// stubReport carries per-render stubbing telemetry: how many tool
// results rendered as stubs and how many original bytes they replaced.
type stubReport struct {
	results    int
	savedBytes int64
}

// stubStats accumulates per-session stubbing telemetry. Keyed by
// session ID on the agent so concurrent sessions sharing an agent do
// not bleed each other's numbers.
type stubStats struct {
	// Invalidations counts promotion events — each one changes the
	// prompt prefix, i.e. one cache invalidation.
	Invalidations int
	// Results is the cumulative count of results stubbed across
	// renders.
	Results int
	// SavedBytes is the cumulative original content replaced by stubs.
	SavedBytes int64
}

// messageTurns returns the turn index of each message: the number of
// user messages strictly before it. Turn numbering matches
// countUserMessages — the first user message starts turn 0.
func messageTurns(msgs []message.Message) []int64 {
	turns := make([]int64, len(msgs))
	var turn int64
	for i, m := range msgs {
		if i > 0 && m.Role == message.User {
			turn++
		}
		turns[i] = turn
	}
	return turns
}

// toolCallFilePath extracts the file path from a tool call's JSON
// input, trying the conventional keys.
func toolCallFilePath(input string) string {
	var fields map[string]any
	if err := json.Unmarshal([]byte(input), &fields); err != nil {
		return ""
	}
	for _, key := range []string{"file_path", "path", "file"} {
		if s, ok := fields[key].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// normalizedPath resolves a tool-call path for comparison: filepath.Abs
// anchors relative paths to the process working directory (the file
// tools resolve against the configured working dir — equal in
// practice, differing only when WorkingDir != process CWD *and* the
// call spells the path absolutely). "internal/x.go" and "x.go" thus
// correctly compare as different files while "./a.go", "a.go", and its
// absolute spelling all match. On case-insensitive filesystems
// (darwin, windows) the key is case-folded so "Foo.go" and "foo.go"
// alias — a deliberate false-positive risk on case-sensitive APFS
// volumes, where the cost is a recoverable extra supersession.
func normalizedPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		abs = strings.ToLower(abs)
	}
	return abs
}

// flagSupersededViewResults marks prior file-read tool results as
// superseded when a successful edit/write/multiedit touched the same
// file. The flag is monotonic — once set it is never cleared — and
// metadata only: the original ToolResult.Content stays in the DB so
// notebook generation and recall still see it. Results are only
// flagged when the write's result follows the read's result in message
// order, so a same-turn view→edit pair is flagged but a re-read after
// the edit is not.
func (a *sessionAgent) flagSupersededViewResults(ctx context.Context, msgs []message.Message) {
	turns := messageTurns(msgs)

	type writeEvent struct {
		msgIdx int
		turn   int64
		tool   string
	}
	// writesByPath indexes successful writes by normalized path, each
	// bucket in message order — O(W) to build instead of scanning all
	// writes per read.
	writesByPath := make(map[string][]writeEvent)

	// Index read/write calls by ID so results can be traced back to
	// their inputs.
	type callInfo struct {
		name   string
		path   string
		msgIdx int
	}
	calls := make(map[string]callInfo)
	for i, m := range msgs {
		if m.Role != message.Assistant {
			continue
		}
		for _, tc := range m.ToolCalls() {
			if !tc.Finished {
				continue
			}
			if !readToolNames[tc.Name] && !writeToolNames[tc.Name] {
				continue
			}
			calls[tc.ID] = callInfo{name: tc.Name, path: toolCallFilePath(tc.Input), msgIdx: i}
		}
	}

	// Collect successful writes in message order.
	for i, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for _, tr := range m.ToolResults() {
			call, ok := calls[tr.ToolCallID]
			if !ok || !writeToolNames[call.name] || call.path == "" || tr.IsError {
				continue
			}
			key := normalizedPath(call.path)
			writesByPath[key] = append(writesByPath[key], writeEvent{msgIdx: i, turn: turns[i], tool: call.name})
		}
	}
	if len(writesByPath) == 0 {
		return
	}

	// Flag each read result whose message precedes a successful write
	// to the same file. The earliest such write owns the flag.
	var updated []message.Message
	for i, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		changed := false
		for j, part := range m.Parts {
			tr, ok := part.(message.ToolResult)
			if !ok || tr.Superseded != nil || tr.IsError ||
				len(tr.Content) < stubMinContentBytes || tr.Data != "" {
				continue
			}
			call, ok := calls[tr.ToolCallID]
			if !ok || !readToolNames[call.name] || call.path == "" {
				continue
			}
			// Writes to the same file that follow this read's message.
			// The bucket is in message order, so the first later one
			// owns the flag.
			var best *writeEvent
			pathWrites := writesByPath[normalizedPath(call.path)]
			for k := range pathWrites {
				if pathWrites[k].msgIdx > i {
					best = &pathWrites[k]
					break
				}
			}
			if best == nil {
				continue
			}
			tr.Superseded = &message.SupersededMark{
				Path:   call.path,
				ByTool: best.tool,
				Turn:   best.turn,
			}
			m.Parts[j] = tr
			changed = true
		}
		if changed {
			updated = append(updated, m)
		}
	}

	for i := range updated {
		// Merge stored marks first: this snapshot may predate a
		// concurrent promotion — without the union the whole-message
		// write would clobber Applied and flip the next render back
		// to verbatim.
		a.mergeSupersededMarks(ctx, &updated[i])
		if err := a.messages.Update(ctx, updated[i]); err != nil {
			slog.Warn("Failed to flag superseded tool result", "session_id", updated[i].SessionID, "error", err)
		}
	}
	if len(updated) > 0 {
		slog.Debug("Flagged superseded tool results", "session_id", sessionIDFromMessages(msgs), "count", len(updated))
	}
}

// promoteSupersededStubs applies pending superseded flags on tool
// results inside the raw window. Callers invoke it only when the
// raw/notebook boundary moved since the last render — the move already
// invalidates the prompt-cache prefix, so stubbing piggybacks on it
// rather than paying an invalidation mid-window. Results in the last
// two completed turns are left pending.
//
// Returns false when any persistence write failed; callers should then
// leave the recorded boundary alone so promotion retries next render
// instead of flip-flopping between stubbed and verbatim renders.
func (a *sessionAgent) promoteSupersededStubs(ctx context.Context, msgs []message.Message, boundary int) bool {
	currentTurn := int64(countUserMessages(msgs))
	turns := messageTurns(msgs)
	promoted := 0
	var saved int64
	persistFailed := false
	for i := boundary; i < len(msgs); i++ {
		m := &msgs[i]
		if m.Role != message.Tool || turns[i] >= currentTurn-stubRecentTurnGuard {
			continue
		}
		var flipped []*message.SupersededMark
		var msgSaved int64
		for j, part := range m.Parts {
			tr, ok := part.(message.ToolResult)
			if !ok || tr.Superseded == nil || tr.Superseded.Applied || tr.IsError {
				continue
			}
			msgSaved += int64(len(tr.Content)) - int64(len(supersededStubText(*tr.Superseded, tr.ToolCallID)))
			tr.Superseded.Applied = true
			m.Parts[j] = tr
			flipped = append(flipped, tr.Superseded)
		}
		if len(flipped) == 0 {
			continue
		}
		a.mergeSupersededMarks(ctx, m)
		if err := a.messages.Update(ctx, *m); err != nil {
			persistFailed = true
			// Revert the in-memory marks so this render matches the
			// DB — otherwise this render stubs while the next flips
			// back to verbatim, flip-flopping the prompt prefix.
			for _, mark := range flipped {
				mark.Applied = false
			}
			slog.Warn("Failed to persist superseded stub", "session_id", m.SessionID, "error", err)
			continue
		}
		promoted += len(flipped)
		saved += msgSaved
	}
	if promoted > 0 {
		slog.Debug("Promoted superseded tool results to stubs",
			"session_id", sessionIDFromMessages(msgs), "count", promoted, "boundary", boundary)
	}
	if !persistFailed && promoted > 0 && a.stubStats != nil {
		// Guard the empty session ID: a sessionless list would leak a
		// "" key the deletion watcher can never clean.
		if sessionID := sessionIDFromMessages(msgs); sessionID != "" {
			stats, _ := a.stubStats.Get(sessionID)
			stats.Invalidations++
			stats.Results += promoted
			stats.SavedBytes += saved
			a.stubStats.Set(sessionID, stats)
		}
	}
	return !persistFailed
}

// supersededStubText renders the placeholder that replaces a stubbed
// tool result. It names the file, the write that superseded it, and the
// recall escape hatch for the original content.
func supersededStubText(mark message.SupersededMark, toolCallID string) string {
	return fmt.Sprintf("[content of %s superseded by %s at turn %d; re-view for current state, or recall(\"result:%s\") for the pre-edit snapshot]",
		mark.Path, mark.ByTool, mark.Turn, toolCallID)
}

// applySupersededStubs returns the message with applied superseded
// results replaced by stub text, plus how many results were stubbed
// and how many original content bytes that removed. The input message
// is left untouched; a clone is made lazily on the first stubbed part.
func applySupersededStubs(m message.Message) (stubbed message.Message, count int, saved int64) {
	cloned := false
	for i, part := range m.Parts {
		tr, ok := part.(message.ToolResult)
		if !ok || tr.Superseded == nil || !tr.Superseded.Applied || tr.IsError {
			continue
		}
		if !cloned {
			m = m.Clone()
			cloned = true
		}
		stub := supersededStubText(*tr.Superseded, tr.ToolCallID)
		saved += int64(len(tr.Content)) - int64(len(stub))
		tr.Content = stub
		m.Parts[i] = tr
		count++
	}
	return m, count, saved
}

// mergeSupersededMarks unions superseded marks from the stored version
// of m into m's parts before a whole-message Update. The flag and
// promote paths both rewrite whole messages from independent snapshots,
// so a stale copy must not lose marks the other path persisted —
// Superseded and Applied are monotonic and merge by union.
func (a *sessionAgent) mergeSupersededMarks(ctx context.Context, m *message.Message) {
	stored, err := a.messages.Get(ctx, m.ID)
	if err != nil {
		return
	}
	storedResults := make(map[string]message.ToolResult)
	for _, tr := range stored.ToolResults() {
		storedResults[tr.ToolCallID] = tr
	}
	for j, part := range m.Parts {
		tr, ok := part.(message.ToolResult)
		if !ok {
			continue
		}
		s, ok := storedResults[tr.ToolCallID]
		if !ok || s.Superseded == nil {
			continue
		}
		if tr.Superseded == nil {
			tr.Superseded = s.Superseded
		} else {
			tr.Superseded.Applied = tr.Superseded.Applied || s.Superseded.Applied
		}
		m.Parts[j] = tr
	}
}

// sessionIDFromMessages extracts the session ID shared by a message
// list, or "" for an empty list.
func sessionIDFromMessages(msgs []message.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	return msgs[0].SessionID
}
