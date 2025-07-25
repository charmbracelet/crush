package prompt

import _ "embed"

//go:embed summarize.md
var summarizePrompt []byte

func SummarizerPrompt() string {
	return string(summarizePrompt)
}
