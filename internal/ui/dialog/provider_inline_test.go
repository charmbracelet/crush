package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func newTestProviderInline() *ProviderInline {
	return NewProviderInline(common.DefaultCommon(nil), "Pick a provider to send this", []ProviderChoice{
		{ID: "hyper", Name: "Hyper", Detail: "Charm account"},
		{ID: "anthropic", Name: "Anthropic", Detail: "paste an API key", NeedsKey: true},
		{ID: ChoiceMoreProviders, Name: "More providers…"},
	})
}

func press(c *ProviderInline, code rune) (bool, tea.Cmd) {
	return c.HandleKey(tea.KeyPressMsg{Code: code})
}

func TestProviderInlineEnterSelectsHighlightedRow(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	var got ProviderChoice
	c.OnSelect = func(ch ProviderChoice) tea.Cmd { got = ch; return nil }

	done, _ := press(c, tea.KeyEnter)

	require.False(t, done, "the card stays while the sign-in runs")
	require.Equal(t, "hyper", got.ID)
}

func TestProviderInlineWrapsAround(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	press(c, tea.KeyUp)
	choice, ok := c.Selected()
	require.True(t, ok)
	require.Equal(t, ChoiceMoreProviders, choice.ID)

	press(c, tea.KeyDown)
	choice, _ = c.Selected()
	require.Equal(t, "hyper", choice.ID)
}

func TestProviderInlineEscapeCancels(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	cancelled := false
	c.OnCancel = func() tea.Cmd { cancelled = true; return nil }

	done, _ := press(c, tea.KeyEscape)

	require.True(t, done)
	require.True(t, cancelled)
}

func TestProviderInlineAsksForAKeyInPlace(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	selected := false
	c.OnSelect = func(ProviderChoice) tea.Cmd { selected = true; return nil }
	var submitted ProviderChoice
	var submittedKey string
	c.OnSubmitKey = func(ch ProviderChoice, key string) tea.Cmd {
		submitted, submittedKey = ch, key
		return nil
	}

	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)
	require.True(t, c.AskingForKey())
	require.False(t, selected, "key-only providers do not open a sign-in dialog")

	c.HandleKey(tea.KeyPressMsg{Code: 's', Text: "s"})
	c.HandleKey(tea.KeyPressMsg{Code: 'k', Text: "k"})
	press(c, tea.KeyEnter)

	require.Equal(t, "anthropic", submitted.ID)
	require.Equal(t, "sk", submittedKey)
}

func TestProviderInlineEmptyKeyIsNotSubmitted(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	called := false
	c.OnSubmitKey = func(ProviderChoice, string) tea.Cmd { called = true; return nil }

	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)
	press(c, tea.KeyEnter)

	require.False(t, called)
}

func TestProviderInlineEscapeFromKeyReturnsToList(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	cancelled := false
	c.OnCancel = func() tea.Cmd { cancelled = true; return nil }

	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)
	done, _ := press(c, tea.KeyEscape)

	require.False(t, done)
	require.False(t, cancelled, "esc steps back before it gives up")
	require.False(t, c.AskingForKey())
}

func TestProviderInlineRejectedKeyCanBeRetried(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	calls := 0
	c.OnSubmitKey = func(ProviderChoice, string) tea.Cmd { calls++; return nil }

	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)
	c.HandleKey(tea.KeyPressMsg{Code: 'a', Text: "a"})
	press(c, tea.KeyEnter)
	require.Equal(t, 1, calls)

	// While the key is being checked, further input is ignored.
	press(c, tea.KeyEnter)
	require.Equal(t, 1, calls)

	c.KeyResult(false)
	require.True(t, c.AskingForKey())
	require.Contains(t, ansi.Strip(c.render(60)), "That key didn't work")

	c.HandleKey(tea.KeyPressMsg{Code: 'b', Text: "b"})
	press(c, tea.KeyEnter)
	require.Equal(t, 2, calls)
}

func TestProviderInlineKeyStepIsAsTallAsTheList(t *testing.T) {
	t.Parallel()

	// The key step needs fewer rows than a full provider list, but the card
	// keeps the list's height so the screen does not jump between steps.
	choices := []ProviderChoice{
		{ID: "hyper", Name: "Hyper"},
		{ID: "anthropic", Name: "Anthropic", NeedsKey: true},
		{ID: "openai", Name: "OpenAI"},
		{ID: "copilot", Name: "GitHub Copilot"},
		{ID: ChoiceMoreProviders, Name: "More providers…"},
	}
	c := NewProviderInline(common.DefaultCommon(nil), "Pick a provider", choices)

	listHeight := c.Height(80)
	require.Equal(t, 7, listHeight, "question, blank line and five rows")
	require.Equal(t, listHeight, len(strings.Split(c.render(60), "\n")))

	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)
	require.True(t, c.AskingForKey())

	require.GreaterOrEqual(t, c.Height(80), listHeight)
	require.Equal(t, listHeight, len(strings.Split(c.render(60), "\n")), "rendered rows match the height")
	require.False(t, c.HeightChanged(), "moving between steps does not change the height")
}

