package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/server"
	"github.com/stretchr/testify/require"
)

// serverDiedBlock mô phỏng đúng thứ nằm trong stderr.log khi server chết vì
// crushrc lỗi: khối lỗi có định dạng của fang, đệm khoảng trắng và xuống dòng,
// lý do thật nằm ở cuối câu.
const serverDiedBlock = "          \n" +
	"   ERROR  \n" +
	"          \n" +
	"  Failed to load configuration: failed to load config from paths [/c/crushrc]:   \n" +
	"  executing shell config /c/crushrc: exit status 1: AGENT_CLI_BASE_URL: đặt AGENT_CLI_BASE_URL trong /c/env.   \n\n"

func TestCleanLogTail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		max  int
		want string
	}{
		{
			name: "styled error block collapses to one line",
			raw:  serverDiedBlock,
			max:  1024,
			want: "ERROR Failed to load configuration: failed to load config from paths [/c/crushrc]: " +
				"executing shell config /c/crushrc: exit status 1: AGENT_CLI_BASE_URL: đặt AGENT_CLI_BASE_URL trong /c/env.",
		},
		{
			name: "ANSI sequences are removed",
			raw:  "\x1b[1;31mERROR\x1b[0m \x1b]0;title\x07boom",
			max:  1024,
			want: "ERROR boom",
		},
		{
			name: "control characters are dropped and newlines become spaces",
			raw:  "a\x00b\x07c\x7fd\r\ne",
			max:  1024,
			want: "abcd e",
		},
		{
			name: "invalid UTF-8 is dropped",
			raw:  "\xa5\x91đ ok",
			max:  1024,
			want: "đ ok",
		},
		{
			name: "only whitespace is empty",
			raw:  " \n\t \r\n ",
			max:  1024,
			want: "",
		},
		{
			name: "empty",
			raw:  "",
			max:  1024,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, cleanLogTail([]byte(tt.raw), tt.max))
		})
	}

	t.Run("long input keeps the end and is marked", func(t *testing.T) {
		t.Parallel()
		raw := strings.Repeat("dòng-cũ ", 500) + "lý-do-thật"
		got := cleanLogTail([]byte(raw), 64)
		require.True(t, strings.HasPrefix(got, "..."), got)
		require.True(t, strings.HasSuffix(got, "lý-do-thật"), got)
		require.LessOrEqual(t, len(got), 64+len("..."))
		require.True(t, utf8.ValidString(got), "cutting must not split a rune: %q", got)
	})

	t.Run("input within the limit is not marked", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "ngắn gọn", cleanLogTail([]byte("ngắn  gọn\n"), 64))
	})
}

func TestReadFileTail(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "stderr.log")
	require.NoError(t, os.WriteFile(path, []byte("0123456789abcdef"), 0o600))

	got, err := readFileTail(path, 6)
	require.NoError(t, err)
	require.Equal(t, "abcdef", string(got))

	got, err = readFileTail(path, 1024)
	require.NoError(t, err)
	require.Equal(t, "0123456789abcdef", string(got), "a file shorter than the limit is read whole")

	_, err = readFileTail(filepath.Join(dir, "missing.log"), 6)
	require.ErrorIs(t, err, os.ErrNotExist)
}

// TestServerStderrPathIsTheSpawnedLog bảo đảm chỗ đọc và chỗ ghi cùng một tệp:
// startDetachedServer tạo stderr.log trong perHostServerDir.
func TestServerStderrPathIsTheSpawnedLog(t *testing.T) {
	t.Setenv("CRUSH_CACHE_DIR", t.TempDir())

	u, err := server.ParseHostURL("unix:///run/user/1000/agent-cli-abc.sock")
	require.NoError(t, err)
	dir, err := perHostServerDir(u)
	require.NoError(t, err)

	require.Equal(t, filepath.Join(dir, serverStderrLogName), serverStderrPath(u))
	require.Equal(t, "stderr.log", serverStderrLogName)
}

// TestWaitForServerReadyShowsServerStderr tái hiện lỗi gốc: server chết lúc nạp
// cấu hình nên không bao giờ sẵn sàng, và người dùng chỉ thấy lỗi thăm dò socket.
// Sau khi sửa, lỗi mang theo phần cuối stderr.log của chính server đó.
func TestWaitForServerReadyShowsServerStderr(t *testing.T) {
	t.Setenv("CRUSH_CACHE_DIR", t.TempDir())
	t.Setenv("CRUSH_SERVER_READY_TIMEOUT", "300ms")

	u, err := server.ParseHostURL("unix://" + filepath.Join(t.TempDir(), "s.sock"))
	require.NoError(t, err)

	writeStderrLog := func(t *testing.T, content string) {
		t.Helper()
		dir, err := perHostServerDir(u)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, serverStderrLogName), []byte(content), 0o600))
	}

	t.Run("reason from the server log is attached", func(t *testing.T) {
		writeStderrLog(t, "\x1b[31m"+serverDiedBlock+"\x1b[0m")

		err := waitForServerReady(context.Background(), u)
		require.Error(t, err)
		msg := err.Error()
		require.Contains(t, msg, "/v1/health", "the original probe error must stay")
		require.Contains(t, msg, "server stderr ("+serverStderrPath(u)+"): ERROR Failed to load configuration")
		require.True(t, strings.HasSuffix(msg, "exit status 1: AGENT_CLI_BASE_URL: đặt AGENT_CLI_BASE_URL trong /c/env"),
			"the server's own final period is dropped because fang adds one: %q", msg)
		require.NotContains(t, msg, "\x1b")
		require.NotContains(t, msg, "\n")

		// Lỗi gốc vẫn lấy ra được qua Unwrap.
		var withLog *serverStderrError
		require.ErrorAs(t, err, &withLog)
		require.NotNil(t, errors.Unwrap(err))
	})

	t.Run("empty log adds nothing", func(t *testing.T) {
		writeStderrLog(t, "")

		err := waitForServerReady(context.Background(), u)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "server stderr")
	})

	t.Run("missing log adds nothing", func(t *testing.T) {
		dir, err := perHostServerDir(u)
		require.NoError(t, err)
		require.NoError(t, os.RemoveAll(filepath.Join(dir, serverStderrLogName)))

		err = waitForServerReady(context.Background(), u)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "server stderr")
	})

	t.Run("cancellation is not a server failure", func(t *testing.T) {
		writeStderrLog(t, serverDiedBlock)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := waitForServerReady(ctx, u)
		require.ErrorIs(t, err, context.Canceled)
		require.NotContains(t, err.Error(), "server stderr")
	})
}
