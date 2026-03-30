package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
)

//go:embed templates/auto_tool_output_guard.md
var autoToolOutputGuardPrompt []byte

//go:embed templates/auto_handoff_guard.md
var autoHandoffGuardPrompt []byte

type toolOutputGuardResponse struct {
	Suspicious bool                              `json:"suspicious"`
	Reason     string                            `json:"reason"`
	Confidence permission.AutoApprovalConfidence `json:"confidence"`
}

const (
	maxAutoHandoffTitleRunes      = 300
	maxAutoHandoffUserRequestRunes = 1200
	maxAutoHandoffContentRunes    = 12000
)

var autoHandoffScopeOnlyHints = []string{
	"scope",
	"topic",
	"unclear",
	"not clear",
	"scope expansion",
	"findings dump",
	"tool output",
	"范围",
	"主题",
	"不明确",
	"不清楚",
	"无关",
	"不相关",
}

var autoHandoffRiskHints = []string{
	"danger",
	"dangerous",
	"unsafe",
	"escalat",
	"credential",
	"secret",
	"token",
	"jailbreak",
	"prompt injection",
	"malicious",
	"exploit",
	"提权",
	"权限",
	"凭据",
	"密钥",
	"注入",
	"越权",
	"危险",
}

var taskPathPattern = regexp.MustCompile(`(?i)\b[a-z]:/[\w\-. /]+|/[\w\-. /]+`)

func (c *coordinator) reviewToolResultForPromptInjection(ctx context.Context, sessionID string, toolResult message.ToolResult, permissionMode session.PermissionMode) (message.ToolResult, error) {
	if permissionMode != session.PermissionModeAuto {
		return toolResult, nil
	}
	if toolResult.Data != "" || strings.TrimSpace(toolResult.Content) == "" {
		return toolResult, nil
	}
	if reviewed, handled := applyLocalAutoToolOutputReview(toolResult); handled {
		return reviewed, nil
	}

	model, providerCfg, err := c.selectedAutoClassifierModel(ctx, true)
	if err != nil {
		return toolResult.WithAutoReview(message.ToolResultAutoReview{
			Suspicious:     true,
			Sanitized:      true,
			DetectorFailed: true,
			Reason:         "Tool output review failed before model selection.",
			Confidence:     string(permission.AutoApprovalConfidenceLow),
		}), err
	}

	prompt := buildToolOutputGuardPrompt(c.cfg.Config(), toolResult)
	raw, err := c.runAutoGuardModel(ctx, model, providerCfg, string(autoToolOutputGuardPrompt), prompt, 256, 0.0)
	if err != nil {
		return toolResult.WithAutoReview(message.ToolResultAutoReview{
			Suspicious:     true,
			Sanitized:      true,
			DetectorFailed: true,
			Reason:         "Tool output review failed; output withheld from the model.",
			Confidence:     string(permission.AutoApprovalConfidenceLow),
		}), nil
	}

	review, err := parseToolOutputGuard(raw)
	if err != nil {
		return toolResult.WithAutoReview(message.ToolResultAutoReview{
			Suspicious:     true,
			Sanitized:      true,
			DetectorFailed: true,
			Reason:         "Tool output review returned an invalid response; output withheld from the model.",
			Confidence:     string(permission.AutoApprovalConfidenceLow),
		}), nil
	}

	return toolResult.WithAutoReview(message.ToolResultAutoReview{
		Suspicious: review.Suspicious,
		Sanitized:  review.Suspicious,
		Reason:     strings.TrimSpace(review.Reason),
		Confidence: string(review.Confidence),
	}), nil
}

func applyLocalAutoToolOutputReview(toolResult message.ToolResult) (message.ToolResult, bool) {
	trustedReadOnly := isTrustedLocalReadOnlyToolResult(toolResult)
	if _, suspicious := suspiciousToolOutputSnippet(toolResult.Content); suspicious {
		// For suspicious local output, defer to the classifier model instead of
		// hard-sanitizing by keyword to reduce false positives.
		return toolResult, false
	}
	if trustedReadOnly {
		return toolResult, true
	}
	return toolResult, false
}

func isTrustedLocalReadOnlyToolResult(toolResult message.ToolResult) bool {
	switch toolResult.Name {
	case tools.ViewToolName,
		tools.LSToolName,
		tools.GlobToolName,
		tools.GrepToolName,
		tools.DiagnosticsToolName,
		tools.ReferencesToolName:
		return true
	case tools.BashToolName:
		return isTrustedLocalReadOnlyBashToolResult(toolResult)
	default:
		return false
	}
}

