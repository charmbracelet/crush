package openai

import "github.com/openai/openai-go/shared/constant"

// Helper to accumulate chunks from a stream
type ChatCompletionAccumulator struct {
	// The up-to-date accumulation of model's responses
	ChatCompletion
	choiceChatCompletionStates []chatCompletionResponseState
	justFinished               chatCompletionResponseState
}

type FinishedChatCompletionToolCall struct {
	ChatCompletionMessageToolCallFunction
	Index int
	Id    string
}

type chatCompletionResponseState struct {
	state chatCompletionResponseStateEnum
	index int
}

type chatCompletionResponseStateEnum int

const (
	emptyResponseState chatCompletionResponseStateEnum = iota
	contentResponseState
	refusalResponseState
	toolResponseState
	finishedResponseState
)

// AddChunk incorporates a chunk into the accumulation. Chunks must be added in order.
// Returns false if the chunk could not be successfully accumulated.
//
// The ChatCompletion field JSON does not get accumulated.
func (acc *ChatCompletionAccumulator) AddChunk(chunk ChatCompletionChunk) bool {
	acc.justFinished = chatCompletionResponseState{}
	if !acc.accumulateDelta(chunk) {
		return false
	}

	// only chunks with choices can cause finished events
	if len(chunk.Choices) == 0 {
		return true
	}

	chunkIndex := int(chunk.Choices[0].Index)
	acc.choiceChatCompletionStates = expandToFit(acc.choiceChatCompletionStates, chunkIndex)
	acc.justFinished = acc.choiceChatCompletionStates[chunkIndex].update(chunk)
	return true
}

// JustFinishedRefusal retrieves the chat completion refusal when it is known to have just been completed.
// The content is "just completed" when the last added chunk no longer contains a content
// delta. If the content is just completed, the content is returned and the boolean is true. Otherwise,
// an empty string is returned and the boolean will be false.
func (acc *ChatCompletionAccumulator) JustFinishedContent() (content string, ok bool) {
	if acc.justFinished.state == contentResponseState {
		return acc.Choices[0].Message.Content, true
	}
	return "", false
}

// JustFinishedRefusal retrieves the chat completion refusal when it is known to have just been completed.
// The refusal is "just completed" when the last added chunk no longer contains a refusal
// delta. If the refusal is just completed, the refusal is returned and the boolean is true. Otherwise,
// an empty string is returned and the boolean will be false.
func (acc *ChatCompletionAccumulator) JustFinishedRefusal() (refusal string, ok bool) {
	if acc.justFinished.state == refusalResponseState {
		return acc.Choices[0].Message.Refusal, true
	}
	return "", false
}

// JustFinishedToolCall retrieves a tool call when it is known to have just been completed.
// A tool call is "just completed" when the last added chunk no longer contains a tool call
// delta or contains a delta for a different tool call. If the tool call is just completed,
// a FinishedChatCompletionToolCall is returned and the boolean is true. Otherwise, an empty
// tool call is returned and the boolean will be false.
//
// You cannot rely on this with a stream that has ParallelToolCalls enabled.
func (acc *ChatCompletionAccumulator) JustFinishedToolCall() (toolcall FinishedChatCompletionToolCall, ok bool) {
	if acc.justFinished.state == toolResponseState {
		f := acc.Choices[0].Message.ToolCalls[acc.justFinished.index].Function
		id := acc.Choices[0].Message.ToolCalls[acc.justFinished.index].ID
		return FinishedChatCompletionToolCall{
			Id:    id,
			Index: acc.justFinished.index,
			ChatCompletionMessageToolCallFunction: ChatCompletionMessageToolCallFunction{
				Name:      f.Name,
				Arguments: f.Arguments,
			},
		}, true
	}
	return FinishedChatCompletionToolCall{}, false
}

// Concatenates a ChatCompletionChunk onto a ChatCompletion. Returns false and
// does nothing if a mismatch is detected.
//
// Ignores the JSON field
func (cc *ChatCompletion) accumulateDelta(chunk ChatCompletionChunk) bool {
	if len(cc.ID) == 0 {
		cc.ID = chunk.ID
	} else if cc.ID != chunk.ID {
		return false
	}

	for _, delta := range chunk.Choices {
		cc.Choices = expandToFit(cc.Choices, int(delta.Index))
		choice := &cc.Choices[delta.Index]

		choice.Index = delta.Index
		choice.FinishReason = delta.FinishReason

		if delta.Delta.Role != "" {
			choice.Message.Role = constant.Assistant(delta.Delta.Role)
		}

		choice.Message.Content += delta.Delta.Content
		choice.Message.Refusal += delta.Delta.Refusal

		for j := range delta.Delta.ToolCalls {
			deltaTool := &delta.Delta.ToolCalls[j]

			choice.Message.ToolCalls = expandToFit(choice.Message.ToolCalls, int(deltaTool.Index))
			tool := &choice.Message.ToolCalls[deltaTool.Index]

			if deltaTool.ID != "" {
				tool.ID = deltaTool.ID
			}
			if deltaTool.Type != "" {
				tool.Type = constant.Function(deltaTool.Type)
			}
			tool.Function.Name += deltaTool.Function.Name
			tool.Function.Arguments += deltaTool.Function.Arguments
		}

		choice.Logprobs.Content = append(choice.Logprobs.Content, delta.Logprobs.Content...)
		choice.Logprobs.Refusal = append(choice.Logprobs.Refusal, delta.Logprobs.Refusal...)
	}

	cc.Usage.CompletionTokens += chunk.Usage.CompletionTokens
	cc.Usage.PromptTokens += chunk.Usage.PromptTokens
	cc.Usage.TotalTokens += chunk.Usage.TotalTokens

	cc.Model = chunk.Model
	cc.Created = chunk.Created
	cc.SystemFingerprint = chunk.SystemFingerprint
	cc.ServiceTier = ChatCompletionServiceTier(chunk.ServiceTier)
	if chunk.Object == chunk.Object.Default() {
		cc.Object = cc.Object.Default()
	}

	return true
}

// Updates the internal response state and returns the previous state if
// the state changed. This ensures that JustFinished events only fire once.
func (prev *chatCompletionResponseState) update(chunk ChatCompletionChunk) (justFinished chatCompletionResponseState) {
	delta := chunk.Choices[0].Delta
	new := chatCompletionResponseState{}
	switch {
	case delta.JSON.Content.IsPresent():
		new.state = contentResponseState
	case delta.JSON.Refusal.IsPresent():
		new.state = refusalResponseState
	case delta.JSON.ToolCalls.IsPresent():
		new.state = toolResponseState
		new.index = int(delta.ToolCalls[0].Index)
	default:
		new.state = finishedResponseState
	}

	if *prev != new {
		justFinished = *prev
	}
	*prev = new

	return
}

func expandToFit[T any](slice []T, index int) []T {
	if index < len(slice) {
		return slice
	}
	if index < cap(slice) {
		return slice[:index+1]
	}
	newSlice := make([]T, index+1)
	copy(newSlice, slice)
	return newSlice
}
