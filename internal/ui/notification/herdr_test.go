package notification_test

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/notification"
	"github.com/stretchr/testify/require"
)

type fakeHerdrNotifier struct {
	calls []struct{ title, body string }
}

func (f *fakeHerdrNotifier) Notify(title, body string) {
	f.calls = append(f.calls, struct{ title, body string }{title, body})
}

func TestHerdrBackend_Send(t *testing.T) {
	t.Parallel()
	fake := &fakeHerdrNotifier{}
	backend := notification.NewHerdrBackend(fake)

	cmd := backend.Send(notification.Notification{Title: "Done", Message: "turn complete"})
	require.NotNil(t, cmd)
	cmd()

	require.Len(t, fake.calls, 1)
	require.Equal(t, "Done", fake.calls[0].title)
	require.Equal(t, "turn complete", fake.calls[0].body)
}
