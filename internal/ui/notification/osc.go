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
	id := fmt.Sprintf("crush-%d", notifySeq.Add(1))

	app_name := "Crush"
	notification_type := "crush-notification"

	sb.WriteString(ansi.DesktopNotification(n.Title, "i="+id, "d=0", "p=title", "a="+app_name, "t="+notification_type))
	if n.Message != "" {
		sb.WriteString(ansi.DesktopNotification(n.Message, "i="+id, "d=0", "p=body", "a="+app_name, "t="+notification_type))
	}

	if len(b.icon) > 0 {
		encoded := base64.StdEncoding.EncodeToString(b.icon)
		sb.WriteString(ansi.DesktopNotification(encoded, "i="+id, "d=0", "p=icon", "e=1", "a="+app_name, "t="+notification_type))
	}

	sb.WriteString(ansi.DesktopNotification("", "i="+id, "d=1", "a="+app_name, "t="+notification_type))

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
