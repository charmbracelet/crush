package automode

import (
	"context"
	"strings"

	"charm.land/fantasy"
)

// LanguageModelResolver builds the classifier's language model. It is
// provided by the agent layer so the automode package stays decoupled
// from provider construction, and it re-reads config on every call so
// config reloads apply.
type LanguageModelResolver func(ctx context.Context) (fantasy.LanguageModel, error)

// languageModelGenerate adapts a fantasy.LanguageModel into the plain
// GenerateFunc the classifier uses. It prefers the response text and
// falls back to reasoning text so thinking models (e.g. Gemma thinking
// variants) still produce parseable output, and caps the completion at
// maxTokens (default 1024 when unset).
func languageModelGenerate(model fantasy.LanguageModel, maxTokens int64) GenerateFunc {
	return func(ctx context.Context, prompt string) (string, error) {
		if maxTokens <= 0 {
			maxTokens = 1024
		}
		temp := 0.0
		resp, err := model.Generate(ctx, fantasy.Call{
			Prompt:          fantasy.Prompt{fantasy.NewUserMessage(prompt)},
			MaxOutputTokens: &maxTokens,
			Temperature:     &temp,
		})
		if err != nil {
			return "", err
		}
		text := resp.Content.Text()
		if strings.TrimSpace(text) == "" {
			text = resp.Content.ReasoningText()
		}
		return text, nil
	}
}
