package app

import (
	"encoding/json"
	"os"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/config"
)

// EvalTelemetryEnvVar names the file a non-interactive run writes its
// per-run telemetry to when set — the eval harness's extraction path
// for numbers that only exist in-process (steps, usage, stub stats,
// recalls). Kept in sync with internal/eval.EvalTelemetryEnvVar; the
// constant is duplicated so app doesn't import eval.
const EvalTelemetryEnvVar = "CRUSH_EVAL_TELEMETRY"

// EvalFlagsEnvVar lists the manifest flag names the run reports
// resolved values for — the harness's no-op detection. Kept in sync
// with internal/eval.EvalFlagsEnvVar.
const EvalFlagsEnvVar = "CRUSH_EVAL_FLAGS"

// emitEvalTelemetry writes the run's telemetry JSON when
// CRUSH_EVAL_TELEMETRY is set. Best-effort: a write failure must never
// fail the run itself.
func (app *App) emitEvalTelemetry(sessionID string, result *fantasy.AgentResult, runErr error, approxSteps int) {
	path := os.Getenv(EvalTelemetryEnvVar)
	if path == "" || app.AgentCoordinator == nil {
		return
	}
	doc := map[string]any{
		"session_id": sessionID,
	}
	// The child reports what each manifest flag actually resolved to —
	// arm intent can silently no-op on a renamed or shadowed option.
	if keys := os.Getenv(EvalFlagsEnvVar); keys != "" {
		doc["resolved_options"] = config.OptionsProjection(
			*app.config.Config().Options, strings.Split(keys, ","))
	}
	if result != nil {
		doc["steps"] = len(result.Steps)
		doc["tokens"] = map[string]int64{
			"input":       result.TotalUsage.InputTokens,
			"output":      result.TotalUsage.OutputTokens,
			"cache_read":  result.TotalUsage.CacheReadTokens,
			"cache_write": result.TotalUsage.CacheCreationTokens,
		}
	} else if approxSteps > 0 {
		// Killed mid-run — approximate from distinct assistant
		// messages observed so the record isn't 0/0 for a run that
		// burned real budget.
		doc["steps"] = approxSteps
	}
	// SessionTelemetry is not on the Coordinator interface — assert so
	// test stubs and alternate coordinators needn't implement it.
	if c, ok := app.AgentCoordinator.(interface {
		SessionTelemetry(string) agent.SessionTelemetry
	}); ok {
		tel := c.SessionTelemetry(sessionID)
		doc["stub_stats"] = map[string]any{
			"invalidations":     tel.StubInvalidations,
			"results":           tel.StubResults,
			"saved_bytes":       tel.StubSavedBytes,
			"boundary_advances": tel.BoundaryAdvances,
		}
		doc["recalls"] = map[string]any{
			"result": tel.ResultRecalls,
			"entry":  tel.EntryRecalls,
			"empty":  tel.EmptyRecalls,
			"cross":  tel.CrossRecalls,
		}
	}
	if m, ok := app.config.Config().Models[config.SelectedModelTypeLarge]; ok {
		doc["model"] = m.Provider + "/" + m.Model
	}
	// The pin covers only the large slot — small/summary resolve from
	// ambient config. Record what they resolved to so a compaction-
	// flag experiment can audit which summarizer actually ran.
	for _, slot := range []config.SelectedModelType{
		config.SelectedModelTypeSmall,
		config.SelectedModelTypeSummary,
	} {
		if m, ok := app.config.Config().Models[slot]; ok {
			doc["model_"+string(slot)] = m.Provider + "/" + m.Model
		}
	}
	if runErr != nil {
		doc["error"] = runErr.Error()
	}
	if data, err := json.Marshal(doc); err == nil {
		_ = os.WriteFile(path, data, 0o644)
	}
}
