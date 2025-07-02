// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/openai/openai-go/internal/apiform"
	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/resp"
	"github.com/openai/openai-go/packages/ssestream"
	"github.com/openai/openai-go/shared/constant"
)

// AudioTranscriptionService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewAudioTranscriptionService] method instead.
type AudioTranscriptionService struct {
	Options []option.RequestOption
}

// NewAudioTranscriptionService generates a new service that applies the given
// options to each request. These options are applied after the parent client's
// options (if there is one), and before any request-specific options.
func NewAudioTranscriptionService(opts ...option.RequestOption) (r AudioTranscriptionService) {
	r = AudioTranscriptionService{}
	r.Options = opts
	return
}

// Transcribes audio into the input language.
func (r *AudioTranscriptionService) New(ctx context.Context, body AudioTranscriptionNewParams, opts ...option.RequestOption) (res *Transcription, err error) {
	opts = append(r.Options[:], opts...)
	path := "audio/transcriptions"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

// Transcribes audio into the input language.
func (r *AudioTranscriptionService) NewStreaming(ctx context.Context, body AudioTranscriptionNewParams, opts ...option.RequestOption) (stream *ssestream.Stream[TranscriptionStreamEventUnion]) {
	var (
		raw *http.Response
		err error
	)
	opts = append(r.Options[:], opts...)
	opts = append([]option.RequestOption{option.WithJSONSet("stream", true)}, opts...)
	path := "audio/transcriptions"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &raw, opts...)
	return ssestream.NewStream[TranscriptionStreamEventUnion](ssestream.NewDecoder(raw), err)
}

// Represents a transcription response returned by model, based on the provided
// input.
type Transcription struct {
	// The transcribed text.
	Text string `json:"text,required"`
	// The log probabilities of the tokens in the transcription. Only returned with the
	// models `gpt-4o-transcribe` and `gpt-4o-mini-transcribe` if `logprobs` is added
	// to the `include` array.
	Logprobs []TranscriptionLogprob `json:"logprobs"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Text        resp.Field
		Logprobs    resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r Transcription) RawJSON() string { return r.JSON.raw }
func (r *Transcription) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type TranscriptionLogprob struct {
	// The token in the transcription.
	Token string `json:"token"`
	// The bytes of the token.
	Bytes []float64 `json:"bytes"`
	// The log probability of the token.
	Logprob float64 `json:"logprob"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Token       resp.Field
		Bytes       resp.Field
		Logprob     resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r TranscriptionLogprob) RawJSON() string { return r.JSON.raw }
func (r *TranscriptionLogprob) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type TranscriptionInclude string

const (
	TranscriptionIncludeLogprobs TranscriptionInclude = "logprobs"
)

// TranscriptionStreamEventUnion contains all possible properties and values from
// [TranscriptionTextDeltaEvent], [TranscriptionTextDoneEvent].
//
// Use the [TranscriptionStreamEventUnion.AsAny] method to switch on the variant.
//
// Use the methods beginning with 'As' to cast the union to one of its variants.
type TranscriptionStreamEventUnion struct {
	// This field is from variant [TranscriptionTextDeltaEvent].
	Delta string `json:"delta"`
	// Any of "transcript.text.delta", "transcript.text.done".
	Type string `json:"type"`
	// This field is a union of [[]TranscriptionTextDeltaEventLogprob],
	// [[]TranscriptionTextDoneEventLogprob]
	Logprobs TranscriptionStreamEventUnionLogprobs `json:"logprobs"`
	// This field is from variant [TranscriptionTextDoneEvent].
	Text string `json:"text"`
	JSON struct {
		Delta    resp.Field
		Type     resp.Field
		Logprobs resp.Field
		Text     resp.Field
		raw      string
	} `json:"-"`
}

// Use the following switch statement to find the correct variant
//
//	switch variant := TranscriptionStreamEventUnion.AsAny().(type) {
//	case TranscriptionTextDeltaEvent:
//	case TranscriptionTextDoneEvent:
//	default:
//	  fmt.Errorf("no variant present")
//	}
func (u TranscriptionStreamEventUnion) AsAny() any {
	switch u.Type {
	case "transcript.text.delta":
		return u.AsTranscriptTextDelta()
	case "transcript.text.done":
		return u.AsTranscriptTextDone()
	}
	return nil
}

