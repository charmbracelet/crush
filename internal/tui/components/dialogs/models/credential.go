package models

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/tui/util"
)

type CredentialSelectedMsg struct {
	Credential string
}

type CredentialOption struct {
	Name        string
	IsDefault   bool
	DisplayName string
}

type CredentialSelector struct {
	width             int
	height            int
	credentials       []CredentialOption
	selectedIndex     int
	providerID        string
	providerName      string
	selectedModel     *ModelOption
	selectedModelType config.SelectedModelType
}

func NewCredentialSelector(providerID, providerName string, credentials []config.ProviderCredential, selectedModel *ModelOption, modelType config.SelectedModelType) *CredentialSelector {
	opts := make([]CredentialOption, len(credentials))
	for i, cred := range credentials {
		displayName := cred.Name
		if cred.Default {
			displayName += " (default)"
		}
		opts[i] = CredentialOption{
			Name:        cred.Name,
			IsDefault:   cred.Default,
			DisplayName: displayName,
		}
	}

	return &CredentialSelector{
		credentials:       opts,
		selectedIndex:     0,
		providerID:        providerID,
		providerName:      providerName,
		selectedModel:     selectedModel,
		selectedModelType: modelType,
	}
}

func (c *CredentialSelector) Init() tea.Cmd {
	return nil
}

func (c *CredentialSelector) Update(msg tea.Msg) (util.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ", "k":
			if c.selectedIndex >= 0 && c.selectedIndex < len(c.credentials) {
				return c, util.CmdHandler(CredentialSelectedMsg{
					Credential: c.credentials[c.selectedIndex].Name,
				})
			}
		case "up", "ctrl+p":
			if c.selectedIndex > 0 {
				c.selectedIndex--
			}
		case "down", "ctrl+n":
			if c.selectedIndex < len(c.credentials)-1 {
				c.selectedIndex++
			}
		case "esc", "q":
			return c, util.CmdHandler(CloseModelDialogMsg{})
		}
	}
	return c, nil
}

func (c *CredentialSelector) SetSize(width, height int) tea.Cmd {
	c.width = width
	c.height = height
	return nil
}

func (c *CredentialSelector) View() string {
	if c.width == 0 {
		c.width = 50
	}
	if c.height == 0 {
		c.height = 20
	}

	t := styles.CurrentTheme()
	titleStyle := t.S().Title.PaddingLeft(1).Width(c.width - 2)
	headerStyle := t.S().Base.MarginLeft(1).Width(c.width - 4)
	itemStyle := t.S().Base.PaddingLeft(1).Width(c.width - 4)
	selectedStyle := t.S().TextSelected.PaddingLeft(1).Width(c.width - 4)

	var content strings.Builder

	content.WriteString(titleStyle.Render(fmt.Sprintf("Select Credential for %s", c.providerName)))
	content.WriteString("\n\n")
	content.WriteString(headerStyle.Render("Choose which credential to use:"))
	content.WriteString("\n\n")

	for i, cred := range c.credentials {
		if i == c.selectedIndex {
			content.WriteString(selectedStyle.Render(fmt.Sprintf("• %s", cred.DisplayName)))
		} else {
			content.WriteString(itemStyle.Render(fmt.Sprintf("  %s", cred.DisplayName)))
		}
		content.WriteString("\n")
	}

	helpStyle := t.S().Subtle.MarginTop(1).PaddingLeft(1)
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("↑↓: Navigate • Enter: Select • Esc: Cancel"))

	return t.S().Base.Render(content.String())
}

func (c *CredentialSelector) SelectedCredential() string {
	if c.selectedIndex >= 0 && c.selectedIndex < len(c.credentials) {
		return c.credentials[c.selectedIndex].Name
	}
	return ""
}
