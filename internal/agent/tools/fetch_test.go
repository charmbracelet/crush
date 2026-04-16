package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyJQ(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		expr string
		want string
	}{
		{
			name: "length of array",
			body: `[1,2,3,4,5]`,
			expr: `length`,
			want: `5`,
		},
		{
			name: "extract field",
			body: `{"name":"crush","version":"1.0"}`,
			expr: `.name`,
			want: `"crush"`,
		},
		{
			name: "count objects in array",
			body: `[{"id":"a"},{"id":"b"},{"id":"c"}]`,
			expr: `length`,
			want: `3`,
		},
		{
			name: "sum nested array lengths",
			body: `[{"models":[1,2]},{"models":[3,4,5]},{"models":[6]}]`,
			expr: `[.[].models | length] | add`,
			want: `6`,
		},
		{
			name: "extract names",
			body: `[{"name":"a"},{"name":"b"}]`,
			expr: `[.[].name]`,
			want: "[\n  \"a\",\n  \"b\"\n]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := applyJQ(tt.body, tt.expr)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestApplyJQErrors(t *testing.T) {
	t.Parallel()

	_, err := applyJQ(`not json`, `.`)
	require.Error(t, err)

	_, err = applyJQ(`[1,2,3]`, `|||`)
	require.Error(t, err)

	_, err = applyJQ(``, `.`)
	require.Error(t, err)
}
