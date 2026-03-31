package model

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/timeline"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

func (m *UI) timelineInfo(width, maxItems int, isSection bool) string {
	t := m.com.Styles

	title := t.ResourceGroupTitle.Render("Recent Activity")
	if isSection {
		title = common.Section(t, title, width)
	}

	list := t.ResourceAdditionalText.Render("None")
	if len(m.timelineEvents) > 0 {
		list = timelineList(t, m.timelineEvents, width, maxItems)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

func timelineList(t *styles.Styles, events []timeline.Event, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}

	visibleLimit := min(len(events), maxItems)
	showOverflow := len(events) > maxItems && maxItems > 1
	if showOverflow {
		visibleLimit = maxItems - 1
	}

	rendered := make([]string, 0, visibleLimit+1)
	for i := len(events) - 1; i >= len(events)-visibleLimit; i-- {
		rendered = append(rendered, timelineListItem(t, events[i], width))
	}

	if showOverflow {
		hidden := len(events) - visibleLimit
		rendered = append(rendered, t.ResourceAdditionalText.Render(fmt.Sprintf("…and %d earlier", hidden)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rendered...)
}

func timelineListItem(t *styles.Styles, event timeline.Event, width int) string {
	title, description := timelineEventSummary(event)
	extra := ""
	if event.Timestamp > 0 {
		extra = t.ResourceAdditionalText.Render(time.UnixMilli(event.Timestamp).Format("15:04"))
	}

	return common.Status(t, common.StatusOpts{
		Title:        t.ResourceName.Render(title),
		Description:  description,
		ExtraContent: extra,
	}, width)
}

func timelineEventSummary(event timeline.Event) (string, string) {
	switch event.Type {
	case timeline.EventModeChanged:
		parts := make([]string, 0, 2)
		if label := timelineLabel(event.CollaborationMode); label != "" {
			parts = append(parts, label)
		}
		if label := timelineLabel(event.PermissionMode); label != "" {
			parts = append(parts, label)
		}
		if len(parts) == 0 {
			return "Mode", "updated"
		}
		return "Mode", strings.Join(parts, " • ")
	case timeline.EventToolStarted:
		title := timelineTitle(event.ToolName, event.Title, "Tool")
		if status := timelineLabel(event.Status); status != "" {
			return title, status
		}
		return title, "started"
	case timeline.EventToolProgress:
		title := timelineTitle(event.ToolName, event.Title, "Tool")
		if content := timelinePreview(event.Content); content != "" {
			return title, content
		}
		if status := timelineLabel(event.Status); status != "" {
			return title, status
		}
		return title, "updated"
	case timeline.EventToolFinished:
		title := timelineTitle(event.ToolName, event.Title, "Tool")
		status := timelineLabel(event.Status)
		content := timelinePreview(event.Content)
		switch {
		case status != "" && content != "":
			return title, status + " • " + content
		case status != "":
			return title, status
		case content != "":
			return title, content
		default:
			return title, "finished"
		}
	case timeline.EventChildSessionStarted:
		return "Subagent", timelineChildDescription("started", event.Title)
	case timeline.EventChildSessionFinished:
		status := timelineLabel(event.Status)
		if status == "" {
			status = "finished"
		}
		return "Subagent", timelineChildDescription(status, event.Title)
	default:
		title := timelineTitle(string(event.Type), event.Title, "Event")
		if content := timelinePreview(event.Content); content != "" {
			return title, content
		}
		return title, ""
	}
}

func timelineChildDescription(status, title string) string {
	title = timelinePreview(title)
	if title == "" {
		return status
	}
	return status + " • " + title
}

func timelineTitle(values ...string) string {
	for _, value := range values {
		if label := timelineLabel(value); label != "" {
			return label
		}
	}
	return "Event"
}

func timelineLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func timelinePreview(value string) string {
	value = ansi.Strip(value)
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	return ansi.Truncate(value, 48, "…")
}
