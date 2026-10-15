package execpolicy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const userSpec = "zai:glm-5.3 zai:glm-5.3-flash main:qwen3.8 " +
	"claude_code:claude-opus-5-5,claude_code:claude-sonnet-5-5,claude_code:claude-haiku-5-5 codex:gpt-6.1-sol"

// Mặc định (không đặt gì): model chính không giới hạn như trước, nhưng giao
// việc sang CLI agent khác provider bị tắt dù CLI có trên máy và có key.
func TestDefaultPolicyDeniesDelegation(t *testing.T) {
	t.Parallel()
	p, err := FromEnv(env(map[string]string{"ANTHROPIC_API_KEY": "x", "OPENAI_API_KEY": "y"}))
	require.NoError(t, err)
	require.False(t, p.Restricted())
	require.True(t, p.AllowsModel("zai", "glm-9"))
	require.Nil(t, p.DelegationModels(BackendClaudeCode))
	require.Nil(t, p.DelegationModels(BackendCodex))
	var d *Denied
	require.ErrorAs(t, p.CheckDelegation(BackendClaudeCode, "claude-opus-5-5"), &d)
	require.Equal(t, CodeCrossProviderNotAllowed, d.Code)
}

// Danh sách của người dùng: model chính chỉ GLM 5.3/5.3-flash và qwen3.8;
// model mới, model cũ, provider khác đều bị chặn (allowlist, không denylist).
func TestAllowlistIsAuthoritativeForMainModels(t *testing.T) {
	t.Parallel()
	p, err := FromEnv(env(map[string]string{AllowedModelsEnv: userSpec}))
	require.NoError(t, err)
	require.True(t, p.Restricted())
	require.True(t, p.AllowsModel("zai", "glm-5.3-flash"))
	require.False(t, p.AllowsModel("ZAI", "glm-5.3"), "provider IDs are exact, like the config")
	require.False(t, p.AllowsModel("claude_code", "claude-opus-5-5"), "a main provider named like a backend borrows nothing")
	require.True(t, p.AllowsModel("main", "qwen3.8"))
	for _, c := range [][2]string{{"zai", "glm-5.4"}, {"zai", "glm-4.6"}, {"main", "qwen3.9"}, {"openai", "gpt-6.1-sol"}, {"anthropic", "claude-opus-5-5"}} {
		require.Falsef(t, p.AllowsModel(c[0], c[1]), "%s:%s", c[0], c[1])
		var d *Denied
		require.ErrorAs(t, p.CheckModel(c[0], c[1]), &d)
		require.Equal(t, CodeModelNotAllowed, d.Code)
		require.Contains(t, d.Allowed, "main:qwen3.8")
	}
}

