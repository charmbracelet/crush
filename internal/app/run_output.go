package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
)

// messageLister lists the stored messages of a session.
type messageLister interface {
	List(ctx context.Context, sessionID string) ([]message.Message, error)
}

// answerPrinter writes the assistant's replies in one session to the
// output of a non-interactive run. As a reply grows, only its new text is
// written.
type answerPrinter struct {
	out       io.Writer
	sessionID string
	// replies holds what has been seen of each reply, by message ID.
	replies map[string]replyState
	// printed is set once any text has been written.
	printed bool
}

// replyState is what has been seen of one reply.
type replyState struct {
	// content is the reply's text as it was last seen.
	content string
	// shown is set once some of content has been written, or when the
	// reply was stored before the run started.
	shown bool
}

// newAnswerPrinter returns a printer for a run in the given session. In a
// continued session, the replies already stored count as printed, so they
// are not repeated. A new session has no stored replies, so the store is
// not read.
func newAnswerPrinter(ctx context.Context, out io.Writer, sessionID string, messages messageLister, continued bool) (*answerPrinter, error) {
	p := &answerPrinter{out: out, sessionID: sessionID, replies: make(map[string]replyState)}
	if !continued {
		return p, nil
	}
	existing, err := messages.List(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list session messages: %w", err)
	}
	for _, msg := range existing {
		if p.isReply(msg) {
			p.replies[msg.ID] = replyState{content: msg.Content().String(), shown: true}
		}
	}
	return p, nil
}

// isReply reports whether msg is an assistant message in the printer's
// session that has at least one part and is not a compaction summary. Its
// text may still be empty, as it is for a message that only calls tools.
func (p *answerPrinter) isReply(msg message.Message) bool {
	return msg.SessionID == p.sessionID &&
		msg.Role == message.Assistant &&
		!msg.IsSummaryMessage &&
		len(msg.Parts) > 0
}

// write prints the text that msg adds to what has been seen of the reply.
// A reply's text normally only grows. Text that is a prefix of what was
// seen is treated as a stale update and adds nothing. Text that no longer
// starts with what was seen has been replaced, which happens when a
// provider request is retried after part of the reply has streamed. The
// new text is printed in full, on a new line if the old text was shown.
func (p *answerPrinter) write(msg message.Message) {
	if !p.isReply(msg) {
		return
	}
	content := msg.Content().String()
	st := p.replies[msg.ID]

	part, ok := strings.CutPrefix(content, st.content)
	switch {
	case ok:
	case strings.HasPrefix(st.content, content):
		return
	default:
		if st.shown {
			fmt.Fprintln(p.out)
		}
		part, st = content, replyState{}
	}

	// Trim leading whitespace. Sometimes the LLM includes leading
	// formatting and indentation, which we don't want here. Until
	// something has been printed, leading newlines are trimmed too.
	switch {
	case !p.printed:
		part = strings.TrimLeft(part, " \t\r\n")
	case st.content == "":
		part = strings.TrimLeft(part, " \t")
	}
	// Ignore initial whitespace-only messages.
	if p.printed || strings.TrimSpace(part) != "" {
		p.printed = true
		st.shown = st.shown || part != ""
		fmt.Fprint(p.out, part)
	}
	st.content = content
	p.replies[msg.ID] = st
}

// flush prints whatever the message events have not delivered. The agent
// stores every message before it reports that it is done, but the events
// carrying the last updates may still be queued at that point, so the
// rest is read from the store.
func (p *answerPrinter) flush(ctx context.Context, messages messageLister) {
	msgs, err := messages.List(ctx, p.sessionID)
	if err != nil {
		if ctx.Err() != nil {
			slog.Debug("Non-interactive: run ended before its output was finished", "session_id", p.sessionID, "error", err)
		} else {
			slog.Warn("Non-interactive: could not read the session's messages; the output may be incomplete", "session_id", p.sessionID, "error", err)
		}
		return
	}
	for _, msg := range msgs {
		p.write(msg)
	}
}

// printUntilDone prints the session's replies as their events arrive,
// until the agent sends its result on done. It then prints from the store
// whatever the events did not deliver. beforeWait runs before each wait,
// and stopSpinner runs when a reply arrives or the run ends.
func (p *answerPrinter) printUntilDone(
	ctx context.Context,
	events <-chan pubsub.Event[message.Message],
	done <-chan error,
	messages messageLister,
	beforeWait, stopSpinner func(),
) error {
	for {
		beforeWait()
		select {
		case err := <-done:
			stopSpinner()
			p.flush(ctx, messages)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, agent.ErrRequestCancelled) {
					slog.Debug("Non-interactive: agent processing cancelled", "session_id", p.sessionID)
					return nil
				}
				return fmt.Errorf("agent processing failed: %w", err)
			}
			return nil

		case event := <-events:
			if p.isReply(event.Payload) {
				stopSpinner()
			}
			p.write(event.Payload)

		case <-ctx.Done():
			stopSpinner()
			return ctx.Err()
		}
	}
}
