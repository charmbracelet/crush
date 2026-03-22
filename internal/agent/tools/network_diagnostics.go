package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/audit"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/sandbox"
	"github.com/charmbracelet/crush/internal/security"
)

// NetworkDiagnosticsParams are the parameters for the network_diagnostics tool.
type NetworkDiagnosticsParams struct {
	Action string `json:"action" description:"Diagnostic action: ping, traceroute, dns-lookup, port-check, connections, interfaces, routes, bandwidth"`
	Target string `json:"target,omitempty" description:"Target host, IP, or port (e.g. 'google.com', '192.168.1.1', 'localhost:8080')"`
	Count  int    `json:"count,omitempty" description:"Number of packets/attempts (default: 4)"`
}

type NetworkDiagnosticsResponseMetadata struct {
	Action    string `json:"action"`
	Target    string `json:"target"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}

const NetworkDiagnosticsToolName = "network_diagnostics"

//go:embed network_diagnostics.md
var networkDiagnosticsDescription []byte

func NewNetworkDiagnosticsTool(
	permissions permission.Service,
	sandboxExec *sandbox.Executor,
	riskAssessor *security.RiskAssessor,
	auditLogger *audit.Logger,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		NetworkDiagnosticsToolName,
		string(networkDiagnosticsDescription),
		func(ctx context.Context, params NetworkDiagnosticsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Action == "" {
				return fantasy.NewTextErrorResponse("action is required (ping, traceroute, dns-lookup, port-check, connections, interfaces, routes, bandwidth)"), nil
			}

			count := params.Count
			if count == 0 {
				count = 4
			}

			risk := riskAssessor.AssessToolCall(NetworkDiagnosticsToolName, params.Action, params.Target)

			sessionID := GetSessionFromContext(ctx)
			approved, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				ToolCallID:  call.ID,
				ToolName:    NetworkDiagnosticsToolName,
				Action:      params.Action,
				Description: fmt.Sprintf("Network diagnostic: %s %s (risk: %s)",
					params.Action, params.Target, risk.Level),
				Path: workingDir,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !approved {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			start := time.Now()

			cmd := buildNetworkDiagCommand(params.Action, params.Target, count)

			sandboxCfg := sandbox.DefaultConfig()
			sandboxCfg.AllowNetwork = true // network diagnostics need network access
			sandboxCfg.WorkingDir = workingDir
			sandboxCfg.Timeout = 2 * time.Minute

			result, execErr := sandboxExec.Execute(ctx, cmd, &sandboxCfg)

			if auditLogger != nil {
				auditLogger.Log(ctx, audit.Event{
					SessionID: sessionID,
					Actor:     "agent",
					Action:    audit.ActionNetworkDiag,
					ToolName:  NetworkDiagnosticsToolName,
					Description: fmt.Sprintf("Network diagnostics: action=%s target=%s",
						params.Action, params.Target),
					Resource: audit.Resource{
						Type: audit.ResourceNetwork,
						Name: params.Target,
					},
					Result: audit.Result{
						Status:  resultStatus(execErr),
						Message: truncateString(result.Stdout, 500),
					},
					RiskScore: risk.Score,
					RiskLevel: string(risk.Level),
					Duration:  time.Since(start),
				})
			}

			if execErr != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Network diagnostic failed: %v\n%s", execErr, result.Stderr)), nil
			}

			output := result.Stdout
			if output == "" {
				output = "No output from network diagnostic."
			}

			metadata := NetworkDiagnosticsResponseMetadata{
				Action:    params.Action,
				Target:    params.Target,
				StartTime: start.UnixMilli(),
				EndTime:   time.Now().UnixMilli(),
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				metadata,
			), nil
		},
	)
}

func buildNetworkDiagCommand(action, target string, count int) string {
	// Validate target against shell injection
	if target != "" {
		if msg := security.ValidateNoShellMeta(target); msg != "" {
			return fmt.Sprintf("echo 'Error: invalid target: %s'", msg)
		}
	}

	qt := security.ShellQuote(target)

	switch strings.ToLower(action) {
	case "ping":
		if target == "" {
			return "echo 'target is required for ping'"
		}
		return fmt.Sprintf("ping -c %d -W 5 %s 2>&1", count, qt)

	case "traceroute":
		if target == "" {
			return "echo 'target is required for traceroute'"
		}
		return fmt.Sprintf("traceroute -m 20 -w 3 %s 2>&1 || tracepath %s 2>&1", qt, qt)

	case "dns-lookup":
		if target == "" {
			return "echo 'target is required for dns-lookup'"
		}
		return fmt.Sprintf(`echo "===== DNS Lookup: %s ====="
echo ""
echo "--- nslookup ---"
nslookup %s 2>&1 || echo "nslookup not available"
echo ""
echo "--- dig ---"
dig %s +short 2>&1 || echo "dig not available"
echo ""
echo "--- host ---"
host %s 2>&1 || echo "host not available"`, qt, qt, qt, qt)

	case "port-check":
		if target == "" {
			return "echo 'target is required for port-check (format: host:port)'"
		}
		parts := strings.SplitN(target, ":", 2)
		if len(parts) != 2 {
			return fmt.Sprintf("echo 'Invalid target format. Use host:port'")
		}
		host, port := security.ShellQuote(parts[0]), security.ShellQuote(parts[1])
		return fmt.Sprintf(`echo "===== Port Check: %s:%s ====="
echo ""
echo "--- TCP Connect ---"
timeout 5 bash -c 'echo >/dev/tcp/%s/%s' 2>/dev/null && echo "OPEN: %s:%s is reachable" || echo "CLOSED: %s:%s is not reachable"
echo ""
echo "--- Nmap (if available) ---"
nmap -p %s %s 2>/dev/null || echo "nmap not available"`, host, port, host, port, host, port, host, port, port, host)

	case "connections":
		return fmt.Sprintf(`echo "===== Network Connections ====="
echo ""
echo "--- Listening Ports ---"
ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null
echo ""
echo "--- Established Connections ---"
ss -tnp 2>/dev/null | head -n %d || netstat -tnp 2>/dev/null | head -n %d
echo ""
echo "--- Connection Summary ---"
ss -s 2>/dev/null || netstat -s 2>/dev/null | head -20`, count*5, count*5)

	case "interfaces":
		return `echo "===== Network Interfaces ====="
echo ""
echo "--- Interface List ---"
ip addr show 2>/dev/null || ifconfig -a 2>/dev/null
echo ""
echo "--- Interface Statistics ---"
ip -s link show 2>/dev/null | head -40
echo ""
echo "--- ARP Table ---"
ip neigh show 2>/dev/null | head -20 || arp -a 2>/dev/null | head -20`

	case "routes":
		return `echo "===== Routing Table ====="
echo ""
echo "--- Routes ---"
ip route show 2>/dev/null || route -n 2>/dev/null || netstat -rn 2>/dev/null
echo ""
echo "--- Default Gateway ---"
ip route show default 2>/dev/null`

	case "bandwidth":
		return `echo "===== Network Bandwidth ====="
echo ""
echo "--- Interface Traffic (1s snapshot) ---"
IFACE=$(ip route show default 2>/dev/null | awk '{print $5}' | head -1)
if [ -n "$IFACE" ]; then
    RX1=$(cat /sys/class/net/$IFACE/statistics/rx_bytes 2>/dev/null)
    TX1=$(cat /sys/class/net/$IFACE/statistics/tx_bytes 2>/dev/null)
    sleep 1
    RX2=$(cat /sys/class/net/$IFACE/statistics/rx_bytes 2>/dev/null)
    TX2=$(cat /sys/class/net/$IFACE/statistics/tx_bytes 2>/dev/null)
    RX_RATE=$(( (RX2 - RX1) / 1024 ))
    TX_RATE=$(( (TX2 - TX1) / 1024 ))
    echo "Interface: $IFACE"
    echo "RX Rate: ${RX_RATE} KB/s"
    echo "TX Rate: ${TX_RATE} KB/s"
else
    echo "Could not determine default interface"
fi
echo ""
echo "--- Total Traffic ---"
cat /proc/net/dev 2>/dev/null | head -10`

	default:
		return fmt.Sprintf("echo 'Unknown action: %s. Supported: ping, traceroute, dns-lookup, port-check, connections, interfaces, routes, bandwidth'", action)
	}
}
