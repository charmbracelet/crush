package editor

import "errors"

// ErrUnavailable is returned by Bridge methods when no editor is attached.
// Tools should treat it as "feature disabled" rather than a hard failure.
var ErrUnavailable = errors.New("editor bridge unavailable")
