// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/apiquery"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/pagination"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/respjson"
	"github.com/openai/openai-go/shared"
	"github.com/openai/openai-go/shared/constant"
)

// BetaThreadRunStepService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewBetaThreadRunStepService] method instead.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
type BetaThreadRunStepService struct {
	Options []option.RequestOption
}

// NewBetaThreadRunStepService generates a new service that applies the given
// options to each request. These options are applied after the parent client's
// options (if there is one), and before any request-specific options.
func NewBetaThreadRunStepService(opts ...option.RequestOption) (r BetaThreadRunStepService) {
	r = BetaThreadRunStepService{}
	r.Options = opts
	return
}

// Retrieves a run step.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadRunStepService) Get(ctx context.Context, threadID string, runID string, stepID string, query BetaThreadRunStepGetParams, opts ...option.RequestOption) (res *RunStep, err error) {
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2")}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	if runID == "" {
		err = errors.New("missing required run_id parameter")
		return
	}
	if stepID == "" {
		err = errors.New("missing required step_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s/runs/%s/steps/%s", threadID, runID, stepID)
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodGet, path, query, &res, opts...)
	return
}

// Returns a list of run steps belonging to a run.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadRunStepService) List(ctx context.Context, threadID string, runID string, query BetaThreadRunStepListParams, opts ...option.RequestOption) (res *pagination.CursorPage[RunStep], err error) {
	var raw *http.Response
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithHeader("OpenAI-Beta", "assistants=v2"), option.WithResponseInto(&raw)}, opts...)
	if threadID == "" {
		err = errors.New("missing required thread_id parameter")
		return
	}
	if runID == "" {
		err = errors.New("missing required run_id parameter")
		return
	}
	path := fmt.Sprintf("threads/%s/runs/%s/steps", threadID, runID)
	cfg, err := requestconfig.NewRequestConfig(ctx, http.MethodGet, path, query, &res, opts...)
	if err != nil {
		return nil, err
	}
	err = cfg.Execute()
	if err != nil {
		return nil, err
	}
	res.SetPageConfig(cfg, raw)
	return res, nil
}

// Returns a list of run steps belonging to a run.
//
// Deprecated: The Assistants API is deprecated in favor of the Responses API
func (r *BetaThreadRunStepService) ListAutoPaging(ctx context.Context, threadID string, runID string, query BetaThreadRunStepListParams, opts ...option.RequestOption) *pagination.CursorPageAutoPager[RunStep] {
	return pagination.NewCursorPageAutoPager(r.List(ctx, threadID, runID, query, opts...))
}

// Text output from the Code Interpreter tool call as part of a run step.
type CodeInterpreterLogs struct {
	// The index of the output in the outputs array.
	Index int64 `json:"index,required"`
	// Always `logs`.
	Type constant.Logs `json:"type,required"`
	// The text output from the Code Interpreter tool call.
	Logs string `json:"logs"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		Type        respjson.Field
		Logs        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterLogs) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterLogs) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CodeInterpreterOutputImage struct {
	// The index of the output in the outputs array.
	Index int64 `json:"index,required"`
	// Always `image`.
	Type  constant.Image                  `json:"type,required"`
	Image CodeInterpreterOutputImageImage `json:"image"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		Type        respjson.Field
		Image       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterOutputImage) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterOutputImage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CodeInterpreterOutputImageImage struct {
	// The [file](https://platform.openai.com/docs/api-reference/files) ID of the
	// image.
	FileID string `json:"file_id"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterOutputImageImage) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterOutputImageImage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Details of the Code Interpreter tool call the run step was involved in.
type CodeInterpreterToolCall struct {
	// The ID of the tool call.
	ID string `json:"id,required"`
	// The Code Interpreter tool call definition.
	CodeInterpreter CodeInterpreterToolCallCodeInterpreter `json:"code_interpreter,required"`
	// The type of tool call. This is always going to be `code_interpreter` for this
	// type of tool call.
	Type constant.CodeInterpreter `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID              respjson.Field
		CodeInterpreter respjson.Field
		Type            respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterToolCall) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The Code Interpreter tool call definition.
