package model

import (
	"slices"
	"time"

	"github.com/charmbracelet/crush/internal/message"
)

// beginLiveTPS starts the live measurement for a newly created assistant
// step. A finished message (a replay or a late event) does not start one.
func (m *UI) beginLiveTPS(msg message.Message) {
	if msg.Role != message.Assistant || msg.IsSummaryMessage || msg.IsFinished() {
		return
	}
	if m.liveTPSMsgID == msg.ID {
		return
	}
	m.liveTPSMsgID = msg.ID
	// The decode clock starts with the first output token, not with the
	// message. Time to first token is request latency, not generation
	// speed, and counting it would understate the rate.
	m.liveTPSStart = time.Time{}
	m.liveTPSBytes = 0
	m.liveTPSTokens = 0
	// Keep the previous step's rate visible until this one has streamed
	// enough output to compute a fresh estimate.
}

// updateLiveTPS advances the live tokens-per-second estimate and folds the
// finished step into the session average. The rate is output tokens over the
// decode window (first token to last), so it measures generation speed
// rather than end-to-end request latency.
func (m *UI) updateLiveTPS(msg message.Message) {
	if msg.Role != message.Assistant || msg.IsSummaryMessage {
		return
	}
	if msg.ID != m.liveTPSMsgID {
		m.beginLiveTPS(msg)
		if m.liveTPSMsgID == "" {
			return
		}
	}

	bytes := assistantOutputBytes(msg)
	if bytes > m.liveTPSBytes {
		delta := bytes - m.liveTPSBytes
		m.liveTPSTokens += int64((delta + 3) / 4)
		m.liveTPSBytes = bytes
		if m.liveTPSStart.IsZero() {
			m.liveTPSStart = time.Now()
		}
	}
	if !m.liveTPSStart.IsZero() {
		if elapsed := time.Since(m.liveTPSStart).Seconds(); elapsed >= 0.25 {
			m.liveTPS = float64(m.liveTPSTokens) / elapsed
		}
	}

	if msg.IsFinished() {
		if m.liveTPSTokens > 0 {
			m.sessionTPSOutput += m.liveTPSTokens
			if decode := decodeDuration(msg); decode > 0 {
				m.sessionTPSDuration += decode
			} else if !m.liveTPSStart.IsZero() {
				m.sessionTPSDuration += time.Since(m.liveTPSStart)
			}
		}
		// Keep the last turn's rate in the sidebar while idle. The next
		// step overwrites it once it has a live estimate.
		m.liveTPSMsgID = ""
	}
}

// resetTPS clears the live rate and the session average. It runs when the
// active model changes so the average reflects only the model in use.
func (m *UI) resetTPS() {
	m.liveTPS = 0
	m.liveTPSMsgID = ""
	m.liveTPSStart = time.Time{}
	m.liveTPSBytes = 0
	m.liveTPSTokens = 0
	m.sessionTPSOutput = 0
	m.sessionTPSDuration = 0
}

// resetSessionTPS seeds the session average from the transcript so the
// sidebar has a number immediately after a session load or switch. Only
// steps from the session's active model are counted so the average does
// not mix models the transcript went through.
func (m *UI) resetSessionTPS(msgs []message.Message) {
	m.resetTPS()
	provider, model := activeModel(msgs)
	for _, msg := range msgs {
		if msg.Role != message.Assistant || msg.IsSummaryMessage || !msg.IsFinished() {
			continue
		}
		if provider != "" && model != "" && (msg.Provider != provider || msg.Model != model) {
			continue
		}
		bytes := assistantOutputBytes(msg)
		if bytes == 0 {
			continue
		}
		decode := decodeDuration(msg)
		if decode <= 0 {
			// Steps recorded before decode timing was measured fall back
			// to the wall clock, which includes time to first token.
			if msg.UpdatedAt <= msg.CreatedAt {
				continue
			}
			decode = time.Duration(max(msg.UpdatedAt-msg.CreatedAt, 1)) * time.Second
		}
		m.sessionTPSOutput += int64((bytes + 3) / 4)
		m.sessionTPSDuration += decode
	}
}

// decodeDuration is the generation window the agent measured for a finished
// step: first output token to finish. It is zero for steps that streamed no
// token or predate the measurement.
func decodeDuration(msg message.Message) time.Duration {
	finish := msg.FinishPart()
	if finish == nil || finish.DecodeMs <= 0 {
		return 0
	}
	return time.Duration(finish.DecodeMs) * time.Millisecond
}

// activeModel returns the provider and model of the last finished,
// non-summary assistant step, which is the model a session load restores.
// Both are empty when the transcript predates model tracking.
func activeModel(msgs []message.Message) (provider, model string) {
	for _, msg := range slices.Backward(msgs) {
		if msg.Role != message.Assistant || msg.IsSummaryMessage || !msg.IsFinished() {
			continue
		}
		return msg.Provider, msg.Model
	}
	return "", ""
}

// tpsStats returns the live and session-average tokens per second.
func (m *UI) tpsStats() (live, average float64) {
	if m.sessionTPSDuration > 0 {
		average = float64(m.sessionTPSOutput) / m.sessionTPSDuration.Seconds()
	}
	return m.liveTPS, average
}

func assistantOutputBytes(msg message.Message) int {
	return len(msg.Content().Text) + len(msg.ReasoningContent().Thinking)
}
