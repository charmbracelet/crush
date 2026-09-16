package chat

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func hookMetadataJSON(t *testing.T, hookInfos []hooks.HookInfo) string {
	t.Helper()
	meta := struct {
		Hook *hooks.HookMetadata `json:"hook"`
	}{
		Hook: &hooks.HookMetadata{
			HookCount: len(hookInfos),
			Hooks:     hookInfos,
		},
	}
	b, err := json.Marshal(meta)
	require.NoError(t, err)
	return string(b)
}

func TestToolOutputHookIndicatorCollapsesCleanHooks(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	styPtr := &sty
	metadata := hookMetadataJSON(t, []hooks.HookInfo{
		{Name: "rtk", Decision: "allow"},
		{Name: "safety", Decision: "allow"},
		{Name: "audit", Decision: "allow"},
	})

	line := toolOutputHookIndicator(styPtr, metadata, 80)

	require.Contains(t, line, "3 hooks ran, all")
	require.NotContains(t, line, "rtk", "individual hook names should not appear in the collapsed summary")
}

func TestToolOutputHookIndicatorSingleCleanHookUsesSingular(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	styPtr := &sty
	metadata := hookMetadataJSON(t, []hooks.HookInfo{
		{Name: "rtk", Decision: "allow"},
	})

	line := toolOutputHookIndicator(styPtr, metadata, 80)

	require.Contains(t, line, "1 hook ran, all")
}

func TestToolOutputHookIndicatorShowsDetailOnDenial(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	styPtr := &sty
	metadata := hookMetadataJSON(t, []hooks.HookInfo{
		{Name: "rtk", Decision: "allow"},
		{Name: "safety", Decision: "deny", Reason: "blocked rm -rf"},
	})

	line := toolOutputHookIndicator(styPtr, metadata, 80)

	require.Contains(t, line, "rtk", "clean hooks should still be listed when any hook is denied")
	require.Contains(t, line, "safety")
	require.Contains(t, line, "blocked rm -rf")
}

func TestToolOutputHookIndicatorShowsDetailOnInputRewrite(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	styPtr := &sty
	metadata := hookMetadataJSON(t, []hooks.HookInfo{
		{Name: "rtk", Decision: "allow", InputRewrite: true},
	})

	line := toolOutputHookIndicator(styPtr, metadata, 80)

	require.Contains(t, line, "rtk")
	require.NotContains(t, line, "ran, all", "a rewrite should not be collapsed into the clean summary")
}

func TestToolOutputHookIndicatorEmptyWithNoHooks(t *testing.T) {
	t.Parallel()

	sty := styles.CharmtonePantera()
	styPtr := &sty

	require.Equal(t, "", toolOutputHookIndicator(styPtr, "", 80))
	require.Equal(t, "", toolOutputHookIndicator(styPtr, hookMetadataJSON(t, nil), 80))
}