type CodeInterpreterToolCallCodeInterpreter struct {
	// The input to the Code Interpreter tool call.
	Input string `json:"input,required"`
	// The outputs from the Code Interpreter tool call. Code Interpreter can output one
	// or more items, including text (`logs`) or images (`image`). Each of these are
	// represented by a different object type.
	Outputs []CodeInterpreterToolCallCodeInterpreterOutputUnion `json:"outputs,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Input       respjson.Field
		Outputs     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterToolCallCodeInterpreter) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterToolCallCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// CodeInterpreterToolCallCodeInterpreterOutputUnion contains all possible
// properties and values from [CodeInterpreterToolCallCodeInterpreterOutputLogs],
// [CodeInterpreterToolCallCodeInterpreterOutputImage].
//
// Use the [CodeInterpreterToolCallCodeInterpreterOutputUnion.AsAny] method to
// switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type CodeInterpreterToolCallCodeInterpreterOutputUnion struct {
	// This field is from variant [CodeInterpreterToolCallCodeInterpreterOutputLogs].
	Logs string `json:"logs"`
	// Any of "logs", "image".
	Type string `json:"type"`
	// This field is from variant [CodeInterpreterToolCallCodeInterpreterOutputImage].
	Image CodeInterpreterToolCallCodeInterpreterOutputImageImage `json:"image"`
	JSON  struct {
		Logs  respjson.Field
		Type  respjson.Field
		Image respjson.Field
		raw   string
	} `json:"-"`
}

// anyCodeInterpreterToolCallCodeInterpreterOutput is implemented by each variant
// of [CodeInterpreterToolCallCodeInterpreterOutputUnion] to add type safety for
// the return type of [CodeInterpreterToolCallCodeInterpreterOutputUnion.AsAny]
type anyCodeInterpreterToolCallCodeInterpreterOutput interface {
	implCodeInterpreterToolCallCodeInterpreterOutputUnion()
}

func (CodeInterpreterToolCallCodeInterpreterOutputLogs) implCodeInterpreterToolCallCodeInterpreterOutputUnion() {
}
func (CodeInterpreterToolCallCodeInterpreterOutputImage) implCodeInterpreterToolCallCodeInterpreterOutputUnion() {
}

// Use the following switch statement to find the correct variant
//
//	switch variant := CodeInterpreterToolCallCodeInterpreterOutputUnion.AsAny().(type) {
//	case openai.CodeInterpreterToolCallCodeInterpreterOutputLogs:
//	case openai.CodeInterpreterToolCallCodeInterpreterOutputImage:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u CodeInterpreterToolCallCodeInterpreterOutputUnion) AsAny() anyCodeInterpreterToolCallCodeInterpreterOutput {
	switch u.Type {
	case "logs":
		return u.AsLogs()
	case "image":
		return u.AsImage()
	}
	return nil
}

func (u CodeInterpreterToolCallCodeInterpreterOutputUnion) AsLogs() (v CodeInterpreterToolCallCodeInterpreterOutputLogs) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u CodeInterpreterToolCallCodeInterpreterOutputUnion) AsImage() (v CodeInterpreterToolCallCodeInterpreterOutputImage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u CodeInterpreterToolCallCodeInterpreterOutputUnion) RawJSON() string { return u.JSON.raw }

func (r *CodeInterpreterToolCallCodeInterpreterOutputUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Text output from the Code Interpreter tool call as part of a run step.
type CodeInterpreterToolCallCodeInterpreterOutputLogs struct {
	// The text output from the Code Interpreter tool call.
	Logs string `json:"logs,required"`
	// Always `logs`.
	Type constant.Logs `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Logs        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterToolCallCodeInterpreterOutputLogs) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterToolCallCodeInterpreterOutputLogs) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CodeInterpreterToolCallCodeInterpreterOutputImage struct {
	Image CodeInterpreterToolCallCodeInterpreterOutputImageImage `json:"image,required"`
	// Always `image`.
	Type constant.Image `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Image       respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterToolCallCodeInterpreterOutputImage) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterToolCallCodeInterpreterOutputImage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type CodeInterpreterToolCallCodeInterpreterOutputImageImage struct {
	// The [file](https://platform.openai.com/docs/api-reference/files) ID of the
	// image.
	FileID string `json:"file_id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterToolCallCodeInterpreterOutputImageImage) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterToolCallCodeInterpreterOutputImageImage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Details of the Code Interpreter tool call the run step was involved in.
type CodeInterpreterToolCallDelta struct {
	// The index of the tool call in the tool calls array.
	Index int64 `json:"index,required"`
	// The type of tool call. This is always going to be `code_interpreter` for this
	// type of tool call.
	Type constant.CodeInterpreter `json:"type,required"`
	// The ID of the tool call.
	ID string `json:"id"`
	// The Code Interpreter tool call definition.
	CodeInterpreter CodeInterpreterToolCallDeltaCodeInterpreter `json:"code_interpreter"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index           respjson.Field
		Type            respjson.Field
		ID              respjson.Field
		CodeInterpreter respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterToolCallDelta) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterToolCallDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The Code Interpreter tool call definition.
type CodeInterpreterToolCallDeltaCodeInterpreter struct {
	// The input to the Code Interpreter tool call.
	Input string `json:"input"`
	// The outputs from the Code Interpreter tool call. Code Interpreter can output one
	// or more items, including text (`logs`) or images (`image`). Each of these are
	// represented by a different object type.
	Outputs []CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion `json:"outputs"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Input       respjson.Field
		Outputs     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r CodeInterpreterToolCallDeltaCodeInterpreter) RawJSON() string { return r.JSON.raw }
func (r *CodeInterpreterToolCallDeltaCodeInterpreter) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion contains all possible
// properties and values from [CodeInterpreterLogs], [CodeInterpreterOutputImage].
//
// Use the [CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion.AsAny] method to
// switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion struct {
	Index int64 `json:"index"`
	// Any of "logs", "image".
	Type string `json:"type"`
	// This field is from variant [CodeInterpreterLogs].
	Logs string `json:"logs"`
	// This field is from variant [CodeInterpreterOutputImage].
	Image CodeInterpreterOutputImageImage `json:"image"`
	JSON  struct {
		Index respjson.Field
		Type  respjson.Field
		Logs  respjson.Field
		Image respjson.Field
		raw   string
	} `json:"-"`
}

