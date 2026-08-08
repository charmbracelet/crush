package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// AgentParams must decode the subagent_type field so UI renderers can label
// the row with the specific subagent name.
func TestAgentParams_DecodesSubagentType(t *testing.T) {
	t.Parallel()

	input := []byte(`{"subagent_type":"code-reviewer","prompt":"review this"}`)

	var params AgentParams
	require.NoError(t, json.Unmarshal(input, &params))
	require.Equal(t, "code-reviewer", params.SubagentType)
	require.Equal(t, "review this", params.Prompt)
}

func TestAgentParams_OmitsSubagentTypeWhenAbsent(t *testing.T) {
	t.Parallel()

	input := []byte(`{"prompt":"search for things"}`)

	var params AgentParams
	require.NoError(t, json.Unmarshal(input, &params))
	require.Empty(t, params.SubagentType)
	require.Equal(t, "search for things", params.Prompt)
}

// AgentParams and AgentDispatchParams must share a wire-compatible shape so
// historical tool-call inputs decode cleanly under both types.
func TestAgentParams_WireCompatibleWithDispatchParams(t *testing.T) {
	t.Parallel()

	wire, err := json.Marshal(AgentDispatchParams{SubagentType: "tester", Prompt: "x"})
	require.NoError(t, err)

	var ap AgentParams
	require.NoError(t, json.Unmarshal(wire, &ap))
	require.Equal(t, "tester", ap.SubagentType)
	require.Equal(t, "x", ap.Prompt)
}
