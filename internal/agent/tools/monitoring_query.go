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

// MonitoringQueryParams are the parameters for the monitoring_query tool.
type MonitoringQueryParams struct {
	System    string `json:"system" description:"Monitoring system: prometheus, node-exporter, system-stats, docker-stats, disk-usage, process-top"`
	Query     string `json:"query,omitempty" description:"Query expression (PromQL for prometheus) or metric name"`
	TimeRange string `json:"time_range,omitempty" description:"Time range: 5m, 15m, 1h, 6h, 24h (default: 15m)"`
	TopN      int    `json:"top_n,omitempty" description:"Number of top results to return (default: 10)"`
}

type MonitoringQueryResponseMetadata struct {
	System    string `json:"system"`
	Query     string `json:"query"`
	TimeRange string `json:"time_range"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}

const MonitoringQueryToolName = "monitoring_query"

//go:embed monitoring_query.md
var monitoringQueryDescription []byte

func NewMonitoringQueryTool(
	permissions permission.Service,
	sandboxExec *sandbox.Executor,
	riskAssessor *security.RiskAssessor,
	auditLogger *audit.Logger,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MonitoringQueryToolName,
		string(monitoringQueryDescription),
		func(ctx context.Context, params MonitoringQueryParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.System == "" {
				return fantasy.NewTextErrorResponse("system is required (prometheus, node-exporter, system-stats, docker-stats, disk-usage, process-top)"), nil
			}

			timeRange := params.TimeRange
			if timeRange == "" {
				timeRange = "15m"
			}
			topN := params.TopN
			if topN == 0 {
				topN = 10
			}

			// Risk assessment (monitoring is read-only, low risk)
			risk := riskAssessor.AssessToolCall(MonitoringQueryToolName, "query", params.System)

			// Permission check
			sessionID := GetSessionFromContext(ctx)
			approved, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				ToolCallID:  call.ID,
				ToolName:    MonitoringQueryToolName,
				Action:      "query",
				Description: fmt.Sprintf("Query %s monitoring (range: %s, risk: %s)",
					params.System, timeRange, risk.Level),
				Path: workingDir,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !approved {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			start := time.Now()

			cmd := buildMonitoringCommand(params.System, params.Query, timeRange, topN)

			sandboxCfg := sandbox.DefaultConfig()
			sandboxCfg.AllowNetwork = (params.System == "prometheus") // only for prometheus API queries
			sandboxCfg.WorkingDir = workingDir
			sandboxCfg.Timeout = 1 * time.Minute

			result, execErr := sandboxExec.Execute(ctx, cmd, &sandboxCfg)

			if auditLogger != nil {
				auditLogger.Log(ctx, audit.Event{
					SessionID:   sessionID,
					Actor:       "agent",
					Action:      audit.ActionMonitoringQuery,
					ToolName:    MonitoringQueryToolName,
					Description: fmt.Sprintf("Monitoring query: system=%s range=%s", params.System, timeRange),
					Resource: audit.Resource{
						Type: audit.ResourceHost,
						Name: params.System,
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
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Monitoring query failed: %v\n%s", execErr, result.Stderr)), nil
			}

			output := result.Stdout
			if output == "" {
				output = "No monitoring data available."
			}

			metadata := MonitoringQueryResponseMetadata{
				System:    params.System,
				Query:     params.Query,
				TimeRange: timeRange,
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

func buildMonitoringCommand(system, query, timeRange string, topN int) string {
	switch strings.ToLower(system) {
	case "system-stats":
		return fmt.Sprintf(`echo "===== System Statistics ====="
echo ""
echo "--- CPU ---"
top -bn1 2>/dev/null | head -5 || uptime
echo ""
echo "--- Memory ---"
free -h 2>/dev/null || cat /proc/meminfo 2>/dev/null | head -5
echo ""
echo "--- Load Average ---"
cat /proc/loadavg 2>/dev/null
echo ""
echo "--- Uptime ---"
uptime 2>/dev/null
echo ""
echo "--- Top %d Processes by CPU ---"
ps aux --sort=-pcpu 2>/dev/null | head -n %d || ps aux 2>/dev/null | head -n %d`, topN, topN+1, topN+1)

	case "disk-usage":
		return fmt.Sprintf(`echo "===== Disk Usage ====="
echo ""
echo "--- Filesystem Usage ---"
df -h 2>/dev/null
echo ""
echo "--- Inode Usage ---"
df -i 2>/dev/null
echo ""
echo "--- Top %d Largest Directories ---"
du -sh /* 2>/dev/null | sort -rh | head -n %d
echo ""
echo "--- Disk I/O ---"
iostat -x 1 1 2>/dev/null || cat /proc/diskstats 2>/dev/null | head -10`, topN, topN)

	case "process-top":
		return fmt.Sprintf(`echo "===== Process Analysis ====="
echo ""
echo "--- Top %d by CPU ---"
ps aux --sort=-pcpu 2>/dev/null | head -n %d
echo ""
echo "--- Top %d by Memory ---"
ps aux --sort=-rss 2>/dev/null | head -n %d
echo ""
echo "--- Process Count ---"
echo "Total: $(ps aux 2>/dev/null | wc -l)"
echo "Running: $(ps aux 2>/dev/null | awk '$8 ~ /R/ {count++} END {print count+0}')"
echo "Zombie: $(ps aux 2>/dev/null | awk '$8 ~ /Z/ {count++} END {print count+0}')"
echo ""
echo "--- Open File Descriptors ---"
cat /proc/sys/fs/file-nr 2>/dev/null`, topN, topN+1, topN, topN+1)

	case "docker-stats":
		return fmt.Sprintf(`echo "===== Docker Statistics ====="
echo ""
echo "--- Running Containers ---"
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null || echo "Docker not available"
echo ""
echo "--- Container Resource Usage ---"
docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}' 2>/dev/null | head -n %d
echo ""
echo "--- Docker Disk Usage ---"
docker system df 2>/dev/null
echo ""
echo "--- Images ---"
docker images --format 'table {{.Repository}}\t{{.Tag}}\t{{.Size}}' 2>/dev/null | head -n %d`, topN+1, topN+1)

	case "node-exporter":
		return `echo "===== Node Exporter Metrics ====="
echo ""
if command -v curl >/dev/null 2>&1 && curl -s --max-time 5 http://localhost:9100/metrics >/dev/null 2>&1; then
    echo "--- CPU ---"
    curl -s http://localhost:9100/metrics 2>/dev/null | grep '^node_cpu_seconds_total' | head -10
    echo ""
    echo "--- Memory ---"
    curl -s http://localhost:9100/metrics 2>/dev/null | grep '^node_memory_' | grep -E '(MemTotal|MemFree|MemAvailable|Buffers|Cached)' | head -10
    echo ""
    echo "--- Disk ---"
    curl -s http://localhost:9100/metrics 2>/dev/null | grep '^node_filesystem_' | grep -v '#' | head -10
else
    echo "Node exporter not available at localhost:9100"
    echo "Falling back to system stats..."
    free -h 2>/dev/null
    df -h 2>/dev/null
fi`

	case "prometheus":
		if query == "" {
			query = "up"
		}
		return fmt.Sprintf(`echo "===== Prometheus Query ====="
echo "Query: %s"
echo "Range: %s"
echo ""
if command -v curl >/dev/null 2>&1; then
    curl -s --max-time 10 'http://localhost:9090/api/v1/query?query=%s' 2>/dev/null | python3 -m json.tool 2>/dev/null || curl -s --max-time 10 'http://localhost:9090/api/v1/query?query=%s' 2>/dev/null
else
    echo "curl not available for Prometheus query"
fi`, query, timeRange, query, query)

	default:
		return fmt.Sprintf("echo 'Unknown monitoring system: %s. Supported: system-stats, disk-usage, process-top, docker-stats, node-exporter, prometheus'", system)
	}
}
