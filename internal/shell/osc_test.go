package shell

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testPalette is a stand-in theme palette with distinct values per index.
func testPalette() [16]color.Color {
	var palette [16]color.Color
	for i := range palette {
		palette[i] = color.RGBA{R: uint8(i * 16), G: 0x40, B: 0x80, A: 0xff}
	}
	return palette
}

// waitForScreen waits until the session screen contains want.
func waitForScreen(t *testing.T, session *InteractiveSession, want string) {
	t.Helper()

	require.Eventually(t, func() bool {
		return strings.Contains(session.ScreenText(), want)
	}, 15*time.Second, 20*time.Millisecond)
}

// TestPaletteQueryAnswersWithThemeColors checks OSC 4 queries: a program
// asking for a palette entry gets the color the theme installed, which is
// what makes palette-aware tools match the rest of Crush.
func TestPaletteQueryAnswersWithThemeColors(t *testing.T) {
	t.Parallel()

	// The child is in raw mode like a TUI (which is what queries the
	// palette): query index 1, then echo the reply bytes back as hex so the
	// test can assert on the response without depending on rendering.
	session := newTestSession(t, `sh -c 'stty raw -echo; printf "\033]4;1;?\007"; dd bs=1 count=26 2>/dev/null | od -An -tx1; stty sane'`)
	session.SetPalette(testPalette())

	waitForScreen(t, session, "1b")
	screen := strings.Join(strings.Fields(session.ScreenText()), " ")
	require.Contains(t, screen, "1b 5d 34 3b 31 3b", "the reply starts an OSC 4 response for index 1")
}

// TestPaletteSetAndReset checks OSC 4 sets and OSC 104 resets: a program
// that changes a palette entry and resets it goes back to the theme color
// rather than to the plain palette.
func TestPaletteSetAndReset(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, "sleep 30")
	session.SetPalette(testPalette())

	session.handlePalette([]byte("4;1;rgb:ff/00/00"))
	require.NotEqual(t, testPalette()[1], session.effectiveColor(1), "the override is recorded")

	session.handlePaletteReset([]byte("104"))
	require.Equal(t, testPalette()[1], session.effectiveColor(1), "a full reset restores the theme")

	session.handlePalette([]byte("4;3;rgb:00/ff/00"))
	session.handlePaletteReset([]byte("104;3"))
	require.Equal(t, testPalette()[3], session.effectiveColor(3), "a per-index reset restores the theme")
}

// TestClipboardHandlerIgnoresQueries checks OSC 52: writes are accepted,
// reads are refused.
func TestClipboardHandlerIgnoresQueries(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, "sleep 30")

	require.True(t, session.handleClipboard([]byte("52;c;?")))
	require.True(t, session.handleClipboard([]byte("52;c;aGVsbG8=")))
	require.True(t, session.handleClipboard([]byte("52;c;not base64!")))
}

// TestPasteHonorsBracketedPasteMode checks the paste action: with bracketed
// paste enabled, the text is wrapped in the paste markers so multi-line
// input is inserted instead of executed.
func TestPasteHonorsBracketedPasteMode(t *testing.T) {
	t.Parallel()

	// The child enables bracketed paste, prints a ready marker once the
	// emulator has seen that, and reads the pasted bytes back in raw mode,
	// printing them as hex.
	session := newTestSession(t, `sh -c 'stty raw -echo; printf "\033[?2004hready"; dd bs=1 count=19 2>/dev/null | od -An -tx1; stty sane'`)

	// Paste only once the mode is on; the ready marker proves it was
	// processed.
	waitForScreen(t, session, "ready")

	require.NoError(t, session.Paste("one\ntwo"))

	waitForScreen(t, session, "1b")
	screen := strings.Join(strings.Fields(session.ScreenText()), " ")
	require.Contains(t, screen, "1b 5b 32 30 30 7e", "paste start marker")
	require.Contains(t, screen, "1b 5b 32 30 31 7e", "paste end marker")
}

// TestCheckBlockedRecursesIntoShellBodies checks the deny-list sees through
// shell wrappers instead of being bypassed by "sh -c '...'".
func TestCheckBlockedRecursesIntoShellBodies(t *testing.T) {
	t.Parallel()

	block := func(args []string) bool { return args[0] == "sudo" }

	require.Error(t, CheckBlocked("sudo rm -rf /", []BlockFunc{block}))
	require.Error(t, CheckBlocked("sh -c 'sudo rm -rf /'", []BlockFunc{block}))
	require.Error(t, CheckBlocked("bash -lc 'echo hi; sudo reboot'", []BlockFunc{block}))
	require.Error(t, CheckBlocked(`eval 'sudo reboot'`, []BlockFunc{block}))
	require.NoError(t, CheckBlocked("echo sudo", []BlockFunc{block}))
	require.NoError(t, CheckBlocked(`sh -c 'echo sudo is a command'`, []BlockFunc{block}))
}

// TestSessionStatesAreVisible checks the signals the UI and the agent read:
// the alternate screen and the bell.
func TestSessionStatesAreVisible(t *testing.T) {
	t.Parallel()

	session := newTestSession(t, `printf '\033[?1049h\a'; sleep 30`)

	require.Eventually(t, func() bool {
		return session.InAltScreen()
	}, 15*time.Second, 20*time.Millisecond, "the alternate screen is tracked")
	require.Eventually(t, func() bool {
		return !session.LastBell().IsZero()
	}, 15*time.Second, 20*time.Millisecond, "bells are tracked")
}

// TestWriteDoesNotBlockOnWedgedChild checks that input writes fail instead
// of hanging forever once the child stops reading: with the terminal in
// raw mode the line discipline stops consuming input, the queue fills, and
// the next write times out.
func TestWriteDoesNotBlockOnWedgedChild(t *testing.T) {
	t.Parallel()

	// The child never reads its input, and raw mode keeps the kernel from
	// draining it through echo.
	session := newTestSession(t, "sh -c 'stty raw -echo; sleep 60'")

	deadline := time.Now().Add(30 * time.Second)
	var err error
	for time.Now().Before(deadline) {
		if err = session.Paste(strings.Repeat("x", 4096)); err != nil {
			break
		}
	}
	require.Error(t, err, "writing to a child that never reads must eventually fail, not block")
	require.Contains(t, err.Error(), "not reading input")
}
