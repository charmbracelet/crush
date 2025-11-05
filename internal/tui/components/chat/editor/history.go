package editor

type historyState struct {
	previouslyScrollingPromptHistory bool
	promptHistoryIndex               int
	historyCache                     []string
}

type History interface {
}

func InitialiseHistory(messages []string) History {
	return historyState{}
}
