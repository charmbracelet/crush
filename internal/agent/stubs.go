package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
)

// stubMinContentBytes is the minimum result size worth stubbing — below
// it the stub text saves nothing.
const stubMinContentBytes = 200

// stubCommandMinBytes is the size floor for command-output stubs: their
// stub carries a head-prefix digest, so smaller results save nothing.
const stubCommandMinBytes = 512

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

// commandToolNames emit re-derivable output: once a result is old
// enough to leave the recency guard, or a re-run makes it redundant,
// a labeled stub suffices.
var commandToolNames = map[string]bool{"bash": true, "grep": true, "glob": true, "ls": true}

// volatileInputKeys are tool-call inputs that do not change what the
// tool runs — dropping them lets two invocations of the same command
// compare equal.
var volatileInputKeys = map[string][]string{
	"bash": {"description", "auto_background_after"},
}

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

// canonicalToolInput normalizes a tool-call input for identity
// comparison: keys are re-marshaled in sorted order, volatile keys are
// dropped, and zero values are removed so "absent" and "default" spell
// the same call. Returns "" for unparseable input.
func canonicalToolInput(name, input string) string {
	var fields map[string]any
	if err := json.Unmarshal([]byte(input), &fields); err != nil {
		return ""
	}
	for _, k := range volatileInputKeys[name] {
		delete(fields, k)
	}
	for k, v := range fields {
		switch t := v.(type) {
		case string:
			if t == "" {
				delete(fields, k)
			}
		case float64:
			if t == 0 {
				delete(fields, k)
			}
		case bool:
			if !t {
				delete(fields, k)
			}
		case nil:
			delete(fields, k)
		}
	}
	canon, err := json.Marshal(fields)
	if err != nil {
		return ""
	}
	return string(canon)
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

// flagPrunableToolResults marks tool results whose stored content no
// longer needs to replay verbatim. Four passes run in priority order —
// the first flag wins:
//
//  1. Write-tool supersession: a successful edit/write/multiedit
//     supersedes earlier reads of the same path.
//  2. Observed modification: a read whose file changed on disk since
//     the result was produced (or was deleted) — catches mutations no
//     tool name reveals: bash redirection, MCP writes, external edits,
//     and view→view re-reads of a changed file.
//  3. Command supersession: a later re-run of the same command tool
//     with identical input — identical output flags the earlier copies
//     as duplicates, differing output flags them as digests.
//  4. Age: remaining large command outputs flag stale and promote
//     once they fall past the recency guard.
//
// Flags are monotonic and metadata only: Content is never rewritten,
// so notebook generation and recall still see the original.
func (a *sessionAgent) flagPrunableToolResults(ctx context.Context, msgs []message.Message) {
	turns := messageTurns(msgs)
	currentTurn := int64(countUserMessages(msgs))

	// Index flaggable calls by ID so results can be traced back to
	// their inputs. Command tools keep a canonical input for re-run
	// comparison.
	type callInfo struct {
		name  string
		path  string
		input string
	}
	calls := make(map[string]callInfo)
	for _, m := range msgs {
		if m.Role != message.Assistant {
			continue
		}
		for _, tc := range m.ToolCalls() {
			if !tc.Finished {
				continue
			}
			switch {
			case readToolNames[tc.Name] || writeToolNames[tc.Name]:
				calls[tc.ID] = callInfo{name: tc.Name, path: toolCallFilePath(tc.Input), input: canonicalToolInput(tc.Name, tc.Input)}
			case commandToolNames[tc.Name]:
				calls[tc.ID] = callInfo{name: tc.Name, input: canonicalToolInput(tc.Name, tc.Input)}
			}
		}
	}
	if len(calls) == 0 {
		return
	}

	dirty := make(map[int]bool)
	mark := func(msgIdx, partIdx int, mk message.SupersededMark) {
		tr, ok := msgs[msgIdx].Parts[partIdx].(message.ToolResult)
		if !ok || tr.Superseded != nil {
			return
		}
		tr.Superseded = &mk
		msgs[msgIdx].Parts[partIdx] = tr
		dirty[msgIdx] = true
	}

	// Pass 1 — write-tool supersession. writesByPath indexes
	// successful writes by normalized path in message order, so the
	// earliest write following a read owns the flag.
	type writeEvent struct {
		msgIdx int
		turn   int64
		tool   string
	}
	writesByPath := make(map[string][]writeEvent)
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
	for i, m := range msgs {
		if m.Role != message.Tool || len(writesByPath) == 0 {
			continue
		}
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
			mark(i, j, message.SupersededMark{
				Path:   call.path,
				ByTool: best.tool,
				Turn:   best.turn,
			})
		}
	}

	// Pass 2 — observed modification. A read carrying a recorded
	// mtime whose file has since changed (or vanished) is stale
	// regardless of which tool mutated it. Results without a recorded
	// mtime are skipped: a failed stat can't distinguish a deleted
	// file from a read that never touched the filesystem.
	for i, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for j, part := range m.Parts {
			tr, ok := part.(message.ToolResult)
			if !ok || tr.Superseded != nil || tr.IsError ||
				len(tr.Content) < stubMinContentBytes || tr.Data != "" ||
				tr.FileMtime == 0 {
				continue
			}
			call, ok := calls[tr.ToolCallID]
			if !ok || !readToolNames[call.name] || call.path == "" {
				continue
			}
			fi, err := os.Stat(call.path)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				mark(i, j, message.SupersededMark{
					Path: call.path,
					Turn: currentTurn,
					Kind: message.StubKindDeleted,
				})
			case err == nil && fi.ModTime().UnixNano() != tr.FileMtime:
				mark(i, j, message.SupersededMark{
					Path: call.path,
					Turn: currentTurn,
					Kind: message.StubKindModified,
				})
			}
		}
	}

	// Pass 3 — command supersession. Re-runs group by (tool, canonical
	// input); against the latest output, identical earlier copies flag
	// as duplicates and differing ones as rerun digests. Reads join the
	// grouping too: an identical re-view of an unchanged file leaves
	// the earlier copy redundant. The latest of each group is exempt
	// from the age pass so a repeated call keeps one verbatim copy.
	keep := make(map[[2]int]bool)
	{
		type cmdOut struct {
			msgIdx  int
			partIdx int
			turn    int64
			content string
		}
		byRun := make(map[string][]cmdOut)
		for i, m := range msgs {
			if m.Role != message.Tool {
				continue
			}
			for j, part := range m.Parts {
				tr, ok := part.(message.ToolResult)
				if !ok || tr.IsError || tr.Data != "" {
					continue
				}
				call, ok := calls[tr.ToolCallID]
				if !ok || call.input == "" ||
					(!commandToolNames[call.name] && !readToolNames[call.name]) {
					continue
				}
				min := stubCommandMinBytes
				if readToolNames[call.name] {
					min = stubMinContentBytes
				}
				if len(tr.Content) < min {
					continue
				}
				key := call.name + "\x00" + call.input
				byRun[key] = append(byRun[key], cmdOut{msgIdx: i, partIdx: j, turn: turns[i], content: tr.Content})
			}
		}
		for _, outs := range byRun {
			if len(outs) < 2 {
				continue
			}
			last := outs[len(outs)-1]
			keep[[2]int{last.msgIdx, last.partIdx}] = true
			for _, o := range outs[:len(outs)-1] {
				kind := message.StubKindRerun
				if o.content == last.content {
					kind = message.StubKindDuplicate
				}
				mark(o.msgIdx, o.partIdx, message.SupersededMark{Turn: last.turn, Kind: kind})
			}
		}
	}

	// Pass 4 — age. Remaining large command outputs flag stale; the
	// recency guard at promotion time is what "old" means.
	for i, m := range msgs {
		if m.Role != message.Tool {
			continue
		}
		for j, part := range m.Parts {
			tr, ok := part.(message.ToolResult)
			if !ok || tr.Superseded != nil || tr.IsError || tr.Data != "" ||
				len(tr.Content) < stubCommandMinBytes || keep[[2]int{i, j}] {
				continue
			}
			call, ok := calls[tr.ToolCallID]
			if !ok || !commandToolNames[call.name] {
				continue
			}
			mark(i, j, message.SupersededMark{Turn: turns[i], Kind: message.StubKindStale})
		}
	}

	var updated []message.Message
	for i := range msgs {
		if dirty[i] {
			updated = append(updated, msgs[i])
		}
	}
	for i := range updated {
		// Merge stored marks first: this snapshot may predate a
		// concurrent promotion — without the union the whole-message
		// write would clobber Applied and flip the next render back
		// to verbatim.
		a.mergeSupersededMarks(ctx, &updated[i])
		if err := a.messages.Update(ctx, updated[i]); err != nil {
			slog.Warn("Failed to flag tool result", "session_id", updated[i].SessionID, "error", err)
		}
	}
	if len(updated) > 0 {
		slog.Debug("Flagged prunable tool results", "session_id", sessionIDFromMessages(msgs), "count", len(updated))
	}
}

// stampReadMtime records the file's modification time on a successful
// file-read result, anchoring pass-2 supersession to the state the
// read actually observed. Path resolution matches the file tools:
// relative paths land on the process working directory.
func stampReadMtime(tr *message.ToolResult, calls []message.ToolCall) {
	if tr.IsError || !readToolNames[tr.Name] {
		return
	}
	for _, tc := range calls {
		if tc.ID != tr.ToolCallID {
			continue
		}
		if p := toolCallFilePath(tc.Input); p != "" {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				tr.FileMtime = fi.ModTime().UnixNano()
			}
		}
		return
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
			msgSaved += int64(len(tr.Content)) - int64(len(tr.Superseded.StubText(tr)))
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
		stub := tr.Superseded.StubText(tr)
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
