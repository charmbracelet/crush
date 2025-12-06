package editor

type historyState struct {
	previouslyScrollingPromptHistory bool
	promptHistoryIndex               int
	historyCache                     []string
}

type History interface {
	ExistingValue() string
	Value() string
	ScrollUp()
	ScrollDown()
}

func InitialiseHistory(existingValue string, messages []string) History {
	msgs := messages
	msgs = append(msgs, existingValue)
	return &historyState{
		historyCache:       msgs,
		promptHistoryIndex: len(msgs) - 1,
	}
}

func (h *historyState) Value() string {
	if len(h.historyCache) > 1 {
		return h.historyCache[h.promptHistoryIndex]
	}
	return h.ExistingValue()
}

func (h *historyState) ExistingValue() string {
	index := len(h.historyCache) - 1
	if index < 0 {
		return ""
	}
	return h.historyCache[index]
}

func (h *historyState) ScrollUp() {
	h.promptHistoryIndex--
	h.clampIndex()
}

func (h *historyState) ScrollDown() {
	h.promptHistoryIndex++
	h.clampIndex()
}

func (h *historyState) clampIndex() {
	if h.promptHistoryIndex > len(h.historyCache)-1 {
		h.promptHistoryIndex = len(h.historyCache) - 1
		return
	}

	if h.promptHistoryIndex < 0 {
		h.promptHistoryIndex = 0
	}
}
