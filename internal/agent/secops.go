// Package agent - secops.go provides SecOps agent integration.
//
// This file registers the SecOps tools (log_analyze, compliance_check,
// security_scan, monitoring_query, network_diagnostics, certificate_audit)
// and wires them into the Crush agent framework alongside the security,
// sandbox, and audit subsystems.
package agent

import (
	"log/slog"
	"os"
	"path/filepath"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/audit"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/sandbox"
	"github.com/charmbracelet/crush/internal/security"
)

// SecOps tool name constants exported for use in agent configuration.
const (
	SecOpsLogAnalyze          = tools.LogAnalyzeToolName
	SecOpsComplianceCheck     = tools.ComplianceCheckToolName
	SecOpsSecurityScan        = tools.SecurityScanToolName
	SecOpsMonitoringQuery     = tools.MonitoringQueryToolName
	SecOpsNetworkDiagnostics  = tools.NetworkDiagnosticsToolName
	SecOpsCertificateAudit    = tools.CertificateAuditToolName
	SecOpsAuditViewer         = tools.AuditViewerToolName
)

// AllSecOpsToolNames returns the names of all SecOps tools for use in
// agent.AllowedTools configuration.
func AllSecOpsToolNames() []string {
	return []string{
		SecOpsLogAnalyze,
		SecOpsComplianceCheck,
		SecOpsSecurityScan,
		SecOpsMonitoringQuery,
		SecOpsNetworkDiagnostics,
		SecOpsCertificateAudit,
		SecOpsAuditViewer,
	}
}

// SecOpsComponents holds the initialized SecOps subsystem instances.
type SecOpsComponents struct {
	CapabilityManager *security.CapabilityManager
	RiskAssessor      *security.RiskAssessor
	SandboxExecutor   *sandbox.Executor
	AuditLogger       *audit.Logger
}

// InitSecOps initializes the SecOps subsystem components. The audit log is
// written to dataDir/audit/secops-audit.log. If dataDir is empty the log
// is written to the working directory.
func InitSecOps(dataDir string) (*SecOpsComponents, error) {
	auditDir := filepath.Join(dataDir, "audit")
	if err := os.MkdirAll(auditDir, 0700); err != nil {
		return nil, err
	}

	auditPath := filepath.Join(auditDir, "secops-audit.log")
	auditLogger, err := audit.NewLogger(auditPath, "crush-secops-hmac-key")
	if err != nil {
		return nil, err
	}

	slog.Info("SecOps audit logger initialized", "path", auditPath)

	return &SecOpsComponents{
		CapabilityManager: security.NewCapabilityManager(),
		RiskAssessor:      security.NewRiskAssessor(),
		SandboxExecutor:   sandbox.NewExecutor(sandbox.DefaultConfig()),
		AuditLogger:       auditLogger,
	}, nil
}

// BuildSecOpsTools creates the set of SecOps agent tools using the given
// components.
func BuildSecOpsTools(
	perm permission.Service,
	secops *SecOpsComponents,
	workingDir string,
) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		tools.NewLogAnalyzeTool(
			perm,
			secops.SandboxExecutor,
			secops.RiskAssessor,
			secops.AuditLogger,
			workingDir,
		),
		tools.NewComplianceCheckTool(
			perm,
			secops.SandboxExecutor,
			secops.RiskAssessor,
			secops.AuditLogger,
			workingDir,
		),
		tools.NewSecurityScanTool(
			perm,
			secops.SandboxExecutor,
			secops.RiskAssessor,
			secops.AuditLogger,
			workingDir,
		),
		tools.NewMonitoringQueryTool(
			perm,
			secops.SandboxExecutor,
			secops.RiskAssessor,
			secops.AuditLogger,
			workingDir,
		),
		tools.NewNetworkDiagnosticsTool(
			perm,
			secops.SandboxExecutor,
			secops.RiskAssessor,
			secops.AuditLogger,
			workingDir,
		),
		tools.NewCertificateAuditTool(
			perm,
			secops.SandboxExecutor,
			secops.RiskAssessor,
			secops.AuditLogger,
			workingDir,
		),
		tools.NewAuditViewerTool(secops.AuditLogger),
	}
}
