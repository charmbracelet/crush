package automode

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
)

// Verdict is the outcome of a classification decision.
type Verdict int

const (
	VerdictAllow Verdict = iota
	VerdictDeny
	VerdictEscalate
)

func (v Verdict) String() string {
	switch v {
	case VerdictAllow:
		return "ALLOW"
	case VerdictDeny:
		return "DENY"
	default:
		return "ESCALATE"
	}
}

// GenerateFunc sends a prompt to the classifier LLM and returns the text
// response. It is satisfied by an adapter over fantasy.LanguageModel.
type GenerateFunc func(ctx context.Context, prompt string) (string, error)

// ClassifierConfig holds classifier tuning parameters.
type ClassifierConfig struct {
	// Prompts holds optional custom classifier prompt templates. Empty
	// templates fall back to the built-ins.
	Prompts PromptSet
	// Environment is prose describing trusted repos, domains, buckets,
	// and services, injected into the classifier prompts.
	Environment []string
	// TranscriptMaxChars caps the reasoning-blind transcript excerpt.
	// 0 uses the default cap.
	TranscriptMaxChars int
	// FailOpen controls behavior when the classifier is unavailable or
	// returns an unparsable response. When true, failures auto-allow;
	// when false (default), failures escalate to the host prompt and
	// unparsable stage-2 output denies conservatively.
	FailOpen bool
}

// defaultTranscriptMaxChars is the default cap on the transcript excerpt
// passed to the LLM classifier.
const defaultTranscriptMaxChars = 24000

func (c *ClassifierConfig) capTranscript(transcript string) string {
	max := c.TranscriptMaxChars
	if max <= 0 {
		max = defaultTranscriptMaxChars
	}
	if len(transcript) <= max {
		return transcript
	}
	return transcript[len(transcript)-max:]
}

// classifier evaluates tool calls through static rules and the LLM.
type classifier struct {
	generate GenerateFunc
	config   ClassifierConfig
	rules    *Rules
}

// classify evaluates a tool call and returns a verdict. toolName is
// expected lowercase; command, filePath, and transcript may be empty.
func (c *classifier) classify(ctx context.Context, toolName, command, filePath, cwd, transcript string) (Verdict, string) {
	// Tier 1: static rules.
	if c.rules.IsReadOnly(toolName) {
		return VerdictAllow, ""
	}
	if c.rules.IsLowRisk(toolName) {
		return VerdictAllow, ""
	}
	if c.isTrustedProjectWrite(toolName, filePath, cwd) {
		return VerdictAllow, ""
	}

	// Protected paths always go to the classifier (or escalate when no
	// model is configured). They are never auto-allowed.
	if c.rules.IsProtectedPath(filePath) {
		if c.generate == nil {
			return VerdictEscalate, "protected path requires classification"
		}
		return c.runLLM(ctx, cwd, toolName, buildParams(command, filePath), c.config.capTranscript(transcript))
	}

	if command != "" {
		if c.rules.IsSafeCommand(command) {
			return VerdictAllow, ""
		}
		if c.rules.IsDangerous(command) {
			return VerdictDeny, "command matches a statically dangerous pattern"
		}
		if c.rules.IsBanned(command) {
			return VerdictDeny, "command uses a banned program"
		}
	}

	// Tier 2: LLM classifier.
	if c.generate == nil {
		return VerdictEscalate, "no classifier model configured"
	}
	return c.runLLM(ctx, cwd, toolName, buildParams(command, filePath), c.config.capTranscript(transcript))
}

// isTrustedProjectWrite allows in-project file writes, which are
// recoverable via version control, but never for protected paths.
func (c *classifier) isTrustedProjectWrite(toolName, filePath, cwd string) bool {
	if !c.rules.IsWriteTool(toolName) || filePath == "" || cwd == "" {
		return false
	}
	return !c.rules.IsProtectedPath(filePath) && isInProject(filePath, cwd)
}

func buildParams(command, filePath string) string {
	if command != "" {
		data, _ := json.Marshal(map[string]string{"command": command})
		return string(data)
	}
	if filePath != "" {
		data, _ := json.Marshal(map[string]string{"file_path": filePath})
		return string(data)
	}
	return "{}"
}

func isInProject(path, cwd string) bool {
	if path == "" || cwd == "" {
		return false
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func (c *classifier) runLLM(ctx context.Context, cwd, toolName, paramsJSON, transcript string) (Verdict, string) {
	stage1Output, err := c.generate(ctx, c.config.Prompts.RenderStage1(cwd, toolName, paramsJSON, c.config.Environment, transcript))
	if err != nil {
		return c.failResult("classifier unavailable: " + err.Error())
	}

	if parseStage1(stage1Output) == VerdictAllow {
		return VerdictAllow, ""
	}

	stage2Output, err := c.generate(ctx, c.config.Prompts.RenderStage2(cwd, toolName, paramsJSON, c.config.Environment, transcript))
	if err != nil {
		return c.failResult("classifier Stage 2 unavailable: " + err.Error())
	}

	s2 := parseStage2(stage2Output)
	if s2.Verdict == VerdictEscalate && strings.Contains(s2.Reason, "unparseable") {
		// Unparsable classifier output: deny conservatively unless
		// fail-open is configured.
		if c.config.FailOpen {
			return VerdictAllow, "unparsable classifier output; fail-open"
		}
		return VerdictDeny, "unparsable classifier output; blocking conservatively"
	}
	return s2.Verdict, s2.Reason
}

// failResult maps a classifier-unavailable condition to a verdict based
// on the fail-open setting.
func (c *classifier) failResult(reason string) (Verdict, string) {
	if c.config.FailOpen {
		return VerdictAllow, reason + "; fail-open"
	}
	return VerdictEscalate, reason
}