func (u TranscriptionStreamEventUnion) AsTranscriptTextDelta() (v TranscriptionTextDeltaEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

func (u TranscriptionStreamEventUnion) AsTranscriptTextDone() (v TranscriptionTextDoneEvent) {
	apijson.UnmarshalRoot(json.RawMessage(u.JSON.raw), &v)
	return
}

// Returns the unmodified JSON received from the API
func (u TranscriptionStreamEventUnion) RawJSON() string { return u.JSON.raw }

func (r *TranscriptionStreamEventUnion) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// TranscriptionStreamEventUnionLogprobs is an implicit subunion of
// [TranscriptionStreamEventUnion]. TranscriptionStreamEventUnionLogprobs provides
// convenient access to the sub-properties of the union.
//
// For type safety it is recommended to directly use a variant of the
// [TranscriptionStreamEventUnion].
//
// If the underlying value is not a json object, one of the following properties
// will be valid: OfTranscriptionTextDeltaEventLogprobs
// OfTranscriptionTextDoneEventLogprobs]
type TranscriptionStreamEventUnionLogprobs struct {
	// This field will be present if the value is a
	// [[]TranscriptionTextDeltaEventLogprob] instead of an object.
	OfTranscriptionTextDeltaEventLogprobs []TranscriptionTextDeltaEventLogprob `json:",inline"`
	// This field will be present if the value is a
	// [[]TranscriptionTextDoneEventLogprob] instead of an object.
	OfTranscriptionTextDoneEventLogprobs []TranscriptionTextDoneEventLogprob `json:",inline"`
	JSON                                 struct {
		OfTranscriptionTextDeltaEventLogprobs resp.Field
		OfTranscriptionTextDoneEventLogprobs  resp.Field
		raw                                   string
	} `json:"-"`
}

func (r *TranscriptionStreamEventUnionLogprobs) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when there is an additional text delta. This is also the first event
// emitted when the transcription starts. Only emitted when you
// [create a transcription](https://platform.openai.com/docs/api-reference/audio/create-transcription)
// with the `Stream` parameter set to `true`.
type TranscriptionTextDeltaEvent struct {
	// The text delta that was additionally transcribed.
	Delta string `json:"delta,required"`
	// The type of the event. Always `transcript.text.delta`.
	Type constant.TranscriptTextDelta `json:"type,required"`
	// The log probabilities of the delta. Only included if you
	// [create a transcription](https://platform.openai.com/docs/api-reference/audio/create-transcription)
	// with the `include[]` parameter set to `logprobs`.
	Logprobs []TranscriptionTextDeltaEventLogprob `json:"logprobs"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Delta       resp.Field
		Type        resp.Field
		Logprobs    resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r TranscriptionTextDeltaEvent) RawJSON() string { return r.JSON.raw }
func (r *TranscriptionTextDeltaEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type TranscriptionTextDeltaEventLogprob struct {
	// The token that was used to generate the log probability.
	Token string `json:"token"`
	// The bytes that were used to generate the log probability.
	Bytes []interface{} `json:"bytes"`
	// The log probability of the token.
	Logprob float64 `json:"logprob"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Token       resp.Field
		Bytes       resp.Field
		Logprob     resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r TranscriptionTextDeltaEventLogprob) RawJSON() string { return r.JSON.raw }
func (r *TranscriptionTextDeltaEventLogprob) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

// Emitted when the transcription is complete. Contains the complete transcription
// text. Only emitted when you
// [create a transcription](https://platform.openai.com/docs/api-reference/audio/create-transcription)
// with the `Stream` parameter set to `true`.
type TranscriptionTextDoneEvent struct {
	// The text that was transcribed.
	Text string `json:"text,required"`
	// The type of the event. Always `transcript.text.done`.
	Type constant.TranscriptTextDone `json:"type,required"`
	// The log probabilities of the individual tokens in the transcription. Only
	// included if you
	// [create a transcription](https://platform.openai.com/docs/api-reference/audio/create-transcription)
	// with the `include[]` parameter set to `logprobs`.
	Logprobs []TranscriptionTextDoneEventLogprob `json:"logprobs"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Text        resp.Field
		Type        resp.Field
		Logprobs    resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r TranscriptionTextDoneEvent) RawJSON() string { return r.JSON.raw }
func (r *TranscriptionTextDoneEvent) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type TranscriptionTextDoneEventLogprob struct {
	// The token that was used to generate the log probability.
	Token string `json:"token"`
	// The bytes that were used to generate the log probability.
	Bytes []interface{} `json:"bytes"`
	// The log probability of the token.
	Logprob float64 `json:"logprob"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Token       resp.Field
		Bytes       resp.Field
		Logprob     resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r TranscriptionTextDoneEventLogprob) RawJSON() string { return r.JSON.raw }
func (r *TranscriptionTextDoneEventLogprob) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type AudioTranscriptionNewParams struct {
	// The audio file object (not file name) to transcribe, in one of these formats:
	// flac, mp3, mp4, mpeg, mpga, m4a, ogg, wav, or webm.
	File io.Reader `json:"file,required" format:"binary"`
	// ID of the model to use. The options are `gpt-4o-transcribe`,
	// `gpt-4o-mini-transcribe`, and `whisper-1` (which is powered by our open source
	// Whisper V2 model).
	Model AudioModel `json:"model,omitzero,required"`
	// The language of the input audio. Supplying the input language in
	// [ISO-639-1](https://en.wikipedia.org/wiki/List_of_ISO_639-1_codes) (e.g. `en`)
	// format will improve accuracy and latency.
	Language param.Opt[string] `json:"language,omitzero"`
	// An optional text to guide the model's style or continue a previous audio
	// segment. The
	// [prompt](https://platform.openai.com/docs/guides/speech-to-text#prompting)
	// should match the audio language.
	Prompt param.Opt[string] `json:"prompt,omitzero"`
	// The sampling temperature, between 0 and 1. Higher values like 0.8 will make the
	// output more random, while lower values like 0.2 will make it more focused and
	// deterministic. If set to 0, the model will use
	// [log probability](https://en.wikipedia.org/wiki/Log_probability) to
	// automatically increase the temperature until certain thresholds are hit.
	Temperature param.Opt[float64] `json:"temperature,omitzero"`
	// Additional information to include in the transcription response. `logprobs` will
	// return the log probabilities of the tokens in the response to understand the
	// model's confidence in the transcription. `logprobs` only works with
	// response_format set to `json` and only with the models `gpt-4o-transcribe` and
	// `gpt-4o-mini-transcribe`.
	Include []TranscriptionInclude `json:"include,omitzero"`
	// The format of the output, in one of these options: `json`, `text`, `srt`,
	// `verbose_json`, or `vtt`. For `gpt-4o-transcribe` and `gpt-4o-mini-transcribe`,
	// the only supported format is `json`.
	//
	// Any of "json", "text", "srt", "verbose_json", "vtt".
	ResponseFormat AudioResponseFormat `json:"response_format,omitzero"`
	// The timestamp granularities to populate for this transcription.
	// `response_format` must be set `verbose_json` to use timestamp granularities.
	// Either or both of these options are supported: `word`, or `segment`. Note: There
	// is no additional latency for segment timestamps, but generating word timestamps
	// incurs additional latency.
	TimestampGranularities []string `json:"timestamp_granularities,omitzero"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f AudioTranscriptionNewParams) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }

func (r AudioTranscriptionNewParams) MarshalMultipart() (data []byte, contentType string, err error) {
	buf := bytes.NewBuffer(nil)
	writer := multipart.NewWriter(buf)
	err = apiform.MarshalRoot(r, writer)
	if err != nil {
		writer.Close()
		return nil, "", err
	}
	err = writer.Close()
	if err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}
