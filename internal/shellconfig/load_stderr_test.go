package shellconfig

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

// TestLoadShellConfig_StderrReasonInError kiểm tra lý do crushrc in ra stderr đi
// theo lỗi trả về, cả khi stderr thường lẫn khi chuyển hướng `2>/dev/stderr`
// (dạng mà agent-cli.crushrc.example dùng).
func TestLoadShellConfig_StderrReasonInError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		script   string
		wantAll  []string
		wantExit int
	}{
		{
			name:     "param expansion guard",
			script:   `: "${SHELLCONFIG_TEST_UNSET_X:?thiếu SHELLCONFIG_TEST_UNSET_X}"`,
			wantAll:  []string{"exit status 1", ": thiếu SHELLCONFIG_TEST_UNSET_X"},
			wantExit: 1,
		},
		{
			name:     "param expansion guard with stderr redirect",
			script:   `: "${SHELLCONFIG_TEST_UNSET_X:?thiếu SHELLCONFIG_TEST_UNSET_X}" 2>/dev/stderr`,
			wantAll:  []string{"exit status 1", ": thiếu SHELLCONFIG_TEST_UNSET_X"},
			wantExit: 1,
		},
		{
			name: "guards in a block redirected to stderr",
			script: `{
  : "${SHELLCONFIG_TEST_UNSET_X:?đặt SHELLCONFIG_TEST_UNSET_X trong tệp env}"
  : "${SHELLCONFIG_TEST_UNSET_Y:?đặt SHELLCONFIG_TEST_UNSET_Y}"
} 2>/dev/stderr
provider add main --api-key k`,
			wantAll:  []string{"exit status 1", ": đặt SHELLCONFIG_TEST_UNSET_X trong tệp env"},
			wantExit: 1,
		},
		{
			name:     "echo to stderr",
			script:   "echo 'lý do một' >&2\nexit 3",
			wantAll:  []string{"exit status 3", ": lý do một"},
			wantExit: 3,
		},
		{
			name:     "echo to /dev/stderr",
			script:   "echo 'lý do hai' >/dev/stderr\nexit 4",
			wantAll:  []string{"exit status 4", ": lý do hai"},
			wantExit: 4,
		},
		{
			name:     "quoted /dev/stderr target",
			script:   "echo 'lý do ba' 1>\"/dev/stderr\"\nexit 5",
			wantAll:  []string{"exit status 5", ": lý do ba"},
			wantExit: 5,
		},
		{
			name:     "several lines are joined on one line",
			script:   "echo dong-mot >&2\necho dong-hai >&2\nexit 2",
			wantAll:  []string{"exit status 2", ": dong-mot | dong-hai"},
			wantExit: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "crushrc")
			_, err := LoadShellConfig(t.Context(), path, []byte(tt.script))
			require.Error(t, err)
			for _, want := range tt.wantAll {
				require.Contains(t, err.Error(), want)
			}
			require.Contains(t, err.Error(), "executing shell config "+path+": ")
			require.NotContains(t, err.Error(), "\n", "the error must stay on one line")
			// Lỗi gốc vẫn nằm trong chuỗi wrap, nên mã thoát đọc ra được.
			require.Equal(t, tt.wantExit, shell.ExitCode(err))
		})
	}
}

// TestLoadShellConfig_StderrFromChildProcess kiểm tra stderr của tiến trình con
// (không phải builtin của trình thông dịch) cũng được thu lại.
func TestLoadShellConfig_StderrFromChildProcess(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX sh binary")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(`/bin/sh -c 'echo "từ tiến trình con" >&2; exit 7'`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "exit status 7")
	require.Contains(t, err.Error(), ": từ tiến trình con")
}

