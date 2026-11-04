package exitbanner

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func testSession() *session.Session {
	return &session.Session{ID: "abc123def456", Title: "Fix the flaky test"}
}

// linesOf splits a rendered banner and computes its layout facts: blank lines
// and line widths, so tests can pin the exact blank-line structure of the
// banner without asserting on how the logo or colors look.
type linesOf struct {
	lines []string
}

func newLines(s string) linesOf {
	return linesOf{strings.Split(s, "\n")}
}

func (l linesOf) blankCount() int {
	return len(l.blankIndexes())
}

// blankIndexes returns the indexes of every blank line.
func (l linesOf) blankIndexes() []int {
	var idxs []int
	for i, line := range l.lines {
		if line == "" {
			idxs = append(idxs, i)
		}
	}
	return idxs
}

func (l linesOf) indexContaining(substr string) int {
	for i, line := range l.lines {
		if strings.Contains(line, substr) {
			return i
		}
	}
	return -1
}

// requireSingleBlankBefore asserts that exactly one blank line separates the
// line at index i from the previous non-blank line.
func (l linesOf) requireSingleBlankBefore(t *testing.T, i int) {
	t.Helper()
	require.Greater(t, i, 0, "no content before %q", l.lines)
	require.Empty(t, l.lines[i-1], "expected a blank line before line %d (%q)", i, l.lines[i])
	if i-2 >= 0 {
		require.NotEmpty(t, l.lines[i-2], "expected a single blank line before line %d (%q)", i, l.lines[i])
	}
}

// requireNeverFillsTerminalWidth asserts that every line is narrower than the
// terminal width. A line exactly as wide as the reported terminal width wraps
// even a remainder of one space on terminals that display slightly less than
// they report (scrollbars, terminal-side padding), which printed a blank line
// after every row of the banner.
func (l linesOf) requireNeverFillsTerminalWidth(t *testing.T, width int) {
	t.Helper()
	for i, line := range l.lines {
		require.Less(t, ansi.StringWidth(line), width,
			"line %d fills the terminal width and would wrap on frugal terminals: %q", i, line)
	}
}

func TestRender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		banner  config.ExitBanner
		sess    *session.Session
		want    []string
		notWant []string
		empty   bool
	}{
		{
			name:   "none prints nothing even with a session",
			banner: config.ExitBannerNone,
			sess:   testSession(),
			empty:  true,
		},
		{
			name:   "compact without a session prints nothing",
			banner: config.ExitBannerCompact,
			empty:  true,
		},
		{
			name:   "compact with a session prints only the resume lines",
			banner: config.ExitBannerCompact,
			sess:   testSession(),
			want:   []string{"Session", "Fix the flaky test", "Continue", "crush -s "},
			// No logo, and no padding around the two lines.
			notWant: []string{"Thanks for using Crush!"},
		},
		{
			name:   "default with a session prints the logo and the resume lines",
			banner: config.ExitBannerDefault,
			sess:   testSession(),
			want:   []string{"Thanks for using Crush!", "Session", "Fix the flaky test", "Continue"},
		},
		{
			name:   "default without a session still prints the logo",
			banner: config.ExitBannerDefault,
			want:   []string{"Thanks for using Crush!"},
			// The separator between logo and resume lines must not be left
			// dangling when there is no session.
			notWant: []string{"Continue"},
		},
		{
			name:   "unrecognized values fall back to the full banner",
			banner: config.ExitBanner("wat"),
			sess:   testSession(),
			want:   []string{"Thanks for using Crush!", "Continue"},
		},
		{
			name: "zero value falls back to the full banner",
			sess: testSession(),
			want: []string{"Thanks for using Crush!", "Continue"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Render(tt.banner, tt.sess, 100)
			if tt.empty {
				require.Empty(t, got)
				return
			}
			require.NotEmpty(t, got)
			for _, want := range tt.want {
				require.Contains(t, got, want)
			}
			for _, notWant := range tt.notWant {
				require.NotContains(t, got, notWant)
			}
		})
	}
}

func TestRenderCompactIsBlankLinePlusTwoResumeLines(t *testing.T) {
	t.Parallel()

	got := Render(config.ExitBannerCompact, testSession(), 100)
	lines := strings.Split(got, "\n")
	require.Len(t, lines, 3, "compact banner must be one blank line plus the two resume lines")
	require.Empty(t, lines[0])
	require.Contains(t, lines[1], "Session")
	require.Contains(t, lines[2], "crush -s ")
	// Nothing after the resume lines.
	require.False(t, strings.HasSuffix(got, "\n"))
}

