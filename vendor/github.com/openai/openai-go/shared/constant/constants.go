// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package constant

import (
	"encoding/json"
)

type Constant[T any] interface {
	Default() T
}

// ValueOf gives the default value of a constant from its type. It's helpful when
// constructing constants as variants in a one-of. Note that empty structs are
// marshalled by default. Usage: constant.ValueOf[constant.Foo]()
func ValueOf[T Constant[T]]() T {
	var t T
	return t.Default()
}

type Approximate string                             // Always "approximate"
type Assistant string                               // Always "assistant"
type AssistantDeleted string                        // Always "assistant.deleted"
type Auto string                                    // Always "auto"
type Batch string                                   // Always "batch"
type ChatCompletion string                          // Always "chat.completion"
type ChatCompletionChunk string                     // Always "chat.completion.chunk"
type ChatCompletionDeleted string                   // Always "chat.completion.deleted"
type Click string                                   // Always "click"
type CodeInterpreter string                         // Always "code_interpreter"
type CodeInterpreterCall string                     // Always "code_interpreter_call"
type ComputerCallOutput string                      // Always "computer_call_output"
type ComputerScreenshot string                      // Always "computer_screenshot"
type ComputerUsePreview string                      // Always "computer_use_preview"
type Content string                                 // Always "content"
type Developer string                               // Always "developer"
type DoubleClick string                             // Always "double_click"
type Drag string                                    // Always "drag"
type Embedding string                               // Always "embedding"
type Error string                                   // Always "error"
type File string                                    // Always "file"
type FileCitation string                            // Always "file_citation"
type FilePath string                                // Always "file_path"
type FileSearch string                              // Always "file_search"
type FileSearchCall string                          // Always "file_search_call"
type Files string                                   // Always "files"
type FineTuningJob string                           // Always "fine_tuning.job"
type FineTuningJobCheckpoint string                 // Always "fine_tuning.job.checkpoint"
type FineTuningJobEvent string                      // Always "fine_tuning.job.event"
type Function string                                // Always "function"
type FunctionCall string                            // Always "function_call"
type FunctionCallOutput string                      // Always "function_call_output"
type Image string                                   // Always "image"
type ImageFile string                               // Always "image_file"
type ImageURL string                                // Always "image_url"
type InputAudio string                              // Always "input_audio"
type InputFile string                               // Always "input_file"
type InputImage string                              // Always "input_image"
type InputText string                               // Always "input_text"
type ItemReference string                           // Always "item_reference"
type JSONObject string                              // Always "json_object"
type JSONSchema string                              // Always "json_schema"
type Keypress string                                // Always "keypress"
type LastActiveAt string                            // Always "last_active_at"
type List string                                    // Always "list"
type Logs string                                    // Always "logs"
type Message string                                 // Always "message"
type MessageCreation string                         // Always "message_creation"
type Model string                                   // Always "model"
type Move string                                    // Always "move"
type Other string                                   // Always "other"
type OutputAudio string                             // Always "output_audio"
type OutputText string                              // Always "output_text"
type Reasoning string                               // Always "reasoning"
type Refusal string                                 // Always "refusal"
type Response string                                // Always "response"
type ResponseAudioDelta string                      // Always "response.audio.delta"
type ResponseAudioDone string                       // Always "response.audio.done"
type ResponseAudioTranscriptDelta string            // Always "response.audio.transcript.delta"
type ResponseAudioTranscriptDone string             // Always "response.audio.transcript.done"
type ResponseCodeInterpreterCallCodeDelta string    // Always "response.code_interpreter_call.code.delta"
type ResponseCodeInterpreterCallCodeDone string     // Always "response.code_interpreter_call.code.done"
type ResponseCodeInterpreterCallCompleted string    // Always "response.code_interpreter_call.completed"
type ResponseCodeInterpreterCallInProgress string   // Always "response.code_interpreter_call.in_progress"
type ResponseCodeInterpreterCallInterpreting string // Always "response.code_interpreter_call.interpreting"
type ResponseCompleted string                       // Always "response.completed"
type ResponseContentPartAdded string                // Always "response.content_part.added"
type ResponseContentPartDone string                 // Always "response.content_part.done"
type ResponseCreated string                         // Always "response.created"
type ResponseFailed string                          // Always "response.failed"
type ResponseFileSearchCallCompleted string         // Always "response.file_search_call.completed"
type ResponseFileSearchCallInProgress string        // Always "response.file_search_call.in_progress"
type ResponseFileSearchCallSearching string         // Always "response.file_search_call.searching"
type ResponseFunctionCallArgumentsDelta string      // Always "response.function_call_arguments.delta"
type ResponseFunctionCallArgumentsDone string       // Always "response.function_call_arguments.done"
type ResponseInProgress string                      // Always "response.in_progress"
type ResponseIncomplete string                      // Always "response.incomplete"
type ResponseOutputItemAdded string                 // Always "response.output_item.added"
type ResponseOutputItemDone string                  // Always "response.output_item.done"
type ResponseOutputTextAnnotationAdded string       // Always "response.output_text.annotation.added"
type ResponseOutputTextDelta string                 // Always "response.output_text.delta"
type ResponseOutputTextDone string                  // Always "response.output_text.done"
type ResponseRefusalDelta string                    // Always "response.refusal.delta"
type ResponseRefusalDone string                     // Always "response.refusal.done"
type ResponseWebSearchCallCompleted string          // Always "response.web_search_call.completed"
type ResponseWebSearchCallInProgress string         // Always "response.web_search_call.in_progress"
type ResponseWebSearchCallSearching string          // Always "response.web_search_call.searching"
type Screenshot string                              // Always "screenshot"
type Scroll string                                  // Always "scroll"
type Static string                                  // Always "static"
type SubmitToolOutputs string                       // Always "submit_tool_outputs"
type SummaryText string                             // Always "summary_text"
type System string                                  // Always "system"
type Text string                                    // Always "text"
type TextCompletion string                          // Always "text_completion"
type Thread string                                  // Always "thread"
type ThreadCreated string                           // Always "thread.created"
type ThreadDeleted string                           // Always "thread.deleted"
type ThreadMessage string                           // Always "thread.message"
type ThreadMessageCompleted string                  // Always "thread.message.completed"
type ThreadMessageCreated string                    // Always "thread.message.created"
type ThreadMessageDeleted string                    // Always "thread.message.deleted"
type ThreadMessageDelta string                      // Always "thread.message.delta"
type ThreadMessageInProgress string                 // Always "thread.message.in_progress"
type ThreadMessageIncomplete string                 // Always "thread.message.incomplete"
type ThreadRun string                               // Always "thread.run"
type ThreadRunCancelled string                      // Always "thread.run.cancelled"
type ThreadRunCancelling string                     // Always "thread.run.cancelling"
type ThreadRunCompleted string                      // Always "thread.run.completed"
type ThreadRunCreated string                        // Always "thread.run.created"
type ThreadRunExpired string                        // Always "thread.run.expired"
type ThreadRunFailed string                         // Always "thread.run.failed"
type ThreadRunInProgress string                     // Always "thread.run.in_progress"
type ThreadRunIncomplete string                     // Always "thread.run.incomplete"
type ThreadRunQueued string                         // Always "thread.run.queued"
type ThreadRunRequiresAction string                 // Always "thread.run.requires_action"
type ThreadRunStep string                           // Always "thread.run.step"
type ThreadRunStepCancelled string                  // Always "thread.run.step.cancelled"
type ThreadRunStepCompleted string                  // Always "thread.run.step.completed"
type ThreadRunStepCreated string                    // Always "thread.run.step.created"
type ThreadRunStepDelta string                      // Always "thread.run.step.delta"
type ThreadRunStepExpired string                    // Always "thread.run.step.expired"
type ThreadRunStepFailed string                     // Always "thread.run.step.failed"
type ThreadRunStepInProgress string                 // Always "thread.run.step.in_progress"
type Tool string                                    // Always "tool"
type ToolCalls string                               // Always "tool_calls"
type TranscriptTextDelta string                     // Always "transcript.text.delta"
type TranscriptTextDone string                      // Always "transcript.text.done"
type Type string                                    // Always "type"
type Upload string                                  // Always "upload"
type UploadPart string                              // Always "upload.part"
type URLCitation string                             // Always "url_citation"
type User string                                    // Always "user"
type VectorStore string                             // Always "vector_store"
type VectorStoreDeleted string                      // Always "vector_store.deleted"
type VectorStoreFile string                         // Always "vector_store.file"
type VectorStoreFileContentPage string              // Always "vector_store.file_content.page"
type VectorStoreFileDeleted string                  // Always "vector_store.file.deleted"
type VectorStoreFilesBatch string                   // Always "vector_store.files_batch"
type VectorStoreSearchResultsPage string            // Always "vector_store.search_results.page"
type Wait string                                    // Always "wait"
type Wandb string                                   // Always "wandb"
type WebSearchCall string                           // Always "web_search_call"

