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

// LogAnalyzeParams are the parameters for the log_analyze tool.
type LogAnalyzeParams struct {
	Source      string `json:"source" description:"Log source path or name (e.g. /var/log/syslog, /var/log/auth.log, journalctl)"`
	Pattern     string `json:"pattern,omitempty" description:"Search pattern (regex) to filter log entries"`
	TimeRange   string `json:"time_range,omitempty" description:"Time range to search: 15m, 1h, 24h, 7d (default: 1h)"`
	Severity    string `json:"severity,omitempty" description:"Filter by severity level: ERROR, WARN, INFO, DEBUG"`
	AggregateBy string `json:"aggregate_by,omitempty" description:"Aggregate results by: host, service, message, hour"`
	MaxLines    int    `json:"max_lines,omitempty" description:"Maximum number of log lines to return (default: 200)"`
}

type LogAnalyzeResponseMetadata struct {
	Source      string `json:"source"`
	Pattern     string `json:"pattern"`
	TimeRange   string `json:"time_range"`
	TotalLines  int    `json:"total_lines"`
	MatchCount  int    `json:"match_count"`
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
}

const LogAnalyzeToolName = "log_analyze"

//go:embed log_analyze.md
var logAnalyzeDescription []byte

func NewLogAnalyzeTool(
	permissions permission.Service,
	sandboxExec *sandbox.Executor,
	riskAssessor *security.RiskAssessor,
	auditLogger *audit.Logger,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LogAnalyzeToolName,
		string(logAnalyzeDescription),
		func(ctx context.Context, params LogAnalyzeParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Source == "" {
				return fantasy.NewTextErrorResponse("source is required"), nil
			}

			// Default values
			timeRange := params.TimeRange
			if timeRange == "" {
				timeRange = "1h"
			}
			maxLines := params.MaxLines
			if maxLines == 0 {
				maxLines = 200
			}

			// Risk assessment
			risk := riskAssessor.AssessToolCall(LogAnalyzeToolName, "analyze", params.Source)

			// Permission check
			sessionID := GetSessionFromContext(ctx)
			approved, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				ToolCallID:  call.ID,
				ToolName:    LogAnalyzeToolName,
				Action:      "analyze",
				Description: fmt.Sprintf("Analyze logs from %s (pattern: %s, range: %s, risk: %s)",
					params.Source, params.Pattern, timeRange, risk.Level),
				Path: params.Source,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !approved {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			start := time.Now()

			// Build the log analysis command
			cmd := buildLogAnalyzeCommand(params, timeRange, maxLines)

			// Execute in sandbox
			sandboxCfg := sandbox.DefaultConfig()
			sandboxCfg.AllowNetwork = false
			sandboxCfg.WorkingDir = workingDir
			sandboxCfg.Timeout = 2 * time.Minute

			result, execErr := sandboxExec.Execute(ctx, cmd, &sandboxCfg)

			// Audit log
			if auditLogger != nil {
				auditLogger.Log(ctx, audit.Event{
					SessionID: sessionID,
					Actor:     "agent",
					Action:    audit.ActionLogAnalyze,
					ToolName:  LogAnalyzeToolName,
					Description: fmt.Sprintf("Log analysis: source=%s pattern=%s range=%s",
						params.Source, params.Pattern, timeRange),
					Resource: audit.Resource{
						Type: audit.ResourceFile,
						Name: params.Source,
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
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Log analysis failed: %v\n%s", execErr, result.Stderr)), nil
			}

			output := result.Stdout
			if output == "" {
				output = "No log entries found matching the criteria."
			}

			metadata := LogAnalyzeResponseMetadata{
				Source:    params.Source,
				Pattern:   params.Pattern,
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

func buildLogAnalyzeCommand(params LogAnalyzeParams, timeRange string, maxLines int) string {
	source := params.Source

	// Validate inputs against shell injection
	if msg := security.ValidateNoShellMeta(source); msg != "" {
		return fmt.Sprintf("echo 'Error: invalid source: %s'", msg)
	}
	if params.Pattern != "" {
		if msg := security.ValidateNoShellMeta(params.Pattern); msg != "" {
			return fmt.Sprintf("echo 'Error: invalid pattern: %s'", msg)
		}
	}

	// If source looks like a journalctl unit, use journalctl
	if strings.HasPrefix(source, "journalctl") || strings.HasPrefix(source, "systemd:") {
		unit := strings.TrimPrefix(source, "systemd:")
		cmd := fmt.Sprintf("journalctl --no-pager --since %s -u %s",
			security.ShellQuote(timeRange+" ago"), security.ShellQuote(unit))
		if params.Severity != "" {
			priorities := map[string]string{
				"ERROR": "3", "WARN": "4", "INFO": "6", "DEBUG": "7",
			}
			if p, ok := priorities[strings.ToUpper(params.Severity)]; ok {
				cmd += fmt.Sprintf(" -p %s", p)
			}
		}
		if params.Pattern != "" {
			cmd += fmt.Sprintf(" --grep=%s", security.ShellQuote(params.Pattern))
		}
		cmd += fmt.Sprintf(" | tail -n %d", maxLines)
		return cmd
	}

	// File-based log analysis
	var parts []string

	parts = append(parts, fmt.Sprintf("cat %s 2>/dev/null", security.ShellQuote(source)))

	// Apply severity filter
	if params.Severity != "" {
		parts = append(parts, fmt.Sprintf("grep -i %s", security.ShellQuote(params.Severity)))
	}

	// Apply pattern filter
	if params.Pattern != "" {
		parts = append(parts, fmt.Sprintf("grep -E %s", security.ShellQuote(params.Pattern)))
	}

	// Limit output
	parts = append(parts, fmt.Sprintf("tail -n %d", maxLines))

	// Aggregation
	if params.AggregateBy != "" {
		switch params.AggregateBy {
		case "message":
			parts = append(parts, "sort | uniq -c | sort -rn | head -50")
		case "hour":
			parts = append(parts, "awk '{print $1, $2, substr($3,1,2)\":00\"}' | sort | uniq -c | sort -rn")
		case "host":
			parts = append(parts, "awk '{print $4}' | sort | uniq -c | sort -rn | head -30")
		case "service":
			parts = append(parts, "awk -F'[][]' '{print $2}' | sort | uniq -c | sort -rn | head -30")
		}
	}

	return strings.Join(parts, " | ")
}

func resultStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

func truncateString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
