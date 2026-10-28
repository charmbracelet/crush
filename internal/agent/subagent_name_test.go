package agent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A label gets one stable name, a name stays itself, so the model can use
// either form and every view agrees on what to call the subagent.
func TestSubagentName(t *testing.T) {
	t.Parallel()
	name := SubagentName("date-impl")
	require.True(t, strings.HasSuffix(name, "-date-impl"), name)
	require.Equal(t, name, SubagentName(" date-impl "))
	require.Equal(t, name, SubagentName(name), "a name passes through")
	require.Empty(t, SubagentName(""))
}
