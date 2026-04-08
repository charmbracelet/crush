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

// SecurityScanParams are the parameters for the security_scan tool.
type SecurityScanParams struct {
	ScanType string `json:"scan_type" description:"Scanner to use: trivy, grype, lynis, chkrootkit, rkhunter, secret-scan"`
	Target   string `json:"target" description:"Scan target: image name, file path, directory, or 'localhost'"`
	Severity string `json:"severity,omitempty" description:"Minimum severity to report: CRITICAL, HIGH, MEDIUM, LOW (default: MEDIUM)"`
	Format   string `json:"format,omitempty" description:"Output format: table (default), json, summary"`
}

type SecurityScanResponseMetadata struct {
	ScanType    string `json:"scan_type"`
	Target      string `json:"target"`
	Severity    string `json:"severity"`
	VulnCount   int    `json:"vuln_count"`
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
}

const SecurityScanToolName = "security_scan"

//go:embed security_scan.md
var securityScanDescription []byte

func NewSecurityScanTool(
	permissions permission.Service,
	sandboxExec *sandbox.Executor,
	riskAssessor *security.RiskAssessor,
	auditLogger *audit.Logger,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SecurityScanToolName,
		string(securityScanDescription),
		func(ctx context.Context, params SecurityScanParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.ScanType == "" {
				return fantasy.NewTextErrorResponse("scan_type is required (trivy, grype, lynis, chkrootkit, rkhunter, secret-scan)"), nil
			}
			if params.Target == "" {
				return fantasy.NewTextErrorResponse("target is required"), nil
			}

			severity := params.Severity
			if severity == "" {
				severity = "MEDIUM"
			}

			// Risk assessment
			risk := riskAssessor.AssessToolCall(SecurityScanToolName, "scan", params.Target)

			// Permission check
			sessionID := GetSessionFromContext(ctx)
			approved, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				ToolCallID:  call.ID,
				ToolName:    SecurityScanToolName,
				Action:      "scan",
				Description: fmt.Sprintf("Run %s security scan on %s (severity >= %s, risk: %s)",
					params.ScanType, params.Target, severity, risk.Level),
				Path: workingDir,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !approved {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			start := time.Now()

			// Build scan command
			cmd := buildScanCommand(params.ScanType, params.Target, severity, params.Format)

			// Execute in sandbox with higher resource limits for scanning
			sandboxCfg := sandbox.DefaultConfig()
			sandboxCfg.MaxMemoryBytes = 2 << 30 // 2GB
			sandboxCfg.AllowNetwork = false
			sandboxCfg.WorkingDir = workingDir
			sandboxCfg.Timeout = 10 * time.Minute

			result, execErr := sandboxExec.Execute(ctx, cmd, &sandboxCfg)

			// Audit log
			if auditLogger != nil {
				auditLogger.Log(ctx, audit.Event{
					SessionID: sessionID,
					Actor:     "agent",
					Action:    audit.ActionSecurityScan,
					ToolName:  SecurityScanToolName,
					Description: fmt.Sprintf("Security scan: type=%s target=%s severity=%s",
						params.ScanType, params.Target, severity),
					Resource: audit.Resource{
						Type: inferResourceType(params.Target),
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
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Security scan failed: %v\n%s", execErr, result.Stderr)), nil
			}

			output := result.Stdout
			if output == "" {
				output = "Security scan completed with no findings."
			}

			metadata := SecurityScanResponseMetadata{
				ScanType:  params.ScanType,
				Target:    params.Target,
				Severity:  severity,
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

func buildScanCommand(scanType, target, severity, format string) string {
	// Validate target against injection
	if msg := security.ValidateNoShellMeta(target); msg != "" {
		return fmt.Sprintf("echo 'Error: invalid target: %s'", msg)
	}

	qt := security.ShellQuote(target)

	switch strings.ToLower(scanType) {
	case "trivy":
		cmd := fmt.Sprintf("trivy --severity %s", strings.ToUpper(severity))
		if strings.Contains(target, ":") && !strings.HasPrefix(target, "/") {
			cmd += " image " + qt
		} else if target == "localhost" {
			cmd += " rootfs /"
		} else {
			cmd += " fs " + qt
		}
		if format == "json" {
			cmd += " --format json"
		} else if format == "summary" {
			cmd += " --format table --quiet"
		}
		return cmd

	case "grype":
		cmd := "grype " + qt
		cmd += " --only-fixed --fail-on " + strings.ToLower(severity)
		if format == "json" {
			cmd += " -o json"
		}
		return cmd

	case "lynis":
		if target == "localhost" || target == "/" {
			return "lynis audit system --quick --no-colors 2>&1"
		}
		return fmt.Sprintf("lynis audit system --quick --no-colors --rootdir %s 2>&1", qt)

	case "chkrootkit":
		return "chkrootkit -q 2>&1"

	case "rkhunter":
		return "rkhunter --check --skip-keypress --report-warnings-only 2>&1"

	case "secret-scan":
		// Search for hardcoded secrets in source code
		patterns := []string{
			`grep -rn --include='*.{go,py,js,ts,yaml,yml,json,env,conf,cfg,ini,properties,xml,sh}' -E '(password|secret|api[_-]?key|access[_-]?token|private[_-]?key)\s*[=:]\s*["\x27][^\s]{8,}' %s 2>/dev/null | head -100`,
			`grep -rn --include='*.{go,py,js,ts}' -E 'BEGIN (RSA|DSA|EC|OPENSSH) PRIVATE KEY' %s 2>/dev/null | head -20`,
			`find %s -name '.env' -o -name '*.pem' -o -name '*.key' -o -name 'credentials*' 2>/dev/null | head -20`,
		}
		var cmds []string
		for _, p := range patterns {
			cmds = append(cmds, fmt.Sprintf(p, qt))
		}
		return `echo "=== Secret Scan Results ==="; echo ""; echo "--- Hardcoded Secrets ---"; ` +
			cmds[0] + `; echo ""; echo "--- Private Keys ---"; ` +
			cmds[1] + `; echo ""; echo "--- Sensitive Files ---"; ` +
			cmds[2]

	default:
		return fmt.Sprintf("echo 'Unknown scanner: %s. Supported: trivy, grype, lynis, chkrootkit, rkhunter, secret-scan'", scanType)
	}
}

func inferResourceType(target string) audit.ResourceType {
	if strings.Contains(target, ":") && !strings.HasPrefix(target, "/") {
		return audit.ResourceContainer
	}
	if target == "localhost" || target == "/" {
		return audit.ResourceHost
	}
	return audit.ResourceFile
}
