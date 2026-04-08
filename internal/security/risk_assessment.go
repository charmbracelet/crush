package security

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// RiskLevel classifies the severity of an assessed risk.
type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// RiskAction describes the required approval flow for a given risk level.
type RiskAction string

const (
	ActionAutoApprove RiskAction = "auto_approve"
	ActionUserConfirm RiskAction = "user_confirm"
	ActionAdminReview RiskAction = "admin_review"
	ActionBlock       RiskAction = "block"
)

// RiskFactor is a single contributing factor to the overall risk score.
type RiskFactor struct {
	Name     string `json:"name"`
	Weight   int    `json:"weight"`
	Evidence string `json:"evidence"`
}

// RiskAssessment is the result of evaluating a command or action.
type RiskAssessment struct {
	Score   int          `json:"score"`
	Level   RiskLevel    `json:"level"`
	Action  RiskAction   `json:"action"`
	Factors []RiskFactor `json:"factors"`
}

// RiskAssessor evaluates the risk of commands and actions.
type RiskAssessor struct {
	bannedCommands      []string
	sensitivePatterns   []string
	credentialPatterns  []*regexp.Regexp
	systemModifyCommands []string
}

// NewRiskAssessor creates a risk assessor with sane defaults.
func NewRiskAssessor() *RiskAssessor {
	return &RiskAssessor{
		bannedCommands: []string{
			"rm -rf /",
			"mkfs",
			"dd if=",
			":(){ :|:& };:",
			"> /dev/sda",
			"chmod -R 777 /",
			"chown -R",
		},
		sensitivePatterns: []string{
			"/etc/shadow",
			"/etc/passwd",
			"/etc/sudoers",
			"/root/.ssh",
			"/.aws/credentials",
			"/.aws/config",
			"/.kube/config",
			"/.docker/config.json",
			"/proc/*/environ",
			"/var/log/auth.log",
			"/var/log/secure",
			"/etc/ssl/private",
			"/.gnupg/",
			"/.ssh/id_",
			"/.env",
		},
		credentialPatterns: compilePatterns([]string{
			`(?i)password\s*[=:]`,
			`(?i)api[_-]?key\s*[=:]`,
			`(?i)secret[_-]?key\s*[=:]`,
			`(?i)access[_-]?token\s*[=:]`,
			`(?i)--password[\s=]`,
			`(?i)-p\s+['"]`,
			`(?i)bearer\s+[a-zA-Z0-9\-._~+/]+=*`,
			`(?i)BEGIN\s+(RSA|DSA|EC|OPENSSH)\s+PRIVATE\s+KEY`,
		}),
		systemModifyCommands: []string{
			"chown", "chmod", "chgrp",
			"useradd", "userdel", "usermod",
			"groupadd", "groupdel", "groupmod",
			"passwd", "visudo",
			"systemctl enable", "systemctl disable",
			"systemctl start", "systemctl stop", "systemctl restart",
			"service",
			"iptables", "ufw",
			"mount", "umount",
			"fdisk", "parted", "mkfs",
			"crontab -e", "crontab -r",
		},
	}
}

