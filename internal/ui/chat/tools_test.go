package chat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPartialJSONFields guards the recovery of complete top-level fields
// from a JSON object that is cut off mid-stream.
func TestPartialJSONFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  map[string]any
	}{
		{name: "empty", input: "", want: map[string]any{}},
		{name: "object open only", input: "{", want: map[string]any{}},
		{name: "key without value", input: `{"pattern"`, want: map[string]any{}},
		{name: "value still streaming", input: `{"pattern":"fo`, want: map[string]any{}},
		{
			name:  "value closed, object not",
			input: `{"pattern":"fo"`,
			want:  map[string]any{"pattern": "fo"},
		},
		{
			name:  "several complete fields",
			input: `{"pattern":"foo","path":".","literal_text":true`,
			want:  map[string]any{"pattern": "foo", "path": ".", "literal_text": true},
		},
		{
			name:  "duplicate key overwrites",
			input: `{"pattern":"a","pattern":"b"`,
			want:  map[string]any{"pattern": "b"},
		},
		{
			name:  "trailing garbage after complete object",
			input: `{"pattern":"foo"} extra`,
			want:  map[string]any{"pattern": "foo"},
		},
		{
			name:  "numbers are recovered",
			input: `{"count":42,"ratio":1.5`,
			want:  map[string]any{"count": float64(42), "ratio": 1.5},
		},
		{
			name:  "nested values are skipped",
			input: `{"pattern":"foo","meta":{"a":"x","b":true},"include":"*.go"`,
			want:  map[string]any{"pattern": "foo", "include": "*.go"},
		},
		{
			name:  "scalar input yields nothing",
			input: `"garbage"`,
			want:  map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := partialJSONFields(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}