func isTrustedLocalReadOnlyBashToolResult(toolResult message.ToolResult) bool {
	if toolResult.IsError || strings.TrimSpace(toolResult.Metadata) == "" {
		return false
	}

	var meta tools.BashResponseMetadata
	if err := json.Unmarshal([]byte(toolResult.Metadata), &meta); err != nil {
		return false
	}

	return meta.SafeReadOnly
}

func suspiciousToolOutputSnippet(content string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(content))
	if lower == "" {
		return "", false
	}

	suspiciousSnippets := []string{
		"ignore previous instructions",
		"ignore all previous instructions",
		"ignore all instructions",
		"disregard previous instructions",
		"follow these instructions instead",
		"run this command",
		"execute this command",
		"execute this script",
		"show me your system prompt",
		"reveal your system prompt",
		"print your system prompt",
		"<system>",
		"</system>",
		"assistant: ignore",
		"user: ignore",
	}
	for _, snippet := range suspiciousSnippets {
		if strings.Contains(lower, snippet) {
			return snippet, true
		}
	}
	return "", false
}

func buildToolOutputGuardPrompt(cfg *config.Config, toolResult message.ToolResult) string {
	var sb strings.Builder
	sb.WriteString("Tool output to review:\n")
	sb.WriteString("- tool: ")
	sb.WriteString(toolResult.Name)
	sb.WriteString("\n- error: ")
	if toolResult.IsError {
		sb.WriteString("true")
	} else {
		sb.WriteString("false")
	}
	sb.WriteString("\n- output:\n")
	sb.WriteString(toolResult.Content)

	if cfg != nil && cfg.Permissions != nil && cfg.Permissions.AutoMode != nil && len(cfg.Permissions.AutoMode.BlockRules) > 0 {
		sb.WriteString("\n\nBlock rules:\n")
		for _, item := range cfg.Permissions.AutoMode.BlockRules {
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(item))
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func parseToolOutputGuard(raw string) (toolOutputGuardResponse, error) {
	raw = strings.TrimSpace(raw)
	if matches := autoClassifierCodeFenceRegex.FindStringSubmatch(raw); len(matches) == 2 {
		raw = strings.TrimSpace(matches[1])
	}
	var payload toolOutputGuardResponse
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return toolOutputGuardResponse{}, fmt.Errorf("failed to parse tool output guard response: %w", err)
	}
	if payload.Confidence == "" {
		payload.Confidence = permission.AutoApprovalConfidenceLow
	}
	return payload, nil
}

func (c *coordinator) reviewHandoffText(ctx context.Context, sess session.Session, title, content string) (permission.AutoClassification, error) {
	model, providerCfg, err := c.selectedAutoClassifierModel(ctx, true)
	if err != nil {
		return permission.AutoClassification{}, err
	}

	title = truncateForAutoGuard(strings.TrimSpace(title), maxAutoHandoffTitleRunes)
	content = truncateForAutoGuard(strings.TrimSpace(content), maxAutoHandoffContentRunes)
	recentUserRequest := truncateForAutoGuard(c.latestUserRequestForHandoff(ctx, sess.ID), maxAutoHandoffUserRequestRunes)

	var sb strings.Builder
	sb.WriteString("Session title:\n")
	sb.WriteString(strings.TrimSpace(sess.Title))
	sb.WriteString("\n\nCollaboration mode:\n")
	sb.WriteString(string(sess.CollaborationMode))
	sb.WriteString("\n\nPermission mode:\n")
	sb.WriteString(string(sess.PermissionMode))
	if recentUserRequest != "" {
		sb.WriteString("\n\nLatest user request in session:\n")
		sb.WriteString(recentUserRequest)
	}
	sb.WriteString("\n\nCandidate handoff title:\n")
	sb.WriteString(title)
	sb.WriteString("\n\nCandidate handoff content:\n")
	sb.WriteString(content)
	sb.WriteString("\n\nReviewer note:\n")
	sb.WriteString("- Candidate handoff content can include file paths, line numbers, and concise findings from read-only exploration.\n")
	sb.WriteString("- Do not block solely because the format looks like findings output when the content remains task-relevant and non-executable.\n")

	if cfg := c.cfg.Config(); cfg != nil && cfg.Permissions != nil && cfg.Permissions.AutoMode != nil {
		if len(cfg.Permissions.AutoMode.Environment) > 0 {
			sb.WriteString("\n\nEnvironment:\n")
			for _, item := range cfg.Permissions.AutoMode.Environment {
				sb.WriteString("- ")
				sb.WriteString(strings.TrimSpace(item))
				sb.WriteByte('\n')
			}
		}
		if len(cfg.Permissions.AutoMode.BlockRules) > 0 {
			sb.WriteString("\nBlock rules:\n")
			for _, item := range cfg.Permissions.AutoMode.BlockRules {
				sb.WriteString("- ")
				sb.WriteString(strings.TrimSpace(item))
				sb.WriteByte('\n')
			}
		}
	}

	raw, err := c.runAutoGuardModel(ctx, model, providerCfg, string(autoHandoffGuardPrompt), sb.String(), 256, 0.0)
	if err != nil {
		return permission.AutoClassification{}, err
	}
	classification, err := parseAutoClassification(raw)
	if err != nil {
		return permission.AutoClassification{}, err
	}
	if shouldAutoAllowTaskRelevantHandoff(classification, title, content) {
		classification.AllowAuto = true
		classification.Reason = "Auto Mode accepted a low-confidence scope mismatch after local safety checks."
		classification.Confidence = permission.AutoApprovalConfidenceLow
	}
	return classification, nil
}

func (c *coordinator) latestUserRequestForHandoff(ctx context.Context, sessionID string) string {
	if c.messages == nil || strings.TrimSpace(sessionID) == "" {
		return ""
	}
	msgs, err := c.messages.List(ctx, sessionID)
	if err != nil {
		return ""
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		if msg.Role != message.User || msg.IsSummaryMessage {
			continue
		}
		text := strings.TrimSpace(msg.Content().Text)
		if text != "" {
			return text
		}
	}
	return ""
}

func shouldAutoAllowTaskRelevantHandoff(review permission.AutoClassification, title, content string) bool {
	if review.AllowAuto || review.Confidence != permission.AutoApprovalConfidenceLow {
		return false
	}
	reason := strings.TrimSpace(review.Reason)
	if reason == "" {
		return false
	}
	if containsNormalizedHint(reason, autoHandoffRiskHints) {
		return false
	}
	if !containsNormalizedHint(reason, autoHandoffScopeOnlyHints) {
		return false
	}
	if strings.TrimSpace(content) == "" {
		return false
	}
	if _, suspicious := suspiciousToolOutputSnippet(content); suspicious {
		return false
	}
	return hasMeaningfulText(title) || hasMeaningfulText(content)
}

func shouldAllowSubagentRunDespiteReview(review permission.AutoClassification, delegatedPrompt, latestUserRequest string) bool {
	if review.AllowAuto || review.Confidence != permission.AutoApprovalConfidenceLow {
		return false
	}
	reason := strings.TrimSpace(review.Reason)
	if reason == "" {
		return false
	}
	if containsNormalizedHint(reason, autoHandoffRiskHints) {
		return false
	}
	if strings.TrimSpace(delegatedPrompt) == "" || strings.TrimSpace(latestUserRequest) == "" {
		return false
	}
	if _, suspicious := suspiciousToolOutputSnippet(delegatedPrompt); suspicious {
		return false
	}
	return likelySameTask(delegatedPrompt, latestUserRequest)
}

func likelySameTask(candidate, userRequest string) bool {
	candidateTokens := taskTokenSet(candidate)
	requestTokens := taskTokenSet(userRequest)
	if len(candidateTokens) == 0 || len(requestTokens) == 0 {
		return false
	}

	common := 0
	for token := range candidateTokens {
		if _, ok := requestTokens[token]; ok {
			common++
		}
	}
	if common >= 3 {
		return true
	}

	shorter := len(candidateTokens)
	if len(requestTokens) < shorter {
		shorter = len(requestTokens)
	}
	if shorter == 0 {
		return false
	}
	return float64(common)/float64(shorter) >= 0.35
}

func taskTokenSet(value string) map[string]struct{} {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return nil
	}
	value = taskPathPattern.ReplaceAllString(value, " ")

	var normalized strings.Builder
	for _, r := range value {
		switch {
		case unicode.Is(unicode.Han, r):
			normalized.WriteByte(' ')
			normalized.WriteRune(r)
			normalized.WriteByte(' ')
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			normalized.WriteRune(r)
		default:
			normalized.WriteByte(' ')
		}
	}

	set := make(map[string]struct{})
	for _, token := range strings.Fields(normalized.String()) {
		if shouldSkipTaskToken(token) {
			continue
		}
		set[token] = struct{}{}
	}
	return set
}

func shouldSkipTaskToken(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return true
	}
	runes := []rune(token)
	if len(runes) == 1 {
		return !unicode.Is(unicode.Han, runes[0])
	}
	if len(runes) < 3 {
		for _, r := range runes {
			if unicode.Is(unicode.Han, r) {
				return false
			}
		}
		return true
	}
	return false
}

func containsNormalizedHint(value string, hints []string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
	if normalized == "" {
		return false
	}
	for _, hint := range hints {
		hint = strings.TrimSpace(strings.ToLower(hint))
		if hint == "" {
			continue
		}
		if strings.Contains(normalized, hint) {
			return true
		}
	}
	return false
}

func hasMeaningfulText(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func truncateForAutoGuard(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "\n...[truncated for auto review]..."
}
