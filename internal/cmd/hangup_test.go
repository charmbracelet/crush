package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// Dưới nohup (SIGHUP đang bị bỏ qua) run/send không đăng ký SIGHUP, kẻo bật
// lại nó và bị huỷ lúc đăng xuất.
func TestStopSignalsRespectNohup(t *testing.T) {
	// Không parallel: đổi trạng thái tín hiệu của cả tiến trình.
	require.Contains(t, stopSignals(), syscall.SIGHUP)
	signal.Ignore(syscall.SIGHUP)
	t.Cleanup(func() { signal.Reset(syscall.SIGHUP) })
	require.NotContains(t, stopSignals(), syscall.SIGHUP)
	require.Equal(t, []os.Signal{os.Interrupt}, withoutIgnoredHangup([]os.Signal{os.Interrupt, syscall.SIGHUP}),
		"the server does not re-enable an ignored SIGHUP either")
}
