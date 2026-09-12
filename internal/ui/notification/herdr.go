package notification

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"
)

// HerdrNotifier is the subset of the herdr client the HerdrBackend
// needs. Defined here so tests can substitute a fake.
type HerdrNotifier interface {
	Notify(title, body string)
}

// HerdrBackend surfaces Crush toasts in herdr's own UI via its
// notification.show socket API. It is only useful when Crush runs
// inside a herdr pane; backend selection falls back to OSC when it
// does not.
type HerdrBackend struct {
	client HerdrNotifier
}

// NewHerdrBackend creates a backend that reports toasts to herdr.
func NewHerdrBackend(client HerdrNotifier) *HerdrBackend {
	return &HerdrBackend{client: client}
}

// Send returns a [tea.Cmd] that asks herdr to show the toast. The
// herdr client enqueues the request on its socket writer without
// blocking, so the command completes immediately.
func (b *HerdrBackend) Send(n Notification) tea.Cmd {
	slog.Debug("Sending herdr notification", "title", n.Title, "message", n.Message)

	return func() tea.Msg {
		b.client.Notify(n.Title, n.Message)
		return nil
	}
}
