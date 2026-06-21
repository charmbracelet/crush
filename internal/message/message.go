package message

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// defaultUpdateDebounce is the default debounce window for [Service.Update].
// Streaming deltas that arrive within the window are coalesced into a
// single SQL write and a single pubsub event. Terminal updates
// (finish/error/cancel/tool-call structural changes) bypass the
// debounce and flush synchronously.
const defaultUpdateDebounce = 33 * time.Millisecond

type CreateMessageParams struct {
	Role             MessageRole
	Parts            []ContentPart
	Model            string
	Provider         string
	IsSummaryMessage bool
}

// Service is the public interface to the message store.
//
// [Service.Update] is eventually consistent: it accepts new state into
// an in-memory buffer and writes it to SQLite plus publishes a
// [pubsub.UpdatedEvent] on the next debounce tick (default
// [defaultUpdateDebounce]) or on the next terminal-state update,
// whichever comes first. Terminal-state updates — those that finish
// the message, add or finish a tool call, or end a reasoning section —
// flush synchronously before [Service.Update] returns.
//
// Callers that need stronger ordering (e.g. tests, shutdown,
// session-switch reads) must use [Service.Flush] or [Service.FlushAll]
// before reading via [Service.Get] / [Service.List]. Without an
// explicit flush, a read can race the debounce timer and miss the
// most recent in-memory state.
type Service interface {
	pubsub.Subscriber[Message]
	Create(ctx context.Context, sessionID string, params CreateMessageParams) (Message, error)
	Update(ctx context.Context, message Message) error
	Get(ctx context.Context, id string) (Message, error)
	List(ctx context.Context, sessionID string) ([]Message, error)
	ListUserMessages(ctx context.Context, sessionID string) ([]Message, error)
	ListAllUserMessages(ctx context.Context) ([]Message, error)
	Delete(ctx context.Context, id string) error
	DeleteSessionMessages(ctx context.Context, sessionID string) error

	// Flush synchronously drains any pending debounced state for the
	// given message ID, performs the SQL write, and publishes the
	// resulting [pubsub.UpdatedEvent]. Idempotent; cheap no-op if no
	// updates are pending. Use this before any read that must observe
	// the latest [Service.Update].
	Flush(ctx context.Context, id string) error

	// FlushAll synchronously drains pending debounced state for every
	// message known to the service. Intended for shutdown and
	// session-switch paths.
	FlushAll(ctx context.Context) error
}

// pendingState holds the in-memory coalescing buffer for a single
// message ID. All fields except where noted are guarded by
// service.mu. The flushing flag serializes concurrent flushers for
// the same ID so SQL writes never reorder.
type pendingState struct {
	// latest is the most recent [Message] passed to [Service.Update]
	// that has not yet been flushed.
	latest Message

	// dirty is true when latest contains state that has not been
	// written to SQL since the last successful flush.
	dirty bool

	// flushing is true while a goroutine is performing the SQL write
	// for this ID. New updates are still accepted (and re-mark dirty)
	// but other flushers must back off.
	flushing bool

	// timer is the active debounce timer, or nil if no flush is
	// scheduled. Stopped and reset when a terminal update preempts
	// the debounce window.
	timer *time.Timer

	// lastFlushed is the snapshot most recently written to SQL. Used
	// as the baseline for terminal-state detection.
	lastFlushed Message

	// hasFlushed is false until the first successful write for this
	// ID; until then lastFlushed is the zero value and must not be
	// treated as a real prior state.
	hasFlushed bool
}

type service struct {
	*pubsub.Broker[Message]
	q        db.Querier
	debounce time.Duration

	mu      sync.Mutex
	pending map[string]*pendingState
}

// ServiceOption configures a [Service] at construction.
type ServiceOption func(*service)

// WithDebounce overrides the debounce window for [Service.Update]. A
// zero or negative value disables debouncing entirely (every update
// flushes synchronously). Intended primarily for tests.
func WithDebounce(d time.Duration) ServiceOption {
	return func(s *service) {
		s.debounce = d
	}
}

