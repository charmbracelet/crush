package replacer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/charmbracelet/crush/internal/message"
)

const defaultReplacerTemplate = `You are a conversation coach evaluating an AI coding assistant's last response.
Your ONLY job is to decide: should the conversation CONTINUE (ask a follow-up) or STOP (the user has a clear next step)?

Be FAST and CONCISE. Self-criticize step by step: (1) What did the assistant do well? (2) What's missing or unclear? (3) Does the user have a concrete next step? Then decide STOP or CONTINUE.

DEFAULT TO CONTINUE. Only stop if the response is genuinely complete AND the user knows exactly what to do next.

## Most Recent Exchange

{{range .Messages}}
{{.Role}}: {{.Text}}
{{end}}

## Decision Rules

STOP only if ALL of these are true:
1. The assistant answered a specific, concrete coding question (not a vague or broad request).
2. The response includes working code, specific file changes, or a clear actionable fix.
3. The assistant explicitly asked a concrete follow-up question about implementation details (e.g. "Should I also update the tests?", "Do you want me to add error handling?").

CONTINUE if ANY of these are true:
- The user's request was vague or broad ("help me", "fix this", "review code", "summarize workspace").
- The assistant did NOT ask a specific follow-up question about what to do next.
- The response is a dry summary or list without offering concrete next steps.
- The response is under 50 words or feels generic / incomplete.
- The user has not yet stated what they actually need help with.
- The assistant only greeted back or gave a social response.

## Output Format

Respond with ONLY this exact JSON. No markdown fences, no explanations:

{"action":"stop","prompt":""}
OR
{"action":"continue","prompt":"What would you like help with today?"}

Rules:
- action MUST be exactly "stop" or "continue".
- If action is "stop", prompt MUST be "".
- If action is "continue", prompt MUST be a natural, concise follow-up question (1 sentence).
- NEVER repeat a prompt that was already suggested in a previous turn of this conversation. If you cannot think of a genuinely different follow-up, choose STOP instead.
- When in doubt, ALWAYS choose CONTINUE. Coding conversations are rarely done in one turn.

Examples:
- user: "hi" | assistant: "hi" → CONTINUE: "What would you like help with today?"
- user: "help me" | assistant: "What do you need?" → CONTINUE: "What are you working on right now?"
- user: "summarize workspace" | assistant: (dry list of files) → CONTINUE: "Is there a specific area or recent change you'd like me to focus on?"
- user: "fix auth.go bug" | assistant: (gives fix) + "Should I also update the tests?" → STOP
- user: "fix auth.go bug" | assistant: (gives fix) + "Let me know if you need anything else." → CONTINUE: "Should I also update the tests for this fix?"
`

// replacerTmpl is parsed once at init to avoid re-parsing on every evaluation.
var replacerTmpl = template.Must(template.New("replacer").Parse(defaultReplacerTemplate))

// MaxPromptMessages limits how many recent messages are included in the coach
// prompt. The coach only needs the most recent exchange to evaluate it.
const MaxPromptMessages = 4

// ReplacerPromptData is the template input for the replacement agent prompt.
type ReplacerPromptData struct {
	Messages []MessageEntry
}

// MessageEntry represents a single message in the conversation history.
type MessageEntry struct {
	Role string
	Text string
}

// BuildReplacerPrompt assembles the replacement agent prompt from conversation history.
// It first looks for a project-local override, then falls back to the embedded default.
func BuildReplacerPrompt(msgs []message.Message) (string, error) {
	tmplText, err := loadReplacerTemplate()
	if err != nil {
		return "", fmt.Errorf("load replacer template: %w", err)
	}

	tmpl, err := template.New("replacer").Parse(tmplText)
	if err != nil {
		return "", fmt.Errorf("parse replacer template: %w", err)
	}

	// Only keep the last N messages — the coach only evaluates the most
	// recent exchange, not the entire session history.
	start := 0
	if len(msgs) > MaxPromptMessages {
		start = len(msgs) - MaxPromptMessages
	}
	recent := msgs[start:]

	entries := make([]MessageEntry, 0, len(recent))
	for _, m := range recent {
		entries = append(entries, MessageEntry{
			Role: string(m.Role),
			Text: messageText(m),
		})
	}

	data := ReplacerPromptData{Messages: entries}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute replacer template: %w", err)
	}
	return buf.String(), nil
}

// loadReplacerTemplate tries to read a project-local override; otherwise returns
// the embedded default.
func loadReplacerTemplate() (string, error) {
	for _, dir := range []string{".crush", ".kimi", "crush"} {
		path := filepath.Join(dir, "skills", "replacer", "prompt.md.tpl")
		if b, err := os.ReadFile(path); err == nil {
			return string(b), nil
		}
	}
	return defaultReplacerTemplate, nil
}

func messageText(m message.Message) string {
	var sb strings.Builder
	for _, part := range m.Parts {
		if tc, ok := part.(message.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
