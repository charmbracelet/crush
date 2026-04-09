package notification

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
)

// notifySeq is an atomic counter for generating unique notification IDs.
var notifySeq atomic.Uint64

// OSCBackend sends desktop notifications using the OSC 99 (kitty) desktop
// notification protocol.
type OSCBackend struct {
	icon []byte
}

// NewOSCBackend creates a new OSC notification backend.
func NewOSCBackend(icon any) *OSCBackend {
	b := &OSCBackend{}
	if data, ok := icon.([]byte); ok && len(data) > 0 {
		b.icon = data
	}
	return b
}

// Send returns a [tea.Raw] command that writes OSC 99 escape sequences to
// the terminal.
func (b *OSCBackend) Send(n Notification) tea.Cmd {
	slog.Debug("Sending OSC notification", "title", n.Title, "message", n.Message)

	var sb strings.Builder
	id := fmt.Sprintf("crush-%d", notifySeq.Add(1))

	sb.WriteString(osc99(n.Title, "i="+id, "d=0", "p=title"))

	if n.Message != "" {
		sb.WriteString(osc99(n.Message, "i="+id, "d=0", "p=body"))
	}

	if len(b.icon) > 0 {
		encoded := base64.StdEncoding.EncodeToString(b.icon)
		sb.WriteString(osc99(encoded, "i="+id, "d=0", "p=icon", "e=1"))
	}

	sb.WriteString(osc99("", "i="+id, "d=1"))

	return tea.Raw(sb.String())
}

// osc99 builds a single OSC 99 escape sequence.
func osc99(payload string, metadata ...string) string {
	return fmt.Sprintf("\x1b]99;%s;%s\x07", strings.Join(metadata, ":"), payload)
}
