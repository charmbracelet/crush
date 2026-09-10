// Package pinentry detects terminal-based GPG pinentry dialogs
// (pinentry-curses, pinentry-tty) spawned by gpg-agent while Crush owns the
// terminal, and publishes events so the TUI can hand the terminal over for
// the passphrase prompt and reclaim it afterwards.
//
// gpg-agent spawns the pinentry program itself, so the dialog is not a
// child of the shell command that triggered the signing operation. The
// only reliable way to notice it is to watch the process table. The
// watcher polls while Crush is executing shell commands (and while a
// prompt is active), and publishes an [Event] whenever the derived state
// changes:
//
//   - Active: a terminal pinentry dialog is present (or transitioning
//     between dialogs) and needs exclusive control of the terminal.
//   - TouchPending: the passphrase dialog closed (or never appeared
//     because the credentials are cached by gpg-agent) but gpg is still
//     waiting, which almost always means a security key is waiting for a
//     touch.
package pinentry

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
)

// Event describes the state of terminal-based GPG pinentry dialogs.
type Event struct {
	// Active reports that a terminal pinentry dialog is displayed (or in
	// a short transition window between two dialogs) and needs exclusive
	// control of the terminal.
	Active bool `json:"active"`
	// TouchPending reports that gpg is still waiting after the passphrase
	// dialog closed (or never appeared because credentials were cached),
	// which almost always means a security key is waiting for a touch.
	TouchPending bool `json:"touch_pending"`
	// CtrlLRedraw reports that the active pinentry dialog is an ncurses
	// flavor whose UI can be repainted by injecting Ctrl-L into the
	// terminal input queue. Only meaningful when Active is true.
	CtrlLRedraw bool `json:"ctrl_l_redraw,omitempty"`
}

// watcherState is the state of the pinentry dialog tracker.
type watcherState uint8

const (
	// stateIdle: no terminal pinentry dialog is present.
	stateIdle watcherState = iota
	// stateDialog: a terminal pinentry dialog is present.
	stateDialog
	// stateClosing: the dialog disappeared recently; it may be replaced by
	// a follow-up dialog (e.g. passphrase retry), so the terminal handover
	// is kept for a short window to avoid flicker.
	stateClosing
)

const (
	defaultPollInterval = 100 * time.Millisecond
	defaultReopenWindow = 300 * time.Millisecond
	defaultTouchGrace   = 800 * time.Millisecond
	defaultTouchMax     = 3 * time.Minute
)

// Service watches the process table for terminal-based pinentry dialogs
// and publishes [Event] state changes. Use [Service.TrackCommand] to tell
// the watcher when Crush is executing shell commands; polling only happens
// while commands run or a dialog is active.
type Service struct {
	*pubsub.Broker[Event]

	mu        sync.Mutex
	wake      chan struct{}
	started   bool
	tracked   int
	state     watcherState
	closingAt time.Time
	// gpgWait is the base time for the security-key touch grace period.
	// It is refreshed while a dialog is open so the clock effectively
	// starts when the last dialog closes, and set on the first gpg
	// sighting when no dialog ever appears (cached credentials).
	gpgWait time.Time
	// ctrlL tracks whether the currently active pinentry dialog redraws
	// on Ctrl-L (ncurses flavors).
	ctrlL bool
	last  Event

	lister       Lister
	pollInterval time.Duration
	reopenWindow time.Duration
	touchGrace   time.Duration
	touchMax     time.Duration
}

// NewService creates a new pinentry watcher service.
func NewService() *Service {
	return &Service{
		Broker:       pubsub.NewBroker[Event](),
		wake:         make(chan struct{}, 1),
		lister:       psLister,
		pollInterval: defaultPollInterval,
		reopenWindow: defaultReopenWindow,
		touchGrace:   defaultTouchGrace,
		touchMax:     defaultTouchMax,
	}
}

var defaultService = NewService()

// DefaultService returns the process-wide pinentry watcher. The shell
// execution layer reports command lifetimes to it via [TrackCommand], and
// the app starts and subscribes to it.
func DefaultService() *Service {
	return defaultService
}

// TrackCommand marks the start of a shell command on the default service
// and returns a function that must be called when the command finishes.
func TrackCommand() func() {
	return defaultService.TrackCommand()
}

// TrackCommand marks the start of a shell command, keeping the watcher
// polling at full rate while it runs. The returned function marks the end
// of the command and is safe to call multiple times.
func (s *Service) TrackCommand() func() {
	s.mu.Lock()
	s.tracked++
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			if s.tracked > 0 {
				s.tracked--
			}
			s.mu.Unlock()
		})
	}
}

// Start launches the watch loop. It is a no-op if the service is already
// running, and the service may be started again after the given context is
// cancelled.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			s.started = false
			s.mu.Unlock()
		}()
		s.loop(ctx)
	}()
}

// loop polls the process table while shell commands are tracked or a
// pinentry dialog is active, and sleeps otherwise.
func (s *Service) loop(ctx context.Context) {
	for {
		s.mu.Lock()
		busy := s.tracked > 0 || s.state != stateIdle || !s.gpgWait.IsZero()
		s.mu.Unlock()

		if !busy {
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			}
			continue
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(s.pollInterval):
		}

		procs, err := s.lister(ctx)
		if err != nil {
			slog.Debug("Pinentry watcher: failed to list processes", "error", err)
			continue
		}
		s.tick(time.Now(), procs)
	}
}

// tick advances the watcher state machine with a process table snapshot.
func (s *Service) tick(now time.Time, procs []Proc) {
	pin, ctrlL, gpg := classify(procs)

	s.mu.Lock()
	switch s.state {
	case stateIdle:
		if pin {
			s.state = stateDialog
		}
	case stateDialog:
		if !pin {
			s.state = stateClosing
			s.closingAt = now
		}
	case stateClosing:
		if pin {
			s.state = stateDialog
		} else if now.Sub(s.closingAt) >= s.reopenWindow {
			s.state = stateIdle
		}
	}

	if pin {
		s.ctrlL = ctrlL
	}
	if s.state == stateIdle {
		s.ctrlL = false
	}

	if !gpg {
		s.gpgWait = time.Time{}
	} else if s.state != stateIdle || s.gpgWait.IsZero() {
		// While a dialog is open (or may reopen), gpg waiting is explained
		// by the dialog itself: keep resetting the touch clock so the
		// grace period starts when the last dialog closes. When no dialog
		// is involved, the clock starts at the first gpg sighting.
		s.gpgWait = now
	}

	waiting := !s.gpgWait.IsZero() && now.Sub(s.gpgWait) >= s.touchGrace && now.Sub(s.gpgWait) <= s.touchMax
	ev := Event{
		Active:       s.state != stateIdle,
		TouchPending: s.state == stateIdle && gpg && waiting,
		CtrlLRedraw:  s.state != stateIdle && s.ctrlL,
	}
	changed := ev != s.last
	if changed {
		s.last = ev
	}
	s.mu.Unlock()

	if changed {
		slog.Debug("Pinentry state changed", "active", ev.Active, "touch_pending", ev.TouchPending)
		s.PublishMustDeliver(context.Background(), pubsub.UpdatedEvent, ev)
	}
}
