package notification

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
)

const osc99QueryID = "crush-osc99-query"

// notifySeq is a counter for generating unique notification IDs.
var notifySeq uint64

// DetectOSC99Support parses an OSC response sequence and returns true if it
// indicates OSC 99 notification support. This function should be called from
// the capabilities detection layer to determine terminal support.
func DetectOSC99Support(seq string) bool {
	var ok bool

	p := ansi.NewParser()
	p.SetHandler(ansi.Handler{
		HandleOsc: func(cmd int, data []byte) {
			if cmd != 99 {
				return
			}

			response := strings.TrimPrefix(string(data), "99;")
			metadata, payload, found := strings.Cut(response, ";")
			if !found {
				return
			}

			var hasID, hasQuery bool
			for field := range strings.SplitSeq(metadata, ":") {
				hasID = hasID || field == "i="+osc99QueryID
				hasQuery = hasQuery || field == "p=?"
			}
			if !hasID || !hasQuery {
				return
			}

			ok = isOSC99CapacityPayload(payload)
		},
	})

	for i := 0; i < len(seq); i++ {
		p.Advance(seq[i])
	}

	return ok
}

func isOSC99CapacityPayload(payload string) bool {
	for field := range strings.SplitSeq(payload, ":") {
		key, value, found := strings.Cut(field, "=")
		if !found || key != "p" {
			continue
		}

		for item := range strings.SplitSeq(value, ",") {
			if item == "title" {
				return true
			}
		}
	}

	return false
}

// OSC99QuerySequence returns the OSC 99 query sequence used to detect
// terminal support. This should be sent during capability detection.
func OSC99QuerySequence() string {
	return ansi.DesktopNotification("", "i="+osc99QueryID, "p=?")
}

// OSC99Backend sends desktop notifications using OSC 99.
type OSC99Backend struct {
	icon []byte
}

// NewOSC99Backend creates a new OSC 99 notification backend.
func NewOSC99Backend(icon any) *OSC99Backend {
	b := &OSC99Backend{}
	if data, ok := icon.([]byte); ok && len(data) > 0 {
		b.icon = data
	}
	return b
}

// Send returns a [tea.Raw] command that writes OSC 99 escape sequences to the
// terminal.
func (b *OSC99Backend) Send(n Notification) tea.Cmd {
	slog.Debug("Sending OSC 99 notification", "title", n.Title, "message", n.Message)

	var sb strings.Builder
	notifySeq++
	id := fmt.Sprintf("crush-%d", notifySeq)

	appName := "Crush"
	notificationType := "crush-notification"

	sb.WriteString(ansi.DesktopNotification(n.Title, "i="+id, "d=0", "p=title", "a="+appName, "t="+notificationType))
	if n.Message != "" {
		sb.WriteString(ansi.DesktopNotification(n.Message, "i="+id, "d=0", "p=body", "a="+appName, "t="+notificationType))
	}

	if len(b.icon) > 0 {
		encoded := base64.StdEncoding.EncodeToString(b.icon)
		sb.WriteString(ansi.DesktopNotification(encoded, "i="+id, "d=0", "p=icon", "e=1", "a="+appName, "t="+notificationType))
	}

	sb.WriteString(ansi.DesktopNotification("", "i="+id, "d=1", "a="+appName, "t="+notificationType))

	return tea.Raw(sb.String())
}

// OSC777Backend sends desktop notifications using OSC 777.
type OSC777Backend struct{}

// NewOSC777Backend creates a new OSC 777 notification backend.
func NewOSC777Backend() *OSC777Backend {
	return &OSC777Backend{}
}

// Send returns a [tea.Raw] command that writes an OSC 777 escape sequence to
// the terminal.
func (b *OSC777Backend) Send(n Notification) tea.Cmd {
	slog.Debug("Sending OSC 777 notification", "title", n.Title, "message", n.Message)

	return tea.Raw(ansi.URxvtExt("notify", n.Title, n.Message))
}
