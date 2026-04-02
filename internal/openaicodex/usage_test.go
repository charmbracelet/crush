package openaicodex

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUsageEndpoint(t *testing.T) {
	t.Parallel()

	got, err := usageEndpoint("https://chatgpt.com/backend-api/codex")
	require.NoError(t, err)
	require.Equal(t, "https://chatgpt.com/backend-api/wham/usage", got)

	got, err = usageEndpoint("https://chatgpt.com/backend-api")
	require.NoError(t, err)
	require.Equal(t, "https://chatgpt.com/backend-api/wham/usage", got)
}

func TestParseUsagePayload(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"plan_type":"pro",
		"rate_limit":{
			"primary_window":{"used_percent":42,"limit_window_seconds":18000,"reset_at":2000000000},
			"secondary_window":{"used_percent":77,"limit_window_seconds":604800,"reset_at":2000100000}
		},
		"additional_rate_limits":[
			{
				"limit_name":"codex_other",
				"metered_feature":"codex_other",
				"rate_limit":{
					"primary_window":{"used_percent":21,"limit_window_seconds":3600,"reset_at":2000200000}
				}
			}
		],
		"credits":{"has_credits":true,"unlimited":false,"balance":"9.99"}
	}`)

	report, err := parseUsagePayload(body)
	require.NoError(t, err)
	require.Equal(t, "pro", report.PlanType)
	require.Len(t, report.Snapshots, 2)

	main := report.PreferredSnapshot()
	require.NotNil(t, main)
	require.Equal(t, "codex", main.LimitID)
	require.NotNil(t, main.Primary)
	require.Equal(t, int64(300), main.Primary.WindowMinutes)
	require.NotNil(t, main.Secondary)
	require.Equal(t, int64(10080), main.Secondary.WindowMinutes)
	require.NotNil(t, main.Credits)
	require.Equal(t, "9.99", main.Credits.Balance)
}

func TestFormatStatusSummary(t *testing.T) {
	t.Parallel()

	now := time.Unix(2000000000, 0)
	report := &UsageReport{
		Snapshots: []Snapshot{
			{
				LimitID: "codex",
				Primary: &Window{
					UsedPercent:   12,
					WindowMinutes: 300,
					ResetsAtUnix:  now.Add(5 * time.Minute).Unix(),
				},
				Secondary: &Window{
					UsedPercent:   34,
					WindowMinutes: 10080,
					ResetsAtUnix:  now.Add(24 * time.Hour).Unix(),
				},
			},
		},
	}

	summary := FormatStatusSummary(report, now)
	require.Contains(t, summary, "Codex limits:")
	require.Contains(t, summary, "5h 12%")
	require.Contains(t, summary, "weekly 34%")
}
