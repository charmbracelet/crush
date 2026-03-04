package muse

// TickMsg is sent periodically to check if Muse should trigger.
type TickMsg struct{}

// TriggerStartMsg is sent when Muse trigger begins (for state updates in Update).
type TriggerStartMsg struct {
	Prompt string
}

// TriggerCompleteMsg is sent when Muse thinking completes.
type TriggerCompleteMsg struct {
	Prompt string
	Error  error
}

// PromptEditedMsg is sent when the Muse prompt is edited.
type PromptEditedMsg struct {
	Text string
}

// IntervalEditedMsg is sent when Muse interval is edited.
type IntervalEditedMsg struct {
	Value int
}