// TestLoadShellConfig_StderrTailIsBounded kiểm tra stderr dài chỉ giữ phần đuôi
// (nơi có lý do) và lỗi không phình theo.
func TestLoadShellConfig_StderrTailIsBounded(t *testing.T) {
	t.Parallel()

	script := `i=0
while [ "$i" -lt 400 ]; do
  echo "dong-nhieu-$i xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" >&2
  i=$((i+1))
done
echo "ly-do-that-su" >&2
exit 1`
	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(script))
	require.Error(t, err)

	msg := err.Error()
	require.Contains(t, msg, "ly-do-that-su")
	require.Contains(t, msg, ": ...", "a truncated tail must be marked")
	require.NotContains(t, msg, "dong-nhieu-0 ")
	// Đường dẫn + tiền tố + tối đa maxStderrTailBytes byte đuôi (+ dấu nối dòng).
	require.Less(t, len(msg), len(path)*2+maxStderrTailBytes+200)
	require.True(t, utf8.ValidString(msg), "cutting the tail must not split a rune")
}

// TestLoadShellConfig_StderrIsSanitized kiểm tra ANSI và ký tự điều khiển bị bỏ
// khỏi phần đuôi trước khi đính vào lỗi.
func TestLoadShellConfig_StderrIsSanitized(t *testing.T) {
	t.Parallel()

	script := "printf '\\033[31mmàu đỏ\\033[0m\\a\\r\\n\\tcó\\ttab\\n' >&2\nexit 1"
	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(script))
	require.Error(t, err)

	require.Contains(t, err.Error(), ": màu đỏ | có tab")
	require.NotContains(t, err.Error(), "\x1b")
	require.NotContains(t, err.Error(), "\a")
	require.NotContains(t, err.Error(), "\r")
	require.NotContains(t, err.Error(), "\t")
}

