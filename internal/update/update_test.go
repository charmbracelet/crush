package update

import (
	"context"
	"testing"

	"github.com/charmbracelet/crush/internal/version"
	"github.com/stretchr/testify/require"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{
			name:     "equal versions",
			v1:       "1.0.0",
			v2:       "1.0.0",
			expected: 0,
		},
		{
			name:     "v1 less than v2 - patch",
			v1:       "1.0.0",
			v2:       "1.0.1",
			expected: -1,
		},
		{
			name:     "v1 less than v2 - minor",
			v1:       "1.0.0",
			v2:       "1.1.0",
			expected: -1,
		},
		{
			name:     "v1 less than v2 - major",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: -1,
		},
		{
			name:     "v1 greater than v2",
			v1:       "2.0.0",
			v2:       "1.9.9",
			expected: 1,
		},
		{
			name:     "with v prefix",
			v1:       "v1.0.0",
			v2:       "v1.0.1",
			expected: -1,
		},
		{
			name:     "different lengths",
			v1:       "1.0",
			v2:       "1.0.0",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestCheckForUpdate_DevelopmentVersion(t *testing.T) {
	// Test that development versions don't trigger updates.
	ctx := context.Background()

	// Temporarily set version to development version.
	originalVersion := version.Version
	version.Version = "unknown"
	defer func() {
		version.Version = originalVersion
	}()

	info, err := CheckForUpdate(ctx)
	require.NoError(t, err)
	require.NotNil(t, info)
	require.False(t, info.Available)
}
