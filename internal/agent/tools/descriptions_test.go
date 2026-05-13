package tools

import (
	"testing"

	"github.com/taigrr/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestToolDescriptions_NoPanic verifies that all tool description templates
// render without panicking. This catches mismatches between template fields
// and their corresponding data structs.
func TestToolDescriptions_NoPanic(t *testing.T) {
	t.Parallel()

	t.Run("bash", func(t *testing.T) {
		t.Parallel()
		styles := []config.TrailerStyle{
			config.TrailerStyleNone,
			config.TrailerStyleAssistedBy,
			config.TrailerStyleCoAuthoredBy,
		}
		for _, style := range styles {
			attribution := &config.Attribution{
				TrailerStyle: style,
			}
			desc := bashDescription(attribution, "test-model")
			require.NotEmpty(t, desc)

			// Verify model name appears in assisted-by style.
			if style == config.TrailerStyleAssistedBy {
				require.Contains(t, desc, "test-model")
			}
		}
	})

	t.Run("view", func(t *testing.T) {
		t.Parallel()
		desc := viewDescription()
		require.NotEmpty(t, desc)
		require.Contains(t, desc, "2000") // DefaultReadLimit
	})

	t.Run("grep", func(t *testing.T) {
		t.Parallel()
		desc := grepDescription()
		require.NotEmpty(t, desc)
	})

	t.Run("glob", func(t *testing.T) {
		t.Parallel()
		desc := globDescription()
		require.NotEmpty(t, desc)
	})

	t.Run("ls", func(t *testing.T) {
		t.Parallel()
		desc := lsDescription()
		require.NotEmpty(t, desc)
	})

	t.Run("download", func(t *testing.T) {
		t.Parallel()
		desc := downloadDescription()
		require.NotEmpty(t, desc)
	})

	t.Run("sourcegraph", func(t *testing.T) {
		t.Parallel()
		desc := sourcegraphDescription()
		require.NotEmpty(t, desc)
	})

	t.Run("crush_logs", func(t *testing.T) {
		t.Parallel()
		desc := crushLogsDescription()
		require.NotEmpty(t, desc)
	})

	t.Run("fetch", func(t *testing.T) {
		t.Parallel()
		desc := renderToolDescription(fetchDescriptionTpl)
		require.NotEmpty(t, desc)
	})

	t.Run("web_fetch", func(t *testing.T) {
		t.Parallel()
		desc := renderToolDescription(webFetchDescriptionTpl)
		require.NotEmpty(t, desc)
	})

	t.Run("web_search", func(t *testing.T) {
		t.Parallel()
		desc := renderToolDescription(webSearchDescriptionTpl)
		require.NotEmpty(t, desc)
	})
}
