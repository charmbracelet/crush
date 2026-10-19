package agent

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

const (
	routerMaxActivityRunes = 2000
	routerMaxSnippetRunes  = 300
)

// stepActivity summarizes the latest step of the agent loop for the
// router: the model's text, the tools it called, and what they returned.
// It is what tells the classifier where the work stands, so it can pick
// a model for the next step rather than for the prompt as a whole.
func stepActivity(steps []fantasy.StepResult) string {
	if len(steps) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range steps[len(steps)-1].Content {
		switch c := c.(type) {
		case fantasy.TextContent:
			if s := strings.TrimSpace(c.Text); s != "" {
				fmt.Fprintf(&b, "assistant: %s\n", firstRunes(s, routerMaxSnippetRunes))
			}
		case fantasy.ToolCallContent:
			fmt.Fprintf(&b, "called %s %s\n", c.ToolName, firstRunes(c.Input, routerMaxSnippetRunes))
		case fantasy.ToolResultContent:
			fmt.Fprintf(&b, "%s returned: %s\n", c.ToolName, firstRunes(toolResultText(c.Result), routerMaxSnippetRunes))
		}
	}
	return firstRunes(strings.TrimSpace(b.String()), routerMaxActivityRunes)
}

func toolResultText(r fantasy.ToolResultOutputContent) string {
	switch r := r.(type) {
	case fantasy.ToolResultOutputContentText:
		return r.Text
	case fantasy.ToolResultOutputContentError:
		if r.Error != nil {
			return "error: " + r.Error.Error()
		}
	case fantasy.ToolResultOutputContentMedia:
		return "(media)"
	}
	return ""
}

// callOptionsModel applies a routed model's own call options to every
// request. Fantasy fixes call options for a whole stream, but a routed
// turn can change models between steps, and one model's max tokens or
// provider options must not be sent to another.
type callOptionsModel struct {
	fantasy.LanguageModel
	opts TurnOptions
}

func withCallOptions(lm fantasy.LanguageModel, opts TurnOptions) fantasy.LanguageModel {
	return &callOptionsModel{LanguageModel: lm, opts: opts}
}

func (m *callOptionsModel) apply(call fantasy.Call) fantasy.Call {
	call.MaxOutputTokens = nil
	if m.opts.MaxOutputTokens > 0 {
		v := m.opts.MaxOutputTokens
		call.MaxOutputTokens = &v
	}
	call.ProviderOptions = m.opts.ProviderOptions
	call.Temperature = m.opts.Temperature
	call.TopP = m.opts.TopP
	call.TopK = m.opts.TopK
	call.FrequencyPenalty = m.opts.FrequencyPenalty
	call.PresencePenalty = m.opts.PresencePenalty
	return call
}

func (m *callOptionsModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	return m.LanguageModel.Generate(ctx, m.apply(call))
}

func (m *callOptionsModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return m.LanguageModel.Stream(ctx, m.apply(call))
}
