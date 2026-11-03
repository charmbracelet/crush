package subagents

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestValidate_QuestionToolNeverGrantedToSubagents verifies the interactive
// question tool is rejected in tools: because subagents never receive it.
func TestValidate_QuestionToolNeverGrantedToSubagents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tools   ToolList
		wantErr bool
	}{
		{name: "question alone is rejected", tools: ToolList{"question"}, wantErr: true},
		{name: "question with view is rejected", tools: ToolList{"question", "view"}, wantErr: true},
		{name: "view alone is accepted", tools: ToolList{"view"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := Subagent{Name: "x", Description: "d", Tools: tt.tools}
			if tt.wantErr {
				require.ErrorContains(t, s.Validate(), `"question"`)
				return
			}
			require.NoError(t, s.Validate())
		})
	}
}
