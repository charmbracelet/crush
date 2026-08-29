package agent

import (
	"context"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
)

// modelSelection is the large/small provider+model pair one run uses. It
// is fixed when the run starts and never re-read from config afterwards,
// which is what keeps two concurrent runs on one workspace from stealing
// each other's model.
type modelSelection struct {
	Large config.SelectedModel
	Small config.SelectedModel
}

// runModels is one run's own built model pair.
//
// It is deliberately mutable, but private to the run: fantasy calls
// AgentStreamCall.ModelProvider on every step and on every retry
// attempt, and that is the mechanism a 401 -> OnAuthRefresh -> retry
// uses to pick up refreshed credentials (the credential is baked into
// the provider client, so it needs a rebuilt model). The refresh path
// rebuilds this pair rather than the shared agent's, so credentials
// still reach a live stream while no other run — and no workspace-wide
// model change — can alter this turn's model between steps.
type runModels struct {
	// selection is what the pair was built from. Sub-agent turns
	// inherit the selection rather than the built models: the pair has
	// to be rebuilt with sub-agent provider settings.
	selection modelSelection
	// isSubAgent records how the pair was built so a rebuild produces
	// the same provider flavor (Copilot tags sub-agent traffic
	// differently).
	isSubAgent bool

	large *csync.Value[Model]
	small *csync.Value[Model]
}

// newRunModels wraps an already-built pair. Callers that need the models
// built from config go through coordinator.buildRunModels.
func newRunModels(selection modelSelection, isSubAgent bool, large, small Model) *runModels {
	return &runModels{
		selection:  selection,
		isSubAgent: isSubAgent,
		large:      csync.NewValue(large),
		small:      csync.NewValue(small),
	}
}

// Large returns the run's large model.
func (r *runModels) Large() Model { return r.large.Get() }

// Small returns the run's small model.
func (r *runModels) Small() Model { return r.small.Get() }

// set replaces the pair after a rebuild.
func (r *runModels) set(large, small Model) {
	r.large.Set(large)
	r.small.Set(small)
}

// modelRefresher rebuilds the models of whatever LLM call is asking, so
// a credential refresh reaches the wire. A nil modelRefresher means the
// caller has no models to rebuild yet.
type modelRefresher func(ctx context.Context) error

// requestedModelsContextKey carries the model selection a client asked
// for from the boundary that starts a run (backend.runAgent for the
// client/server path, app.RunNonInteractive for the in-process one) into
// coordinator.run, which resolves it into the run's own runModels.
type requestedModelsContextKey struct{}

type requestedModels struct {
	large *config.SelectedModel
	small *config.SelectedModel
}

// WithRequestedModels returns ctx tagged with the models this run must
// use. A nil entry means "unspecified": the run falls back to the
// workspace's configured selection for that slot when it starts.
//
// The model is a property of the run, not of the workspace. One
// workspace on the shared server serves an attached TUI and headless
// `crush run` prompts at the same time, and a one-shot `crush run -m`
// must not change what anything else runs on — nor rewrite the
// project's saved model.
func WithRequestedModels(ctx context.Context, large, small *config.SelectedModel) context.Context {
	if large == nil && small == nil {
		return ctx
	}
	return context.WithValue(ctx, requestedModelsContextKey{}, requestedModels{large: large, small: small})
}

// RequestedModelsFromContext reports the selection [WithRequestedModels]
// tagged ctx with. Exported because the boundary packages and their
// tests need to read it; safe to call on any context.
func RequestedModelsFromContext(ctx context.Context) (large, small *config.SelectedModel) {
	requested, _ := ctx.Value(requestedModelsContextKey{}).(requestedModels)
	return requested.large, requested.small
}

// runModelsContextKey carries the active run's own model pair down into
// its tool calls, so a sub-agent started by one of them inherits the
// selection of the run that spawned it. sessionAgent.Run stamps it from
// the call, which is what keeps a queued prompt's turn (its own call,
// its own models) from inheriting the turn that dequeued it.
type runModelsContextKey struct{}

func withRunModels(ctx context.Context, models *runModels) context.Context {
	return context.WithValue(ctx, runModelsContextKey{}, models)
}

func runModelsFromContext(ctx context.Context) *runModels {
	models, _ := ctx.Value(runModelsContextKey{}).(*runModels)
	return models
}
