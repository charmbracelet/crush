package prompt

import (
	"github.com/charmbracelet/crush/internal/fur/provider"
)

func TitlePrompt(_ provider.InferenceProvider) string {
	return `you will generate a short title based on the first message a user begins a conversation with
- ensure it is not more than 50 characters long
- the title should be a summary of the user's message
- it should be one line long
- do not use quotes or colons
- the entire text you return will be used as the title
- never return anything that is more than one sentence (one line) long`
}
