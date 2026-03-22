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

// CertificateAuditParams are the parameters for the certificate_audit tool.
type CertificateAuditParams struct {
	Action string `json:"action" description:"Audit action: check-expiry, inspect, verify-chain, scan-dir, check-host"`
	Target string `json:"target" description:"Certificate file path, directory, or hostname:port"`
	WarnDays int  `json:"warn_days,omitempty" description:"Days before expiry to warn (default: 30)"`
}

type CertificateAuditResponseMetadata struct {
	Action    string `json:"action"`
	Target    string `json:"target"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
}

const CertificateAuditToolName = "certificate_audit"

//go:embed certificate_audit.md
var certificateAuditDescription []byte

func NewCertificateAuditTool(
	permissions permission.Service,
	sandboxExec *sandbox.Executor,
	riskAssessor *security.RiskAssessor,
	auditLogger *audit.Logger,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CertificateAuditToolName,
		string(certificateAuditDescription),
		func(ctx context.Context, params CertificateAuditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Action == "" {
				return fantasy.NewTextErrorResponse("action is required (check-expiry, inspect, verify-chain, scan-dir, check-host)"), nil
			}
			if params.Target == "" {
				return fantasy.NewTextErrorResponse("target is required"), nil
			}

			warnDays := params.WarnDays
			if warnDays == 0 {
				warnDays = 30
			}

			risk := riskAssessor.AssessToolCall(CertificateAuditToolName, params.Action, params.Target)

			sessionID := GetSessionFromContext(ctx)
			approved, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				ToolCallID:  call.ID,
				ToolName:    CertificateAuditToolName,
				Action:      params.Action,
				Description: fmt.Sprintf("Certificate audit: %s on %s (risk: %s)",
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

			cmd := buildCertificateCommand(params.Action, params.Target, warnDays)

			sandboxCfg := sandbox.DefaultConfig()
			sandboxCfg.AllowNetwork = (params.Action == "check-host") // only for remote host checks
			sandboxCfg.WorkingDir = workingDir
			sandboxCfg.Timeout = 2 * time.Minute

			result, execErr := sandboxExec.Execute(ctx, cmd, &sandboxCfg)

			if auditLogger != nil {
				auditLogger.Log(ctx, audit.Event{
					SessionID: sessionID,
					Actor:     "agent",
					Action:    audit.ActionCertificateAudit,
					ToolName:  CertificateAuditToolName,
					Description: fmt.Sprintf("Certificate audit: action=%s target=%s",
						params.Action, params.Target),
					Resource: audit.Resource{
						Type: audit.ResourceCertificate,
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
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Certificate audit failed: %v\n%s", execErr, result.Stderr)), nil
			}

			output := result.Stdout
			if output == "" {
				output = "Certificate audit completed with no output."
			}

			metadata := CertificateAuditResponseMetadata{
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

func buildCertificateCommand(action, target string, warnDays int) string {
	switch strings.ToLower(action) {
	case "check-expiry":
		return fmt.Sprintf(`echo "===== Certificate Expiry Check ====="
echo ""
if [ -f "%s" ]; then
    ENDDATE=$(openssl x509 -enddate -noout -in "%s" 2>/dev/null | cut -d= -f2)
    ENDSEC=$(date -d "$ENDDATE" +%%s 2>/dev/null)
    NOWSEC=$(date +%%s)
    if [ -n "$ENDSEC" ]; then
        DAYS_LEFT=$(( (ENDSEC - NOWSEC) / 86400 ))
        SUBJECT=$(openssl x509 -subject -noout -in "%s" 2>/dev/null)
        echo "File: %s"
        echo "Subject: $SUBJECT"
        echo "Expires: $ENDDATE"
        echo "Days until expiry: $DAYS_LEFT"
        if [ "$DAYS_LEFT" -le 0 ]; then
            echo "STATUS: [EXPIRED]"
        elif [ "$DAYS_LEFT" -le %d ]; then
            echo "STATUS: [WARNING] Expires within %d days"
        else
            echo "STATUS: [OK]"
        fi
    else
        echo "Could not parse certificate date"
    fi
else
    echo "File not found: %s"
fi`, target, target, target, target, warnDays, warnDays, target)

	case "inspect":
		return fmt.Sprintf(`echo "===== Certificate Inspection ====="
echo ""
if [ -f "%s" ]; then
    echo "--- Subject & Issuer ---"
    openssl x509 -in "%s" -noout -subject -issuer 2>/dev/null
    echo ""
    echo "--- Validity ---"
    openssl x509 -in "%s" -noout -dates 2>/dev/null
    echo ""
    echo "--- Serial Number ---"
    openssl x509 -in "%s" -noout -serial 2>/dev/null
    echo ""
    echo "--- Fingerprint ---"
    openssl x509 -in "%s" -noout -fingerprint -sha256 2>/dev/null
    echo ""
    echo "--- Subject Alternative Names ---"
    openssl x509 -in "%s" -noout -text 2>/dev/null | grep -A1 "Subject Alternative Name"
    echo ""
    echo "--- Key Usage ---"
    openssl x509 -in "%s" -noout -text 2>/dev/null | grep -A1 "Key Usage"
    echo ""
    echo "--- Signature Algorithm ---"
    openssl x509 -in "%s" -noout -text 2>/dev/null | grep "Signature Algorithm" | head -1
else
    echo "File not found: %s"
fi`, target, target, target, target, target, target, target, target, target)

	case "verify-chain":
		return fmt.Sprintf(`echo "===== Certificate Chain Verification ====="
echo ""
if [ -f "%s" ]; then
    echo "--- Chain Verification ---"
    openssl verify -verbose "%s" 2>&1
    echo ""
    echo "--- Chain Details ---"
    openssl crl2pkcs7 -nocrl -certfile "%s" 2>/dev/null | openssl pkcs7 -print_certs -noout 2>/dev/null
else
    echo "File not found: %s"
fi`, target, target, target, target)

	case "scan-dir":
		return fmt.Sprintf(`echo "===== Certificate Directory Scan ====="
echo "Scanning: %s"
echo "Warning threshold: %d days"
echo ""
FOUND=0
EXPIRED=0
WARNING=0
OK=0
NOW=$(date +%%s)
for CERT in $(find "%s" -type f \( -name '*.pem' -o -name '*.crt' -o -name '*.cer' -o -name '*.cert' \) 2>/dev/null); do
    ENDDATE=$(openssl x509 -enddate -noout -in "$CERT" 2>/dev/null | cut -d= -f2)
    if [ -z "$ENDDATE" ]; then continue; fi
    FOUND=$((FOUND+1))
    ENDSEC=$(date -d "$ENDDATE" +%%s 2>/dev/null)
    if [ -z "$ENDSEC" ]; then continue; fi
    DAYS_LEFT=$(( (ENDSEC - NOW) / 86400 ))
    SUBJECT=$(openssl x509 -subject -noout -in "$CERT" 2>/dev/null | sed 's/subject=//')
    if [ "$DAYS_LEFT" -le 0 ]; then
        echo "[EXPIRED] $CERT ($SUBJECT) - Expired $((DAYS_LEFT * -1)) days ago"
        EXPIRED=$((EXPIRED+1))
    elif [ "$DAYS_LEFT" -le %d ]; then
        echo "[WARNING] $CERT ($SUBJECT) - Expires in $DAYS_LEFT days"
        WARNING=$((WARNING+1))
    else
        echo "[OK] $CERT ($SUBJECT) - Expires in $DAYS_LEFT days"
        OK=$((OK+1))
    fi
done
echo ""
echo "===== Summary ====="
echo "Certificates found: $FOUND"
echo "Expired: $EXPIRED"
echo "Warning (<%d days): $WARNING"
echo "OK: $OK"`, target, warnDays, target, warnDays, warnDays)

	case "check-host":
		if !strings.Contains(target, ":") {
			target = target + ":443"
		}
		return fmt.Sprintf(`echo "===== Remote Host Certificate Check ====="
echo "Host: %s"
echo ""
echo "--- Certificate ---"
echo | openssl s_client -servername %s -connect %s 2>/dev/null | openssl x509 -noout -subject -issuer -dates -fingerprint 2>/dev/null
echo ""
echo "--- Chain ---"
echo | openssl s_client -servername %s -connect %s -showcerts 2>/dev/null | grep -E '(subject|issuer|depth)' | head -20
echo ""
echo "--- Protocol & Cipher ---"
echo | openssl s_client -connect %s 2>/dev/null | grep -E '(Protocol|Cipher|Verify)'`,
			target,
			strings.Split(target, ":")[0], target,
			strings.Split(target, ":")[0], target,
			target)

	default:
		return fmt.Sprintf("echo 'Unknown action: %s. Supported: check-expiry, inspect, verify-chain, scan-dir, check-host'", action)
	}
}
