package notification

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/x/ansi"

	tea "charm.land/bubbletea/v2"
)

// notifySeq is an atomic counter for generating unique notification IDs.
var notifySeq atomic.Uint64

// OSCBackend sends desktop notifications using multiple OSC protocols to
// maximize terminal compatibility. It emits OSC 99 and OSC 777 in a
// single write.
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

// Send returns a [tea.Raw] command that writes OSC escape sequences to the
// terminal. It emits two protocols:
//   - OSC 99: title, body, icon.
//   - OSC 777: title, body.
func (b *OSCBackend) Send(n Notification) tea.Cmd {
	slog.Debug("Sending OSC notification", "title", n.Title, "message", n.Message)

	var sb strings.Builder
	id := fmt.Sprintf("crush-%d", notifySeq.Add(1))

	// OSC 99
	sb.WriteString(ansi.DesktopNotification(n.Title, "i="+id, "d=0", "p=title"))

	if n.Message != "" {
		sb.WriteString(ansi.DesktopNotification(n.Message, "i="+id, "d=0", "p=body"))
	}

	if len(b.icon) > 0 {
		encoded := base64.StdEncoding.EncodeToString(b.icon)
		sb.WriteString(ansi.DesktopNotification(encoded, "i="+id, "d=0", "p=icon", "e=1"))
	}

	sb.WriteString(ansi.DesktopNotification("", "i="+id, "d=1"))

	// OSC 777
	sb.WriteString(ansi.URxvtExt("notify", n.Title, n.Message))

	return tea.Raw(sb.String())
}
