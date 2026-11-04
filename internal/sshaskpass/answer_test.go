//go:build !windows

package sshaskpass

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
)

// driveTerminal plays the part of sshpass: it waits for the prompt on
// the master side of a pty, then types reply.
func driveTerminal(t *testing.T, master *os.File, wantPrompt, reply string) <-chan string {
	t.Helper()
	seen := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(master)
		var got strings.Builder
		for !strings.Contains(got.String(), wantPrompt) {
			b, err := reader.ReadByte()
			if err != nil {
				break
			}
			got.WriteByte(b)
		}
		seen <- got.String()
		_, _ = io.WriteString(master, reply)
		// Drain whatever else arrives so writes to the slave never block.
		_, _ = io.Copy(io.Discard, reader)
	}()
	return seen
}

func openPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()
	master, slave, err := pty.Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = slave.Close()
		_ = master.Close()
	})
	return master, slave
}

func TestAskTerminalReadsSecretFromDriver(t *testing.T) {
	t.Parallel()
	master, slave := openPTY(t)
	prompt := "dev@192.168.220.199's password: "
	seen := driveTerminal(t, master, "password:", "ccdcredteam123!\n")

	answer, err := askTerminal(slave, prompt, KindPassword)
	require.NoError(t, err)
	require.Equal(t, "ccdcredteam123!", answer)
	require.True(t, strings.HasPrefix(prompt, <-seen))
}

func TestAskTerminalReadsConfirmationFromDriver(t *testing.T) {
	t.Parallel()
	master, slave := openPTY(t)
	prompt := "Are you sure you want to continue connecting (yes/no/[fingerprint])? "
	seen := driveTerminal(t, master, "(yes/no", "yes\r\n")

	answer, err := askTerminal(slave, prompt, KindConfirm)
	require.NoError(t, err)
	require.Equal(t, "yes", answer)
	require.True(t, strings.HasPrefix(prompt, <-seen))
}

func TestAnswerUsesDriverTerminal(t *testing.T) {
	t.Parallel()
	master, slave := openPTY(t)
	driveTerminal(t, master, "password:", "hunter2\n")

	secret, err := answer(t.Context(), "user@host's password: ", "", slave.Name())
	require.NoError(t, err)
	require.Equal(t, "hunter2", secret)
}

func TestAnswerWithoutTerminalGoesToUI(t *testing.T) {
	t.Parallel()
	noTTY := filepath.Join(t.TempDir(), "no-such-tty")

	_, err := answer(t.Context(), "user@host's password: ", "", noTTY)
	require.ErrorIs(t, err, ErrNoSocket)
}

func TestAnswerSendsTouchToUIEvenWithTerminal(t *testing.T) {
	t.Parallel()
	_, slave := openPTY(t)

	_, err := answer(t.Context(), "Confirm user presence for key ED25519-SK SHA256:abc", "", slave.Name())
	require.ErrorIs(t, err, ErrNoSocket)
}
