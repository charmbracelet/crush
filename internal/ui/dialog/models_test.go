package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

func TestModelTypeSummaryConfig(t *testing.T) {
	t.Parallel()
	require.Equal(t, config.SelectedModelTypeSummary, ModelTypeSummary.Config())
}

func TestModelTypeSummaryString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Summary", ModelTypeSummary.String())
}

func TestModelTypeSummaryPlaceholder(t *testing.T) {
	t.Parallel()
	require.Equal(t, summaryModelInputPlaceholder, ModelTypeSummary.Placeholder())
}

func TestSummaryModelsID(t *testing.T) {
	t.Parallel()
	require.Equal(t, "summary-models", SummaryModelsID)
}

func TestModelsIDConstant(t *testing.T) {
	t.Parallel()
	require.Equal(t, "models", ModelsID)
	require.NotEqual(t, ModelsID, SummaryModelsID)
}
