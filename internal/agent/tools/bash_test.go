package tools

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveBannedCommandsListWithEmptyConfig(t *testing.T) {
	cfg := config.ToolBash{
		DisableDefaultCommands: false,
		BannedCommands:         []string{},
		BannedSubCommands:      []config.BannedToolArgsAndOrParams{},
	}

	bannedCmds := resolveBannedCommandsList(cfg)
	require.Len(t, bannedCmds, len(defaultBannedCommands))
	assert.Equal(t, defaultBannedCommands, bannedCmds)
}

func TestResolveBannedCommandsListWithEmptyConfigWithDefaultDisabled(t *testing.T) {
	cfg := config.ToolBash{
		DisableDefaultCommands: true,
		BannedCommands:         []string{},
		BannedSubCommands:      []config.BannedToolArgsAndOrParams{},
	}

	bannedCmds := resolveBannedCommandsList(cfg)
	require.Len(t, bannedCmds, 0)
	assert.Equal(t, []string{}, bannedCmds)
}

func TestResolveBannedCommandsListWithDefaultDisabledWithBannedCommands(t *testing.T) {
	cfg := config.ToolBash{
		DisableDefaultCommands: true,
		BannedCommands: []string{
			"pacman",
			"mount",
		},
		BannedSubCommands: []config.BannedToolArgsAndOrParams{},
	}

	bannedCmds := resolveBannedCommandsList(cfg)
	require.Len(t, bannedCmds, 2)
	assert.Equal(t, []string{"pacman", "mount"}, bannedCmds)
}

func TestResolveBannedCommandsListWithDefaultAndAddtionalBannedCommands(t *testing.T) {
	additionalBannedCommands := []string{"lazygit", "btop"}
	cfg := config.ToolBash{
		DisableDefaultCommands: false,
		BannedCommands:         additionalBannedCommands,
		BannedSubCommands:      []config.BannedToolArgsAndOrParams{},
	}

	bannedCmds := resolveBannedCommandsList(cfg)
	require.Len(t, bannedCmds, len(defaultBannedCommands)+len(additionalBannedCommands))
}

func TestResolveBlockFuncsWithDefaultDisabledSubCommands(t *testing.T) {
}
