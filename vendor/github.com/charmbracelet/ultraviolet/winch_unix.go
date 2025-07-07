//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || aix || zos
// +build darwin dragonfly freebsd linux netbsd openbsd solaris aix zos

package uv

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/charmbracelet/x/term"
	"github.com/charmbracelet/x/termios"
)

func (l *WinChReceiver) receiveEvents(ctx context.Context, f term.File, evch chan<- Event) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)

	sendWinSize := func(w, h int) {
		select {
		case <-ctx.Done():
		case evch <- WindowSizeEvent{w, h}:
		}
	}

	sendPixelSize := func(w, h int) {
		select {
		case <-ctx.Done():
		case evch <- WindowPixelSizeEvent{w, h}:
		}
	}

	defer signal.Stop(sig)

	// Send the initial window size.
	winsize, err := termios.GetWinsize(int(f.Fd()))
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendWinSize(int(winsize.Col), int(winsize.Row))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		sendPixelSize(int(winsize.Xpixel), int(winsize.Ypixel))
	}()

	// Wait for all goroutines to finish before continuing.
	wg.Wait()

	// Listen for window size changes.
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-sig:
			winsize, err := termios.GetWinsize(int(f.Fd()))
			if err != nil {
				return err
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				sendWinSize(int(winsize.Col), int(winsize.Row))
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				sendPixelSize(int(winsize.Xpixel), int(winsize.Ypixel))
			}()

			// Wait for all goroutines to finish before continuing.
			wg.Wait()
		}
	}
}
