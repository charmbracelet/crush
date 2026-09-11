package pinentry

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestParsePS(t *testing.T) {
	t.Parallel()

	out := []byte(`    1 /sbin/launchd
  338 /usr/libexec/logd
 1337 /opt/homebrew/bin/pinentry-curses
 1442 gpg
 2499 gpg-agent
 3000 /usr/bin/Google Chrome Helper
bad line here
`)

	procs := parsePS(out)
	require.Len(t, procs, 6)
	require.Equal(t, Proc{PID: 1, Name: "launchd", Path: "/sbin/launchd"}, procs[0])
	require.Equal(t, Proc{PID: 1337, Name: "pinentry-curses", Path: "/opt/homebrew/bin/pinentry-curses"}, procs[2])
	// Bare names (Linux ps style) carry no path.
	require.Equal(t, Proc{PID: 1442, Name: "gpg"}, procs[3])
	// gpg-agent must not be confused with gpg.
	require.Equal(t, Proc{PID: 2499, Name: "gpg-agent"}, procs[4])
	// Commands with spaces keep the full remainder as their name.
	require.Equal(t, Proc{PID: 3000, Name: "Google Chrome Helper", Path: "/usr/bin/Google Chrome Helper"}, procs[5])
}

func TestClassify(t *testing.T) {
	t.Parallel()

	handover, ctrlL, gpg := classify([]Proc{{PID: 1, Name: "pinentry-curses"}, {PID: 2, Name: "gpg"}})
	require.True(t, handover)
	require.True(t, ctrlL)
	require.True(t, gpg)

	handover, ctrlL, gpg = classify([]Proc{{PID: 1, Name: "pinentry-tty"}})
	require.True(t, handover)
	require.False(t, ctrlL, "pinentry-tty reads raw bytes; Ctrl-L would corrupt the passphrase")
	require.False(t, gpg)

	handover, ctrlL, _ = classify([]Proc{{PID: 1, Name: "pinentry"}})
	require.True(t, handover)
	require.True(t, ctrlL, "an unresolvable plain pinentry defaults to the curses behavior")

	// GUI pinentry flavors never need the terminal.
	handover, ctrlL, gpg = classify([]Proc{{PID: 1, Name: "pinentry-mac"}, {PID: 2, Name: "pinentry-qt"}})
	require.False(t, handover)
	require.False(t, ctrlL)
	require.False(t, gpg)

	// gpg-agent is not gpg.
	handover, ctrlL, gpg = classify([]Proc{{PID: 1, Name: "gpg-agent"}})
	require.False(t, handover)
	require.False(t, ctrlL)
	require.False(t, gpg)
}

func TestResolveFlavor(t *testing.T) {
	t.Parallel()

	t.Run("direct names", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, flavorCurses, resolveFlavor(Proc{Name: "pinentry-curses"}))
		require.Equal(t, flavorTTY, resolveFlavor(Proc{Name: "pinentry-tty"}))
		require.Equal(t, flavorGUI, resolveFlavor(Proc{Name: "pinentry-mac"}))
	})

	t.Run("symlink to curses", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		real := filepath.Join(dir, "pinentry-curses")
		require.NoError(t, os.WriteFile(real, []byte(""), 0o755))
		link := filepath.Join(dir, "pinentry")
		require.NoError(t, os.Symlink(real, link))
		require.Equal(t, flavorCurses, resolveFlavor(Proc{PID: 1, Name: "pinentry", Path: link}))
	})

	t.Run("alternatives chain to gui", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		real := filepath.Join(dir, "pinentry-qt")
		require.NoError(t, os.WriteFile(real, []byte(""), 0o755))
		alt := filepath.Join(dir, "alt-pinentry")
		require.NoError(t, os.Symlink(real, alt))
		link := filepath.Join(dir, "pinentry")
		require.NoError(t, os.Symlink(alt, link))
		require.Equal(t, flavorGUI, resolveFlavor(Proc{PID: 1, Name: "pinentry", Path: link}))
	})

	t.Run("real binary defaults to curses", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		real := filepath.Join(dir, "pinentry")
		require.NoError(t, os.WriteFile(real, []byte(""), 0o755))
		require.Equal(t, flavorCurses, resolveFlavor(Proc{PID: 1, Name: "pinentry", Path: real}))
	})

	t.Run("unresolvable defaults to curses", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, flavorCurses, resolveFlavor(Proc{PID: 1, Name: "pinentry"}))
	})
}

func newTestService() *Service {
	s := NewService()
	s.reopenWindow = 300 * time.Millisecond
	s.touchGrace = 800 * time.Millisecond
	s.touchMax = 3 * time.Minute
	return s
}

func tickAll(s *Service, start time.Time, procs []Proc, interval time.Duration, n int) time.Time {
	now := start
	for range n {
		s.tick(now, procs)
		now = now.Add(interval)
	}
	return now
}

