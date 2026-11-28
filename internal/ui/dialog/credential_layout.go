package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// credentialLayout is the shared shape of the SSH and GPG credential
// dialogs: a header, a message saying what is asked, the key rows that
// identify what is being unlocked or trusted, then the input (or a blank
// row for confirmations) and help. Every part is already rendered and
// wrapped to the content width.
type credentialLayout struct {
	header  string
	message string
	keyRows []string
	// body is the input row; confirm prompts have none.
	body string
	help string
	// confirm marks a confirmation, which has no input and gets a blank
	// row above the help instead.
	confirm bool
}

// fit lays the parts out in height rows, shedding blank rows, then message
// lines, then key rows, and returns how many rows sit above the input.
func (l credentialLayout) fit(height int) (string, int) {
	avail := height - lipgloss.Height(l.header) - lipgloss.Height(l.help)
	if !l.confirm {
		avail -= lipgloss.Height(l.body)
	}

	keys := clampDialogLines(strings.Join(l.keyRows, "\n"), avail)
	avail -= blockHeight(keys)
	message := clampDialogLines(l.message, avail)
	avail -= blockHeight(message)
	take := func() int {
		if avail <= 0 {
			return 0
		}
		avail--
		return 1
	}
	sep := 0
	if message != "" && keys != "" {
		sep = take()
	}
	gap := take()
	spacer := 0
	if l.confirm {
		spacer = take()
	}

	lines := []string{l.header}
	lines = append(lines, make([]string, gap)...)
	if message != "" {
		lines = append(lines, message)
	}
	lines = append(lines, make([]string, sep)...)
	if keys != "" {
		lines = append(lines, keys)
	}
	if !l.confirm {
		lines = append(lines, l.body)
	}
	lines = append(lines, make([]string, spacer)...)
	lines = append(lines, l.help)
	above := gap + blockHeight(message) + sep + blockHeight(keys)
	return strings.Join(lines, "\n"), above
}
