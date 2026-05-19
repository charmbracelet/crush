package critic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchLSPDiagnostics_NilManager(t *testing.T) {
	t.Parallel()
	diags, err := FetchLSPDiagnostics(context.Background(), nil, nil, 0)
	require.NoError(t, err)
	require.Nil(t, diags)
}

func TestFetchLSPDiagnostics_EmptyPaths(t *testing.T) {
	t.Parallel()
	diags, err := FetchLSPDiagnostics(context.Background(), nil, []string{}, 0)
	require.NoError(t, err)
	require.Nil(t, diags)
}

func TestSeverityString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "error", severityString(1))
	require.Equal(t, "warning", severityString(2))
	require.Equal(t, "information", severityString(3))
	require.Equal(t, "hint", severityString(4))
	require.Equal(t, "unknown", severityString(99))
}
