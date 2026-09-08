package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/ui/common"
)

func TestMCPList_ChannelBadge(t *testing.T) {
	t.Parallel()
	styles := common.DefaultCommon(nil).Styles

	channelSrv := []mcp.ClientInfo{
		{Name: "webhook", State: mcp.StateConnected, Channel: true},
	}
	if out := mcpList(styles, channelSrv, 80, 10); !strings.Contains(out, "channel") {
		t.Errorf("expected a channel badge for an active channel server, got:\n%s", out)
	}

	plainSrv := []mcp.ClientInfo{
		{Name: "webhook", State: mcp.StateConnected, Channel: false},
	}
	if out := mcpList(styles, plainSrv, 80, 10); strings.Contains(out, "channel") {
		t.Errorf("non-channel server must not show a channel badge, got:\n%s", out)
	}
}