func TestWatcherPassphraseFlow(t *testing.T) {
	t.Parallel()

	s := newTestService()
	now := time.Now()
	pin := []Proc{{PID: 10, Name: "pinentry-curses"}, {PID: 20, Name: "gpg"}}

	// Dialog appears: terminal handover requested, dialog redraws on
	// Ctrl-L.
	s.tick(now, pin)
	require.Equal(t, Event{Active: true, CtrlLRedraw: true}, s.last)

	// Dialog closes and gpg finishes immediately: keep the handover for
	// the reopen window, then release.
	now = now.Add(100 * time.Millisecond)
	s.tick(now, nil)
	require.Equal(t, Event{Active: true, CtrlLRedraw: true}, s.last, "reopen window should keep the handover")

	tickAll(s, now, nil, 100*time.Millisecond, 4)
	require.Equal(t, Event{}, s.last)
	require.Equal(t, stateIdle, s.state)
}

func TestWatcherDialogReopenDebounce(t *testing.T) {
	t.Parallel()

	s := newTestService()
	now := time.Now()
	pin := []Proc{{PID: 10, Name: "pinentry"}}

	s.tick(now, pin)
	require.Equal(t, Event{Active: true, CtrlLRedraw: true}, s.last)

	// The dialog disappears and a new one (passphrase retry) appears
	// within the reopen window: no event flapping.
	now = now.Add(100 * time.Millisecond)
	s.tick(now, nil)
	now = now.Add(100 * time.Millisecond)
	s.tick(now, pin)
	require.Equal(t, Event{Active: true, CtrlLRedraw: true}, s.last, "no release while a follow-up dialog opens")
	require.Equal(t, stateDialog, s.state)
}

func TestWatcherGUIPinentryIgnored(t *testing.T) {
	t.Parallel()

	// A "pinentry" dispatcher resolving to a GUI flavor must not trigger
	// a terminal handover.
	dir := t.TempDir()
	real := filepath.Join(dir, "pinentry-mac")
	require.NoError(t, os.WriteFile(real, []byte(""), 0o755))
	link := filepath.Join(dir, "pinentry")
	require.NoError(t, os.Symlink(real, link))

	s := newTestService()
	s.tick(time.Now(), []Proc{{PID: 10, Name: "pinentry", Path: link}})
	require.Equal(t, Event{}, s.last)
	require.Equal(t, stateIdle, s.state)
}

func TestWatcherTouchAfterPassphrase(t *testing.T) {
	t.Parallel()

	s := newTestService()
	now := time.Now()
	pinAndGPG := []Proc{{PID: 10, Name: "pinentry-curses"}, {PID: 20, Name: "gpg"}}
	gpgOnly := []Proc{{PID: 20, Name: "gpg"}}

	// Passphrase dialog with gpg waiting behind it.
	s.tick(now, pinAndGPG)
	require.Equal(t, Event{Active: true, CtrlLRedraw: true}, s.last)

	// Dialog closes, gpg keeps running: handover released after the
	// reopen window, no touch hint yet.
	now = now.Add(100 * time.Millisecond)
	s.tick(now, gpgOnly)
	now = tickAll(s, now, gpgOnly, 100*time.Millisecond, 4)
	require.Equal(t, Event{}, s.last, "handover released, touch hint still within grace")

	// After the grace period gpg is still waiting: touch hint.
	tickAll(s, now, gpgOnly, 100*time.Millisecond, 10)
	require.Equal(t, Event{TouchPending: true}, s.last)

	// Touch happened, gpg exited: hint cleared.
	s.tick(now, nil)
	require.Equal(t, Event{}, s.last)
	require.Equal(t, stateIdle, s.state)
}

func TestWatcherTouchWithCachedCredentials(t *testing.T) {
	t.Parallel()

	s := newTestService()
	now := time.Now()
	gpgOnly := []Proc{{PID: 20, Name: "gpg2"}}

	// gpg starts waiting without ever showing a dialog (cached
	// passphrase). No hint before the grace period.
	now = tickAll(s, now, gpgOnly, 100*time.Millisecond, 7)
	require.Equal(t, Event{}, s.last)

	// After the grace period with gpg still waiting: touch hint.
	tickAll(s, now, gpgOnly, 100*time.Millisecond, 2)
	require.Equal(t, Event{TouchPending: true}, s.last)
}

func TestWatcherTouchExpiry(t *testing.T) {
	t.Parallel()

	s := newTestService()
	s.touchGrace = 100 * time.Millisecond
	s.touchMax = 500 * time.Millisecond
	now := time.Now()
	gpgOnly := []Proc{{PID: 20, Name: "gpg"}}

	now = tickAll(s, now, gpgOnly, 100*time.Millisecond, 2)
	require.Equal(t, Event{TouchPending: true}, s.last)

	// Past the maximum wait, assume gpg is stuck on something else and
	// drop the hint.
	tickAll(s, now, gpgOnly, 100*time.Millisecond, 5)
	require.Equal(t, Event{}, s.last)
}