func TestProviderInlineShortListStillFitsTheKeyStep(t *testing.T) {
	t.Parallel()

	c := NewProviderInline(common.DefaultCommon(nil), "Pick a provider", []ProviderChoice{
		{ID: "anthropic", Name: "Anthropic", NeedsKey: true},
	})

	require.Equal(t, keyStepHeight, c.Height(80), "never shorter than the key step itself")
}

func TestProviderInlineHeightFollowsTheChoices(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	require.Equal(t, 5, c.Height(80), "question, blank line and three rows")
	require.False(t, c.HeightChanged())

	c.choices = append(c.choices, ProviderChoice{ID: "x", Name: "X"})
	require.True(t, c.HeightChanged())
}

func TestProviderInputHasOnlyAPrompt(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)
	c.HandleKey(tea.KeyPressMsg{Code: 'a', Text: "a"})

	lines := strings.Split(ansi.Strip(c.render(60)), "\n")
	input := lines[2]
	require.True(t, strings.HasPrefix(input, "> "), "the input starts with a plain prompt: %q", input)
	require.NotContains(t, input, "┃", "no bar in front of the prompt")
	require.NotContains(t, input, "›")
}

func TestProviderInlineMasksTheKey(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)
	c.HandleKey(tea.KeyPressMsg{Code: 's', Text: "s"})
	c.HandleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	c.HandleKey(tea.KeyPressMsg{Code: 'c', Text: "c"})

	view := ansi.Strip(c.render(60))
	require.NotContains(t, view, "sec", "the key must not be echoed")
	require.Contains(t, view, "Anthropic")
	require.Contains(t, view, "API key")
}

func TestProviderInlineRendersTitleAndRows(t *testing.T) {
	t.Parallel()

	view := ansi.Strip(newTestProviderInline().render(100))

	require.Contains(t, view, "Pick a provider to send this")
	require.Contains(t, view, "Charm account")
	require.Contains(t, view, "More providers…")
}

func TestProviderInlineHelpFollowsTheStep(t *testing.T) {
	t.Parallel()

	helps := func(c *ProviderInline) string {
		var parts []string
		for _, b := range c.ShortHelp() {
			parts = append(parts, b.Help().Desc)
		}
		return strings.Join(parts, ",")
	}

	c := newTestProviderInline()
	require.Equal(t, "choose,continue,keep editing", helps(c))

	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)
	require.Equal(t, "save and send,back", helps(c))
}

func TestProviderInlineFitsNarrowTerminals(t *testing.T) {
	t.Parallel()

	for _, l := range strings.Split(ansi.Strip(newTestProviderInline().render(30)), "\n") {
		require.LessOrEqual(t, ansi.StringWidth(l), 30, "line %q overflows", l)
	}
}

func TestProviderInlineCursorIsRelativeToTheEditorArea(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	c.SetFocused(true)
	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)
	c.HandleKey(tea.KeyPressMsg{Code: 'a', Text: "a"})
	c.HandleKey(tea.KeyPressMsg{Code: 'b', Text: "b"})

	// The area sits well away from the screen's origin; the cursor must not
	// include that offset, because the UI adds it.
	scr := uv.NewScreenBuffer(100, 40)
	cur := c.Draw(scr, uv.Rect(20, 30, 70, 4))

	require.NotNil(t, cur)
	require.Equal(t, 2, cur.Y, "the input is the third row of the card")
	require.Less(t, cur.X, 20, "no screen-space X offset")
	require.Greater(t, cur.X, providerInlineIndent, "after the prompt and typed characters")
}

func TestProviderInlineShowsNoCursorUnlessAskingForAKey(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	c.SetFocused(true)
	require.Nil(t, c.Draw(uv.NewScreenBuffer(100, 40), uv.Rect(0, 30, 80, 5)))
}

func TestProviderInlineOnChooseSeesEveryRow(t *testing.T) {
	t.Parallel()

	c := newTestProviderInline()
	var chosen []string
	c.OnChoose = func(ch ProviderChoice) { chosen = append(chosen, ch.ID) }
	c.OnSelect = func(ProviderChoice) tea.Cmd { return nil }

	press(c, tea.KeyEnter)
	press(c, tea.KeyDown)
	press(c, tea.KeyEnter)

	require.Equal(t, []string{"hyper", "anthropic"}, chosen)
	require.True(t, c.AskingForKey(), "a key-only row still asks for its key")
}