func NewService(q db.Querier, opts ...ServiceOption) Service {
	s := &service{
		Broker:   pubsub.NewBroker[Message](),
		q:        q,
		debounce: defaultUpdateDebounce,
		pending:  make(map[string]*pendingState),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *service) Delete(ctx context.Context, id string) error {
	message, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	err = s.q.DeleteMessage(ctx, message.ID)
	if err != nil {
		return err
	}
	// Drop any pending coalesced state for this ID. We never want to
	// flush back over a deleted row.
	s.mu.Lock()
	if p, ok := s.pending[id]; ok {
		if p.timer != nil {
			p.timer.Stop()
		}
		delete(s.pending, id)
	}
	s.mu.Unlock()
	// Clone the message before publishing to avoid race conditions with
	// concurrent modifications to the Parts slice.
	s.Publish(pubsub.DeletedEvent, message.Clone())
	return nil
}

func (s *service) Create(ctx context.Context, sessionID string, params CreateMessageParams) (Message, error) {
	if params.Role != Assistant {
		params.Parts = append(params.Parts, Finish{
			Reason: "stop",
		})
	}
	partsJSON, err := marshalParts(params.Parts)
	if err != nil {
		return Message{}, err
	}
	isSummary := int64(0)
	if params.IsSummaryMessage {
		isSummary = 1
	}
	dbMessage, err := s.q.CreateMessage(ctx, db.CreateMessageParams{
		ID:               uuid.New().String(),
		SessionID:        sessionID,
		Role:             string(params.Role),
		Parts:            string(partsJSON),
		Model:            sql.NullString{String: string(params.Model), Valid: true},
		Provider:         sql.NullString{String: params.Provider, Valid: params.Provider != ""},
		IsSummaryMessage: isSummary,
	})
	if err != nil {
		return Message{}, err
	}
	message, err := s.fromDBItem(dbMessage)
	if err != nil {
		return Message{}, err
	}
	// Clone the message before publishing to avoid race conditions with
	// concurrent modifications to the Parts slice.
	s.Publish(pubsub.CreatedEvent, message.Clone())
	return message, nil
}

func (s *service) DeleteSessionMessages(ctx context.Context, sessionID string) error {
	messages, err := s.List(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if message.SessionID == sessionID {
			err = s.Delete(ctx, message.ID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Update accepts a new state for a message and either flushes
// synchronously (terminal updates, debounce <= 0) or buffers it until
// the next debounce tick. See [Service] for the contract.
func (s *service) Update(ctx context.Context, msg Message) error {
	cloned := msg.Clone()

	// Zero or negative debounce: flush every update synchronously. This
	// preserves the pre-coalescing behaviour for tests and any caller
	// that explicitly opted out via [WithDebounce].
	if s.debounce <= 0 {
		s.mu.Lock()
		p, ok := s.pending[msg.ID]
		if !ok {
			p = &pendingState{}
			s.pending[msg.ID] = p
		}
		p.latest = cloned
		p.dirty = true
		s.mu.Unlock()
		return s.flushOne(ctx, msg.ID, true)
	}

	s.mu.Lock()
	p, ok := s.pending[msg.ID]
	if !ok {
		p = &pendingState{}
		s.pending[msg.ID] = p
	}
	p.latest = cloned
	p.dirty = true

	var prev *Message
	if p.hasFlushed {
		prev = &p.lastFlushed
	}
	terminal := shouldFlushNow(prev, &cloned)

	if terminal {
		if p.timer != nil {
			p.timer.Stop()
			p.timer = nil
		}
		s.mu.Unlock()
		return s.flushOne(ctx, msg.ID, true)
	}

	// Debounce: schedule a single flush per pending state. If a flush
	// is already running we let it finish; the trailing dirty bit will
	// be picked up by the next Update or by Flush.
	if p.timer == nil && !p.flushing {
		id := msg.ID
		p.timer = time.AfterFunc(s.debounce, func() {
			// Detached from caller ctx so a cancelled stream context
			// does not strand the buffered write.
			_ = s.flushOne(context.Background(), id, false)
		})
	}
	s.mu.Unlock()
	return nil
}

// Flush implements [Service.Flush].
func (s *service) Flush(ctx context.Context, id string) error {
	return s.flushOne(ctx, id, true)
}

// FlushAll implements [Service.FlushAll]. It snapshots every ID with
// outstanding work — either dirty buffered state or a flush already in
// flight — then drains each one. Picking up in-flight IDs ensures
// FlushAll cannot return while a timer-fired write is still mid-SQL,
// which is what shutdown and session-switch callers rely on.
func (s *service) FlushAll(ctx context.Context) error {
	s.mu.Lock()
	ids := make([]string, 0, len(s.pending))
	for id, p := range s.pending {
		if p.dirty || p.flushing {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	var firstErr error
	for _, id := range ids {
		if err := s.flushOne(ctx, id, true); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// flushOne drains a single message ID. When syncCaller is true the
// caller is willing to wait through a concurrent in-flight flush so
// that, on return, lastFlushed equals latest at the moment of return.
// When false (timer-fired path) we bail if another flusher is already
// running; that flusher will pick up the trailing dirty bit.
//
// Order matters: a sync caller must wait for any in-flight flush to
// drain even when the buffer is currently clean — that in-flight
// write has not yet updated the SQL row, so returning early would
// violate the contract that on success lastFlushed reflects the most
// recent state.
func (s *service) flushOne(ctx context.Context, id string, syncCaller bool) error {
	for {
		s.mu.Lock()
		p, ok := s.pending[id]
		if !ok {
			s.mu.Unlock()
			return nil
		}
		if p.flushing {
			if !syncCaller {
				s.mu.Unlock()
				return nil
			}
			s.mu.Unlock()
			// Brief yield; in-flight write should land in <1ms typical.
			time.Sleep(time.Millisecond)
			continue
		}
		if !p.dirty {
			s.mu.Unlock()
			return nil
		}

		if p.timer != nil {
			p.timer.Stop()
			p.timer = nil
		}
		snap := p.latest
		// Decide whether this snapshot represents a terminal event
		// against the prior baseline. We must do this before resetting
		// dirty/flushing because shouldFlushNow looks at p.lastFlushed
		// (which is what was on disk before this write).
		var prev *Message
		if p.hasFlushed {
			prev = &p.lastFlushed
		}
		isTerminal := shouldFlushNow(prev, &snap)
		p.flushing = true
		p.dirty = false
		s.mu.Unlock()

		err := s.write(ctx, snap)

		s.mu.Lock()
		p.flushing = false
		if err == nil {
			p.lastFlushed = snap
			p.hasFlushed = true
		} else {
			// Restore dirty so the next caller retries.
			p.dirty = true
		}
		// If a delta arrived during the SQL write and we are a sync
		// caller, the user expects that delta to land too.
		wasDirty := p.dirty
		s.mu.Unlock()

		if err != nil {
			return err
		}

		// Terminal events — message finished, tool call added or
		// finished, reasoning ended — use the bounded must-deliver
		// path so they never get dropped under channel contention.
		if isTerminal {
			s.PublishMustDeliver(ctx, pubsub.UpdatedEvent, snap)
		} else {
			s.Publish(pubsub.UpdatedEvent, snap)
		}

		if wasDirty && syncCaller {
			continue
		}
		return nil
	}
}

// write performs the unguarded SQL write + UpdatedAt stamp. Caller
// owns publishing.
func (s *service) write(ctx context.Context, msg Message) error {
	parts, err := marshalParts(msg.Parts)
	if err != nil {
		return err
	}
	finishedAt := sql.NullInt64{}
	if f := msg.FinishPart(); f != nil {
		finishedAt.Int64 = f.Time
		finishedAt.Valid = true
	}
	if err := s.q.UpdateMessage(ctx, db.UpdateMessageParams{
		ID:         msg.ID,
		Parts:      string(parts),
		FinishedAt: finishedAt,
	}); err != nil {
		return err
	}
	return nil
}

// shouldFlushNow returns true when next represents a structural
// change that must not be silently coalesced: the message just
// finished, the tool-call set grew, a tool call transitioned to
// finished, or reasoning just finished. prev is the last-flushed
// snapshot (or nil if no write has landed yet).
func shouldFlushNow(prev, next *Message) bool {
	if next.IsFinished() {
		return true
	}

	var prevCalls []ToolCall
	var prevReasoningFinishedAt int64
	if prev != nil {
		prevCalls = prev.ToolCalls()
		prevReasoningFinishedAt = prev.ReasoningContent().FinishedAt
	}
	nextCalls := next.ToolCalls()
	if len(nextCalls) != len(prevCalls) {
		return true
	}
	for i := range nextCalls {
		// Bounds-safe: lengths are equal here.
		if nextCalls[i].Finished != prevCalls[i].Finished {
			return true
		}
		// A tool call's input only matters once it has landed (Finished
		// flips true). Earlier deltas to Input are debounced with the
		// rest of the streaming state.
	}
	if next.ReasoningContent().FinishedAt > 0 && prevReasoningFinishedAt == 0 {
		return true
	}
	return false
}

func (s *service) Get(ctx context.Context, id string) (Message, error) {
	dbMessage, err := s.q.GetMessage(ctx, id)
	if err != nil {
		return Message{}, err
	}
	return s.fromDBItem(dbMessage)
}

func (s *service) List(ctx context.Context, sessionID string) ([]Message, error) {
	dbMessages, err := s.q.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(dbMessages))
	for i, dbMessage := range dbMessages {
		messages[i], err = s.fromDBItem(dbMessage)
		if err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func (s *service) ListUserMessages(ctx context.Context, sessionID string) ([]Message, error) {
	dbMessages, err := s.q.ListUserMessagesBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(dbMessages))
	for i, dbMessage := range dbMessages {
		messages[i], err = s.fromDBItem(dbMessage)
		if err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func (s *service) ListAllUserMessages(ctx context.Context) ([]Message, error) {
	dbMessages, err := s.q.ListAllUserMessages(ctx)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, len(dbMessages))
	for i, dbMessage := range dbMessages {
		messages[i], err = s.fromDBItem(dbMessage)
		if err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func (s *service) fromDBItem(item db.Message) (Message, error) {
	parts, err := unmarshalParts([]byte(item.Parts))
	if err != nil {
		return Message{}, err
	}
	return Message{
		ID:               item.ID,
		SessionID:        item.SessionID,
		Role:             MessageRole(item.Role),
		Parts:            parts,
		Model:            item.Model.String,
		Provider:         item.Provider.String,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		IsSummaryMessage: item.IsSummaryMessage != 0,
	}, nil
}

type partType string

const (
	reasoningType  partType = "reasoning"
	textType       partType = "text"
	imageURLType   partType = "image_url"
	binaryType     partType = "binary"
	toolCallType   partType = "tool_call"
	toolResultType partType = "tool_result"
	finishType     partType = "finish"
)

type partWrapper struct {
	Type partType    `json:"type"`
	Data ContentPart `json:"data"`
}

func marshalParts(parts []ContentPart) ([]byte, error) {
	wrappedParts := make([]partWrapper, len(parts))

	for i, part := range parts {
		var typ partType

		switch part.(type) {
		case ReasoningContent:
			typ = reasoningType
		case TextContent:
			typ = textType
		case ImageURLContent:
			typ = imageURLType
		case BinaryContent:
			typ = binaryType
		case ToolCall:
			typ = toolCallType
		case ToolResult:
			typ = toolResultType
		case Finish:
			typ = finishType
		default:
			return nil, fmt.Errorf("unknown part type: %T", part)
		}

		wrappedParts[i] = partWrapper{
			Type: typ,
			Data: part,
		}
	}
	return json.Marshal(wrappedParts)
}

func unmarshalParts(data []byte) ([]ContentPart, error) {
	temp := []json.RawMessage{}

	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, err
	}

	parts := make([]ContentPart, 0)

	for _, rawPart := range temp {
		var wrapper struct {
			Type partType        `json:"type"`
			Data json.RawMessage `json:"data"`
		}

		if err := json.Unmarshal(rawPart, &wrapper); err != nil {
			return nil, err
		}

		switch wrapper.Type {
		case reasoningType:
			part := ReasoningContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case textType:
			part := TextContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case imageURLType:
			part := ImageURLContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case binaryType:
			part := BinaryContent{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case toolCallType:
			part := ToolCall{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case toolResultType:
			part := ToolResult{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		case finishType:
			part := Finish{}
			if err := json.Unmarshal(wrapper.Data, &part); err != nil {
				return nil, err
			}
			parts = append(parts, part)
		default:
			return nil, fmt.Errorf("unknown part type: %s", wrapper.Type)
		}
	}

	return parts, nil
}
