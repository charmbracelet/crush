// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/openai/openai-go/internal/apiform"
	"github.com/openai/openai-go/internal/apijson"
	"github.com/openai/openai-go/internal/requestconfig"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/resp"
)

// AudioTranslationService contains methods and other services that help with
// interacting with the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewAudioTranslationService] method instead.
type AudioTranslationService struct {
	Options []option.RequestOption
}

// NewAudioTranslationService generates a new service that applies the given
// options to each request. These options are applied after the parent client's
// options (if there is one), and before any request-specific options.
func NewAudioTranslationService(opts ...option.RequestOption) (r AudioTranslationService) {
	r = AudioTranslationService{}
	r.Options = opts
	return
}

// Translates audio into English.
func (r *AudioTranslationService) New(ctx context.Context, body AudioTranslationNewParams, opts ...option.RequestOption) (res *Translation, err error) {
	opts = append(r.Options[:], opts...)
	path := "audio/translations"
	err = requestconfig.ExecuteNewRequest(ctx, http.MethodPost, path, body, &res, opts...)
	return
}

type Translation struct {
	Text string `json:"text,required"`
	// Metadata for the response, check the presence of optional fields with the
	// [resp.Field.IsPresent] method.
	JSON struct {
		Text        resp.Field
		ExtraFields map[string]resp.Field
		raw         string
	} `json:"-"`
}

// Returns the unmodified JSON received from the API
func (r Translation) RawJSON() string { return r.JSON.raw }
func (r *Translation) UnmarshalJSON(data []byte) error {
	return apijson.UnmarshalRoot(data, r)
}

type AudioTranslationNewParams struct {
	// The audio file object (not file name) translate, in one of these formats: flac,
	// mp3, mp4, mpeg, mpga, m4a, ogg, wav, or webm.
	File io.Reader `json:"file,required" format:"binary"`
	// ID of the model to use. Only `whisper-1` (which is powered by our open source
	// Whisper V2 model) is currently available.
	Model AudioModel `json:"model,omitzero,required"`
	// An optional text to guide the model's style or continue a previous audio
	// segment. The
	// [prompt](https://platform.openai.com/docs/guides/speech-to-text#prompting)
	// should be in English.
	Prompt param.Opt[string] `json:"prompt,omitzero"`
	// The sampling temperature, between 0 and 1. Higher values like 0.8 will make the
	// output more random, while lower values like 0.2 will make it more focused and
	// deterministic. If set to 0, the model will use
	// [log probability](https://en.wikipedia.org/wiki/Log_probability) to
	// automatically increase the temperature until certain thresholds are hit.
	Temperature param.Opt[float64] `json:"temperature,omitzero"`
	// The format of the output, in one of these options: `json`, `text`, `srt`,
	// `verbose_json`, or `vtt`.
	//
	// Any of "json", "text", "srt", "verbose_json", "vtt".
	ResponseFormat AudioTranslationNewParamsResponseFormat `json:"response_format,omitzero"`
	paramObj
}

// IsPresent returns true if the field's value is not omitted and not the JSON
// "null". To check if this field is omitted, use [param.IsOmitted].
func (f AudioTranslationNewParams) IsPresent() bool { return !param.IsOmitted(f) && !f.IsNull() }

func (r AudioTranslationNewParams) MarshalMultipart() (data []byte, contentType string, err error) {
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

// The format of the output, in one of these options: `json`, `text`, `srt`,
// `verbose_json`, or `vtt`.
type AudioTranslationNewParamsResponseFormat string

const (
	AudioTranslationNewParamsResponseFormatJSON        AudioTranslationNewParamsResponseFormat = "json"
	AudioTranslationNewParamsResponseFormatText        AudioTranslationNewParamsResponseFormat = "text"
	AudioTranslationNewParamsResponseFormatSRT         AudioTranslationNewParamsResponseFormat = "srt"
	AudioTranslationNewParamsResponseFormatVerboseJSON AudioTranslationNewParamsResponseFormat = "verbose_json"
	AudioTranslationNewParamsResponseFormatVTT         AudioTranslationNewParamsResponseFormat = "vtt"
)
