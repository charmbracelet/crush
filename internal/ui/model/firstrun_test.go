package model

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/catwalk/v3/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/tips"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// newFirstRunTestUI builds a UI wired to a workspace with the given config.
func newFirstRunTestUI(t *testing.T, cfg *config.Config) *UI {
	t.Helper()

	u := newTestUI()
	u.state = uiLanding
	u.dialog = dialog.NewOverlay()
	u.com.Workspace = &testWorkspace{cfg: cfg}
	return u
}

func emptyConfig() *config.Config {
	return &config.Config{
		Options:   &config.Options{},
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
}

func TestProviderCardChoices(t *testing.T) {
	t.Parallel()

	t.Run("lists featured providers the catalog offers, then more", func(t *testing.T) {
		t.Parallel()

		choices := providerCardChoices([]catwalk.Provider{
			{ID: catwalk.InferenceProviderOpenAI},
			{ID: "hyper"},
			{ID: catwalk.InferenceProviderAnthropic},
			{ID: catwalk.InferenceProviderGemini},
		})

		var ids []string
		for _, c := range choices {
			ids = append(ids, c.ID)
		}
		require.Equal(t, []string{"hyper", "anthropic", "openai", dialog.ChoiceMoreProviders}, ids)
	})

	t.Run("always offers the full list", func(t *testing.T) {
		t.Parallel()

		choices := providerCardChoices(nil)
		require.Len(t, choices, 1)
		require.Equal(t, dialog.ChoiceMoreProviders, choices[0].ID)
	})

	t.Run("does not promise an Anthropic sign-in", func(t *testing.T) {
		t.Parallel()

		for _, c := range providerCardChoices([]catwalk.Provider{{ID: catwalk.InferenceProviderAnthropic}}) {
			if c.ID == string(catwalk.InferenceProviderAnthropic) {
				require.NotContains(t, c.Detail, "sign in")
			}
		}
	})
}

func TestDefaultSelection(t *testing.T) {
	t.Parallel()

	models := []catwalk.Model{
		{ID: "small", DefaultMaxTokens: 100},
		{ID: "big", DefaultMaxTokens: 900},
	}

	t.Run("uses the default large model", func(t *testing.T) {
		t.Parallel()

		got, ok := defaultSelection(catwalk.Provider{ID: "p", DefaultLargeModelID: "big", Models: models})
		require.True(t, ok)
		require.Equal(t, config.SelectedModel{Model: "big", Provider: "p", MaxTokens: 900}, got)
	})

	t.Run("falls back to the first model", func(t *testing.T) {
		t.Parallel()

		got, ok := defaultSelection(catwalk.Provider{ID: "p", DefaultLargeModelID: "gone", Models: models})
		require.True(t, ok)
		require.Equal(t, "small", got.Model)
	})

	t.Run("reports a provider with no models", func(t *testing.T) {
		t.Parallel()

		_, ok := defaultSelection(catwalk.Provider{ID: "p"})
		require.False(t, ok)
	})
}

func TestCredentialSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pc   config.ProviderConfig
		want string
	}{
		{"env var", config.ProviderConfig{Name: "Anthropic", APIKey: "$ANTHROPIC_API_KEY"}, "ANTHROPIC_API_KEY"},
		{"braced env var", config.ProviderConfig{Name: "Anthropic", APIKey: "${ANTHROPIC_API_KEY}"}, "ANTHROPIC_API_KEY"},
		{"stored key", config.ProviderConfig{Name: "Anthropic", APIKey: "sk-ant-secret"}, "your Anthropic key"},
		{"ant login", config.ProviderConfig{Name: "Anthropic", APIKey: "$ANTHROPIC_API_KEY", ExternalAuth: config.ExternalAuthAnt}, "your ant login"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, credentialSource(tt.pc))
		})
	}
}

