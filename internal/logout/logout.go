// Package logout implements the CLI logout prompts: small Bubble Tea
// programs in interactive terminals and plain text prompts in
// non-interactive sessions.
package logout

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"
)

// interactive reports whether the prompts should run their TUIs: both stdin
// and stdout must be terminals, otherwise the plain text fallbacks are used.
func interactive() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}

// Confirm asks a yes/no question and reports the answer. In interactive
// terminals it shows selectable Yes/No buttons (No is the default);
// otherwise it falls back to a plain (y/N) prompt.
func Confirm(question string) (bool, error) {
	if interactive() {
		return runConfirm(question)
	}

	fmt.Printf("%s (y/N) ", question)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false, nil
	}
	switch response {
	case "y", "Y", "yes", "Yes", "YES":
		return true, nil
	default:
		return false, nil
	}
}

// Choose asks a multiple choice question and returns the selected option
// index, or -1 when canceled. In interactive terminals the options are
// navigable with a ">" cursor; otherwise a numbered list is printed.
func Choose(question string, options []string) (int, error) {
	if interactive() {
		return runChoose(question, options)
	}

	fmt.Println(question)
	for i, option := range options {
		fmt.Printf("  %d. %s\n", i+1, option)
	}
	fmt.Printf("Select (1-%d): ", len(options))

	var choice int
	_, err := fmt.Scanln(&choice)
	if err != nil || choice < 1 || choice > len(options) {
		return -1, nil
	}
	return choice - 1, nil
}
