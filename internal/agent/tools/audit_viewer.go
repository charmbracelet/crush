package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/audit"
)

// AuditViewerParams are the parameters for the audit_viewer tool.
type AuditViewerParams struct {
	Action      string `json:"action" description:"Operation: query, verify, summary"`
	Actor       string `json:"actor,omitempty" description:"Filter by actor name"`
	EventAction string `json:"event_action,omitempty" description:"Filter by event action (e.g. security_scan, command_execute)"`
	RiskLevel   string `json:"risk_level,omitempty" description:"Filter by risk level: LOW, MEDIUM, HIGH, CRITICAL"`
	SessionID   string `json:"session_id,omitempty" description:"Filter by session ID"`
	Since       string `json:"since,omitempty" description:"Start time: ISO 8601 timestamp or relative duration (1h, 24h, 7d)"`
	Limit       int    `json:"limit,omitempty" description:"Max results (default 50, max 500)"`
}

const AuditViewerToolName = "audit_viewer"

//go:embed audit_viewer.md
var auditViewerDescription []byte

// NewAuditViewerTool creates an audit_viewer tool backed by the given logger.
func NewAuditViewerTool(auditLogger *audit.Logger) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		AuditViewerToolName,
		string(auditViewerDescription),
		func(ctx context.Context, params AuditViewerParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Action == "" {
				return fantasy.NewTextErrorResponse("action is required: query, verify, or summary"), nil
			}

			switch params.Action {
			case "verify":
				return handleAuditVerify(auditLogger)
			case "summary":
				return handleAuditSummary(ctx, auditLogger, params)
			case "query":
				return handleAuditQuery(ctx, auditLogger, params)
			default:
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("unknown action %q — must be one of: query, verify, summary", params.Action),
				), nil
			}
		},
	)
}

// handleAuditVerify checks the HMAC integrity chain.
func handleAuditVerify(logger *audit.Logger) (fantasy.ToolResponse, error) {
	ok, count := logger.Verify()
	if ok {
		return fantasy.NewTextResponse(fmt.Sprintf(
			"✓ Audit chain integrity OK — %d event(s) verified.", count,
		)), nil
	}
	return fantasy.NewTextResponse(fmt.Sprintf(
		"✗ Audit chain INTEGRITY FAILURE detected at event index %d. "+
			"The audit log may have been tampered with.", count,
	)), nil
}

// handleAuditQuery queries the audit log with the provided filters.
func handleAuditQuery(ctx context.Context, logger *audit.Logger, params AuditViewerParams) (fantasy.ToolResponse, error) {
	filter, err := buildFilter(params)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	events := logger.Query(ctx, filter)
	if len(events) == 0 {
		return fantasy.NewTextResponse("No audit events match the specified filters."), nil
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Audit Log Query Results (%d events)\n\n", len(events))
	fmt.Fprintf(&sb, "| Time | Actor | Action | Resource | Risk | Status |\n")
	fmt.Fprintf(&sb, "|------|-------|--------|----------|------|--------|\n")

	for _, ev := range events {
		fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s(%d) | %s |\n",
			ev.Timestamp.Format("2006-01-02 15:04:05"),
			ev.Actor,
			ev.Action,
			ev.Resource.Name,
			ev.RiskLevel,
			ev.RiskScore,
			ev.Result.Status,
		)
	}

	return fantasy.NewTextResponse(sb.String()), nil
}

// handleAuditSummary produces statistical aggregates.
func handleAuditSummary(ctx context.Context, logger *audit.Logger, params AuditViewerParams) (fantasy.ToolResponse, error) {
	filter, err := buildFilter(params)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	// For summary, get all matching events (up to 500).
	filter.Limit = 500
	events := logger.Query(ctx, filter)

	if len(events) == 0 {
		return fantasy.NewTextResponse("No audit events found for the specified filters."), nil
	}

	actionCounts := make(map[audit.EventAction]int)
	riskCounts := make(map[string]int)
	actorCounts := make(map[string]int)
	successCount, deniedCount, errorCount := 0, 0, 0

	for _, ev := range events {
		actionCounts[ev.Action]++
		riskCounts[ev.RiskLevel]++
		actorCounts[ev.Actor]++
		switch ev.Result.Status {
		case "success":
			successCount++
		case "denied":
			deniedCount++
		default:
			errorCount++
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Audit Log Summary (%d events)\n\n", len(events))

	fmt.Fprintf(&sb, "### By Risk Level\n")
	for _, level := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"} {
		if n := riskCounts[level]; n > 0 {
			fmt.Fprintf(&sb, "- %s: %d\n", level, n)
		}
	}

	fmt.Fprintf(&sb, "\n### By Action\n")
	for action, count := range actionCounts {
		fmt.Fprintf(&sb, "- %s: %d\n", action, count)
	}

	fmt.Fprintf(&sb, "\n### By Actor\n")
	for actor, count := range actorCounts {
		fmt.Fprintf(&sb, "- %s: %d\n", actor, count)
	}

	fmt.Fprintf(&sb, "\n### Outcomes\n")
	fmt.Fprintf(&sb, "- Success: %d\n- Denied: %d\n- Error: %d\n",
		successCount, deniedCount, errorCount)

	return fantasy.NewTextResponse(sb.String()), nil
}

// buildFilter converts AuditViewerParams to an audit.Filter.
func buildFilter(params AuditViewerParams) (audit.Filter, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	filter := audit.Filter{
		Actor:     params.Actor,
		Action:    audit.EventAction(params.EventAction),
		RiskLevel: params.RiskLevel,
		SessionID: params.SessionID,
		Limit:     limit,
	}

	if params.Since != "" {
		start, err := parseSince(params.Since)
		if err != nil {
			return audit.Filter{}, fmt.Errorf("invalid since value %q: %w", params.Since, err)
		}
		filter.StartTime = start
	}

	return filter, nil
}

// parseSince interprets an ISO 8601 timestamp or a relative duration like "24h".
func parseSince(s string) (time.Time, error) {
	// Try ISO 8601 first.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try relative duration (e.g. "1h", "7d").
	if strings.HasSuffix(s, "d") {
		days, err := parsePositiveInt(s[:len(s)-1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid day count in %q", s)
		}
		return time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour), nil
	}

	dur, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().UTC().Add(-dur), nil
}

func parsePositiveInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("not a positive integer: %q", s)
	}
	return n, nil
}
