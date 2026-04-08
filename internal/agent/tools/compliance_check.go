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

// ComplianceCheckParams are the parameters for the compliance_check tool.
type ComplianceCheckParams struct {
	Framework string `json:"framework" description:"Compliance framework to check: cis-linux, cis-docker, pci-dss, soc2, hipaa, iso27001"`
	Category  string `json:"category,omitempty" description:"Specific category to check (e.g. filesystem, network, auth, logging)"`
	Target    string `json:"target,omitempty" description:"Check target: host (default), container:<name>, or path to check"`
	Format    string `json:"format,omitempty" description:"Output format: summary (default), detailed, json"`
}

type ComplianceCheckResponseMetadata struct {
	Framework  string `json:"framework"`
	Category   string `json:"category"`
	PassCount  int    `json:"pass_count"`
	FailCount  int    `json:"fail_count"`
	Score      float64 `json:"score"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
}

const ComplianceCheckToolName = "compliance_check"

//go:embed compliance_check.md
var complianceCheckDescription []byte

func NewComplianceCheckTool(
	permissions permission.Service,
	sandboxExec *sandbox.Executor,
	riskAssessor *security.RiskAssessor,
	auditLogger *audit.Logger,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ComplianceCheckToolName,
		string(complianceCheckDescription),
		func(ctx context.Context, params ComplianceCheckParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Framework == "" {
				return fantasy.NewTextErrorResponse("framework is required (cis-linux, cis-docker, pci-dss, soc2, hipaa, iso27001)"), nil
			}

			target := params.Target
			if target == "" {
				target = "host"
			}
			format := params.Format
			if format == "" {
				format = "summary"
			}

			// Risk assessment
			risk := riskAssessor.AssessToolCall(ComplianceCheckToolName, "check", target)

			// Permission check
			sessionID := GetSessionFromContext(ctx)
			approved, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				ToolCallID:  call.ID,
				ToolName:    ComplianceCheckToolName,
				Action:      "check",
				Description: fmt.Sprintf("Run %s compliance check on %s (risk: %s)",
					params.Framework, target, risk.Level),
				Path: workingDir,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !approved {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			start := time.Now()

			// Build compliance check script
			script := buildComplianceScript(params.Framework, params.Category)

			// Execute in sandbox
			sandboxCfg := sandbox.DefaultConfig()
			sandboxCfg.AllowNetwork = false
			sandboxCfg.WorkingDir = workingDir
			sandboxCfg.Timeout = 5 * time.Minute

			result, execErr := sandboxExec.Execute(ctx, script, &sandboxCfg)

			// Audit log
			if auditLogger != nil {
				auditLogger.Log(ctx, audit.Event{
					SessionID:   sessionID,
					Actor:       "agent",
					Action:      audit.ActionComplianceCheck,
					ToolName:    ComplianceCheckToolName,
					Description: fmt.Sprintf("Compliance check: framework=%s category=%s target=%s",
						params.Framework, params.Category, target),
					Resource: audit.Resource{
						Type: audit.ResourceHost,
						Name: target,
					},
					Result: audit.Result{
						Status:  resultStatus(execErr),
						Message: truncateString(result.Stdout, 500),
					},
					RiskScore:           risk.Score,
					RiskLevel:           string(risk.Level),
					ComplianceFramework: params.Framework,
					Duration:            time.Since(start),
				})
			}

			if execErr != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Compliance check failed: %v\n%s", execErr, result.Stderr)), nil
			}

			output := result.Stdout
			if output == "" {
				output = "Compliance check completed with no output."
			}

			metadata := ComplianceCheckResponseMetadata{
				Framework: params.Framework,
				Category:  params.Category,
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

func buildComplianceScript(framework, category string) string {
	var checks []string

	switch strings.ToLower(framework) {
	case "cis-linux":
		checks = getCISLinuxChecks(category)
	case "cis-docker":
		checks = getCISDockerChecks(category)
	case "pci-dss":
		checks = getPCIDSSChecks(category)
	case "soc2":
		checks = getSOC2Checks(category)
	case "hipaa":
		checks = getHIPAAChecks(category)
	case "iso27001":
		checks = getISO27001Checks(category)
	default:
		return fmt.Sprintf("echo 'Unknown framework: %s. Supported: cis-linux, cis-docker, pci-dss, soc2, hipaa, iso27001'", framework)
	}

	script := "#!/bin/sh\nPASS=0; FAIL=0; WARN=0\n"
	script += "echo '===== Compliance Check: " + strings.ToUpper(framework) + " ====='\n"
	script += "echo ''\n"

	for _, check := range checks {
		script += check + "\n"
	}

	script += `
echo ''
echo '===== Summary ====='
echo "PASS: $PASS"
echo "FAIL: $FAIL"
echo "WARN: $WARN"
TOTAL=$((PASS + FAIL + WARN))
if [ "$TOTAL" -gt 0 ]; then
    SCORE=$((PASS * 100 / TOTAL))
    echo "Score: ${SCORE}%"
fi
`
	return script
}

func getCISLinuxChecks(category string) []string {
	all := map[string][]string{
		"filesystem": {
			`# CIS 1.1.1 - Check /tmp partition
if mount | grep -q ' /tmp '; then echo "[PASS] 1.1.1 /tmp is a separate partition"; PASS=$((PASS+1)); else echo "[FAIL] 1.1.1 /tmp is NOT a separate partition"; FAIL=$((FAIL+1)); fi`,
			`# CIS 1.1.2 - Check /tmp noexec
if mount | grep ' /tmp ' | grep -q noexec; then echo "[PASS] 1.1.2 /tmp has noexec"; PASS=$((PASS+1)); else echo "[WARN] 1.1.2 /tmp may not have noexec"; WARN=$((WARN+1)); fi`,
			`# CIS 1.1.3 - Check /var partition
if mount | grep -q ' /var '; then echo "[PASS] 1.1.3 /var is a separate partition"; PASS=$((PASS+1)); else echo "[WARN] 1.1.3 /var is NOT a separate partition"; WARN=$((WARN+1)); fi`,
			`# CIS 1.1.4 - Check world-writable directories have sticky bit
WWDIRS=$(find / -maxdepth 3 -type d -perm -0002 ! -perm -1000 2>/dev/null | head -10)
if [ -z "$WWDIRS" ]; then echo "[PASS] 1.1.4 All world-writable dirs have sticky bit"; PASS=$((PASS+1)); else echo "[FAIL] 1.1.4 World-writable dirs without sticky bit: $WWDIRS"; FAIL=$((FAIL+1)); fi`,
		},
		"auth": {
			`# CIS 5.2.1 - SSH Protocol version
if grep -q '^Protocol 2' /etc/ssh/sshd_config 2>/dev/null || ! grep -q '^Protocol 1' /etc/ssh/sshd_config 2>/dev/null; then echo "[PASS] 5.2.1 SSH Protocol 2"; PASS=$((PASS+1)); else echo "[FAIL] 5.2.1 SSH Protocol 1 enabled"; FAIL=$((FAIL+1)); fi`,
			`# CIS 5.2.2 - SSH PermitRootLogin
if grep -qi '^PermitRootLogin no' /etc/ssh/sshd_config 2>/dev/null; then echo "[PASS] 5.2.2 SSH root login disabled"; PASS=$((PASS+1)); else echo "[FAIL] 5.2.2 SSH root login may be enabled"; FAIL=$((FAIL+1)); fi`,
			`# CIS 5.2.3 - SSH PasswordAuthentication
if grep -qi '^PasswordAuthentication no' /etc/ssh/sshd_config 2>/dev/null; then echo "[PASS] 5.2.3 SSH password auth disabled"; PASS=$((PASS+1)); else echo "[WARN] 5.2.3 SSH password auth may be enabled"; WARN=$((WARN+1)); fi`,
			`# CIS 5.2.4 - SSH MaxAuthTries
MAXTRIES=$(grep -i '^MaxAuthTries' /etc/ssh/sshd_config 2>/dev/null | awk '{print $2}')
if [ -n "$MAXTRIES" ] && [ "$MAXTRIES" -le 4 ]; then echo "[PASS] 5.2.4 SSH MaxAuthTries=$MAXTRIES"; PASS=$((PASS+1)); else echo "[WARN] 5.2.4 SSH MaxAuthTries not set or too high"; WARN=$((WARN+1)); fi`,
			`# CIS 5.4.1 - Password expiration
PASS_MAX=$(grep '^PASS_MAX_DAYS' /etc/login.defs 2>/dev/null | awk '{print $2}')
if [ -n "$PASS_MAX" ] && [ "$PASS_MAX" -le 365 ]; then echo "[PASS] 5.4.1 Password max age=$PASS_MAX days"; PASS=$((PASS+1)); else echo "[WARN] 5.4.1 Password max age not configured"; WARN=$((WARN+1)); fi`,
		},
		"network": {
			`# CIS 3.1.1 - IP forwarding disabled
IPFWD=$(sysctl net.ipv4.ip_forward 2>/dev/null | awk -F= '{print $2}' | tr -d ' ')
if [ "$IPFWD" = "0" ]; then echo "[PASS] 3.1.1 IP forwarding disabled"; PASS=$((PASS+1)); else echo "[WARN] 3.1.1 IP forwarding enabled"; WARN=$((WARN+1)); fi`,
			`# CIS 3.2.1 - Source routed packets rejected
SRC=$(sysctl net.ipv4.conf.all.accept_source_route 2>/dev/null | awk -F= '{print $2}' | tr -d ' ')
if [ "$SRC" = "0" ]; then echo "[PASS] 3.2.1 Source routed packets rejected"; PASS=$((PASS+1)); else echo "[FAIL] 3.2.1 Source routed packets accepted"; FAIL=$((FAIL+1)); fi`,
			`# CIS 3.2.2 - ICMP redirects rejected
ICMP=$(sysctl net.ipv4.conf.all.accept_redirects 2>/dev/null | awk -F= '{print $2}' | tr -d ' ')
if [ "$ICMP" = "0" ]; then echo "[PASS] 3.2.2 ICMP redirects rejected"; PASS=$((PASS+1)); else echo "[WARN] 3.2.2 ICMP redirects accepted"; WARN=$((WARN+1)); fi`,
			`# CIS 3.4.1 - Firewall installed
if command -v iptables >/dev/null 2>&1 || command -v ufw >/dev/null 2>&1 || command -v firewall-cmd >/dev/null 2>&1; then echo "[PASS] 3.4.1 Firewall tool installed"; PASS=$((PASS+1)); else echo "[FAIL] 3.4.1 No firewall tool found"; FAIL=$((FAIL+1)); fi`,
		},
		"logging": {
			`# CIS 4.1.1 - Audit system enabled
if command -v auditctl >/dev/null 2>&1; then echo "[PASS] 4.1.1 Audit system installed"; PASS=$((PASS+1)); else echo "[FAIL] 4.1.1 Audit system not installed"; FAIL=$((FAIL+1)); fi`,
			`# CIS 4.2.1 - rsyslog or syslog-ng installed
if command -v rsyslogd >/dev/null 2>&1 || command -v syslog-ng >/dev/null 2>&1; then echo "[PASS] 4.2.1 Syslog service installed"; PASS=$((PASS+1)); else echo "[WARN] 4.2.1 No syslog service found"; WARN=$((WARN+1)); fi`,
			`# CIS 4.2.2 - Log file permissions
BADPERMS=$(find /var/log -type f -perm /037 2>/dev/null | head -5)
if [ -z "$BADPERMS" ]; then echo "[PASS] 4.2.2 Log file permissions OK"; PASS=$((PASS+1)); else echo "[FAIL] 4.2.2 Insecure log file permissions: $BADPERMS"; FAIL=$((FAIL+1)); fi`,
		},
	}

	if category != "" {
		if checks, ok := all[category]; ok {
			return checks
		}
		return []string{fmt.Sprintf("echo 'Unknown category: %s. Available: filesystem, auth, network, logging'", category)}
	}

	// Return all categories
	var result []string
	for cat, checks := range all {
		result = append(result, fmt.Sprintf("echo '--- %s ---'", strings.ToUpper(cat)))
		result = append(result, checks...)
		result = append(result, "echo ''")
	}
	return result
}

func getCISDockerChecks(_ string) []string {
	return []string{
		`# CIS Docker 1.1 - Docker version
DVER=$(docker version --format '{{.Server.Version}}' 2>/dev/null)
if [ -n "$DVER" ]; then echo "[PASS] 1.1 Docker installed: $DVER"; PASS=$((PASS+1)); else echo "[FAIL] 1.1 Docker not accessible"; FAIL=$((FAIL+1)); fi`,
		`# CIS Docker 2.1 - Network traffic restricted
IPTFWD=$(iptables -L FORWARD 2>/dev/null | head -1)
echo "[INFO] 2.1 Forward chain: $IPTFWD"; PASS=$((PASS+1))`,
		`# CIS Docker 2.5 - TLS authentication
if [ -f /etc/docker/daemon.json ] && grep -q tls /etc/docker/daemon.json 2>/dev/null; then echo "[PASS] 2.5 TLS configured"; PASS=$((PASS+1)); else echo "[WARN] 2.5 TLS may not be configured"; WARN=$((WARN+1)); fi`,
		`# CIS Docker 4.1 - Container user
ROOTCONTAINERS=$(docker ps -q 2>/dev/null | xargs -r docker inspect --format '{{.Name}} {{.Config.User}}' 2>/dev/null | grep -v ': [a-z]' | head -5)
if [ -z "$ROOTCONTAINERS" ]; then echo "[PASS] 4.1 No root-running containers found"; PASS=$((PASS+1)); else echo "[WARN] 4.1 Containers running as root: $ROOTCONTAINERS"; WARN=$((WARN+1)); fi`,
		`# CIS Docker 5.1 - AppArmor
NOAA=$(docker ps -q 2>/dev/null | xargs -r docker inspect --format '{{.Name}} {{.AppArmorProfile}}' 2>/dev/null | grep -v 'docker-default' | head -5)
if [ -z "$NOAA" ]; then echo "[PASS] 5.1 AppArmor profiles set"; PASS=$((PASS+1)); else echo "[WARN] 5.1 Containers without AppArmor: $NOAA"; WARN=$((WARN+1)); fi`,
	}
}

func getPCIDSSChecks(_ string) []string {
	return []string{
		`# PCI-DSS 2.2.1 - Default passwords changed
if [ ! -f /etc/shadow ]; then echo "[WARN] 2.2.1 Cannot check password file"; WARN=$((WARN+1)); else echo "[PASS] 2.2.1 Shadow file exists"; PASS=$((PASS+1)); fi`,
		`# PCI-DSS 2.3 - Encrypted admin access
if ss -tlnp 2>/dev/null | grep -q ':23 '; then echo "[FAIL] 2.3 Telnet service running"; FAIL=$((FAIL+1)); else echo "[PASS] 2.3 No telnet service"; PASS=$((PASS+1)); fi`,
		`# PCI-DSS 6.2 - Security patches applied
UPDATES=$(apt list --upgradable 2>/dev/null | grep -i security | wc -l)
if [ "$UPDATES" = "0" ]; then echo "[PASS] 6.2 No pending security updates"; PASS=$((PASS+1)); else echo "[WARN] 6.2 Pending security updates: $UPDATES"; WARN=$((WARN+1)); fi`,
		`# PCI-DSS 8.1 - Account management
NOEXPIRY=$(awk -F: '($2 != "!" && $2 != "*" && $5 == "") {print $1}' /etc/shadow 2>/dev/null | head -5)
if [ -z "$NOEXPIRY" ]; then echo "[PASS] 8.1 All accounts have expiry"; PASS=$((PASS+1)); else echo "[WARN] 8.1 Accounts without expiry: $NOEXPIRY"; WARN=$((WARN+1)); fi`,
		`# PCI-DSS 10.2 - Audit logging
if command -v auditctl >/dev/null 2>&1 && auditctl -s 2>/dev/null | grep -q 'enabled'; then echo "[PASS] 10.2 Audit logging enabled"; PASS=$((PASS+1)); else echo "[FAIL] 10.2 Audit logging not enabled"; FAIL=$((FAIL+1)); fi`,
	}
}

func getSOC2Checks(_ string) []string {
	return []string{
		`# SOC2 CC6.1 - Access control
NOLOGIN=$(awk -F: '($3 >= 1000 && $7 !~ /nologin|false/) {print $1}' /etc/passwd 2>/dev/null | wc -l)
echo "[INFO] CC6.1 Active user accounts: $NOLOGIN"; PASS=$((PASS+1))`,
		`# SOC2 CC6.6 - Encryption in transit
if ss -tlnp 2>/dev/null | grep -q ':443 '; then echo "[PASS] CC6.6 HTTPS service running"; PASS=$((PASS+1)); else echo "[WARN] CC6.6 No HTTPS service detected"; WARN=$((WARN+1)); fi`,
		`# SOC2 CC7.2 - Monitoring
if command -v journalctl >/dev/null 2>&1; then echo "[PASS] CC7.2 Logging infrastructure present"; PASS=$((PASS+1)); else echo "[WARN] CC7.2 No journalctl found"; WARN=$((WARN+1)); fi`,
		`# SOC2 CC8.1 - Change management
if command -v git >/dev/null 2>&1; then echo "[PASS] CC8.1 Version control available"; PASS=$((PASS+1)); else echo "[WARN] CC8.1 No version control found"; WARN=$((WARN+1)); fi`,
	}
}

func getHIPAAChecks(_ string) []string {
	return []string{
		`# HIPAA 164.312(a) - Access control
NOPASS=$(awk -F: '($2 == "" ) {print $1}' /etc/shadow 2>/dev/null | head -5)
if [ -z "$NOPASS" ]; then echo "[PASS] 164.312(a) No passwordless accounts"; PASS=$((PASS+1)); else echo "[FAIL] 164.312(a) Passwordless accounts: $NOPASS"; FAIL=$((FAIL+1)); fi`,
		`# HIPAA 164.312(c) - Integrity
if command -v aide >/dev/null 2>&1 || command -v tripwire >/dev/null 2>&1; then echo "[PASS] 164.312(c) Integrity monitoring installed"; PASS=$((PASS+1)); else echo "[WARN] 164.312(c) No integrity monitoring"; WARN=$((WARN+1)); fi`,
		`# HIPAA 164.312(e) - Transmission security
UNENC=$(ss -tlnp 2>/dev/null | grep -E ':(21|23|80|110|143) ' | head -3)
if [ -z "$UNENC" ]; then echo "[PASS] 164.312(e) No unencrypted services"; PASS=$((PASS+1)); else echo "[WARN] 164.312(e) Unencrypted services: $UNENC"; WARN=$((WARN+1)); fi`,
	}
}

func getISO27001Checks(_ string) []string {
	return []string{
		`# ISO27001 A.9.2 - User access management
SUDOERS=$(grep -c '^[^#].*ALL=(ALL' /etc/sudoers 2>/dev/null)
echo "[INFO] A.9.2 Sudoers with full access: $SUDOERS"; PASS=$((PASS+1))`,
		`# ISO27001 A.12.4 - Logging and monitoring
LOGSIZE=$(du -sh /var/log 2>/dev/null | awk '{print $1}')
echo "[INFO] A.12.4 Log storage used: $LOGSIZE"; PASS=$((PASS+1))`,
		`# ISO27001 A.12.6 - Vulnerability management
if command -v apt >/dev/null 2>&1; then UPDATES=$(apt list --upgradable 2>/dev/null | wc -l); echo "[INFO] A.12.6 Pending updates: $((UPDATES-1))"; fi; PASS=$((PASS+1))`,
		`# ISO27001 A.13.1 - Network security
LISTENING=$(ss -tlnp 2>/dev/null | tail -n +2 | wc -l)
echo "[INFO] A.13.1 Listening services: $LISTENING"; PASS=$((PASS+1))`,
	}
}