func TestWelcomeView(t *testing.T) {
	t.Parallel()

	t.Run("without a provider it is only the intro", func(t *testing.T) {
		t.Parallel()

		u := newFirstRunTestUI(t, emptyConfig())
		u.firstRun.noProvider = true

		view := ansi.Strip(u.welcomeView(100))
		require.Contains(t, view, introLine1)
		require.Contains(t, view, introLine2)
		require.NotContains(t, view, "Ask anything", "the instruction is anchored at the bottom instead")
		require.NotContains(t, view, "/init")
	})

	t.Run("the intro steps aside while a provider is asked for", func(t *testing.T) {
		t.Parallel()

		u := newFirstRunTestUI(t, emptyConfig())
		u.firstRun.noProvider = true
		u.firstRun.held = &heldMessage{content: "write tests"}

		require.Empty(t, u.welcomeView(100))
	})

	t.Run("names the model and where its key comes from", func(t *testing.T) {
		t.Parallel()

		cfg := emptyConfig()
		cfg.Providers.Set("anthropic", config.ProviderConfig{ID: "anthropic", Name: "Anthropic", APIKey: "$ANTHROPIC_API_KEY"})
		u := newFirstRunTestUI(t, cfg)
		u.agentReady = true
		u.agentModel = workspace.AgentModel{
			CatwalkCfg: catwalk.Model{Name: "Claude Sonnet 5.5"},
			ModelCfg:   config.SelectedModel{Provider: "anthropic", Model: "claude"},
		}

		view := ansi.Strip(u.welcomeView(100))
		require.Contains(t, view, "Using Claude Sonnet 5.5 through ANTHROPIC_API_KEY.")
		require.Contains(t, view, "Switch any time with ctrl+l.")
		require.Contains(t, view, introLine1)
	})

	t.Run("offers /init only while the project needs it", func(t *testing.T) {
		t.Parallel()

		cfg := emptyConfig()
		cfg.Providers.Set("anthropic", config.ProviderConfig{ID: "anthropic", Name: "Anthropic", APIKey: "$ANTHROPIC_API_KEY"})
		u := newFirstRunTestUI(t, cfg)
		u.firstRun.needsInit = true

		require.Contains(t, ansi.Strip(u.welcomeView(100)), "No AGENTS.md in this project yet. /init creates one.")

		u.firstRun.needsInit = false
		require.NotContains(t, ansi.Strip(u.welcomeView(100)), "/init")
	})
}

func TestFirstRunPromptView(t *testing.T) {
	t.Parallel()

	t.Run("the instruction has no model icon", func(t *testing.T) {
		t.Parallel()

		u := newFirstRunTestUI(t, emptyConfig())
		u.firstRun.noProvider = true

		view := ansi.Strip(u.firstRunPromptView(100))
		require.Equal(t, askAnythingLine, view)
		require.NotContains(t, view, "◇", "that icon is reserved for a model name or response")
	})

	t.Run("shows a held message instead", func(t *testing.T) {
		t.Parallel()

		u := newFirstRunTestUI(t, emptyConfig())
		u.firstRun.noProvider = true
		u.firstRun.held = &heldMessage{content: "write tests\nfor the handler"}

		view := ansi.Strip(u.firstRunPromptView(100))
		require.Contains(t, view, "▌ write tests")
		require.NotContains(t, view, "for the handler")
		require.Contains(t, view, "held · sends as soon as a model is ready")
		require.NotContains(t, view, "Ask anything")
	})
}

func TestAnchorBottom(t *testing.T) {
	t.Parallel()

	lines := func(s string) []string { return strings.Split(ansi.Strip(s), "\n") }

	t.Run("pins the bottom block to the last row", func(t *testing.T) {
		t.Parallel()

		got := lines(anchorBottom(20, 10, "top", "bottom"))
		require.Len(t, got, 10)
		require.Equal(t, "", strings.TrimSpace(got[0]), "one row of padding first")
		require.Equal(t, "top", strings.TrimSpace(got[1]))
		require.Equal(t, "bottom", strings.TrimSpace(got[9]))
	})

	t.Run("a multi-line bottom block stays together", func(t *testing.T) {
		t.Parallel()

		got := lines(anchorBottom(20, 10, "top", "one\ntwo"))
		require.Equal(t, "one", strings.TrimSpace(got[8]))
		require.Equal(t, "two", strings.TrimSpace(got[9]))
	})

	t.Run("drops the top block before it would cut the bottom one", func(t *testing.T) {
		t.Parallel()

		got := lines(anchorBottom(20, 4, "top\nof\nthe\npage", "bottom"))
		require.Len(t, got, 4)
		require.Contains(t, strings.Join(got, "\n"), "bottom")
		require.NotContains(t, strings.Join(got, "\n"), "page")
	})

	t.Run("works without a top block", func(t *testing.T) {
		t.Parallel()

		got := lines(anchorBottom(20, 6, "", "bottom"))
		require.Equal(t, "bottom", strings.TrimSpace(got[5]))
	})
}

func testChoices() []dialog.ProviderChoice {
	return []dialog.ProviderChoice{
		{ID: "hyper", Name: "Hyper"},
		{ID: "anthropic", Name: "Anthropic", NeedsKey: true},
		{ID: dialog.ChoiceMoreProviders, Name: "More providers…"},
	}
}

func TestProviderCardReplacesTheEditor(t *testing.T) {
	t.Parallel()

	u := newFirstRunTestUI(t, emptyConfig())
	u.firstRun.noProvider = true
	u.firstRun.held = &heldMessage{content: "now write tests"}

	require.Nil(t, u.showProviderCard(testChoices()))

	require.NotNil(t, u.providerCard(), "the card is the active inline editor")
	require.False(t, u.textarea.Focused(), "the textarea gives up focus while the card asks")
	require.Equal(t, uiFocusEditor, u.focus)
}

