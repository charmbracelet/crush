package exitbanner

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

var testStyles = styles.CharmtonePantera()

func testSession() *session.Session {
	return &session.Session{ID: "abc123def456", Title: "Fix the flaky test"}
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

			got := Render(&testStyles, tt.banner, tt.sess, 100)
			if tt.empty {
				require.Empty(t, got)
				return
			}
			require.NotEmpty(t, got)
			for _, want := range tt.want {
				require.Contains(t, ansi.Strip(got), want)
			}
			for _, notWant := range tt.notWant {
				require.NotContains(t, ansi.Strip(got), notWant)
			}
		})
	}
}

func TestRenderCompactIsExactlyTwoLines(t *testing.T) {
	t.Parallel()

	got := Render(&testStyles, config.ExitBannerCompact, testSession(), 100)
	require.Len(t, strings.Split(got, "\n"), 2)
}

func TestRenderTruncatesLongTitles(t *testing.T) {
	t.Parallel()

	sess := testSession()
	sess.Title = strings.Repeat("long ", 60)

	got := Render(&testStyles, config.ExitBannerCompact, sess, 40)
	for _, line := range strings.Split(got, "\n") {
		require.LessOrEqual(t, ansi.StringWidth(line), 40, "line wider than the terminal: %q", line)
	}
	require.Contains(t, got, "…")
}

func TestRenderFlattensNewlinesInTitles(t *testing.T) {
	t.Parallel()

	sess := testSession()
	sess.Title = "first\nsecond"

	got := Render(&testStyles, config.ExitBannerCompact, sess, 100)
	require.Len(t, strings.Split(got, "\n"), 2)
	require.Contains(t, got, "first second")
}

func TestRenderFitsTheGivenWidth(t *testing.T) {
	t.Parallel()

	for _, width := range []int{40, 80, 200} {
		for _, banner := range []config.ExitBanner{config.ExitBannerCompact, config.ExitBannerDefault} {
			got := Render(&testStyles, banner, testSession(), width)
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

	got := Render(&testStyles, config.ExitBannerCompact, sess, 0)
	for _, line := range strings.Split(got, "\n") {
		require.LessOrEqual(t, ansi.StringWidth(line), FallbackWidth)
	}
}

func TestRenderUsesActiveTheme(t *testing.T) {
	t.Parallel()

	theme, err := styles.HostThemeStyles("", map[string]string{
		"primary": "#12ab34", "secondary": "#56cd78", "fg_base": "#9aefbc",
	}, nil)
	require.NoError(t, err)

	for _, banner := range []config.ExitBanner{config.ExitBannerDefault, config.ExitBannerCompact} {
		t.Run(string(banner), func(t *testing.T) {
			t.Parallel()
			got := Render(&theme, banner, testSession(), 100)
			require.Contains(t, got, "\x1b[1mSession  \x1b[mFix the flaky test")
			require.Contains(t, got, "\x1b[1mContinue \x1b[mcrush -s 3d4a966")
			if banner == config.ExitBannerDefault {
				require.Contains(t, got, "38;2;86;205;120m")
				require.Contains(t, got, "\n   Thanks for using Crush!")
			}
		})
	}
}
