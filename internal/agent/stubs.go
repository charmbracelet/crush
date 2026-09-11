package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

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

// sameFilePath reports whether two tool-call paths refer to the same
// file. Both are resolved the way the file tools resolve them —
// filepath.Abs anchors relative paths to the process working
// directory — so "internal/x.go" and "x.go" correctly compare as
// different files while "./a.go", "a.go", and its absolute spelling
// all match.
func sameFilePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return absA == absB
	}
	return filepath.Clean(a) == filepath.Clean(b)
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
		path   string
		msgIdx int
		turn   int64
		tool   string
	}
	var writes []writeEvent

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
			writes = append(writes, writeEvent{path: call.path, msgIdx: i, turn: turns[i], tool: call.name})
		}
	}
	if len(writes) == 0 {
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
			var best *writeEvent
			for k := range writes {
				w := &writes[k]
				if w.msgIdx <= i || !sameFilePath(w.path, call.path) {
					continue
				}
				if best == nil || w.msgIdx < best.msgIdx {
					best = w
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

	for _, m := range updated {
		if err := a.messages.Update(ctx, m); err != nil {
			slog.Warn("Failed to flag superseded tool result", "session_id", m.SessionID, "error", err)
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
		changed := false
		for j, part := range m.Parts {
			tr, ok := part.(message.ToolResult)
			if !ok || tr.Superseded == nil || tr.Superseded.Applied || tr.IsError {
				continue
			}
			saved += int64(len(tr.Content)) - int64(len(supersededStubText(*tr.Superseded, tr.ToolCallID)))
			tr.Superseded.Applied = true
			m.Parts[j] = tr
			changed = true
			promoted++
		}
		if changed {
			if err := a.messages.Update(ctx, *m); err != nil {
				persistFailed = true
				slog.Warn("Failed to persist superseded stub", "session_id", m.SessionID, "error", err)
			}
		}
	}
	if promoted > 0 {
		slog.Debug("Promoted superseded tool results to stubs",
			"session_id", sessionIDFromMessages(msgs), "count", promoted, "boundary", boundary)
	}
	if !persistFailed && promoted > 0 && a.stubStats != nil {
		sessionID := sessionIDFromMessages(msgs)
		stats, _ := a.stubStats.Get(sessionID)
		stats.Invalidations++
		stats.Results += promoted
		stats.SavedBytes += saved
		a.stubStats.Set(sessionID, stats)
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

// sessionIDFromMessages extracts the session ID shared by a message
// list, or "" for an empty list.
func sessionIDFromMessages(msgs []message.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	return msgs[0].SessionID
}