// anyCodeInterpreterToolCallDeltaCodeInterpreterOutput is implemented by each
// variant of [CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion] to add type
// safety for the return type of
// [CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion.AsAny]
type anyCodeInterpreterToolCallDeltaCodeInterpreterOutput interface {
	implCodeInterpreterToolCallDeltaCodeInterpreterOutputUnion()
}

func (CodeInterpreterLogs) implCodeInterpreterToolCallDeltaCodeInterpreterOutputUnion()        {}
func (CodeInterpreterOutputImage) implCodeInterpreterToolCallDeltaCodeInterpreterOutputUnion() {}

// Use the following switch statement to find the correct variant
//
//	switch variant := CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion.AsAny().(type) {
//	case openai.CodeInterpreterLogs:
//	case openai.CodeInterpreterOutputImage:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion) AsAny() anyCodeInterpreterToolCallDeltaCodeInterpreterOutput {
	switch u.Type {
	case "logs":
		return u.AsLogs()
	case "image":
		return u.AsImage()
	}
	return nil
}

func (u CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion) AsLogs() (v CodeInterpreterLogs) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion) AsImage() (v CodeInterpreterOutputImage) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion) RawJSON() string { return u.JSON.raw }

func (r *CodeInterpreterToolCallDeltaCodeInterpreterOutputUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FileSearchToolCall struct {
	// The ID of the tool call object.
	ID string `json:"id,required"`
	// For now, this is always going to be an empty object.
	FileSearch FileSearchToolCallFileSearch `json:"file_search,required"`
	// The type of tool call. This is always going to be `file_search` for this type of
	// tool call.
	Type constant.FileSearch `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		FileSearch  respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchToolCall) RawJSON() string { return r.JSON.raw }
func (r *FileSearchToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// For now, this is always going to be an empty object.
type FileSearchToolCallFileSearch struct {
	// The ranking options for the file search.
	RankingOptions FileSearchToolCallFileSearchRankingOptions `json:"ranking_options"`
	// The results of the file search.
	Results []FileSearchToolCallFileSearchResult `json:"results"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		RankingOptions respjson.Field
		Results        respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchToolCallFileSearch) RawJSON() string { return r.JSON.raw }
func (r *FileSearchToolCallFileSearch) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The ranking options for the file search.
type FileSearchToolCallFileSearchRankingOptions struct {
	// The ranker to use for the file search. If not specified will use the `auto`
	// ranker.
	//
	// Any of "auto", "default_2024_08_21".
	Ranker string `json:"ranker,required"`
	// The score threshold for the file search. All values must be a floating point
	// number between 0 and 1.
	ScoreThreshold float64 `json:"score_threshold,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Ranker         respjson.Field
		ScoreThreshold respjson.Field
		ExtraFields    map[string]respjson.Field
		raw            string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchToolCallFileSearchRankingOptions) RawJSON() string { return r.JSON.raw }
func (r *FileSearchToolCallFileSearchRankingOptions) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// A result instance of the file search.
type FileSearchToolCallFileSearchResult struct {
	// The ID of the file that result was found in.
	FileID string `json:"file_id,required"`
	// The name of the file that result was found in.
	FileName string `json:"file_name,required"`
	// The score of the result. All values must be a floating point number between 0
	// and 1.
	Score float64 `json:"score,required"`
	// The content of the result that was found. The content is only included if
	// requested via the include query parameter.
	Content []FileSearchToolCallFileSearchResultContent `json:"content"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileID      respjson.Field
		FileName    respjson.Field
		Score       respjson.Field
		Content     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchToolCallFileSearchResult) RawJSON() string { return r.JSON.raw }
func (r *FileSearchToolCallFileSearchResult) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FileSearchToolCallFileSearchResultContent struct {
	// The text content of the file.
	Text string `json:"text"`
	// The type of the content.
	//
	// Any of "text".
	Type string `json:"type"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Text        respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchToolCallFileSearchResultContent) RawJSON() string { return r.JSON.raw }
func (r *FileSearchToolCallFileSearchResultContent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FileSearchToolCallDelta struct {
	// For now, this is always going to be an empty object.
	FileSearch any `json:"file_search,required"`
	// The index of the tool call in the tool calls array.
	Index int64 `json:"index,required"`
	// The type of tool call. This is always going to be `file_search` for this type of
	// tool call.
	Type constant.FileSearch `json:"type,required"`
	// The ID of the tool call object.
	ID string `json:"id"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		FileSearch  respjson.Field
		Index       respjson.Field
		Type        respjson.Field
		ID          respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FileSearchToolCallDelta) RawJSON() string { return r.JSON.raw }
func (r *FileSearchToolCallDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FunctionToolCall struct {
	// The ID of the tool call object.
	ID string `json:"id,required"`
	// The definition of the function that was called.
	Function FunctionToolCallFunction `json:"function,required"`
	// The type of tool call. This is always going to be `function` for this type of
	// tool call.
	Type constant.Function `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Function    respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FunctionToolCall) RawJSON() string { return r.JSON.raw }
func (r *FunctionToolCall) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The definition of the function that was called.
type FunctionToolCallFunction struct {
	// The arguments passed to the function.
	Arguments string `json:"arguments,required"`
	// The name of the function.
	Name string `json:"name,required"`
	// The output of the function. This will be `null` if the outputs have not been
	// [submitted](https://platform.openai.com/docs/api-reference/runs/submitToolOutputs)
	// yet.
	Output string `json:"output,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Arguments   respjson.Field
		Name        respjson.Field
		Output      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FunctionToolCallFunction) RawJSON() string { return r.JSON.raw }
func (r *FunctionToolCallFunction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type FunctionToolCallDelta struct {
	// The index of the tool call in the tool calls array.
	Index int64 `json:"index,required"`
	// The type of tool call. This is always going to be `function` for this type of
	// tool call.
	Type constant.Function `json:"type,required"`
	// The ID of the tool call object.
	ID string `json:"id"`
	// The definition of the function that was called.
	Function FunctionToolCallDeltaFunction `json:"function"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Index       respjson.Field
		Type        respjson.Field
		ID          respjson.Field
		Function    respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FunctionToolCallDelta) RawJSON() string { return r.JSON.raw }
func (r *FunctionToolCallDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The definition of the function that was called.
type FunctionToolCallDeltaFunction struct {
	// The arguments passed to the function.
	Arguments string `json:"arguments"`
	// The name of the function.
	Name string `json:"name"`
	// The output of the function. This will be `null` if the outputs have not been
	// [submitted](https://platform.openai.com/docs/api-reference/runs/submitToolOutputs)
	// yet.
	Output string `json:"output,nullable"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Arguments   respjson.Field
		Name        respjson.Field
		Output      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r FunctionToolCallDeltaFunction) RawJSON() string { return r.JSON.raw }
func (r *FunctionToolCallDeltaFunction) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Details of the message creation by the run step.
type MessageCreationStepDetails struct {
	MessageCreation MessageCreationStepDetailsMessageCreation `json:"message_creation,required"`
	// Always `message_creation`.
	Type constant.MessageCreation `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		MessageCreation respjson.Field
		Type            respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageCreationStepDetails) RawJSON() string { return r.JSON.raw }
func (r *MessageCreationStepDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type MessageCreationStepDetailsMessageCreation struct {
	// The ID of the message that was created by this run step.
	MessageID string `json:"message_id,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		MessageID   respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r MessageCreationStepDetailsMessageCreation) RawJSON() string { return r.JSON.raw }
func (r *MessageCreationStepDetailsMessageCreation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Represents a step in execution of a run.
type RunStep struct {
	// The identifier of the run step, which can be referenced in API endpoints.
	ID string `json:"id,required"`
	// The ID of the
	// [assistant](https://platform.openai.com/docs/api-reference/assistants)
	// associated with the run step.
	AssistantID string `json:"assistant_id,required"`
	// The Unix timestamp (in seconds) for when the run step was cancelled.
	CancelledAt int64 `json:"cancelled_at,required"`
	// The Unix timestamp (in seconds) for when the run step completed.
	CompletedAt int64 `json:"completed_at,required"`
	// The Unix timestamp (in seconds) for when the run step was created.
	CreatedAt int64 `json:"created_at,required"`
	// The Unix timestamp (in seconds) for when the run step expired. A step is
	// considered expired if the parent run is expired.
	ExpiredAt int64 `json:"expired_at,required"`
	// The Unix timestamp (in seconds) for when the run step failed.
	FailedAt int64 `json:"failed_at,required"`
	// The last error associated with this run step. Will be `null` if there are no
	// errors.
	LastError RunStepLastError `json:"last_error,required"`
	// Set of 16 key-value pairs that can be attached to an object. This can be useful
	// for storing additional information about the object in a structured format, and
	// querying for objects via API or the dashboard.
	//
	// Keys are strings with a maximum length of 64 characters. Values are strings with
	// a maximum length of 512 characters.
	Metadata shared.Metadata `json:"metadata,required"`
	// The object type, which is always `thread.run.step`.
	Object constant.ThreadRunStep `json:"object,required"`
	// The ID of the [run](https://platform.openai.com/docs/api-reference/runs) that
	// this run step is a part of.
	RunID string `json:"run_id,required"`
	// The status of the run step, which can be either `in_progress`, `cancelled`,
	// `failed`, `completed`, or `expired`.
	//
	// Any of "in_progress", "cancelled", "failed", "completed", "expired".
	Status RunStepStatus `json:"status,required"`
	// The details of the run step.
	StepDetails RunStepStepDetailsUnion `json:"step_details,required"`
	// The ID of the [thread](https://platform.openai.com/docs/api-reference/threads)
	// that was run.
	ThreadID string `json:"thread_id,required"`
	// The type of run step, which can be either `message_creation` or `tool_calls`.
	//
	// Any of "message_creation", "tool_calls".
	Type RunStepType `json:"type,required"`
	// Usage statistics related to the run step. This value will be `null` while the
	// run step's status is `in_progress`.
	Usage RunStepUsage `json:"usage,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		AssistantID respjson.Field
		CancelledAt respjson.Field
		CompletedAt respjson.Field
		CreatedAt   respjson.Field
		ExpiredAt   respjson.Field
		FailedAt    respjson.Field
		LastError   respjson.Field
		Metadata    respjson.Field
		Object      respjson.Field
		RunID       respjson.Field
		Status      respjson.Field
		StepDetails respjson.Field
		ThreadID    respjson.Field
		Type        respjson.Field
		Usage       respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r RunStep) RawJSON() string { return r.JSON.raw }
func (r *RunStep) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The last error associated with this run step. Will be `null` if there are no
// errors.
type RunStepLastError struct {
	// One of `server_error` or `rate_limit_exceeded`.
	//
	// Any of "server_error", "rate_limit_exceeded".
	Code string `json:"code,required"`
	// A human-readable description of the error.
	Message string `json:"message,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Code        respjson.Field
		Message     respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r RunStepLastError) RawJSON() string { return r.JSON.raw }
func (r *RunStepLastError) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The status of the run step, which can be either `in_progress`, `cancelled`,
// `failed`, `completed`, or `expired`.
type RunStepStatus string

const (
	RunStepStatusInProgress RunStepStatus = "in_progress"
	RunStepStatusCancelled  RunStepStatus = "cancelled"
	RunStepStatusFailed     RunStepStatus = "failed"
	RunStepStatusCompleted  RunStepStatus = "completed"
	RunStepStatusExpired    RunStepStatus = "expired"
)

// RunStepStepDetailsUnion contains all possible properties and values from
// [MessageCreationStepDetails], [ToolCallsStepDetails].
//
// Use the [RunStepStepDetailsUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type RunStepStepDetailsUnion struct {
	// This field is from variant [MessageCreationStepDetails].
	MessageCreation MessageCreationStepDetailsMessageCreation `json:"message_creation"`
	// Any of "message_creation", "tool_calls".
	Type string `json:"type"`
	// This field is from variant [ToolCallsStepDetails].
	ToolCalls []ToolCallUnion `json:"tool_calls"`
	JSON      struct {
		MessageCreation respjson.Field
		Type            respjson.Field
		ToolCalls       respjson.Field
		raw             string
	} `json:"-"`
}

// anyRunStepStepDetails is implemented by each variant of
// [RunStepStepDetailsUnion] to add type safety for the return type of
// [RunStepStepDetailsUnion.AsAny]
type anyRunStepStepDetails interface {
	implRunStepStepDetailsUnion()
}

func (MessageCreationStepDetails) implRunStepStepDetailsUnion() {}
func (ToolCallsStepDetails) implRunStepStepDetailsUnion()       {}

// Use the following switch statement to find the correct variant
//
//	switch variant := RunStepStepDetailsUnion.AsAny().(type) {
//	case openai.MessageCreationStepDetails:
//	case openai.ToolCallsStepDetails:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u RunStepStepDetailsUnion) AsAny() anyRunStepStepDetails {
	switch u.Type {
	case "message_creation":
		return u.AsMessageCreation()
	case "tool_calls":
		return u.AsToolCalls()
	}
	return nil
}

func (u RunStepStepDetailsUnion) AsMessageCreation() (v MessageCreationStepDetails) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u RunStepStepDetailsUnion) AsToolCalls() (v ToolCallsStepDetails) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u RunStepStepDetailsUnion) RawJSON() string { return u.JSON.raw }

func (r *RunStepStepDetailsUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The type of run step, which can be either `message_creation` or `tool_calls`.
type RunStepType string

const (
	RunStepTypeMessageCreation RunStepType = "message_creation"
	RunStepTypeToolCalls       RunStepType = "tool_calls"
)

// Usage statistics related to the run step. This value will be `null` while the
// run step's status is `in_progress`.
type RunStepUsage struct {
	// Number of completion tokens used over the course of the run step.
	CompletionTokens int64 `json:"completion_tokens,required"`
	// Number of prompt tokens used over the course of the run step.
	PromptTokens int64 `json:"prompt_tokens,required"`
	// Total number of tokens used (prompt + completion).
	TotalTokens int64 `json:"total_tokens,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		CompletionTokens respjson.Field
		PromptTokens     respjson.Field
		TotalTokens      respjson.Field
		ExtraFields      map[string]respjson.Field
		raw              string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r RunStepUsage) RawJSON() string { return r.JSON.raw }
func (r *RunStepUsage) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// The delta containing the fields that have changed on the run step.
type RunStepDelta struct {
	// The details of the run step.
	StepDetails RunStepDeltaStepDetailsUnion `json:"step_details"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		StepDetails respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r RunStepDelta) RawJSON() string { return r.JSON.raw }
func (r *RunStepDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// RunStepDeltaStepDetailsUnion contains all possible properties and values from
// [RunStepDeltaMessageDelta], [ToolCallDeltaObject].
//
// Use the [RunStepDeltaStepDetailsUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type RunStepDeltaStepDetailsUnion struct {
	// Any of "message_creation", "tool_calls".
	Type string `json:"type"`
	// This field is from variant [RunStepDeltaMessageDelta].
	MessageCreation RunStepDeltaMessageDeltaMessageCreation `json:"message_creation"`
	// This field is from variant [ToolCallDeltaObject].
	ToolCalls []ToolCallDeltaUnion `json:"tool_calls"`
	JSON      struct {
		Type            respjson.Field
		MessageCreation respjson.Field
		ToolCalls       respjson.Field
		raw             string
	} `json:"-"`
}

// anyRunStepDeltaStepDetails is implemented by each variant of
// [RunStepDeltaStepDetailsUnion] to add type safety for the return type of
// [RunStepDeltaStepDetailsUnion.AsAny]
type anyRunStepDeltaStepDetails interface {
	implRunStepDeltaStepDetailsUnion()
}

func (RunStepDeltaMessageDelta) implRunStepDeltaStepDetailsUnion() {}
func (ToolCallDeltaObject) implRunStepDeltaStepDetailsUnion()      {}

// Use the following switch statement to find the correct variant
//
//	switch variant := RunStepDeltaStepDetailsUnion.AsAny().(type) {
//	case openai.RunStepDeltaMessageDelta:
//	case openai.ToolCallDeltaObject:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u RunStepDeltaStepDetailsUnion) AsAny() anyRunStepDeltaStepDetails {
	switch u.Type {
	case "message_creation":
		return u.AsMessageCreation()
	case "tool_calls":
		return u.AsToolCalls()
	}
	return nil
}

func (u RunStepDeltaStepDetailsUnion) AsMessageCreation() (v RunStepDeltaMessageDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u RunStepDeltaStepDetailsUnion) AsToolCalls() (v ToolCallDeltaObject) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u RunStepDeltaStepDetailsUnion) RawJSON() string { return u.JSON.raw }

func (r *RunStepDeltaStepDetailsUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Represents a run step delta i.e. any changed fields on a run step during
// streaming.
type RunStepDeltaEvent struct {
	// The identifier of the run step, which can be referenced in API endpoints.
	ID string `json:"id,required"`
	// The delta containing the fields that have changed on the run step.
	Delta RunStepDelta `json:"delta,required"`
	// The object type, which is always `thread.run.step.delta`.
	Object constant.ThreadRunStepDelta `json:"object,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ID          respjson.Field
		Delta       respjson.Field
		Object      respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r RunStepDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *RunStepDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Details of the message creation by the run step.
type RunStepDeltaMessageDelta struct {
	// Always `message_creation`.
	Type            constant.MessageCreation                `json:"type,required"`
	MessageCreation RunStepDeltaMessageDeltaMessageCreation `json:"message_creation"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type            respjson.Field
		MessageCreation respjson.Field
		ExtraFields     map[string]respjson.Field
		raw             string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r RunStepDeltaMessageDelta) RawJSON() string { return r.JSON.raw }
func (r *RunStepDeltaMessageDelta) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type RunStepDeltaMessageDeltaMessageCreation struct {
	// The ID of the message that was created by this run step.
	MessageID string `json:"message_id"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		MessageID   respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r RunStepDeltaMessageDeltaMessageCreation) RawJSON() string { return r.JSON.raw }
func (r *RunStepDeltaMessageDeltaMessageCreation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type RunStepInclude string

const (
	RunStepIncludeStepDetailsToolCallsFileSearchResultsContent RunStepInclude = "step_details.tool_calls[*].file_search.results[*].content"
)

// ToolCallUnion contains all possible properties and values from
// [CodeInterpreterToolCall], [FileSearchToolCall], [FunctionToolCall].
//
// Use the [ToolCallUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ToolCallUnion struct {
	ID string `json:"id"`
	// This field is from variant [CodeInterpreterToolCall].
	CodeInterpreter CodeInterpreterToolCallCodeInterpreter `json:"code_interpreter"`
	// Any of "code_interpreter", "file_search", "function".
	Type string `json:"type"`
	// This field is from variant [FileSearchToolCall].
	FileSearch FileSearchToolCallFileSearch `json:"file_search"`
	// This field is from variant [FunctionToolCall].
	Function FunctionToolCallFunction `json:"function"`
	JSON     struct {
		ID              respjson.Field
		CodeInterpreter respjson.Field
		Type            respjson.Field
		FileSearch      respjson.Field
		Function        respjson.Field
		raw             string
	} `json:"-"`
}

// anyToolCall is implemented by each variant of [ToolCallUnion] to add type safety
// for the return type of [ToolCallUnion.AsAny]
type anyToolCall interface {
	implToolCallUnion()
}

func (CodeInterpreterToolCall) implToolCallUnion() {}
func (FileSearchToolCall) implToolCallUnion()      {}
func (FunctionToolCall) implToolCallUnion()        {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ToolCallUnion.AsAny().(type) {
//	case openai.CodeInterpreterToolCall:
//	case openai.FileSearchToolCall:
//	case openai.FunctionToolCall:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ToolCallUnion) AsAny() anyToolCall {
	switch u.Type {
	case "code_interpreter":
		return u.AsCodeInterpreter()
	case "file_search":
		return u.AsFileSearch()
	case "function":
		return u.AsFunction()
	}
	return nil
}

func (u ToolCallUnion) AsCodeInterpreter() (v CodeInterpreterToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolCallUnion) AsFileSearch() (v FileSearchToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolCallUnion) AsFunction() (v FunctionToolCall) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ToolCallUnion) RawJSON() string { return u.JSON.raw }

func (r *ToolCallUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// ToolCallDeltaUnion contains all possible properties and values from
// [CodeInterpreterToolCallDelta], [FileSearchToolCallDelta],
// [FunctionToolCallDelta].
//
// Use the [ToolCallDeltaUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type ToolCallDeltaUnion struct {
	Index int64 `json:"index"`
	// Any of "code_interpreter", "file_search", "function".
	Type string `json:"type"`
	ID   string `json:"id"`
	// This field is from variant [CodeInterpreterToolCallDelta].
	CodeInterpreter CodeInterpreterToolCallDeltaCodeInterpreter `json:"code_interpreter"`
	// This field is from variant [FileSearchToolCallDelta].
	FileSearch any `json:"file_search"`
	// This field is from variant [FunctionToolCallDelta].
	Function FunctionToolCallDeltaFunction `json:"function"`
	JSON     struct {
		Index           respjson.Field
		Type            respjson.Field
		ID              respjson.Field
		CodeInterpreter respjson.Field
		FileSearch      respjson.Field
		Function        respjson.Field
		raw             string
	} `json:"-"`
}

// anyToolCallDelta is implemented by each variant of [ToolCallDeltaUnion] to add
// type safety for the return type of [ToolCallDeltaUnion.AsAny]
type anyToolCallDelta interface {
	implToolCallDeltaUnion()
}

func (CodeInterpreterToolCallDelta) implToolCallDeltaUnion() {}
func (FileSearchToolCallDelta) implToolCallDeltaUnion()      {}
func (FunctionToolCallDelta) implToolCallDeltaUnion()        {}

// Use the following switch statement to find the correct variant
//
//	switch variant := ToolCallDeltaUnion.AsAny().(type) {
//	case openai.CodeInterpreterToolCallDelta:
//	case openai.FileSearchToolCallDelta:
//	case openai.FunctionToolCallDelta:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u ToolCallDeltaUnion) AsAny() anyToolCallDelta {
	switch u.Type {
	case "code_interpreter":
		return u.AsCodeInterpreter()
	case "file_search":
		return u.AsFileSearch()
	case "function":
		return u.AsFunction()
	}
	return nil
}

func (u ToolCallDeltaUnion) AsCodeInterpreter() (v CodeInterpreterToolCallDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolCallDeltaUnion) AsFileSearch() (v FileSearchToolCallDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u ToolCallDeltaUnion) AsFunction() (v FunctionToolCallDelta) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u ToolCallDeltaUnion) RawJSON() string { return u.JSON.raw }

func (r *ToolCallDeltaUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Details of the tool call.
type ToolCallDeltaObject struct {
	// Always `tool_calls`.
	Type constant.ToolCalls `json:"type,required"`
	// An array of tool calls the run step was involved in. These can be associated
	// with one of three types of tools: `code_interpreter`, `file_search`, or
	// `function`.
	ToolCalls []ToolCallDeltaUnion `json:"tool_calls"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		Type        respjson.Field
		ToolCalls   respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolCallDeltaObject) RawJSON() string { return r.JSON.raw }
func (r *ToolCallDeltaObject) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Details of the tool call.
type ToolCallsStepDetails struct {
	// An array of tool calls the run step was involved in. These can be associated
	// with one of three types of tools: `code_interpreter`, `file_search`, or
	// `function`.
	ToolCalls []ToolCallUnion `json:"tool_calls,required"`
	// Always `tool_calls`.
	Type constant.ToolCalls `json:"type,required"`
	// JSON contains metadata for fields, check presence with [respjson.Field.Valid].
	JSON struct {
		ToolCalls   respjson.Field
		Type        respjson.Field
		ExtraFields map[string]respjson.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r ToolCallsStepDetails) RawJSON() string { return r.JSON.raw }
func (r *ToolCallsStepDetails) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type BetaThreadRunStepGetParams struct {
	// A list of additional fields to include in the response. Currently the only
	// supported value is `step_details.tool_calls[*].file_search.results[*].content`
	// to fetch the file search result content.
	//
	// See the
	// [file search tool documentation](https://platform.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings)
	// for more information.
	Include []RunStepInclude `query:"include,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [BetaThreadRunStepGetParams]'s query parameters as
// `url.Values`.
func (r BetaThreadRunStepGetParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

type BetaThreadRunStepListParams struct {
	// A cursor for use in pagination. `after` is an object ID that defines your place
	// in the list. For instance, if you make a list request and receive 100 objects,
	// ending with obj_foo, your subsequent call can include after=obj_foo in order to
	// fetch the next page of the list.
	After param.Opt[string] `query:"after,omitzero" json:"-"`
	// A cursor for use in pagination. `before` is an object ID that defines your place
	// in the list. For instance, if you make a list request and receive 100 objects,
	// starting with obj_foo, your subsequent call can include before=obj_foo in order
	// to fetch the previous page of the list.
	Before param.Opt[string] `query:"before,omitzero" json:"-"`
	// A limit on the number of objects to be returned. Limit can range between 1 and
	// 100, and the default is 20.
	Limit param.Opt[int64] `query:"limit,omitzero" json:"-"`
	// A list of additional fields to include in the response. Currently the only
	// supported value is `step_details.tool_calls[*].file_search.results[*].content`
	// to fetch the file search result content.
	//
	// See the
	// [file search tool documentation](https://platform.openai.com/docs/assistants/tools/file-search#customizing-file-search-settings)
	// for more information.
	Include []RunStepInclude `query:"include,omitzero" json:"-"`
	// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
	// order and `desc` for descending order.
	//
	// Any of "asc", "desc".
	Order BetaThreadRunStepListParamsOrder `query:"order,omitzero" json:"-"`
	paramObj
}

// URLQuery serializes [BetaThreadRunStepListParams]'s query parameters as
// `url.Values`.
func (r BetaThreadRunStepListParams) URLQuery() (v url.Values, err error) {
	return apiquery.MarshalWithSettings(r, apiquery.QuerySettings{
		ArrayFormat:  apiquery.ArrayQueryFormatBrackets,
		NestedFormat: apiquery.NestedQueryFormatBrackets,
	})
}

// Sort order by the `created_at` timestamp of the objects. `asc` for ascending
// order and `desc` for descending order.
type BetaThreadRunStepListParamsOrder string

const (
	BetaThreadRunStepListParamsOrderAsc  BetaThreadRunStepListParamsOrder = "asc"
	BetaThreadRunStepListParamsOrderDesc BetaThreadRunStepListParamsOrder = "desc"
)