// Bật giao việc: chỉ model liệt kê của từng backend; Fable, Astra, model cũ,
// model lạ đều bị từ chối; backend không liệt kê thì không có.
func TestDelegationModelAllowlist(t *testing.T) {
	t.Parallel()
	p, err := FromEnv(env(map[string]string{AllowedModelsEnv: userSpec, DelegationEnv: "on"}))
	require.NoError(t, err)
	require.Equal(t, []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"}, p.DelegationModels(BackendClaudeCode))
	require.Equal(t, []string{"gpt-6.1-sol"}, p.DelegationModels(BackendCodex))
	require.NoError(t, p.CheckDelegation(BackendClaudeCode, "claude-haiku-5-5"))
	for _, c := range [][2]string{
		{BackendClaudeCode, "claude-fable-5-1"}, {BackendClaudeCode, "claude-opus-4-1"},
		{BackendClaudeCode, "claude-opus-6-0"}, {BackendCodex, "gpt-6-astra"}, {BackendCodex, "gpt-6-luna"},
	} {
		var d *Denied
		require.ErrorAsf(t, p.CheckDelegation(c[0], c[1]), &d, "%s:%s", c[0], c[1])
		require.Equal(t, CodeModelNotAllowed, d.Code)
	}

	only, err := FromEnv(env(map[string]string{AllowedModelsEnv: "zai:glm-5.3", DelegationEnv: "on"}))
	require.NoError(t, err)
	require.Nil(t, only.DelegationModels(BackendCodex), "a backend with no listed model stays off")
	var d *Denied
	require.ErrorAs(t, only.CheckDelegation(BackendCodex, "gpt-6.1-sol"), &d)
	require.Equal(t, CodeExternalAgentNotAllowed, d.Code)
}

// Alias của CLI (tự trỏ sang model mới nhất) không được làm ranh giới; giá
// trị sai cú pháp chặn lúc khởi động.
func TestParseRejectsAliasesAndGarbage(t *testing.T) {
	t.Parallel()
	for _, spec := range []string{"claude_code:opus", "claude_code:fable", "codex:latest", "glm-5.3", "zai:", ":glm",
		"claude_code:sonnet[1m]", "claude_code:claude-3-5-haiku-latest", "codex:gpt-6-latest"} {
		_, err := Parse(spec)
		require.Errorf(t, err, "%q", spec)
	}
	_, err := FromEnv(env(map[string]string{DelegationEnv: "maybe"}))
	require.Error(t, err)
	var d *Denied
	require.False(t, errors.As(err, &d))
}

// Lệnh bash gọi thẳng CLI agent bị nhận ra, kể cả qua đường dẫn, launcher và
// npx; lệnh thường thì không.
func TestExternalAgentCommand(t *testing.T) {
	t.Parallel()
	hits := [][]string{
		{"claude", "-p", "hi"},
		{"/home/u/.local/bin/claude", "--dangerously-skip-permissions"},
		{"codex", "exec", "x"},
		{"env", "FOO=1", "-u", "BAR", "codex", "exec"},
		{"timeout", "30", "claude", "-p", "x"},
		{"nohup", "gemini", "-p", "x"},
		{"sudo", "-E", "opencode", "run"},
		{"sudo", "-u", "bob", "claude", "-p", "x"},
		{"nice", "-n", "5", "codex", "exec"},
		{"timeout", "-k", "5", "60", "claude"},
		{"npx", "-y", "@anthropic-ai/claude-code", "-p", "x"},
		{"bunx", "@openai/codex", "exec"},
		{"npx", "claude", "-p", "x"},
		{"npm", "exec", "--", "claude", "-p", "x"},
		{"pnpm", "dlx", "codex"},
		{"/home/u/.nvm/versions/node/bin/claude.exe", "-p"},
		{"./node_modules/.bin/claude-code"},
		{"env", "-S", "claude -p hi"},
		{"doas", "-u", "bob", "claude"},
		{"env", "doas", "claude"},
		{"xargs", "-n", "1", "codex"},
		{"find", ".", "-exec", "claude", "-p", "{}", ";"},
		{"bash", "-c", "cd x && claude -p hi"},
		{"sh", "-c", "A=1 codex exec x"},
		{"agent-cli", "run", "--yolo", "x"},
		{"crush", "server", "--host", "unix:///tmp/s"},
	}
	for _, a := range hits {
		_, ok := ExternalAgentCommand(a)
		require.Truef(t, ok, "%q", a)
	}
	misses := [][]string{
		{"git", "status"}, {"go", "test", "./..."}, {"agent-cli", "jobs"}, {"crush", "--version"},
		{"goose", "-dir", "migrations", "postgres", "up"}, {"agent-cli", "ps", "--all"}, {"bash", "-c", "echo claude"},
		{"find", ".", "-name", "*.go"}, {"npm", "exec", "prettier"},
		{"npx", "prettier", "--write", "."}, {"echo", "claude"}, {"timeout", "5", "make", "test"},
		{"env", "-u", "claude", "ls"}, {"sudo", "-u", "codex", "id"},
	}
	for _, a := range misses {
		_, ok := ExternalAgentCommand(a)
		require.Falsef(t, ok, "%q", a)
	}
}

// Chính sách đọc từ tệp env của agent-cli (chỉ hai khoá, bỏ qua dòng khác kể
// cả khoá API) và từ env đã export (thắng tệp).
func TestLoadReadsEnvFileAndProcessEnv(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	envFile := filepath.Join(dir, "env")
	require.NoError(t, os.WriteFile(envFile, []byte(`# comment
AGENT_CLI_API_KEY='secret-must-not-matter'
export AGENT_CLI_ALLOWED_MODELS="zai:glm-5.3 claude_code:claude-opus-5-5"  # trailing comment
AGENT_CLI_DELEGATION=on   # bật giao việc
`), 0o600))

	p, err := load(env(map[string]string{"AGENT_CLI_ENV_FILE": envFile}))
	require.NoError(t, err)
	require.True(t, p.AllowsModel("zai", "glm-5.3"))
	require.False(t, p.AllowsModel("zai", "glm-4.6"))
	require.Equal(t, []string{"claude-opus-5-5"}, p.DelegationModels(BackendClaudeCode))

	// Env đã export thắng tệp.
	p, err = load(env(map[string]string{"AGENT_CLI_ENV_FILE": envFile, DelegationEnv: "off", AllowedModelsEnv: "zai:glm-4.6"}))
	require.NoError(t, err)
	require.True(t, p.AllowsModel("zai", "glm-4.6"))
	require.Nil(t, p.DelegationModels(BackendClaudeCode))

	// Không có tệp: mặc định (không giới hạn model chính, giao việc tắt).
	p, err = load(env(map[string]string{"HOME": dir}))
	require.NoError(t, err)
	require.False(t, p.Restricted())
	require.Nil(t, p.DelegationModels(BackendCodex))

	file, err := readEnvFile(envFile)
	require.NoError(t, err)
	require.Len(t, file, 2, "only the two policy keys are kept")
}

func TestShellValue(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		`"a b"`: "a b", `'a b' # c`: "a b", `on   # x`: "on", ` off `: "off", `"unterminated`: "unterminated",
	} {
		require.Equal(t, want, shellValue(in), in)
	}
}

