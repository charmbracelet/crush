package config

import (
	"encoding/json"
	"fmt"
)

// TimestampFormat selects how completion timestamps appear in the TUI.
type TimestampFormat string

const (
	TimestampFormat12Hour TimestampFormat = "12h"
	TimestampFormat24Hour TimestampFormat = "24h"
	TimestampFormatOff    TimestampFormat = "off"
)

// UnmarshalJSON rejects unsupported timestamp formats.
func (f *TimestampFormat) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	format := TimestampFormat(value)
	switch format {
	case "", TimestampFormat12Hour, TimestampFormat24Hour, TimestampFormatOff:
		*f = format
		return nil
	default:
		return fmt.Errorf("timestamps expects 12h, 24h, or off, got %q", value)
	}
}

// Layout returns the time layout, or an empty string when timestamps are off.
// The zero value uses the default 12-hour format.
func (f TimestampFormat) Layout() string {
	switch f {
	case TimestampFormat24Hour:
		return "15:04"
	case TimestampFormatOff:
		return ""
	default:
		return "3:04 PM"
	}
}

// TimestampFormat returns the configured format, defaulting to 12-hour time.
func (c *Config) TimestampFormat() TimestampFormat {
	if c == nil || c.Options == nil || c.Options.TUI == nil || c.Options.TUI.Timestamps == "" {
		return TimestampFormat12Hour
	}
	return c.Options.TUI.Timestamps
}
