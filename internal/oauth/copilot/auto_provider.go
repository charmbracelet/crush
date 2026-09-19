package copilot

import (
	"context"
	"fmt"
	"maps"

	"charm.land/fantasy"
)

const sessionTokenHeader = "Copilot-Session-Token"

type autoProvider struct {
	fantasy.Provider
	resolver *AutoResolver
}

// NewAutoProvider decorates provider so the auto pseudo-model is resolved
// before Fantasy chooses and serializes the concrete model's API.
func NewAutoProvider(provider fantasy.Provider, resolver *AutoResolver) fantasy.Provider {
	return &autoProvider{Provider: provider, resolver: resolver}
}

func (p *autoProvider) LanguageModel(ctx context.Context, modelID string) (fantasy.LanguageModel, error) {
	if modelID != AutoModelID {
		return p.Provider.LanguageModel(ctx, modelID)
	}
	return &autoLanguageModel{provider: p.Provider, resolver: p.resolver}, nil
}

type autoLanguageModel struct {
	provider fantasy.Provider
	resolver *AutoResolver
}

func (m *autoLanguageModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	model, token, err := m.resolve(ctx)
	if err != nil {
		return nil, err
	}
	call.Headers = withSessionToken(call.Headers, token)
	return model.Generate(ctx, call)
}

func (m *autoLanguageModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	model, token, err := m.resolve(ctx)
	if err != nil {
		return nil, err
	}
	call.Headers = withSessionToken(call.Headers, token)
	return model.Stream(ctx, call)
}

func (m *autoLanguageModel) GenerateObject(ctx context.Context, call fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	model, token, err := m.resolve(ctx)
	if err != nil {
		return nil, err
	}
	call.Headers = withSessionToken(call.Headers, token)
	return model.GenerateObject(ctx, call)
}

func (m *autoLanguageModel) StreamObject(ctx context.Context, call fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	model, token, err := m.resolve(ctx)
	if err != nil {
		return nil, err
	}
	call.Headers = withSessionToken(call.Headers, token)
	return model.StreamObject(ctx, call)
}

func (m *autoLanguageModel) Provider() string { return m.provider.Name() }

func (m *autoLanguageModel) Model() string { return AutoModelID }

func (m *autoLanguageModel) resolve(ctx context.Context) (fantasy.LanguageModel, string, error) {
	selection, err := m.resolver.resolve(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("resolve Copilot auto model: %w", err)
	}
	model, err := m.provider.LanguageModel(ctx, selection.model)
	if err != nil {
		return nil, "", fmt.Errorf("create Copilot auto model %q: %w", selection.model, err)
	}
	return model, selection.sessionToken, nil
}

func withSessionToken(headers map[string]string, token string) map[string]string {
	headers = maps.Clone(headers)
	if headers == nil {
		headers = make(map[string]string)
	}
	headers[sessionTokenHeader] = token
	return headers
}

var (
	_ fantasy.Provider      = (*autoProvider)(nil)
	_ fantasy.LanguageModel = (*autoLanguageModel)(nil)
)
