package log

// Formatter is a formatter for log messages.
type Formatter uint8

const (
	// TextFormatter is a formatter that formats log messages as text. Suitable for
	// console output and log files.
	TextFormatter Formatter = iota
	// JSONFormatter is a formatter that formats log messages as JSON.
	JSONFormatter
	// LogfmtFormatter is a formatter that formats log messages as logfmt.
	LogfmtFormatter
)

var (
	// TimestampKey is the key for the timestamp.
	TimestampKey = "time"
	// MessageKey is the key for the message.
	MessageKey = "msg"
	// LevelKey is the key for the level.
	LevelKey = "level"
	// CallerKey is the key for the caller.
	CallerKey = "caller"
	// PrefixKey is the key for the prefix.
	PrefixKey = "prefix"
)
