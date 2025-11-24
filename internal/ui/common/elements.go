package common

import (
	"cmp"
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

func PrettyPath(t *styles.Styles, path string, width int) string {
	formatted := home.Short(path)
	return t.Muted.Width(width).Render(formatted)
}

type ModelContextInfo struct {
	ContextUsed  int64
	ModelContext int64
	Cost         float64
}

func ModelInfo(t *styles.Styles, modelName string, reasoningInfo string, context *ModelContextInfo, width int) string {
	modelIcon := t.Subtle.Render(styles.ModelIcon)
	modelName = t.Base.Render(modelName)
	modelInfo := fmt.Sprintf("%s %s", modelIcon, modelName)

	parts := []string{
		modelInfo,
	}

	if reasoningInfo != "" {
		parts = append(parts, t.Subtle.PaddingLeft(2).Render(reasoningInfo))
	}

	if context != nil {
		parts = append(parts, formatTokensAndCost(t, context.ContextUsed, context.ModelContext, context.Cost))
	}

	return lipgloss.NewStyle().Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
	)
}

func formatTokensAndCost(t *styles.Styles, tokens, contextWindow int64, cost float64) string {
	var formattedTokens string
	switch {
	case tokens >= 1_000_000:
		formattedTokens = fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	case tokens >= 1_000:
		formattedTokens = fmt.Sprintf("%.1fK", float64(tokens)/1_000)
	default:
		formattedTokens = fmt.Sprintf("%d", tokens)
	}

	if strings.HasSuffix(formattedTokens, ".0K") {
		formattedTokens = strings.Replace(formattedTokens, ".0K", "K", 1)
	}
	if strings.HasSuffix(formattedTokens, ".0M") {
		formattedTokens = strings.Replace(formattedTokens, ".0M", "M", 1)
	}

	percentage := (float64(tokens) / float64(contextWindow)) * 100

	formattedCost := t.Muted.Render(fmt.Sprintf("$%.2f", cost))

	formattedTokens = t.Subtle.Render(fmt.Sprintf("(%s)", formattedTokens))
	formattedPercentage := t.Muted.Render(fmt.Sprintf("%d%%", int(percentage)))
	formattedTokens = fmt.Sprintf("%s %s", formattedPercentage, formattedTokens)
	if percentage > 80 {
		formattedTokens = fmt.Sprintf("%s %s", styles.WarningIcon, formattedTokens)
	}

	return fmt.Sprintf("%s %s", formattedTokens, formattedCost)
}

type StatusOpts struct {
	Icon             string // if empty no icon will be shown
	Title            string
	TitleColor       color.Color
	Description      string
	DescriptionColor color.Color
	ExtraContent     string // additional content to append after the description
}

func Status(t *styles.Styles, opts StatusOpts, width int) string {
	icon := opts.Icon
	title := opts.Title
	description := opts.Description

	titleColor := cmp.Or(opts.TitleColor, t.Muted.GetForeground())
	descriptionColor := cmp.Or(opts.DescriptionColor, t.Subtle.GetForeground())

	title = t.Base.Foreground(titleColor).Render(title)

	if description != "" {
		extraContentWidth := lipgloss.Width(opts.ExtraContent)
		if extraContentWidth > 0 {
			extraContentWidth += 1
		}
		description = ansi.Truncate(description, width-lipgloss.Width(icon)-lipgloss.Width(title)-2-extraContentWidth, "…")
		description = t.Base.Foreground(descriptionColor).Render(description)
	}

	content := []string{}
	if icon != "" {
		content = append(content, icon)
	}
	content = append(content, title)
	if description != "" {
		content = append(content, description)
	}
	if opts.ExtraContent != "" {
		content = append(content, opts.ExtraContent)
	}

	return strings.Join(content, " ")
}