// Tệp env viết bằng dạng shell khác (declare -x, readonly, KEY+=, nhiều lệnh
// một dòng) thì báo lỗi, không lặng lẽ bỏ ranh giới.
func TestEnvFileUnusualSpellingsFailClosed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i, line := range []string{
		"declare -x AGENT_CLI_ALLOWED_MODELS=zai:glm-5.3",
		"typeset -x AGENT_CLI_ALLOWED_MODELS=zai:glm-5.3",
		"readonly AGENT_CLI_DELEGATION=on",
		"X=1; AGENT_CLI_ALLOWED_MODELS=zai:glm-5.3",
		"AGENT_CLI_ALLOWED_MODELS+=zai:glm-5.3",
	} {
		f := filepath.Join(dir, fmt.Sprintf("env%d", i))
		require.NoError(t, os.WriteFile(f, []byte(line+"\n"), 0o600))
		_, err := readEnvFile(f)
		require.Errorf(t, err, "%q", line)
	}
	ok := filepath.Join(dir, "ok")
	require.NoError(t, os.WriteFile(ok, []byte("export\tAGENT_CLI_DELEGATION=on\r\n# AGENT_CLI_ALLOWED_MODELS=commented\n"), 0o600))
	got, err := readEnvFile(ok)
	require.NoError(t, err)
	require.Equal(t, "on", strings.TrimSpace(got[DelegationEnv]))
}

// Ảnh chụp env tin cậy: giá trị .env/cấu hình dự án đặt sau khi chụp không
// tới chính sách, không tới server con; biến khác giữ nguyên.
func TestChildEnvUsesTheSnapshot(t *testing.T) {
	// Không parallel: đổi ảnh chụp cấp gói.
	prev := snapshot
	t.Cleanup(func() { snapshot = prev })
	snapshot = map[string]string{AllowedModelsEnv: "zai:glm-5.3", "HOME": "/home/u"}

	env := ChildEnv([]string{
		"PATH=/usr/bin",
		AllowedModelsEnv + "=claude_code:claude-fable-5-1",
		DelegationEnv + "=on",
		EnvFileEnv + "=/repo/policy.env",
		"HOME=/repo",
	})
	require.ElementsMatch(t, []string{"PATH=/usr/bin", AllowedModelsEnv + "=zai:glm-5.3", "HOME=/home/u"}, env)
	require.Equal(t, "zai:glm-5.3", TrustedEnv(AllowedModelsEnv))
	require.Empty(t, TrustedEnv(DelegationEnv), "set after the snapshot: ignored")
}
