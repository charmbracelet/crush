package cmd

import (
	"os"
	"os/signal"
	"syscall"
)

// stopSignals trả các tín hiệu mà run/send coi là "dừng": Ctrl+C, SIGTERM,
// và SIGHUP (terminal đóng, SSH rớt) trừ khi tiến trình đang bỏ qua nó.
// Đăng ký SIGHUP khi nó đang bị bỏ qua (chạy dưới nohup) sẽ bật lại nó, và
// `nohup agent-cli run ... &` bị huỷ lúc đăng xuất.
func stopSignals() []os.Signal {
	sigs := []os.Signal{os.Interrupt, syscall.SIGTERM}
	if !signal.Ignored(syscall.SIGHUP) {
		sigs = append(sigs, syscall.SIGHUP)
	}
	return sigs
}

// withoutIgnoredHangup bỏ SIGHUP khỏi sigs khi tiến trình đang bỏ qua nó
// (vd `nohup agent-cli server &`): đăng ký sẽ bật lại nó và server tắt lúc
// đăng xuất.
func withoutIgnoredHangup(sigs []os.Signal) []os.Signal {
	if !signal.Ignored(syscall.SIGHUP) {
		return sigs
	}
	out := make([]os.Signal, 0, len(sigs))
	for _, s := range sigs {
		if s != syscall.SIGHUP {
			out = append(out, s)
		}
	}
	return out
}
