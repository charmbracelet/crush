package message

import "time"

const (
	defaultStreamCheckpointInterval = 25 * time.Millisecond
	defaultStreamCheckpointBytes    = 32 * 1024
)

// StreamAccumulator buffers provider deltas outside the copyable Message
// value. It materializes buffered content only at UI checkpoints or structural
// boundaries, avoiding a whole-string copy for every provider token.
type StreamAccumulator struct {
	message *Message

	text      []byte
	reasoning []byte
	toolArgs  map[string][]byte

	lastCheckpoint time.Time
	interval       time.Duration
	byteThreshold  int
}

// NewStreamAccumulator creates an accumulator owned by one active assistant
// step. Accumulators must not be shared between concurrent runs.
func NewStreamAccumulator(message *Message) *StreamAccumulator {
	return &StreamAccumulator{
		message:        message,
		toolArgs:       make(map[string][]byte),
		lastCheckpoint: time.Now(),
		interval:       defaultStreamCheckpointInterval,
		byteThreshold:  defaultStreamCheckpointBytes,
	}
}

// AppendText buffers an assistant text delta.
func (s *StreamAccumulator) AppendText(delta string) {
	s.text = append(s.text, delta...)
}

// AppendReasoning buffers a reasoning delta.
func (s *StreamAccumulator) AppendReasoning(delta string) {
	s.reasoning = append(s.reasoning, delta...)
}

// AppendToolArguments buffers streamed arguments for a tool call.
func (s *StreamAccumulator) AppendToolArguments(id, delta string) {
	s.toolArgs[id] = append(s.toolArgs[id], delta...)
}

// ToolArguments returns the currently buffered tool arguments for id.
func (s *StreamAccumulator) ToolArguments(id string) string {
	return string(s.toolArgs[id])
}

// Empty reports whether no text or reasoning delta is waiting to be applied.
func (s *StreamAccumulator) Empty() bool {
	return len(s.text) == 0 && len(s.reasoning) == 0
}

// CheckpointIfDue materializes buffered deltas when enough time or data has
// accumulated. It reports whether the message changed.
func (s *StreamAccumulator) CheckpointIfDue(now time.Time) bool {
	if s.Empty() {
		return false
	}
	if len(s.text)+len(s.reasoning) < s.byteThreshold && now.Sub(s.lastCheckpoint) < s.interval {
		return false
	}
	return s.Checkpoint(now)
}

// Checkpoint materializes all buffered text and reasoning into the message.
func (s *StreamAccumulator) Checkpoint(now time.Time) bool {
	if s.Empty() {
		return false
	}
	if len(s.reasoning) > 0 {
		s.message.AppendReasoningContent(string(s.reasoning))
		s.reasoning = s.reasoning[:0]
	}
	if len(s.text) > 0 {
		s.message.AppendContent(string(s.text))
		s.text = s.text[:0]
	}
	s.lastCheckpoint = now
	return true
}

// Reset drops uncommitted provider deltas, for example before a retry.
func (s *StreamAccumulator) Reset() {
	s.text = s.text[:0]
	s.reasoning = s.reasoning[:0]
	clear(s.toolArgs)
	s.lastCheckpoint = time.Now()
}