func (c Approximate) Default() Approximate                     { return "approximate" }
func (c Assistant) Default() Assistant                         { return "assistant" }
func (c AssistantDeleted) Default() AssistantDeleted           { return "assistant.deleted" }
func (c Auto) Default() Auto                                   { return "auto" }
func (c Batch) Default() Batch                                 { return "batch" }
func (c ChatCompletion) Default() ChatCompletion               { return "chat.completion" }
func (c ChatCompletionChunk) Default() ChatCompletionChunk     { return "chat.completion.chunk" }
func (c ChatCompletionDeleted) Default() ChatCompletionDeleted { return "chat.completion.deleted" }
func (c Click) Default() Click                                 { return "click" }
func (c CodeInterpreter) Default() CodeInterpreter             { return "code_interpreter" }
func (c CodeInterpreterCall) Default() CodeInterpreterCall     { return "code_interpreter_call" }
func (c ComputerCallOutput) Default() ComputerCallOutput       { return "computer_call_output" }
func (c ComputerScreenshot) Default() ComputerScreenshot       { return "computer_screenshot" }
func (c ComputerUsePreview) Default() ComputerUsePreview       { return "computer_use_preview" }
func (c Content) Default() Content                             { return "content" }
func (c Developer) Default() Developer                         { return "developer" }
func (c DoubleClick) Default() DoubleClick                     { return "double_click" }
func (c Drag) Default() Drag                                   { return "drag" }
func (c Embedding) Default() Embedding                         { return "embedding" }
func (c Error) Default() Error                                 { return "error" }
func (c File) Default() File                                   { return "file" }
func (c FileCitation) Default() FileCitation                   { return "file_citation" }
func (c FilePath) Default() FilePath                           { return "file_path" }
func (c FileSearch) Default() FileSearch                       { return "file_search" }
func (c FileSearchCall) Default() FileSearchCall               { return "file_search_call" }
func (c Files) Default() Files                                 { return "files" }
func (c FineTuningJob) Default() FineTuningJob                 { return "fine_tuning.job" }
func (c FineTuningJobCheckpoint) Default() FineTuningJobCheckpoint {
	return "fine_tuning.job.checkpoint"
}
func (c FineTuningJobEvent) Default() FineTuningJobEvent { return "fine_tuning.job.event" }
func (c Function) Default() Function                     { return "function" }
func (c FunctionCall) Default() FunctionCall             { return "function_call" }
func (c FunctionCallOutput) Default() FunctionCallOutput { return "function_call_output" }
func (c Image) Default() Image                           { return "image" }
func (c ImageFile) Default() ImageFile                   { return "image_file" }
func (c ImageURL) Default() ImageURL                     { return "image_url" }
func (c InputAudio) Default() InputAudio                 { return "input_audio" }
func (c InputFile) Default() InputFile                   { return "input_file" }
func (c InputImage) Default() InputImage                 { return "input_image" }
func (c InputText) Default() InputText                   { return "input_text" }
func (c ItemReference) Default() ItemReference           { return "item_reference" }
func (c JSONObject) Default() JSONObject                 { return "json_object" }
func (c JSONSchema) Default() JSONSchema                 { return "json_schema" }
func (c Keypress) Default() Keypress                     { return "keypress" }
func (c LastActiveAt) Default() LastActiveAt             { return "last_active_at" }
func (c List) Default() List                             { return "list" }
func (c Logs) Default() Logs                             { return "logs" }
func (c Message) Default() Message                       { return "message" }
func (c MessageCreation) Default() MessageCreation       { return "message_creation" }
func (c Model) Default() Model                           { return "model" }
func (c Move) Default() Move                             { return "move" }
func (c Other) Default() Other                           { return "other" }
func (c OutputAudio) Default() OutputAudio               { return "output_audio" }
func (c OutputText) Default() OutputText                 { return "output_text" }
func (c Reasoning) Default() Reasoning                   { return "reasoning" }
func (c Refusal) Default() Refusal                       { return "refusal" }
func (c Response) Default() Response                     { return "response" }
func (c ResponseAudioDelta) Default() ResponseAudioDelta { return "response.audio.delta" }
func (c ResponseAudioDone) Default() ResponseAudioDone   { return "response.audio.done" }
func (c ResponseAudioTranscriptDelta) Default() ResponseAudioTranscriptDelta {
	return "response.audio.transcript.delta"
}
func (c ResponseAudioTranscriptDone) Default() ResponseAudioTranscriptDone {
	return "response.audio.transcript.done"
}
func (c ResponseCodeInterpreterCallCodeDelta) Default() ResponseCodeInterpreterCallCodeDelta {
	return "response.code_interpreter_call.code.delta"
}
func (c ResponseCodeInterpreterCallCodeDone) Default() ResponseCodeInterpreterCallCodeDone {
	return "response.code_interpreter_call.code.done"
}
func (c ResponseCodeInterpreterCallCompleted) Default() ResponseCodeInterpreterCallCompleted {
	return "response.code_interpreter_call.completed"
}
func (c ResponseCodeInterpreterCallInProgress) Default() ResponseCodeInterpreterCallInProgress {
	return "response.code_interpreter_call.in_progress"
}
func (c ResponseCodeInterpreterCallInterpreting) Default() ResponseCodeInterpreterCallInterpreting {
	return "response.code_interpreter_call.interpreting"
}
func (c ResponseCompleted) Default() ResponseCompleted { return "response.completed" }
func (c ResponseContentPartAdded) Default() ResponseContentPartAdded {
	return "response.content_part.added"
}
func (c ResponseContentPartDone) Default() ResponseContentPartDone {
	return "response.content_part.done"
}
func (c ResponseCreated) Default() ResponseCreated { return "response.created" }
func (c ResponseFailed) Default() ResponseFailed   { return "response.failed" }
func (c ResponseFileSearchCallCompleted) Default() ResponseFileSearchCallCompleted {
	return "response.file_search_call.completed"
}
func (c ResponseFileSearchCallInProgress) Default() ResponseFileSearchCallInProgress {
	return "response.file_search_call.in_progress"
}
func (c ResponseFileSearchCallSearching) Default() ResponseFileSearchCallSearching {
	return "response.file_search_call.searching"
}
func (c ResponseFunctionCallArgumentsDelta) Default() ResponseFunctionCallArgumentsDelta {
	return "response.function_call_arguments.delta"
}
func (c ResponseFunctionCallArgumentsDone) Default() ResponseFunctionCallArgumentsDone {
	return "response.function_call_arguments.done"
}
func (c ResponseInProgress) Default() ResponseInProgress { return "response.in_progress" }
func (c ResponseIncomplete) Default() ResponseIncomplete { return "response.incomplete" }
func (c ResponseOutputItemAdded) Default() ResponseOutputItemAdded {
	return "response.output_item.added"
}
func (c ResponseOutputItemDone) Default() ResponseOutputItemDone { return "response.output_item.done" }
func (c ResponseOutputTextAnnotationAdded) Default() ResponseOutputTextAnnotationAdded {
	return "response.output_text.annotation.added"
}
func (c ResponseOutputTextDelta) Default() ResponseOutputTextDelta {
	return "response.output_text.delta"
}
func (c ResponseOutputTextDone) Default() ResponseOutputTextDone { return "response.output_text.done" }
func (c ResponseRefusalDelta) Default() ResponseRefusalDelta     { return "response.refusal.delta" }
func (c ResponseRefusalDone) Default() ResponseRefusalDone       { return "response.refusal.done" }
func (c ResponseWebSearchCallCompleted) Default() ResponseWebSearchCallCompleted {
	return "response.web_search_call.completed"
}
func (c ResponseWebSearchCallInProgress) Default() ResponseWebSearchCallInProgress {
	return "response.web_search_call.in_progress"
}
func (c ResponseWebSearchCallSearching) Default() ResponseWebSearchCallSearching {
	return "response.web_search_call.searching"
}
func (c Screenshot) Default() Screenshot                         { return "screenshot" }
func (c Scroll) Default() Scroll                                 { return "scroll" }
func (c Static) Default() Static                                 { return "static" }
func (c SubmitToolOutputs) Default() SubmitToolOutputs           { return "submit_tool_outputs" }
func (c SummaryText) Default() SummaryText                       { return "summary_text" }
func (c System) Default() System                                 { return "system" }
func (c Text) Default() Text                                     { return "text" }
func (c TextCompletion) Default() TextCompletion                 { return "text_completion" }
func (c Thread) Default() Thread                                 { return "thread" }
func (c ThreadCreated) Default() ThreadCreated                   { return "thread.created" }
func (c ThreadDeleted) Default() ThreadDeleted                   { return "thread.deleted" }
func (c ThreadMessage) Default() ThreadMessage                   { return "thread.message" }
func (c ThreadMessageCompleted) Default() ThreadMessageCompleted { return "thread.message.completed" }
func (c ThreadMessageCreated) Default() ThreadMessageCreated     { return "thread.message.created" }
func (c ThreadMessageDeleted) Default() ThreadMessageDeleted     { return "thread.message.deleted" }
func (c ThreadMessageDelta) Default() ThreadMessageDelta         { return "thread.message.delta" }
func (c ThreadMessageInProgress) Default() ThreadMessageInProgress {
	return "thread.message.in_progress"
}
func (c ThreadMessageIncomplete) Default() ThreadMessageIncomplete {
	return "thread.message.incomplete"
}
func (c ThreadRun) Default() ThreadRun                     { return "thread.run" }
func (c ThreadRunCancelled) Default() ThreadRunCancelled   { return "thread.run.cancelled" }
func (c ThreadRunCancelling) Default() ThreadRunCancelling { return "thread.run.cancelling" }
func (c ThreadRunCompleted) Default() ThreadRunCompleted   { return "thread.run.completed" }
func (c ThreadRunCreated) Default() ThreadRunCreated       { return "thread.run.created" }
func (c ThreadRunExpired) Default() ThreadRunExpired       { return "thread.run.expired" }
func (c ThreadRunFailed) Default() ThreadRunFailed         { return "thread.run.failed" }
func (c ThreadRunInProgress) Default() ThreadRunInProgress { return "thread.run.in_progress" }
func (c ThreadRunIncomplete) Default() ThreadRunIncomplete { return "thread.run.incomplete" }
func (c ThreadRunQueued) Default() ThreadRunQueued         { return "thread.run.queued" }
func (c ThreadRunRequiresAction) Default() ThreadRunRequiresAction {
	return "thread.run.requires_action"
}
func (c ThreadRunStep) Default() ThreadRunStep                   { return "thread.run.step" }
func (c ThreadRunStepCancelled) Default() ThreadRunStepCancelled { return "thread.run.step.cancelled" }
func (c ThreadRunStepCompleted) Default() ThreadRunStepCompleted { return "thread.run.step.completed" }
func (c ThreadRunStepCreated) Default() ThreadRunStepCreated     { return "thread.run.step.created" }
func (c ThreadRunStepDelta) Default() ThreadRunStepDelta         { return "thread.run.step.delta" }
func (c ThreadRunStepExpired) Default() ThreadRunStepExpired     { return "thread.run.step.expired" }
func (c ThreadRunStepFailed) Default() ThreadRunStepFailed       { return "thread.run.step.failed" }
func (c ThreadRunStepInProgress) Default() ThreadRunStepInProgress {
	return "thread.run.step.in_progress"
}
func (c Tool) Default() Tool                               { return "tool" }
func (c ToolCalls) Default() ToolCalls                     { return "tool_calls" }
func (c TranscriptTextDelta) Default() TranscriptTextDelta { return "transcript.text.delta" }
func (c TranscriptTextDone) Default() TranscriptTextDone   { return "transcript.text.done" }
func (c Type) Default() Type                               { return "type" }
func (c Upload) Default() Upload                           { return "upload" }
func (c UploadPart) Default() UploadPart                   { return "upload.part" }
func (c URLCitation) Default() URLCitation                 { return "url_citation" }
func (c User) Default() User                               { return "user" }
func (c VectorStore) Default() VectorStore                 { return "vector_store" }
func (c VectorStoreDeleted) Default() VectorStoreDeleted   { return "vector_store.deleted" }
func (c VectorStoreFile) Default() VectorStoreFile         { return "vector_store.file" }
func (c VectorStoreFileContentPage) Default() VectorStoreFileContentPage {
	return "vector_store.file_content.page"
}
func (c VectorStoreFileDeleted) Default() VectorStoreFileDeleted { return "vector_store.file.deleted" }
func (c VectorStoreFilesBatch) Default() VectorStoreFilesBatch   { return "vector_store.files_batch" }
func (c VectorStoreSearchResultsPage) Default() VectorStoreSearchResultsPage {
	return "vector_store.search_results.page"
}
func (c Wait) Default() Wait                   { return "wait" }
func (c Wandb) Default() Wandb                 { return "wandb" }
func (c WebSearchCall) Default() WebSearchCall { return "web_search_call" }

