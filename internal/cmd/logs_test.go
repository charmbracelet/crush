package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/nxadm/tail"
	"github.com/stretchr/testify/require"
)

func TestLogFilePathLocal(t *testing.T) {
	t.Parallel()

	got := logFilePath("/proj/.crush", false, "unix:///ignored.sock")
	require.Equal(t, filepath.Join("/proj/.crush", "logs", "crush.log"), got)
}

func TestLogFilePathClientServer(t *testing.T) {
	t.Parallel()

	a := logFilePath("/proj/.crush", true, "unix:///run/a.sock")
	b := logFilePath("/proj/.crush", true, "unix:///run/b.sock")
	require.NotEqual(t, a, b)
	require.Equal(t, "crush.log", filepath.Base(a))
	require.Equal(t, config.GlobalCacheDir(), filepath.Dir(filepath.Dir(a)))

	// Must match the path the server itself writes to.
	u, err := server.ParseHostURL("unix:///run/a.sock")
	require.NoError(t, err)
	require.Equal(t, serverLogPath(u), a)
	require.Contains(t, a, "server-"+safeHostName(u))
}

// forwardLogLines phải thoát khi tail đóng kênh. Trước bản sửa, nhận từ kênh đã
// đóng cho line là nil và `line.Err` panic (logs -f sập khi tail tự dừng).
func TestForwardLogLines_ClosedChannelReturnsTailError(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]error{
		"tail stopped cleanly": nil,
		"tail failed":          errors.New("Unable to open file x: permission denied"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lines := make(chan *tail.Line)
			close(lines)

			done := make(chan error, 1)
			go func() {
				done <- forwardLogLines(t.Context(), lines, func() error { return want }, func(string) {
					t.Error("nothing should be emitted from a closed channel")
				})
			}()
			select {
			case err := <-done:
				require.Equal(t, want, err)
			case <-time.After(5 * time.Second):
				t.Fatal("forwardLogLines did not return after the channel was closed")
			}
		})
	}
}

// Dòng thường được chuyển theo thứ tự, dòng lỗi của tail bị bỏ qua, kênh đóng
// sau khi đã giao hết dòng thì trả lỗi của tail.
func TestForwardLogLines_EmitsLinesThenStopsOnClose(t *testing.T) {
	t.Parallel()

	lines := make(chan *tail.Line, 4)
	lines <- &tail.Line{Text: "one"}
	lines <- &tail.Line{Err: errors.New("bad line")}
	lines <- &tail.Line{Text: "two"}
	close(lines)

	var got []string
	tailErr := errors.New("tail gave up")
	err := forwardLogLines(t.Context(), lines, func() error { return tailErr }, func(s string) { got = append(got, s) })
	require.Equal(t, []string{"one", "two"}, got)
	require.Equal(t, tailErr, err)
}

// Huỷ ctx thì thoát với nil dù kênh vẫn mở (Ctrl+C khi đang theo dõi).
func TestForwardLogLines_ContextCancelReturnsNil(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	lines := make(chan *tail.Line) // không bao giờ có dòng, không đóng
	done := make(chan error, 1)
	go func() {
		done <- forwardLogLines(ctx, lines, func() error { return errors.New("unused") }, func(string) {})
	}()

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("forwardLogLines did not return after the context was cancelled")
	}
}

// Với tail THẬT: dòng ghi thêm vào tệp được chuyển đi, và khi tail dừng (kênh
// Lines đóng) vòng lặp thoát chứ không panic.
func TestForwardLogLines_RealTailStopClosesChannel(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crush.log")
	require.NoError(t, os.WriteFile(path, []byte("old line\n"), 0o600))

	tl, err := tail.TailFile(path, tail.Config{
		Follow:   true,
		ReOpen:   true,
		Poll:     true, // không phụ thuộc inotify của môi trường test
		Logger:   tail.DiscardingLogger,
		Location: &tail.SeekInfo{Offset: 0, Whence: io.SeekEnd},
	})
	require.NoError(t, err)

	got := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- forwardLogLines(t.Context(), tl.Lines, tl.Err, func(s string) {
			select {
			case got <- s:
			default: // chỉ cần dòng đầu; không để emit chặn vòng lặp
			}
		})
	}()

	// tail tự nhảy tới cuối tệp bất đồng bộ; dòng ghi trước lúc đó bị bỏ qua. Ghi
	// lại đều đặn tới khi có một dòng được chuyển đi thay vì đoán thời điểm.
	appendLine := func(text string) {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		require.NoError(t, err)
		_, err = f.WriteString(text + "\n")
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(10 * time.Second)
	for n := 1; ; n++ {
		appendLine("probe " + strconv.Itoa(n))
		select {
		case line := <-got:
			require.True(t, strings.HasPrefix(line, "probe "), "unexpected line %q", line)
		case <-ticker.C:
			continue
		case <-deadline:
			t.Fatal("no appended line was forwarded")
		}
		break
	}

	require.NoError(t, tl.Stop())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("forwardLogLines did not return after tail stopped")
	}
}
