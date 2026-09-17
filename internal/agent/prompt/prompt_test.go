package prompt

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPromptDat_ToolEnabled(t *testing.T) {
	t.Parallel()

	data := PromptDat{
		Config: config.Config{
			Options: &config.Options{
				DisabledTools: []string{"edit", "multiedit"},
			},
		},
	}

	require.False(t, data.ToolEnabled("edit"))
	require.False(t, data.ToolEnabled("multiedit"))
	require.True(t, data.ToolEnabled("write"))
}

func TestCoderPrompt_OmitsDisabledEditTools(t *testing.T) {
	t.Parallel()

	const coderTemplate = `<editing_files>
**Available edit tools:**
{{if .ToolEnabled "edit"}}- ` + "`edit`" + `
{{end}}{{if .ToolEnabled "multiedit"}}- ` + "`multiedit`" + `
{{end}}{{if .ToolEnabled "write"}}- ` + "`write`" + `
{{end}}
</editing_files>`

	p, err := NewPrompt("coder", coderTemplate, WithTimeFunc(func() time.Time {
		return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	}))
	require.NoError(t, err)

	store := config.NewTestStore(&config.Config{
		Options: &config.Options{
			DisabledTools: []string{"edit", "multiedit"},
		},
	})

	out, err := p.Build(context.Background(), "test", "test", store)
	require.NoError(t, err)

	require.NotContains(t, out, "`edit`")
	require.NotContains(t, out, "`multiedit`")
	require.Contains(t, out, "`write`")
	require.Contains(t, out, "**Available edit tools:**")
}

func TestCoderPrompt_IncludesEnabledEditTools(t *testing.T) {
	t.Parallel()

	const coderTemplate = `<editing_files>
{{if .ToolEnabled "edit"}}- ` + "`edit`" + `
{{end}}{{if .ToolEnabled "multiedit"}}- ` + "`multiedit`" + `
{{end}}
</editing_files>`

	p, err := NewPrompt("coder", coderTemplate)
	require.NoError(t, err)

	store := config.NewTestStore(&config.Config{
		Options: &config.Options{},
	})

	out, err := p.Build(context.Background(), "test", "test", store)
	require.NoError(t, err)

	require.True(t, strings.Contains(out, "`edit`"))
	require.True(t, strings.Contains(out, "`multiedit`"))
}
