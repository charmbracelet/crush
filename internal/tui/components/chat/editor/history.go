package editor

type historyState struct {
	previouslyScrollingPromptHistory bool
	promptHistoryIndex               int
	historyCache                     []string
}

type History interface {
	ScrollUp()
	ScrollDown()
}

func InitialiseHistory(messages []string) History {
	return &historyState{}
}

func (h *historyState) ScrollUp() {}

func (h *historyState) ScrollDown() {}
