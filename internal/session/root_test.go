package session

import (
	"context"
	"errors"
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

type flakyGetter struct {
	sessions fakeGetter
	fail     bool
}

func (f *flakyGetter) Get(ctx context.Context, id string) (Session, error) {
	if f.fail {
		return Session{}, errors.New("db unavailable")
	}
	return f.sessions.Get(ctx, id)
}

// TestRoots guards that Roots remembers a resolution, so a later failing
// lookup cannot change which session a subagent belongs to, and that a
// failure is reported rather than guessed around.
func TestRoots(t *testing.T) {
	t.Parallel()

	getter := &flakyGetter{sessions: fakeGetter{
		"root":  {ID: "root"},
		"child": {ID: "child", ParentSessionID: "root"},
		"other": {ID: "other", ParentSessionID: "root"},
	}}
	roots := new(Roots)

	got, err := roots.Resolve(t.Context(), getter, "child")
	require.NoError(t, err)
	require.Equal(t, "root", got)

	getter.fail = true
	got, err = roots.Resolve(t.Context(), getter, "child")
	require.NoError(t, err)
	require.Equal(t, "root", got, "a remembered root survives a failing lookup")

	got, err = roots.Resolve(t.Context(), getter, "other")
	require.Error(t, err)
	require.Equal(t, "other", got)

	getter.fail = false
	got, err = roots.Resolve(t.Context(), getter, "other")
	require.NoError(t, err)
	require.Equal(t, "root", got, "a failure is not remembered")

	var none *Roots
	got, err = none.Resolve(t.Context(), getter, "child")
	require.NoError(t, err)
	require.Equal(t, "root", got, "a nil Roots still resolves, without remembering")
}
