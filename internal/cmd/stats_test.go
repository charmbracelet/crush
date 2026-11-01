package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeStatsOrdersTimeSeries(t *testing.T) {
	t.Parallel()

	projects := []ProjectStats{
		{
			ProjectPath: "/proj-a",
			Stats: &Stats{
				UsageByDay: []DailyUsage{
					{Day: "2026-01-04", SessionCount: 1},
					{Day: "2026-01-08", SessionCount: 2},
					{Day: "2026-01-01", SessionCount: 3},
				},
				UsageByHour: []HourlyUsage{
					{Hour: 14, SessionCount: 1},
					{Hour: 9, SessionCount: 2},
					{Hour: 22, SessionCount: 3},
					{Hour: 5, SessionCount: 4},
				},
				UsageByDayOfWeek: []DayOfWeekUsage{
					{DayOfWeek: 3, DayName: "Wednesday", SessionCount: 1},
					{DayOfWeek: 0, DayName: "Sunday", SessionCount: 2},
					{DayOfWeek: 6, DayName: "Saturday", SessionCount: 3},
				},
				RecentActivity: []DailyActivity{
					{Day: "2026-01-08", SessionCount: 5},
					{Day: "2026-01-04", SessionCount: 3},
					{Day: "2026-01-01", SessionCount: 1},
				},
				HourDayHeatmap: []HourDayHeatmapPt{
					{DayOfWeek: 3, Hour: 14, SessionCount: 1},
					{DayOfWeek: 0, Hour: 9, SessionCount: 2},
					{DayOfWeek: 3, Hour: 5, SessionCount: 3},
					{DayOfWeek: 0, Hour: 22, SessionCount: 4},
				},
			},
		},
		{
			ProjectPath: "/proj-b",
			Stats: &Stats{
				UsageByDay: []DailyUsage{
					{Day: "2026-01-06", SessionCount: 4},
					{Day: "2026-01-02", SessionCount: 1},
				},
				UsageByHour: []HourlyUsage{
					{Hour: 11, SessionCount: 5},
					{Hour: 9, SessionCount: 1},
				},
				UsageByDayOfWeek: []DayOfWeekUsage{
					{DayOfWeek: 5, DayName: "Friday", SessionCount: 2},
					{DayOfWeek: 1, DayName: "Monday", SessionCount: 1},
				},
				RecentActivity: []DailyActivity{
					{Day: "2026-01-06", SessionCount: 2},
					{Day: "2026-01-02", SessionCount: 1},
				},
				HourDayHeatmap: []HourDayHeatmapPt{
					{DayOfWeek: 5, Hour: 11, SessionCount: 2},
					{DayOfWeek: 1, Hour: 11, SessionCount: 1},
				},
			},
		},
	}

	merged := mergeStats(projects)

	// UsageByDay: newest first.
	require.Len(t, merged.UsageByDay, 5)
	require.Equal(t, "2026-01-08", merged.UsageByDay[0].Day)
	require.Equal(t, "2026-01-06", merged.UsageByDay[1].Day)
	require.Equal(t, "2026-01-04", merged.UsageByDay[2].Day)
	require.Equal(t, "2026-01-02", merged.UsageByDay[3].Day)
	require.Equal(t, "2026-01-01", merged.UsageByDay[4].Day)

	// RecentActivity: oldest first.
	require.Len(t, merged.RecentActivity, 5)
	require.Equal(t, "2026-01-01", merged.RecentActivity[0].Day)
	require.Equal(t, "2026-01-02", merged.RecentActivity[1].Day)
	require.Equal(t, "2026-01-04", merged.RecentActivity[2].Day)
	require.Equal(t, "2026-01-06", merged.RecentActivity[3].Day)
	require.Equal(t, "2026-01-08", merged.RecentActivity[4].Day)

	// UsageByHour: ascending by hour.
	require.Len(t, merged.UsageByHour, 5)
	require.Equal(t, 5, merged.UsageByHour[0].Hour)
	require.Equal(t, 9, merged.UsageByHour[1].Hour)
	require.Equal(t, 11, merged.UsageByHour[2].Hour)
	require.Equal(t, 14, merged.UsageByHour[3].Hour)
	require.Equal(t, 22, merged.UsageByHour[4].Hour)

	// UsageByDayOfWeek: ascending by day of week (Sun=0 .. Sat=6).
	require.Len(t, merged.UsageByDayOfWeek, 5)
	require.Equal(t, 0, merged.UsageByDayOfWeek[0].DayOfWeek) // Sunday
	require.Equal(t, 1, merged.UsageByDayOfWeek[1].DayOfWeek) // Monday
	require.Equal(t, 3, merged.UsageByDayOfWeek[2].DayOfWeek) // Wednesday
	require.Equal(t, 5, merged.UsageByDayOfWeek[3].DayOfWeek) // Friday
	require.Equal(t, 6, merged.UsageByDayOfWeek[4].DayOfWeek) // Saturday

	// HourDayHeatmap: ascending by (day_of_week, hour).
	require.Len(t, merged.HourDayHeatmap, 6)
	require.Equal(t, 0, merged.HourDayHeatmap[0].DayOfWeek)
	require.Equal(t, 9, merged.HourDayHeatmap[0].Hour)
	require.Equal(t, 0, merged.HourDayHeatmap[1].DayOfWeek)
	require.Equal(t, 22, merged.HourDayHeatmap[1].Hour)
	require.Equal(t, 1, merged.HourDayHeatmap[2].DayOfWeek)
	require.Equal(t, 11, merged.HourDayHeatmap[2].Hour)
	require.Equal(t, 3, merged.HourDayHeatmap[3].DayOfWeek)
	require.Equal(t, 5, merged.HourDayHeatmap[3].Hour)
	require.Equal(t, 3, merged.HourDayHeatmap[4].DayOfWeek)
	require.Equal(t, 14, merged.HourDayHeatmap[4].Hour)
	require.Equal(t, 5, merged.HourDayHeatmap[5].DayOfWeek)
	require.Equal(t, 11, merged.HourDayHeatmap[5].Hour)
}