// TestLoadShellConfig_StderrNotDuplicatedForBuiltinErrors kiểm tra builtin cấu
// hình (vừa in usage ra stderr vừa trả chính nó làm lỗi) không bị lặp thông báo.
func TestLoadShellConfig_StderrNotDuplicatedForBuiltinErrors(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(`provider add main --bogus-flag x`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown flag")
	require.Equal(t, 1, strings.Count(err.Error(), "unknown flag"), "got: %v", err)
}

// TestLoadShellConfig_StderrOnSuccessIsNotAnError kiểm tra script thành công mà
// có in stderr (cảnh báo) vẫn nạp bình thường và không rò ra stderr thật.
func TestLoadShellConfig_StderrOnSuccessIsNotAnError(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crushrc")
	script := "echo 'cảnh báo nhẹ' >&2\necho 'cảnh báo hai' 2>/dev/stderr >&2\nprovider add main --api-key k"
	data, err := LoadShellConfig(t.Context(), path, []byte(script))
	require.NoError(t, err)
	require.Contains(t, string(data), `"main"`)
}

// TestLoadShellConfig_ExampleCrushrcNamesMissingVariable chạy chính tệp mẫu
// crushrc khi thiếu AGENT_CLI_BASE_URL: lỗi phải nêu tên biến, đây là đúng lỗi
// người dùng gặp khi cài mới mà chưa điền tệp env.
func TestLoadShellConfig_ExampleCrushrcNamesMissingVariable(t *testing.T) {
	example := filepath.Join("..", "..", "agent-cli.crushrc.example")
	src, err := os.ReadFile(example)
	if err != nil {
		t.Skipf("example crushrc not found: %v", err)
	}

	dir := t.TempDir()
	envFile := filepath.Join(dir, "env")
	t.Setenv("AGENT_CLI_ENV_FILE", envFile)
	t.Setenv("AGENT_CLI_PROVIDER_ID", "")
	t.Setenv("AGENT_CLI_BASE_URL", "")
	t.Setenv("AGENT_CLI_MODEL", "")

	// Chỉ so tên biến và đường dẫn tệp env, không so nguyên câu chữ của tệp mẫu,
	// để sửa lời nhắc trong tệp mẫu không làm đỏ test này.
	_, err = LoadShellConfig(t.Context(), filepath.Join(dir, "crushrc"), src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exit status 1")
	require.Contains(t, err.Error(), "AGENT_CLI_BASE_URL")
	require.Contains(t, err.Error(), envFile)

	// Biến thứ hai cũng được báo khi biến thứ nhất đã có.
	t.Setenv("AGENT_CLI_BASE_URL", "http://127.0.0.1:1/v1")
	_, err = LoadShellConfig(t.Context(), filepath.Join(dir, "crushrc"), src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "AGENT_CLI_MODEL")
	require.Contains(t, err.Error(), envFile)
}

// TestHelperLoadShellConfigDevStderr là tiến trình con của
// TestLoadShellConfig_DevStderrLeavesRealStderrAlone, không phải test độc lập.
func TestHelperLoadShellConfigDevStderr(t *testing.T) {
	if os.Getenv("SHELLCONFIG_TEST_HELPER") != "1" {
		t.Skip("helper process for TestLoadShellConfig_DevStderrLeavesRealStderrAlone")
	}
	// Giống cmd.Execute: log sớm bị bỏ, nếu không slog mặc định sẽ ghi vào chính
	// stderr mà test đang canh.
	slog.SetDefault(slog.New(slog.DiscardHandler))
	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(`: "${SHELLCONFIG_TEST_UNSET_X:?thiếu SHELLCONFIG_TEST_UNSET_X}" 2>/dev/stderr`))
	os.Stdout.WriteString("HELPER-ERR=" + err.Error() + "\n")
}

// TestLoadShellConfig_DevStderrLeavesRealStderrAlone tái hiện lỗi gốc ở chế độ
// client/server: stderr của tiến trình là một tệp log, và mở /dev/stderr bằng `>`
// (O_TRUNC) làm xoá sạch tệp đó. Sau khi sửa, lý do đi vào lỗi trả về còn tệp log
// không bị đụng tới.
func TestLoadShellConfig_DevStderrLeavesRealStderrAlone(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("relies on Linux /dev/stderr re-opening the underlying file")
	}

	logFile, err := os.CreateTemp(t.TempDir(), "stderr.log")
	require.NoError(t, err)
	_, err = logFile.WriteString("dòng có sẵn của server\n")
	require.NoError(t, err)

	var stdout bytes.Buffer
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperLoadShellConfigDevStderr$")
	cmd.Env = append(os.Environ(), "SHELLCONFIG_TEST_HELPER=1")
	cmd.Stdout = &stdout
	cmd.Stderr = logFile
	require.NoError(t, cmd.Run())
	require.NoError(t, logFile.Close())

	require.Contains(t, stdout.String(), "HELPER-ERR=")
	require.Contains(t, stdout.String(), ": thiếu SHELLCONFIG_TEST_UNSET_X")

	got, err := os.ReadFile(logFile.Name())
	require.NoError(t, err)
	require.Equal(t, "dòng có sẵn của server\n", string(got),
		"crushrc must not open or write the process's real stderr")
}