func TestBackingOutOfTheCardRestoresTheMessage(t *testing.T) {
	t.Parallel()

	u := newFirstRunTestUI(t, emptyConfig())
	u.firstRun.noProvider = true
	u.firstRun.held = &heldMessage{content: "now write tests"}
	u.showProviderCard(testChoices())

	done, _ := u.providerCard().HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape})

	require.True(t, done, "esc dismisses the card")
	require.Nil(t, u.firstRun.held)
	require.Equal(t, "now write tests", u.textarea.Value())
}

func TestCloseProviderCardGivesTheEditorBack(t *testing.T) {
	t.Parallel()

	u := newFirstRunTestUI(t, emptyConfig())
	u.showProviderCard(testChoices())

	u.closeProviderCard()

	require.Nil(t, u.activeInline)
	require.True(t, u.textarea.Focused())
	require.Nil(t, u.closeProviderCard(), "closing twice is a no-op")
}

func TestProviderKeyRejectionKeepsTheCardOpen(t *testing.T) {
	t.Parallel()

	u := newFirstRunTestUI(t, emptyConfig())
	u.firstRun.noProvider = true
	u.showProviderCard(testChoices())
	card := u.providerCard()
	card.OnSubmitKey = func(dialog.ProviderChoice, string) tea.Cmd { return nil }

	// Choose Anthropic, type a key and submit it.
	card.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	card.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, card.AskingForKey())
	card.HandleKey(tea.KeyPressMsg{Code: 'k', Text: "k"})
	card.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})

	cmd := u.handleProviderKeyResult(providerKeyResultMsg{rejected: true})

	require.Nil(t, cmd)
	require.NotNil(t, u.providerCard(), "a rejected key leaves the card for another try")
	require.True(t, u.isOnboarding())
}

func TestProviderKeyResultAfterLeavingTheCardIsIgnored(t *testing.T) {
	t.Parallel()

	u := newFirstRunTestUI(t, emptyConfig())
	require.Nil(t, u.handleProviderKeyResult(providerKeyResultMsg{rejected: true}))
}

func TestReleaseHeldMessageWithNothingHeld(t *testing.T) {
	t.Parallel()

	u := newFirstRunTestUI(t, emptyConfig())
	require.Nil(t, u.releaseHeldMessage())
}

func TestPermissionsTipShowsOnceForEdits(t *testing.T) {
	t.Parallel()

	u := newFirstRunTestUI(t, emptyConfig())
	u.tips = tips.Open(filepath.Join(t.TempDir(), "tips.json"))

	tip, claim := u.permissionsTipFor(tools.BashToolName)
	require.Empty(t, tip, "only edits get the tip")
	require.Nil(t, claim)

	tip, claim = u.permissionsTipFor(tools.EditToolName)
	require.Equal(t, permissionsTip, tip)
	require.NotNil(t, claim)
	require.Nil(t, claim(), "claiming the tip returns no message")

	tip, claim = u.permissionsTipFor(tools.EditToolName)
	require.Empty(t, tip, "the tip is shown once")
	require.Nil(t, claim)
}

func TestFoldingTipOnlyForLongOutputAndOnlyOnce(t *testing.T) {
	t.Parallel()

	long := func(lines int) message.ToolResult {
		content := ""
		for range lines {
			content += "line\n"
		}
		return message.ToolResult{Content: content}
	}

	u := newFirstRunTestUI(t, emptyConfig())
	u.tips = tips.Open(filepath.Join(t.TempDir(), "tips.json"))

	require.Nil(t, u.foldingTipFor(long(5)), "short output is not folded")
	require.Nil(t, u.foldingTipFor(message.ToolResult{Content: "x\n\n\n\n\n\n\n\n\n\n\n\n\n", IsError: true}), "errors are not folded")
	require.NotNil(t, u.foldingTipFor(long(40)))

	ok, err := u.tips.Claim(tips.Folding)
	require.NoError(t, err)
	require.True(t, ok)
	require.Nil(t, u.foldingTipFor(long(40)), "already shown")
}

func TestFirstRunDialogFront(t *testing.T) {
	t.Parallel()

	newUI := func(noProvider bool, dlg dialog.Dialog) *UI {
		u := newFirstRunTestUI(t, emptyConfig())
		u.firstRun.noProvider = noProvider
		u.dialog.OpenDialog(dlg)
		return u
	}
	keyStep := func() dialog.Dialog {
		d, _ := dialog.NewAPIKeyInput(common.DefaultCommon(nil), true, catwalk.Provider{}, config.SelectedModel{}, config.SelectedModelTypeLarge)
		return d
	}

	require.True(t, newUI(true, keyStep()).firstRunDialogFront(),
		"a first-run step hides the card, landing page and status line")
	require.False(t, newUI(false, keyStep()).firstRunDialogFront(),
		"outside first run nothing is hidden")
	require.False(t, newUI(true, dialog.NewQuit(common.DefaultCommon(nil), 0)).firstRunDialogFront(),
		"unrelated dialogs keep the rest of the screen")

	u := newFirstRunTestUI(t, emptyConfig())
	u.firstRun.noProvider = true
	require.False(t, u.firstRunDialogFront(), "with no dialog open the card shows")
}
