package agent

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/execpolicy"
	"github.com/stretchr/testify/require"
)

// Model chính (large/small) ngoài danh sách bị chặn trước khi dựng provider:
// mọi lượt, sub-agent và đặt tiêu đề dựng model qua đây. Không đặt danh sách
// thì như cũ.
func TestCheckModelPolicy(t *testing.T) {
	t.Parallel()
	pol, err := execpolicy.Parse("zai:glm-5.3 zai:glm-5.3-flash main:qwen3.8")
	require.NoError(t, err)

	glm := config.SelectedModel{Provider: "zai", Model: "glm-5.3"}
	flash := config.SelectedModel{Provider: "zai", Model: "glm-5.3-flash"}
	require.NoError(t, checkModelPolicy(pol, glm, flash))
	require.NoError(t, checkModelPolicy(pol, config.SelectedModel{Provider: "main", Model: "qwen3.8"}))

	for _, bad := range []config.SelectedModel{
		{Provider: "zai", Model: "glm-4.6"},
		{Provider: "zai", Model: "glm-5.4"},
		{Provider: "openai", Model: "gpt-6.1-sol"},
	} {
		err := checkModelPolicy(pol, glm, bad)
		var denied *execpolicy.Denied
		require.ErrorAsf(t, err, &denied, "%v", bad)
		require.Equal(t, execpolicy.CodeModelNotAllowed, denied.Code)
	}

	require.NoError(t, checkModelPolicy(execpolicy.Policy{}, config.SelectedModel{Provider: "any", Model: "thing"}))
}