func TestRedirectDevStderr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want string
	}{
		{"fd 2", `cmd 2>/dev/stderr`, `cmd 2>&2`},
		{"default fd", `echo hi >/dev/stderr`, `echo hi >&2`},
		{"spaces around operator", `echo hi 2> /dev/stderr`, `echo hi 2>&2`},
		{"explicit fd 1", `echo hi 1>/dev/stderr`, `echo hi 1>&2`},
		{"append", `echo hi >>/dev/stderr`, `echo hi >&2`},
		{"both streams", `echo hi &>/dev/stderr`, `echo hi >&2`},
		{"both streams append", `echo hi &>>/dev/stderr`, `echo hi >&2`},
		{"double quoted", `echo hi 2>"/dev/stderr"`, `echo hi 2>&2`},
		{"single quoted", `echo hi 2>'/dev/stderr'`, `echo hi 2>&2`},
		{"proc alias", `echo hi 2>/proc/self/fd/2`, `echo hi 2>&2`},
		{"fd alias", `echo hi 2>/dev/fd/2`, `echo hi 2>&2`},
		{
			name: "block with multibyte text before the redirect",
			src:  "{\n  : \"${X:?đặt X trong ${f} (vd http://127.0.0.1:<port>/v1)}\"\n} 2>/dev/stderr\nnext",
			want: "{\n  : \"${X:?đặt X trong ${f} (vd http://127.0.0.1:<port>/v1)}\"\n} 2>&2\nnext",
		},
		{"inside command substitution", `v=$(echo hi 2>/dev/stderr)`, `v=$(echo hi 2>&2)`},
		{"inside function body", "f() {\n  echo hi >/dev/stderr\n}", "f() {\n  echo hi >&2\n}"},
		{
			name: "several redirects, order preserved",
			src:  `a 2>/dev/stderr; b >/dev/stderr; c 2>/dev/null`,
			want: `a 2>&2; b >&2; c 2>/dev/null`,
		},
		{"stdout target untouched", `echo hi >/dev/stdout`, `echo hi >/dev/stdout`},
		{"null target untouched", `echo hi 2>/dev/null`, `echo hi 2>/dev/null`},
		{"input redirect untouched", `cat </dev/stderr`, `cat </dev/stderr`},
		{"other fd untouched", `echo hi 3>/dev/stderr`, `echo hi 3>/dev/stderr`},
		{"longer path untouched", `echo hi 2>/dev/stderrx`, `echo hi 2>/dev/stderrx`},
		{"dynamic target untouched", `echo hi 2>"$LOG"`, `echo hi 2>"$LOG"`},
		{"existing dup untouched", `echo hi >&2`, `echo hi >&2`},
		{"text in string untouched", `echo '2>/dev/stderr'`, `echo '2>/dev/stderr'`},
		{"text in comment untouched", "# 2>/dev/stderr\necho hi", "# 2>/dev/stderr\necho hi"},
		{"line continuation inside the target untouched", "echo hi 2>/dev/\\\nstderr", "echo hi 2>/dev/\\\nstderr"},
		{"syntax error returned as is", `echo "chưa đóng 2>/dev/stderr`, `echo "chưa đóng 2>/dev/stderr`},
		{"empty", ``, ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, redirectDevStderr(tt.src))
		})
	}
}

func TestStderrTail(t *testing.T) {
	t.Parallel()

	t.Run("keeps only the last bytes", func(t *testing.T) {
		t.Parallel()
		tail := newStderrTail(10)
		_, _ = tail.Write([]byte("0123456789"))
		_, _ = tail.Write([]byte("abc"))
		lines, truncated := tail.lines()
		require.Equal(t, []string{"3456789abc"}, lines)
		require.True(t, truncated)
	})

	t.Run("single write larger than the limit", func(t *testing.T) {
		t.Parallel()
		tail := newStderrTail(4)
		n, err := tail.Write([]byte("0123456789"))
		require.NoError(t, err)
		require.Equal(t, 10, n, "Write must report the full length or io.Copy would fail")
		lines, truncated := tail.lines()
		require.Equal(t, []string{"6789"}, lines)
		require.True(t, truncated)
	})

	t.Run("under the limit is not truncated", func(t *testing.T) {
		t.Parallel()
		tail := newStderrTail(64)
		_, _ = tail.Write([]byte("một\nhai\n\n  ba  \n"))
		lines, truncated := tail.lines()
		require.Equal(t, []string{"một", "hai", "ba"}, lines)
		require.False(t, truncated)
	})

	t.Run("cut in the middle of a rune drops the broken bytes", func(t *testing.T) {
		t.Parallel()
		// "đ" là 2 byte; giới hạn lẻ cắt đôi rune đầu tiên.
		tail := newStderrTail(5)
		_, _ = tail.Write([]byte("đđđ"))
		lines, _ := tail.lines()
		require.Equal(t, []string{"đđ"}, lines)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		lines, truncated := newStderrTail(8).lines()
		require.Empty(t, lines)
		require.False(t, truncated)
	})
}
