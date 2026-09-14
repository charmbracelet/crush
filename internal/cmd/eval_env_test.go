package cmd

import (
	"testing"

	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/eval"
	"github.com/charmbracelet/crush/internal/notebook"
	"github.com/stretchr/testify/require"
)

// The CRUSH_EVAL_* contract is duplicated across packages — eval
// (driver), app (telemetry emission), agent (step cap). A drift
// silently breaks extraction or misclassifies which budget fired.
func TestEvalEnvVarConstantsAgree(t *testing.T) {
	require.Equal(t, app.EvalTelemetryEnvVar, eval.EvalTelemetryEnvVar)
	require.Equal(t, app.EvalFlagsEnvVar, eval.EvalFlagsEnvVar)
	require.Equal(t, agent.EvalMaxStepsEnvVar, eval.EvalMaxStepsEnvVar)
}

// NormalizeOptions inlines 25000 for notebook_raw_token_budget because
// config can't import notebook — pin the drift trap here instead.
func TestRawTokenBudgetMaterializationMatches(t *testing.T) {
	c := &config.Config{}
	c.NormalizeOptions()
	require.EqualValues(t, notebook.DefaultRawTokenBudget, c.Options.NotebookRawTokenBudget)
}
