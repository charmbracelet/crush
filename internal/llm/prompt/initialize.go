package prompt

import _ "embed"

//go:embed init.md
var initPrompt []byte

func Initialize() string {
	return string(initPrompt)
}
