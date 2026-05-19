package replacer

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/charmbracelet/crush/internal/message"
)

const defaultReplacerTemplate = `You are a conversation coach observing an interaction between a user and an AI coding assistant.
Your ONLY job is to decide whether the conversation should STOP or CONTINUE with a follow-up question.

## Conversation History

{{range .Messages}}
{{.Role}}: {{.Text}}
{{end}}

## Your Task

Evaluate the assistant's MOST RECENT response. Choose exactly one:

**STOP** if the assistant's response is thorough, actionable, AND ends with an invitation for the user to ask more (e.g., a question like "Would you like me to...?" or "Let me know if...").

**CONTINUE** if ANY of these are true:
- The user's last message was just a greeting ("hi", "hello", "hey", "good morning", etc.) and the assistant only greeted back.
- The user's request is broad or vague (e.g., "summarize workspace", "help me", "fix this", "review code") and the assistant did not ask clarifying questions before answering.
- The assistant's response is a dry summary or list without offering next steps or deeper analysis.
- The assistant's response is extremely short (under 30 words) and does not answer a concrete question.
- The assistant's response is generic and does not provide actionable, specific help.
- The user has not yet stated what they actually need help with.

## Output Format

You MUST respond with ONLY this exact JSON object. Do not add markdown fences, explanations, or any other text:

{"action":"stop","prompt":""}
OR
{"action":"continue","prompt":"What would you like help with today?"}

Rules:
- action MUST be exactly "stop" or "continue".
- If action is "stop", prompt MUST be "".
- If action is "continue", prompt MUST be a natural, concise follow-up question (1 sentence).
- When in doubt, prefer CONTINUE. It is better to ask for clarification than to end the conversation too early.
- Example: user says "hi", assistant says "hi" → CONTINUE with "What would you like help with today?"
- Example: user says "summarize workspace", assistant gives a dry summary → CONTINUE with "Is there a specific area or recent change you'd like me to focus on?"
- Example: user says "help me", assistant says "What do you need?" → CONTINUE with "What are you working on right now?"
- Example: user says "fix the bug in auth.go", assistant gives a specific fix and asks "Should I also update the tests?" → STOP
`

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
func BuildReplacerPrompt(msgs []message.Message) (string, error) {
	tmpl, err := template.New("replacer").Parse(defaultReplacerTemplate)
	if err != nil {
		return "", fmt.Errorf("parse replacer template: %w", err)
	}

	entries := make([]MessageEntry, 0, len(msgs))
	for _, m := range msgs {
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

func messageText(m message.Message) string {
	for _, part := range m.Parts {
		if tc, ok := part.(message.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
