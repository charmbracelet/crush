package editor

type historyState struct {
	previouslyScrollingPromptHistory bool
	promptHistoryIndex               int
	existingValue                    string
	historyCache                     []string
}

type History interface {
	ExistingValue() string
	Value() string
	ScrollUp()
	ScrollDown()
}

func InitialiseHistory(existingValue string, messages []string) History {
	return &historyState{
		existingValue:      existingValue,
		historyCache:       messages,
		promptHistoryIndex: len(messages) - 1,
	}
}

func (h *historyState) Value() string {
	if len(h.historyCache) == 0 {
		return h.existingValue
	}
	return h.historyCache[h.promptHistoryIndex]
}

func (h *historyState) ExistingValue() string {
	return h.existingValue
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
