package proto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubagentReportPartRoundTrip(t *testing.T) {
	t.Parallel()

	parts := []ContentPart{
		SubagentReport{Name: "code-reviewer-1", Status: "completed", Output: "looks good"},
	}

	encoded, err := MarshalParts(parts)
	require.NoError(t, err)

	decoded, err := UnmarshalParts(encoded)
	require.NoError(t, err)
	require.Equal(t, parts, decoded)
}