func (c Approximate) MarshalJSON() ([]byte, error)                           { return marshalString(c) }
func (c Assistant) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c AssistantDeleted) MarshalJSON() ([]byte, error)                      { return marshalString(c) }
func (c Auto) MarshalJSON() ([]byte, error)                                  { return marshalString(c) }
func (c Batch) MarshalJSON() ([]byte, error)                                 { return marshalString(c) }
func (c ChatCompletion) MarshalJSON() ([]byte, error)                        { return marshalString(c) }
func (c ChatCompletionChunk) MarshalJSON() ([]byte, error)                   { return marshalString(c) }
func (c ChatCompletionDeleted) MarshalJSON() ([]byte, error)                 { return marshalString(c) }
func (c Click) MarshalJSON() ([]byte, error)                                 { return marshalString(c) }
func (c CodeInterpreter) MarshalJSON() ([]byte, error)                       { return marshalString(c) }
func (c CodeInterpreterCall) MarshalJSON() ([]byte, error)                   { return marshalString(c) }
func (c ComputerCallOutput) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c ComputerScreenshot) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c ComputerUsePreview) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c Content) MarshalJSON() ([]byte, error)                               { return marshalString(c) }
func (c Developer) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c DoubleClick) MarshalJSON() ([]byte, error)                           { return marshalString(c) }
func (c Drag) MarshalJSON() ([]byte, error)                                  { return marshalString(c) }
func (c Embedding) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c Error) MarshalJSON() ([]byte, error)                                 { return marshalString(c) }
func (c File) MarshalJSON() ([]byte, error)                                  { return marshalString(c) }
func (c FileCitation) MarshalJSON() ([]byte, error)                          { return marshalString(c) }
func (c FilePath) MarshalJSON() ([]byte, error)                              { return marshalString(c) }
func (c FileSearch) MarshalJSON() ([]byte, error)                            { return marshalString(c) }
func (c FileSearchCall) MarshalJSON() ([]byte, error)                        { return marshalString(c) }
func (c Files) MarshalJSON() ([]byte, error)                                 { return marshalString(c) }
func (c FineTuningJob) MarshalJSON() ([]byte, error)                         { return marshalString(c) }
func (c FineTuningJobCheckpoint) MarshalJSON() ([]byte, error)               { return marshalString(c) }
func (c FineTuningJobEvent) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c Function) MarshalJSON() ([]byte, error)                              { return marshalString(c) }
func (c FunctionCall) MarshalJSON() ([]byte, error)                          { return marshalString(c) }
func (c FunctionCallOutput) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c Image) MarshalJSON() ([]byte, error)                                 { return marshalString(c) }
func (c ImageFile) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c ImageURL) MarshalJSON() ([]byte, error)                              { return marshalString(c) }
func (c InputAudio) MarshalJSON() ([]byte, error)                            { return marshalString(c) }
func (c InputFile) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c InputImage) MarshalJSON() ([]byte, error)                            { return marshalString(c) }
func (c InputText) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c ItemReference) MarshalJSON() ([]byte, error)                         { return marshalString(c) }
func (c JSONObject) MarshalJSON() ([]byte, error)                            { return marshalString(c) }
func (c JSONSchema) MarshalJSON() ([]byte, error)                            { return marshalString(c) }
func (c Keypress) MarshalJSON() ([]byte, error)                              { return marshalString(c) }
func (c LastActiveAt) MarshalJSON() ([]byte, error)                          { return marshalString(c) }
func (c List) MarshalJSON() ([]byte, error)                                  { return marshalString(c) }
func (c Logs) MarshalJSON() ([]byte, error)                                  { return marshalString(c) }
func (c Message) MarshalJSON() ([]byte, error)                               { return marshalString(c) }
func (c MessageCreation) MarshalJSON() ([]byte, error)                       { return marshalString(c) }
func (c Model) MarshalJSON() ([]byte, error)                                 { return marshalString(c) }
func (c Move) MarshalJSON() ([]byte, error)                                  { return marshalString(c) }
func (c Other) MarshalJSON() ([]byte, error)                                 { return marshalString(c) }
func (c OutputAudio) MarshalJSON() ([]byte, error)                           { return marshalString(c) }
func (c OutputText) MarshalJSON() ([]byte, error)                            { return marshalString(c) }
func (c Reasoning) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c Refusal) MarshalJSON() ([]byte, error)                               { return marshalString(c) }
func (c Response) MarshalJSON() ([]byte, error)                              { return marshalString(c) }
func (c ResponseAudioDelta) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c ResponseAudioDone) MarshalJSON() ([]byte, error)                     { return marshalString(c) }
func (c ResponseAudioTranscriptDelta) MarshalJSON() ([]byte, error)          { return marshalString(c) }
func (c ResponseAudioTranscriptDone) MarshalJSON() ([]byte, error)           { return marshalString(c) }
func (c ResponseCodeInterpreterCallCodeDelta) MarshalJSON() ([]byte, error)  { return marshalString(c) }
func (c ResponseCodeInterpreterCallCodeDone) MarshalJSON() ([]byte, error)   { return marshalString(c) }
func (c ResponseCodeInterpreterCallCompleted) MarshalJSON() ([]byte, error)  { return marshalString(c) }
func (c ResponseCodeInterpreterCallInProgress) MarshalJSON() ([]byte, error) { return marshalString(c) }
func (c ResponseCodeInterpreterCallInterpreting) MarshalJSON() ([]byte, error) {
	return marshalString(c)
}
func (c ResponseCompleted) MarshalJSON() ([]byte, error)                  { return marshalString(c) }
func (c ResponseContentPartAdded) MarshalJSON() ([]byte, error)           { return marshalString(c) }
func (c ResponseContentPartDone) MarshalJSON() ([]byte, error)            { return marshalString(c) }
func (c ResponseCreated) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c ResponseFailed) MarshalJSON() ([]byte, error)                     { return marshalString(c) }
func (c ResponseFileSearchCallCompleted) MarshalJSON() ([]byte, error)    { return marshalString(c) }
func (c ResponseFileSearchCallInProgress) MarshalJSON() ([]byte, error)   { return marshalString(c) }
func (c ResponseFileSearchCallSearching) MarshalJSON() ([]byte, error)    { return marshalString(c) }
func (c ResponseFunctionCallArgumentsDelta) MarshalJSON() ([]byte, error) { return marshalString(c) }
func (c ResponseFunctionCallArgumentsDone) MarshalJSON() ([]byte, error)  { return marshalString(c) }
func (c ResponseInProgress) MarshalJSON() ([]byte, error)                 { return marshalString(c) }
func (c ResponseIncomplete) MarshalJSON() ([]byte, error)                 { return marshalString(c) }
func (c ResponseOutputItemAdded) MarshalJSON() ([]byte, error)            { return marshalString(c) }
func (c ResponseOutputItemDone) MarshalJSON() ([]byte, error)             { return marshalString(c) }
func (c ResponseOutputTextAnnotationAdded) MarshalJSON() ([]byte, error)  { return marshalString(c) }
func (c ResponseOutputTextDelta) MarshalJSON() ([]byte, error)            { return marshalString(c) }
func (c ResponseOutputTextDone) MarshalJSON() ([]byte, error)             { return marshalString(c) }
func (c ResponseRefusalDelta) MarshalJSON() ([]byte, error)               { return marshalString(c) }
func (c ResponseRefusalDone) MarshalJSON() ([]byte, error)                { return marshalString(c) }
func (c ResponseWebSearchCallCompleted) MarshalJSON() ([]byte, error)     { return marshalString(c) }
func (c ResponseWebSearchCallInProgress) MarshalJSON() ([]byte, error)    { return marshalString(c) }
func (c ResponseWebSearchCallSearching) MarshalJSON() ([]byte, error)     { return marshalString(c) }
func (c Screenshot) MarshalJSON() ([]byte, error)                         { return marshalString(c) }
func (c Scroll) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c Static) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c SubmitToolOutputs) MarshalJSON() ([]byte, error)                  { return marshalString(c) }
func (c SummaryText) MarshalJSON() ([]byte, error)                        { return marshalString(c) }
func (c System) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c Text) MarshalJSON() ([]byte, error)                               { return marshalString(c) }
func (c TextCompletion) MarshalJSON() ([]byte, error)                     { return marshalString(c) }
func (c Thread) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c ThreadCreated) MarshalJSON() ([]byte, error)                      { return marshalString(c) }
func (c ThreadDeleted) MarshalJSON() ([]byte, error)                      { return marshalString(c) }
func (c ThreadMessage) MarshalJSON() ([]byte, error)                      { return marshalString(c) }
func (c ThreadMessageCompleted) MarshalJSON() ([]byte, error)             { return marshalString(c) }
func (c ThreadMessageCreated) MarshalJSON() ([]byte, error)               { return marshalString(c) }
func (c ThreadMessageDeleted) MarshalJSON() ([]byte, error)               { return marshalString(c) }
func (c ThreadMessageDelta) MarshalJSON() ([]byte, error)                 { return marshalString(c) }
func (c ThreadMessageInProgress) MarshalJSON() ([]byte, error)            { return marshalString(c) }
func (c ThreadMessageIncomplete) MarshalJSON() ([]byte, error)            { return marshalString(c) }
func (c ThreadRun) MarshalJSON() ([]byte, error)                          { return marshalString(c) }
func (c ThreadRunCancelled) MarshalJSON() ([]byte, error)                 { return marshalString(c) }
func (c ThreadRunCancelling) MarshalJSON() ([]byte, error)                { return marshalString(c) }
func (c ThreadRunCompleted) MarshalJSON() ([]byte, error)                 { return marshalString(c) }
func (c ThreadRunCreated) MarshalJSON() ([]byte, error)                   { return marshalString(c) }
func (c ThreadRunExpired) MarshalJSON() ([]byte, error)                   { return marshalString(c) }
func (c ThreadRunFailed) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c ThreadRunInProgress) MarshalJSON() ([]byte, error)                { return marshalString(c) }
func (c ThreadRunIncomplete) MarshalJSON() ([]byte, error)                { return marshalString(c) }
func (c ThreadRunQueued) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c ThreadRunRequiresAction) MarshalJSON() ([]byte, error)            { return marshalString(c) }
func (c ThreadRunStep) MarshalJSON() ([]byte, error)                      { return marshalString(c) }
func (c ThreadRunStepCancelled) MarshalJSON() ([]byte, error)             { return marshalString(c) }
func (c ThreadRunStepCompleted) MarshalJSON() ([]byte, error)             { return marshalString(c) }
func (c ThreadRunStepCreated) MarshalJSON() ([]byte, error)               { return marshalString(c) }
func (c ThreadRunStepDelta) MarshalJSON() ([]byte, error)                 { return marshalString(c) }
func (c ThreadRunStepExpired) MarshalJSON() ([]byte, error)               { return marshalString(c) }
func (c ThreadRunStepFailed) MarshalJSON() ([]byte, error)                { return marshalString(c) }
func (c ThreadRunStepInProgress) MarshalJSON() ([]byte, error)            { return marshalString(c) }
func (c Tool) MarshalJSON() ([]byte, error)                               { return marshalString(c) }
func (c ToolCalls) MarshalJSON() ([]byte, error)                          { return marshalString(c) }
func (c TranscriptTextDelta) MarshalJSON() ([]byte, error)                { return marshalString(c) }
func (c TranscriptTextDone) MarshalJSON() ([]byte, error)                 { return marshalString(c) }
func (c Type) MarshalJSON() ([]byte, error)                               { return marshalString(c) }
func (c Upload) MarshalJSON() ([]byte, error)                             { return marshalString(c) }
func (c UploadPart) MarshalJSON() ([]byte, error)                         { return marshalString(c) }
func (c URLCitation) MarshalJSON() ([]byte, error)                        { return marshalString(c) }
func (c User) MarshalJSON() ([]byte, error)                               { return marshalString(c) }
func (c VectorStore) MarshalJSON() ([]byte, error)                        { return marshalString(c) }
func (c VectorStoreDeleted) MarshalJSON() ([]byte, error)                 { return marshalString(c) }
func (c VectorStoreFile) MarshalJSON() ([]byte, error)                    { return marshalString(c) }
func (c VectorStoreFileContentPage) MarshalJSON() ([]byte, error)         { return marshalString(c) }
func (c VectorStoreFileDeleted) MarshalJSON() ([]byte, error)             { return marshalString(c) }
func (c VectorStoreFilesBatch) MarshalJSON() ([]byte, error)              { return marshalString(c) }
func (c VectorStoreSearchResultsPage) MarshalJSON() ([]byte, error)       { return marshalString(c) }
func (c Wait) MarshalJSON() ([]byte, error)                               { return marshalString(c) }
func (c Wandb) MarshalJSON() ([]byte, error)                              { return marshalString(c) }
func (c WebSearchCall) MarshalJSON() ([]byte, error)                      { return marshalString(c) }

type constant[T any] interface {
	Constant[T]
	*T
}

func marshalString[T ~string, PT constant[T]](v T) ([]byte, error) {
	var zero T
	if v == zero {
		v = PT(&v).Default()
	}
	return json.Marshal(string(v))
}
