package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeGetter map[string]Session

func (f fakeGetter) Get(_ context.Context, id string) (Session, error) {
	return f[id], nil
}

func TestRootID(t *testing.T) {
	t.Parallel()

	sessions := fakeGetter{
		"root":  {ID: "root"},
		"child": {ID: "child", ParentSessionID: "root"},
		"loop":  {ID: "loop", ParentSessionID: "loop"},
	}
	for id, want := range map[string]string{"root": "root", "child": "root", "loop": "loop", "": ""} {
		got, err := RootID(t.Context(), sessions, id)
		require.NoError(t, err)
		require.Equal(t, want, got, id)
	}

	got, err := RootID(t.Context(), nil, "child")
	require.NoError(t, err)
	require.Equal(t, "child", got)
}
