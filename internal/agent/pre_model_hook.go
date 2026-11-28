package agent

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/hooks"
)

type preModelHook struct {
	fantasy.LanguageModel
	runner   *hooks.Runner
	provider string
	model    string
}

func (m *preModelHook) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	if err := m.check(ctx); err != nil {
		return nil, err
	}
	return m.LanguageModel.Generate(ctx, call)
}

func (m *preModelHook) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	if err := m.check(ctx); err != nil {
		return nil, err
	}
	return m.LanguageModel.Stream(ctx, call)
}

func (m *preModelHook) check(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	result := m.runner.Run(ctx, hooks.Event{
		Name:      hooks.EventPreModelUse,
		SessionID: tools.GetSessionFromContext(ctx),
		Provider:  m.provider,
		Model:     m.model,
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if result.Blocked() {
		return fmt.Errorf("PreModelUse hook blocked %s/%s: %s", m.provider, m.model, result.Reason)
	}
	return nil
}
