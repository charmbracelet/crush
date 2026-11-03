package sshaskpass

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
)

// ErrNoSocket is returned when a prompt needs the UI but no prompt
// socket was configured.
var ErrNoSocket = errors.New("no ssh prompt socket configured (askpass unavailable)")

// Answer resolves one askpass prompt: on the controlling terminal when a
// driver such as sshpass owns it, otherwise through the Crush UI.
func Answer(ctx context.Context, prompt, socketPath string) (string, error) {
	return answer(ctx, prompt, socketPath, "/dev/tty")
}

func answer(ctx context.Context, prompt, socketPath, ttyPath string) (string, error) {
	req := ParsePrompt(prompt)
	if tty, ok := driverTerminal(ttyPath, req.Kind); ok {
		defer tty.Close()
		return askTerminal(tty, prompt, req.Kind)
	}
	if socketPath == "" {
		return "", ErrNoSocket
	}
	return AskCredentialOverSocket(ctx, socketPath, req)
}

// driverTerminal opens the controlling terminal, which a Crush-spawned
// ssh only has when a driver like sshpass set one up (the shell starts
// every command in a fresh session). Touch notices always go to the UI.
func driverTerminal(ttyPath string, kind Kind) (*os.File, bool) {
	if kind == KindTouch {
		return nil, false
	}
	tty, err := os.OpenFile(ttyPath, os.O_RDWR, 0)
	return tty, err == nil
}

// askTerminal asks on tty the way OpenSSH would: secrets without echo,
// confirmations with it.
func askTerminal(tty *os.File, prompt string, kind Kind) (string, error) {
	if _, err := io.WriteString(tty, prompt); err != nil {
		return "", err
	}

	if kind == KindConfirm {
		line, err := bufio.NewReader(tty).ReadString('\n')
		if err != nil && (!errors.Is(err, io.EOF) || line == "") {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	secret, err := term.ReadPassword(tty.Fd())
	_, _ = io.WriteString(tty, "\n")
	if err != nil {
		return "", err
	}
	return string(secret), nil
}