func TestWatcherDialogReappearsDuringTouchWait(t *testing.T) {
	t.Parallel()

	s := newTestService()
	s.touchGrace = 100 * time.Millisecond
	now := time.Now()
	gpgOnly := []Proc{{PID: 20, Name: "gpg"}}
	pinAndGPG := []Proc{{PID: 10, Name: "pinentry-tty"}, {PID: 20, Name: "gpg"}}

	// Touch wait detected without a dialog.
	now = tickAll(s, now, gpgOnly, 100*time.Millisecond, 2)
	require.Equal(t, Event{TouchPending: true}, s.last)

	// A dialog appears after all (e.g. confirm dialog): hand over the
	// terminal and clear the touch hint. pinentry-tty does not support
	// Ctrl-L redraw.
	s.tick(now, pinAndGPG)
	require.Equal(t, Event{Active: true}, s.last)
}

func TestTrackCommand(t *testing.T) {
	t.Parallel()

	s := newTestService()
	done := s.TrackCommand()
	require.Equal(t, 1, s.tracked)
	done()
	require.Equal(t, 0, s.tracked)
	// The returned function is idempotent.
	done()
	require.Equal(t, 0, s.tracked)
}

func TestServicePublishesOnStart(t *testing.T) {
	t.Parallel()

	s := newTestService()
	s.pollInterval = 5 * time.Millisecond
	var calls int
	s.lister = func(context.Context) ([]Proc, error) {
		calls++
		if calls > 2 {
			return []Proc{{PID: 10, Name: "pinentry-curses"}}, nil
		}
		return nil, nil
	}

	ctx := t.Context()
	s.Start(ctx)

	sub := s.Subscribe(ctx)
	done := s.TrackCommand()
	defer done()

	select {
	case ev := <-sub:
		require.Equal(t, pubsub.UpdatedEvent, ev.Type)
		require.Equal(t, Event{Active: true, CtrlLRedraw: true}, ev.Payload)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pinentry event")
	}
}

func TestServiceSleepsWithoutTrackedCommands(t *testing.T) {
	t.Parallel()

	s := newTestService()
	s.pollInterval = time.Millisecond
	s.lister = func(context.Context) ([]Proc, error) {
		return []Proc{{PID: 10, Name: "pinentry-curses"}}, nil
	}

	ctx := t.Context()
	s.Start(ctx)

	sub := s.Subscribe(ctx)
	select {
	case ev := <-sub:
		t.Fatalf("unexpected event without tracked commands: %+v", ev.Payload)
	case <-time.After(50 * time.Millisecond):
	}

	// Once a command runs, the watcher wakes up and notices the dialog.
	done := s.TrackCommand()
	defer done()
	select {
	case ev := <-sub:
		require.Equal(t, Event{Active: true, CtrlLRedraw: true}, ev.Payload)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pinentry event")
	}
}

// buildFakeProc builds the given helper source from testdata under the
// given binary name so the process table shows it as that name (e.g.
// "pinentry-curses", "gpg").
func buildFakeProc(t *testing.T, source, name string) string {
	t.Helper()

	goBin, err := exec.LookPath("go")
	require.NoError(t, err)

	dst := filepath.Join(t.TempDir(), name)
	out, err := exec.CommandContext(t.Context(), goBin, "build", "-o", dst, "testdata/"+source).CombinedOutput()
	require.NoError(t, err, "go build: %s", out)
	return dst
}

func TestServiceDetectsRealProcess(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("process name emulation relies on ps(1)")
	}

	fakePinentry := buildFakeProc(t, "fakeproc.go", "pinentry-curses")
	fakeGPG := buildFakeProc(t, "fakeproc.go", "gpg")

	s := newTestService()
	s.lister = psLister
	s.pollInterval = 20 * time.Millisecond
	s.reopenWindow = 50 * time.Millisecond
	s.touchGrace = 100 * time.Millisecond

	ctx := t.Context()
	s.Start(ctx)
	sub := s.Subscribe(ctx)

	expectEvent := func(want Event) {
		t.Helper()
		for {
			select {
			case ev := <-sub:
				if ev.Payload == want {
					return
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("timed out waiting for event %+v", want)
			}
		}
	}

	done := s.TrackCommand()
	defer done()

	// A "pinentry-curses" process appears while gpg runs: handover.
	gpgCmd := exec.CommandContext(ctx, fakeGPG, "30")
	require.NoError(t, gpgCmd.Start())
	defer func() { _ = gpgCmd.Process.Kill() }()
	pinCmd := exec.CommandContext(ctx, fakePinentry, "30")
	require.NoError(t, pinCmd.Start())

	expectEvent(Event{Active: true, CtrlLRedraw: true})

	// The dialog closes but gpg keeps waiting: handover released, then
	// the security key touch hint appears.
	require.NoError(t, pinCmd.Process.Kill())
	_ = pinCmd.Wait()
	expectEvent(Event{})
	expectEvent(Event{TouchPending: true})

	// gpg exits: hint cleared.
	require.NoError(t, gpgCmd.Process.Kill())
	_ = gpgCmd.Wait()
	expectEvent(Event{})
}
