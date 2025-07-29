package prompt

import _ "embed"

//go:embed title.md
var titlePrompt []byte

func TitlePrompt() string {
	return string(titlePrompt)
}
