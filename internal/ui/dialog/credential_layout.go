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
	body    string
	help    string
}

// fit returns the dialog content for an inner height of height rows and
// how many rows sit between the header and the body, for placing the
// input cursor. The header, input and help always keep their rows. Of
// the rest, the key rows go first: on a host key prompt they hold the
// fingerprint, which is the whole point of asking. The message gets what
// remains. Blank rows come last, in order of how much they help reading:
// the one between the message and the keys, then the one under the
// header, then (for confirmations, which have no input) the one above
// the help.
func (l credentialLayout) fit(height int) (string, int) {
	avail := height - lipgloss.Height(l.header) - lipgloss.Height(l.help)
	if l.body != "" {
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
	if l.body == "" {
		spacer = take()
	}

	lines := []string{l.header}
	lines = append(lines, blankRows(gap)...)
	if message != "" {
		lines = append(lines, message)
	}
	lines = append(lines, blankRows(sep)...)
	if keys != "" {
		lines = append(lines, keys)
	}
	if l.body != "" {
		lines = append(lines, l.body)
	}
	lines = append(lines, blankRows(spacer)...)
	lines = append(lines, l.help)
	above := gap + blockHeight(message) + sep + blockHeight(keys)
	return strings.Join(lines, "\n"), above
}

// blockHeight is the row count of s, counting an empty block as none.
func blockHeight(s string) int {
	if s == "" {
		return 0
	}
	return lipgloss.Height(s)
}

func blankRows(n int) []string {
	return make([]string, n)
}