// AssessCommand evaluates a shell command and returns its risk assessment.
func (ra *RiskAssessor) AssessCommand(cmd string) RiskAssessment {
	assessment := RiskAssessment{
		Factors: make([]RiskFactor, 0),
	}

	cmdLower := strings.ToLower(strings.TrimSpace(cmd))

	// 1. Check banned/destructive commands
	for _, banned := range ra.bannedCommands {
		if strings.Contains(cmdLower, strings.ToLower(banned)) {
			assessment.Factors = append(assessment.Factors, RiskFactor{
				Name:     "destructive_command",
				Weight:   50,
				Evidence: "Contains destructive command pattern: " + banned,
			})
		}
	}

	// 2. Check sensitive file access
	for _, pattern := range ra.sensitivePatterns {
		if pathMatch(cmd, pattern) {
			assessment.Factors = append(assessment.Factors, RiskFactor{
				Name:     "sensitive_path",
				Weight:   25,
				Evidence: "Accesses sensitive path: " + pattern,
			})
			break // one hit is enough
		}
	}

	// 3. Check for credential exposure
	for _, re := range ra.credentialPatterns {
		if re.MatchString(cmd) {
			assessment.Factors = append(assessment.Factors, RiskFactor{
				Name:     "credential_exposure",
				Weight:   40,
				Evidence: "Potential credential in command",
			})
			break
		}
	}

	// 4. Check system modification
	for _, sysCmd := range ra.systemModifyCommands {
		if strings.HasPrefix(cmdLower, sysCmd) || strings.Contains(cmdLower, " "+sysCmd) {
			assessment.Factors = append(assessment.Factors, RiskFactor{
				Name:     "system_modification",
				Weight:   30,
				Evidence: "May modify system state via: " + sysCmd,
			})
			break
		}
	}

	// 5. Check for privilege escalation
	if strings.HasPrefix(cmdLower, "sudo ") || strings.HasPrefix(cmdLower, "su ") ||
		strings.Contains(cmdLower, "| sudo ") || strings.Contains(cmdLower, "&& sudo ") {
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:     "privilege_escalation",
			Weight:   35,
			Evidence: "Command attempts privilege escalation",
		})
	}

	// 6. Check for network exfiltration patterns
	if containsExfiltration(cmdLower) {
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:     "data_exfiltration",
			Weight:   45,
			Evidence: "Potential data exfiltration pattern detected",
		})
	}

	// 7. Check for pipe to shell patterns (code injection)
	if containsPipeToShell(cmdLower) {
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:     "code_injection",
			Weight:   40,
			Evidence: "Pipe-to-shell pattern detected (potential code injection)",
		})
	}

	// 8. Check for reverse shell patterns
	if containsReverseShell(cmdLower) {
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:     "reverse_shell",
			Weight:   50,
			Evidence: "Reverse shell pattern detected",
		})
	}

	// 9. Check for environment/secret dumping
	if containsEnvDump(cmdLower) {
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:     "env_dump",
			Weight:   35,
			Evidence: "Environment variable extraction detected",
		})
	}

	// Calculate total score (capped at 100)
	for _, factor := range assessment.Factors {
		assessment.Score += factor.Weight
	}
	if assessment.Score > 100 {
		assessment.Score = 100
	}

	// Classify
	switch {
	case assessment.Score >= 80:
		assessment.Level = RiskCritical
		assessment.Action = ActionBlock
	case assessment.Score >= 60:
		assessment.Level = RiskHigh
		assessment.Action = ActionAdminReview
	case assessment.Score >= 30:
		assessment.Level = RiskMedium
		assessment.Action = ActionUserConfirm
	default:
		assessment.Level = RiskLow
		assessment.Action = ActionAutoApprove
	}

	return assessment
}

// AssessToolCall evaluates a tool invocation (non-bash) and returns its risk.
func (ra *RiskAssessor) AssessToolCall(toolName, action, target string) RiskAssessment {
	assessment := RiskAssessment{
		Factors: make([]RiskFactor, 0),
	}

	// Assign base risk by tool type
	switch toolName {
	case "security_scan":
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:   "security_scan",
			Weight: 20,
			Evidence: "Security scanning tool",
		})
	case "compliance_check":
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:   "compliance_check",
			Weight: 10,
			Evidence: "Compliance checking (read-only)",
		})
	case "log_analyze":
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:   "log_analysis",
			Weight: 5,
			Evidence: "Log analysis (read-only)",
		})
	case "monitoring_query":
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:   "monitoring_query",
			Weight: 5,
			Evidence: "Monitoring query (read-only)",
		})
	case "network_diagnostics":
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:   "network_diagnostics",
			Weight: 15,
			Evidence: "Network diagnostic tool",
		})
	case "certificate_audit":
		assessment.Factors = append(assessment.Factors, RiskFactor{
			Name:   "certificate_audit",
			Weight: 10,
			Evidence: "Certificate audit (read-only)",
		})
	}

	// Check target sensitivity
	for _, pattern := range ra.sensitivePatterns {
		if pathMatch(target, pattern) {
			assessment.Factors = append(assessment.Factors, RiskFactor{
				Name:     "sensitive_target",
				Weight:   25,
				Evidence: "Target involves sensitive path: " + pattern,
			})
			break
		}
	}

	// Calculate total score
	for _, factor := range assessment.Factors {
		assessment.Score += factor.Weight
	}
	if assessment.Score > 100 {
		assessment.Score = 100
	}

	switch {
	case assessment.Score >= 80:
		assessment.Level = RiskCritical
		assessment.Action = ActionBlock
	case assessment.Score >= 60:
		assessment.Level = RiskHigh
		assessment.Action = ActionAdminReview
	case assessment.Score >= 30:
		assessment.Level = RiskMedium
		assessment.Action = ActionUserConfirm
	default:
		assessment.Level = RiskLow
		assessment.Action = ActionAutoApprove
	}

	return assessment
}

func compilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			// Security patterns must compile. Log loudly so operators notice.
			panic(fmt.Sprintf("secops: invalid risk assessment pattern %q: %v", p, err))
		}
		compiled = append(compiled, re)
	}
	return compiled
}

func pathMatch(cmd, pattern string) bool {
	// Check direct string containment
	if strings.Contains(cmd, pattern) {
		return true
	}
	// Try glob match on each whitespace-separated token
	for _, token := range strings.Fields(cmd) {
		if matched, _ := filepath.Match(pattern, token); matched {
			return true
		}
	}
	return false
}

// containsReverseShell checks for common reverse shell patterns.
func containsReverseShell(cmd string) bool {
	patterns := []string{
		`bash\s+-[a-z]*i.*>/dev/tcp/`,            // bash -i reverse shell
		`/dev/tcp/\d`,                             // /dev/tcp connections
		`nc\s+.*-[a-z]*e\s+/bin/(ba)?sh`,         // nc -e /bin/sh
		`ncat\s+.*-[a-z]*e\s+/bin/(ba)?sh`,       // ncat -e
		`python.*socket.*connect`,                 // python reverse shell
		`perl.*socket.*INET`,                      // perl reverse shell
		`ruby.*TCPSocket`,                         // ruby reverse shell
		`php.*fsockopen`,                          // php reverse shell
		`mkfifo.*nc`,                              // named pipe reverse shell
		`socat.*exec`,                             // socat reverse shell
		`telnet.*\|\s*/bin/(ba)?sh`,               // telnet pipe shell
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, cmd); matched {
			return true
		}
	}
	return false
}

// containsEnvDump checks for attempts to dump environment variables or secrets.
func containsEnvDump(cmd string) bool {
	patterns := []string{
		`\benv\b.*\|`,         // env piped somewhere
		`\bprintenv\b`,        // printenv
		`\bset\b\s*\|`,        // set piped
		`cat.*/proc/.*/environ`, // process environment
		`strings.*/proc/.*/environ`,
		`xargs.*-0.*environ`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, cmd); matched {
			return true
		}
	}
	return false
}

func containsExfiltration(cmd string) bool {
	patterns := []string{
		"curl.*-d.*@",       // curl POST with file data
		"curl.*--data.*@",
		"wget.*--post-file",
		"nc.*<",             // netcat sending data
		"scp.*:",            // scp to remote
		"rsync.*:",          // rsync to remote
		"base64.*|.*curl",   // encode and send
		"tar.*|.*nc",        // tar pipe to network
		"cat.*|.*nc",        // cat pipe to network
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, cmd); matched {
			return true
		}
	}
	return false
}

func containsPipeToShell(cmd string) bool {
	patterns := []string{
		`curl.*\|\s*(ba)?sh`,
		`wget.*\|\s*(ba)?sh`,
		`curl.*\|\s*python`,
		`wget.*\|\s*python`,
		`curl.*\|\s*perl`,
		`echo.*\|\s*(ba)?sh`,
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, cmd); matched {
			return true
		}
	}
	return false
}
