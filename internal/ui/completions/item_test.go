package completions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestResourceCompletionValueIsTemplate pins the concrete-vs-template
// distinction the insertion path relies on: a URI still carrying an RFC 6570
// placeholder must not be read eagerly (the server would see the literal
// braces), while a concrete URI must keep the auto-attachment read.
func TestResourceCompletionValueIsTemplate(t *testing.T) {
	t.Parallel()

	require.True(t, ResourceCompletionValue{URI: "cairn://run/{id}/a2ui"}.IsTemplate())
	require.True(t, ResourceCompletionValue{URI: "switchboard://queue/{name}"}.IsTemplate())
	require.False(t, ResourceCompletionValue{URI: "cairn://run/abc123/a2ui"}.IsTemplate())
	require.False(t, ResourceCompletionValue{URI: ""}.IsTemplate())
}
