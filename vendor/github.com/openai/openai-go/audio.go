// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.

package openai

import (
	"github.com/openai/openai-go/option"
)

// AudioService contains methods and other services that help with interacting with
// the openai API.
//
// Note, unlike clients, this service does not read variables from the environment
// automatically. You should not instantiate this service directly, and instead use
// the [NewAudioService] method instead.
type AudioService struct {
	Options        []option.RequestOption
	Transcriptions AudioTranscriptionService
	Translations   AudioTranslationService
	Speech         AudioSpeechService
}

// NewAudioService generates a new service that applies the given options to each
// request. These options are applied after the parent client's options (if there
// is one), and before any request-specific options.
func NewAudioService(opts ...option.RequestOption) (r AudioService) {
	r = AudioService{}
	r.Options = opts
	r.Transcriptions = NewAudioTranscriptionService(opts...)
	r.Translations = NewAudioTranslationService(opts...)
	r.Speech = NewAudioSpeechService(opts...)
	return
}

type AudioModel = string

const (
	AudioModelWhisper1            AudioModel = "whisper-1"
	AudioModelGPT4oTranscribe     AudioModel = "gpt-4o-transcribe"
	AudioModelGPT4oMiniTranscribe AudioModel = "gpt-4o-mini-transcribe"
)

// The format of the output, in one of these options: `json`, `text`, `srt`,
// `verbose_json`, or `vtt`. For `gpt-4o-transcribe` and `gpt-4o-mini-transcribe`,
// the only supported format is `json`.
type AudioResponseFormat string

const (
	AudioResponseFormatJSON        AudioResponseFormat = "json"
	AudioResponseFormatText        AudioResponseFormat = "text"
	AudioResponseFormatSRT         AudioResponseFormat = "srt"
	AudioResponseFormatVerboseJSON AudioResponseFormat = "verbose_json"
	AudioResponseFormatVTT         AudioResponseFormat = "vtt"
)