// TestDefaultBannerLayout pins the exact blank-line layout of the full banner
// for every style width, including the fallback used when stdout is not a TTY:
//
//	(blank line, separating the banner from the shell prompt)
//	logo rows, printed consecutively with no blank lines between them
//	(one blank line between the logo block and the greeting)
//	greeting
//	(one blank line between the greeting and the resume lines)
//	resume lines, when a session is active
//	(nothing after the last line)
func TestDefaultBannerLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		width int
	}{
		{"fallback (non-TTY)", 0},
		{"40", 40},
		{"80", 80},
		{"100", 100},
		{"200", 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			width := tt.width
			if width == 0 {
				width = FallbackWidth
			}

			for name, sess := range map[string]*session.Session{
				"withSession":    testSession(),
				"withoutSession": nil,
			} {
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					got := Render(config.ExitBannerDefault, sess, tt.width)
					l := newLines(got)

					// A single blank line at the very start, separating the
					// banner from the shell command that launched Crush.
					require.Empty(t, l.lines[0])

					// Exactly the expected blank lines: one leading, one
					// before the greeting, and with a session one more before
					// the resume lines. Their exact placement below pins the
					// entire layout; in particular the logo's rows print
					// consecutively with no blank lines between them, and a
					// wrapped greeting is never split by a blank line.
					var wantBlanks []int
					greeting := l.indexContaining("Thanks for using Crush!")
					require.GreaterOrEqual(t, greeting, 0)
					wantBlanks = append(wantBlanks, 0, greeting-1)
					if sess != nil {
						session := l.indexContaining("Session")
						require.GreaterOrEqual(t, session, 0)
						wantBlanks = append(wantBlanks, session-1)
					}
					require.Equal(t, wantBlanks, l.blankIndexes())

					// Exactly one blank line before the greeting, and (with a
					// session) exactly one before the resume lines too.
					l.requireSingleBlankBefore(t, greeting)

					// Nothing but a trailing newline (added by the caller)
					// after the last line of the banner.
					require.NotEmpty(t, l.lines[len(l.lines)-1])

					if sess != nil {
						// Exactly one blank line between the greeting and the
						// resume lines, which are the last lines of the
						// banner (nothing but the trailing newline added by
						// the caller after them).
						session := l.indexContaining("Session")
						require.GreaterOrEqual(t, session, 0)
						l.requireSingleBlankBefore(t, session)
						require.Len(t, l.lines, session+2)
					} else {
						// Without a session, the greeting (possibly wrapped
						// over several lines) is the last thing printed: the
						// remainder after the last blank line is all
						// non-blank.
						require.Equal(t, len(l.lines)-1, len(l.lines)-1)
					}

					// Nothing after the last line of the banner.
					require.NotEmpty(t, l.lines[len(l.lines)-1])
					require.False(t, strings.HasSuffix(got, "\n"))

					// No line is ever as wide as the terminal.
					l.requireNeverFillsTerminalWidth(t, width)
				})
			}
		})
	}
}

func TestGreetingWrapsOnNarrowTerminals(t *testing.T) {
	t.Parallel()

	// The greeting must wrap instead of overflowing a narrow terminal, and
	// every wrapped line must fit, again without ever reaching the terminal
	// width.
	for _, width := range []int{40, 80} {
		contentWidth := width - 2*leftPadding
		greeting := "Thanks for using Crush! " + strings.Repeat("have a gorgeous time. ", 10)
		got := wrapGreeting(greeting, contentWidth)
		for i, line := range strings.Split(got, "\n") {
			require.LessOrEqual(t, ansi.StringWidth(line), contentWidth,
				"greeting line %d overflows the content width at %d columns: %q", i, width, line)
		}

		// Anything wrapping to a multiple of lines must actually wrap rather
		// than overflow or be padded to the full width.
		require.Greater(t, len(strings.Split(got, "\n")), 1,
			"long greeting did not wrap at %d columns", width)
	}
}

func TestRenderTruncatesLongTitles(t *testing.T) {
	t.Parallel()

	sess := testSession()
	sess.Title = strings.Repeat("long ", 60)

	got := Render(config.ExitBannerCompact, sess, 40)
	for _, line := range strings.Split(got, "\n") {
		require.LessOrEqual(t, ansi.StringWidth(line), 40, "line wider than the terminal: %q", line)
	}
	require.Contains(t, got, "…")
}

func TestRenderLongTitlesNeverFillTheTerminal(t *testing.T) {
	t.Parallel()

	sess := testSession()
	sess.Title = strings.Repeat("long ", 60)

	for _, width := range []int{40, 80} {
		for _, banner := range []config.ExitBanner{config.ExitBannerCompact, config.ExitBannerDefault} {
			got := Render(banner, sess, width)
			newLines(got).requireNeverFillsTerminalWidth(t, width)
		}
	}
}

func TestRenderFlattensNewlinesInTitles(t *testing.T) {
	t.Parallel()

	sess := testSession()
	sess.Title = "first\nsecond"

	got := Render(config.ExitBannerCompact, sess, 100)
	lines := strings.Split(got, "\n")
	require.Len(t, lines, 3)
	require.Contains(t, got, "first second")
}

func TestRenderFitsTheGivenWidth(t *testing.T) {
	t.Parallel()

	for _, width := range []int{40, 80, 200} {
		for _, banner := range []config.ExitBanner{config.ExitBannerCompact, config.ExitBannerDefault} {
			got := Render(banner, testSession(), width)
			for _, line := range strings.Split(got, "\n") {
				require.LessOrEqual(t, ansi.StringWidth(line), width,
					"%s banner overflows width %d: %q", banner, width, line)
			}
		}
	}
}

func TestRenderNonPositiveWidthUsesFallback(t *testing.T) {
	t.Parallel()

	sess := testSession()
	sess.Title = strings.Repeat("long ", 60)

	got := Render(config.ExitBannerCompact, sess, 0)
	for _, line := range strings.Split(got, "\n") {
		require.LessOrEqual(t, ansi.StringWidth(line), FallbackWidth)
	}
	newLines(got).requireNeverFillsTerminalWidth(t, FallbackWidth)
}
