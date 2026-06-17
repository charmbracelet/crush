package agent

import (
	"context"
	"errors"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// fakeLanguageModel is a fantasy.LanguageModel whose Stream call always
// returns the configured error (or a configured text response). It is the
// minimal stub needed to exercise the title generation fallback path
// without contacting any real provider.
type fakeLanguageModel struct {
	streamText string
	streamErr  error
}

func (m *fakeLanguageModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not used in title tests")
}

func (m *fakeLanguageModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	return nil, errors.New("test must set streamText or streamErr")
}

func (m *fakeLanguageModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not used in title tests")
}

func (m *fakeLanguageModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not used in title tests")
}

func (m *fakeLanguageModel) Provider() string { return "fake" }
func (m *fakeLanguageModel) Model() string    { return "fake-model" }

// stringStreamResponse is the minimum StreamResponse that produces a
// non-empty text payload, matching what the title path reads from
// resp.Response.Content.Text().
type stringStreamResponse struct {
	resp fantasy.Response
}

func (s *stringStreamResponse) Response() *fantasy.Response { return &s.resp }

func newTitleAgent(t *testing.T, sessions session.Service, model fantasy.LanguageModel) *sessionAgent {
	t.Helper()
	m := Model{
		Model: model,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
		ModelCfg: config.SelectedModel{Provider: "fake", Model: "fake-model"},
	}
	sa := NewSessionAgent(SessionAgentOptions{
		LargeModel: m,
		SmallModel: m,
		Sessions:   sessions,
	}).(*sessionAgent)
	return sa
}

func TestGenerateTitle_FallsBackToUserPrompt_OnModelError(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "ignored")
	require.NoError(t, err)

	agent := newTitleAgent(t, env.sessions, &fakeLanguageModel{
		streamErr: errors.New("no network"),
	})

	short := "Help me fix the build"
	agent.generateTitle(t.Context(), sess.ID, short)

	got, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, short, got.Title, "short prompt should be used verbatim as fallback")
}

func TestGenerateTitle_TruncatesLongPrompt_WithEllipsis(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "ignored")
	require.NoError(t, err)

	agent := newTitleAgent(t, env.sessions, &fakeLanguageModel{
		streamErr: errors.New("no network"),
	})

	// 86 ASCII characters; rune count is 86, well above the 50-char truncation
	// threshold.
	long := "This is a fairly long first message that should definitely be truncated to fifty chars"
	require.Len(t, []rune(long), 86)

	agent.generateTitle(t.Context(), sess.ID, long)

	got, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, string([]rune(long)[:50])+"…", got.Title)
}

func TestGenerateTitle_TruncatesByRune_NotByte(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "ignored")
	require.NoError(t, err)

	agent := newTitleAgent(t, env.sessions, &fakeLanguageModel{
		streamErr: errors.New("no network"),
	})

	// 30 ASCII chars + Chinese characters; byte length is far above
	// 50, but rune count exceeds 50 as well — must be truncated to 50 runes
	// (a byte-based slice would cut in the middle of a multi-byte char).
	prefix := "abcdefghijklmnopqrstuvwxyz1234" // 30 runes
	suffix := "一二三四五六七八九十甲乙丙丁戊己庚辛壬癸子丑寅卯辰巳午未" // 30 runes
	prompt := prefix + suffix
	require.Greater(t, len([]rune(prompt)), 50, "rune count should exceed truncation threshold")
	require.Greater(t, len(prompt), 50, "byte length should exceed rune count")
	require.NotEqual(t, prompt[:50], string([]rune(prompt)[:50]),
		"first 50 bytes must not equal first 50 runes — otherwise the test is not exercising rune-aware truncation")

	agent.generateTitle(t.Context(), sess.ID, prompt)

	got, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)

	want := string([]rune(prompt)[:50]) + "…"
	require.Equal(t, want, got.Title, "truncation must happen at rune boundary, not byte boundary")
}

func TestGenerateTitle_EmptyPrompt_NoOp(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), DefaultSessionName)
	require.NoError(t, err)

	agent := newTitleAgent(t, env.sessions, &fakeLanguageModel{
		streamErr: errors.New("no network"),
	})

	// With empty userPrompt, generateTitle returns early and the
	// session title must remain whatever it was created with.
	agent.generateTitle(t.Context(), sess.ID, "")

	got, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Equal(t, DefaultSessionName, got.Title)
}
