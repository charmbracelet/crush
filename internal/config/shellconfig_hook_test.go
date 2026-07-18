package config_test

import (
	"testing"

	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/stretchr/testify/require"
)

func TestShellConfigHookAdd(t *testing.T) {
	store := loadCrushSh(t, `hook add PreToolUse --matcher "^bash$" --command "echo hi" --name greet --timeout 10`)

	hs := store.Config().Hooks[hooks.EventPreToolUse]
	require.Len(t, hs, 1)
	require.Equal(t, "greet", hs[0].Name)
	require.Equal(t, "^bash$", hs[0].Matcher)
	require.Equal(t, "echo hi", hs[0].Command)
	require.Equal(t, 10, hs[0].Timeout)
}

func TestShellConfigHookRemoveByName(t *testing.T) {
	store := loadCrushSh(t, `hook add PreToolUse --command a --name keep
hook add PreToolUse --command b --name drop
hook remove PreToolUse --name drop`)

	hs := store.Config().Hooks[hooks.EventPreToolUse]
	require.Len(t, hs, 1)
	require.Equal(t, "keep", hs[0].Name)
}

func TestShellConfigHookClearEvent(t *testing.T) {
	store := loadCrushSh(t, `hook add PreToolUse --command a --name a
hook add PreToolUse --command b --name b
hook remove PreToolUse`)

	require.Empty(t, store.Config().Hooks[hooks.EventPreToolUse])
}
