package agent

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newGatedTitleAgent returns an agent whose small (title) model blocks its
// first Stream until release is called. Cleanup releases the gate and drains
// titles before the test database closes.
func newGatedTitleAgent(t *testing.T) (*sessionAgent, fakeEnv, *gatedStreamModel, func()) {
	t.Helper()
	env := testEnv(t)
	small := &gatedStreamModel{
		text:    "Gated Title",
		gate:    make(chan struct{}),
		entered: make(chan struct{}),
	}
	sa := testSessionAgent(env, fastModel{}, small, "system").(*sessionAgent)
	var once sync.Once
	release := func() { once.Do(func() { close(small.gate) }) }
	t.Cleanup(func() {
		release()
		sa.CancelAll()
	})
	return sa, env, small, release
}

func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestTitle_CancelAllStopsInFlightTitle(t *testing.T) {
	t.Parallel()
	sa, env, small, _ := newGatedTitleAgent(t)

	sess, err := env.sessions.Create(t.Context(), "Untitled")
	require.NoError(t, err)

	go sa.GenerateTitle(t.Context(), sess.ID, "hello there")
	waitClosed(t, small.entered, "title stream")

	// The gate is never released: CancelAll must stop the title rather
	// than wait the model out.
	done := make(chan struct{})
	go func() {
		sa.CancelAll()
		close(done)
	}()
	waitClosed(t, done, "CancelAll")

	got, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, DefaultSessionName, got.Title, "a stopped title still saves the fallback before CancelAll returns")
}

func TestTitle_RunDoesNotWaitForTitle(t *testing.T) {
	t.Parallel()
	sa, env, small, release := newGatedTitleAgent(t)

	sess, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	// The title stream stays gated until after Run returns, so Run
	// returning at all proves it does not wait for the title.
	runDone := make(chan struct{})
	var runErr error
	go func() {
		defer close(runDone)
		_, runErr = sa.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "hello there"})
	}()
	waitClosed(t, runDone, "Run")
	require.NoError(t, runErr)
	waitClosed(t, small.entered, "title stream")

	release()
	require.True(t, sa.titles.wait(5*time.Second), "the title finishes once the model answers")

	got, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, "Gated Title", got.Title)
}

func TestTitle_StartDuringDrainDoesNotPanic(t *testing.T) {
	t.Parallel()
	sa, env, small, release := newGatedTitleAgent(t)

	sess, err := env.sessions.Create(t.Context(), "New Session")
	require.NoError(t, err)

	go sa.GenerateTitle(t.Context(), sess.ID, "first")
	waitClosed(t, small.entered, "title stream")

	var wg sync.WaitGroup
	for range 4 {
		wg.Go(sa.CancelAll)
	}
	for range 8 {
		wg.Go(func() { sa.GenerateTitle(t.Context(), sess.ID, "more") })
	}
	time.Sleep(50 * time.Millisecond)
	release()
	wg.Wait()

	// Every title has ended, so a fresh drain returns at once.
	require.True(t, sa.titles.wait(0))
}
