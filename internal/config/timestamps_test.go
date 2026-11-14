package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTimestampFormatConfig(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		json   string
		format TimestampFormat
		layout string
	}{
		{name: "unset", json: `{}`, format: TimestampFormat12Hour, layout: "3:04 PM"},
		{name: "empty", json: `{"options":{"tui":{"timestamps":""}}}`, format: TimestampFormat12Hour, layout: "3:04 PM"},
		{name: "12-hour", json: `{"options":{"tui":{"timestamps":"12h"}}}`, format: TimestampFormat12Hour, layout: "3:04 PM"},
		{name: "24-hour", json: `{"options":{"tui":{"timestamps":"24h"}}}`, format: TimestampFormat24Hour, layout: "15:04"},
		{name: "off", json: `{"options":{"tui":{"timestamps":"off"}}}`, format: TimestampFormatOff},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := loadFromBytes([][]byte{[]byte(tt.json)})
			require.NoError(t, err)
			cfg.NormalizeOptions()
			require.Equal(t, tt.format, cfg.Options.TUI.Timestamps)
			require.Equal(t, tt.format, cfg.TimestampFormat())
			require.Equal(t, tt.layout, cfg.TimestampFormat().Layout())
			cfg.NormalizeOptions()
			require.Equal(t, tt.format, cfg.Options.TUI.Timestamps, "normalization must preserve explicit settings")
		})
	}
}

func TestTimestampFormatRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	for _, value := range []string{`"auto"`, `"25h"`, `"OFF"`, `true`, `24`} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			_, err := loadFromBytes([][]byte{[]byte(`{"options":{"tui":{"timestamps":` + value + `}}}`)})
			require.Error(t, err)
		})
	}
}

func TestTimestampFormatDefaultWithoutOptions(t *testing.T) {
	t.Parallel()

	for _, cfg := range []*Config{nil, {}, {Options: &Options{}}, {Options: &Options{TUI: &TUIOptions{}}}} {
		require.Equal(t, TimestampFormat12Hour, cfg.TimestampFormat())
	}
}
