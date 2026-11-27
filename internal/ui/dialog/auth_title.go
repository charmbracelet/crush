package dialog

import (
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// authTitle renders the title row of a sign-in dialog that is width cells
// wide, including its frame. During onboarding there is no dialog frame,
// so the title is plain text instead of the gradient title bar.
func authTitle(t *styles.Styles, width int, title string, onboarding bool) string {
	if onboarding {
		return t.Dialog.PrimaryText.Render(title)
	}
	titleStyle := t.Dialog.Title
	offset := titleStyle.GetHorizontalFrameSize() + t.Dialog.View.GetHorizontalFrameSize()
	return common.DialogTitle(t, titleStyle.Render(title), width-offset, t.Dialog.TitleGradFromColor, t.Dialog.TitleGradToColor)
}
