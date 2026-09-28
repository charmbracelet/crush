package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeStatsOrdersTimeSeries(t *testing.T) {
	days := []string{"2026-01-08", "2026-01-01", "2026-01-05", "2026-01-03", "2026-01-07", "2026-01-02", "2026-01-06", "2026-01-04"}
	hours := []int{17, 2, 23, 0, 11, 5, 8, 14}
	weekdays := []int{6, 0, 3, 1, 5, 2, 4}
	heat := []HourDayHeatmapPt{
		{DayOfWeek: 1, Hour: 10},
		{DayOfWeek: 0, Hour: 5},
		{DayOfWeek: 1, Hour: 2},
		{DayOfWeek: 0, Hour: 20},
	}

	projects := make([]ProjectStats, 0, len(days))
	for i, day := range days {
		projects = append(projects, ProjectStats{
			ProjectPath: day,
			Stats: &Stats{
				UsageByDay:       []DailyUsage{{Day: day}},
				RecentActivity:   []DailyActivity{{Day: day}},
				UsageByHour:      []HourlyUsage{{Hour: hours[i]}},
				UsageByDayOfWeek: []DayOfWeekUsage{{DayOfWeek: weekdays[i%len(weekdays)]}},
				HourDayHeatmap:   heat,
			},
		})
	}

	for range 10 {
		merged := mergeStats(projects)
		for i := 1; i < len(merged.UsageByDay); i++ {
			require.GreaterOrEqual(t, merged.UsageByDay[i-1].Day, merged.UsageByDay[i].Day)
		}
		for i := 1; i < len(merged.RecentActivity); i++ {
			require.LessOrEqual(t, merged.RecentActivity[i-1].Day, merged.RecentActivity[i].Day)
		}
		for i := 1; i < len(merged.UsageByHour); i++ {
			require.LessOrEqual(t, merged.UsageByHour[i-1].Hour, merged.UsageByHour[i].Hour)
		}
		for i := 1; i < len(merged.UsageByDayOfWeek); i++ {
			require.LessOrEqual(t, merged.UsageByDayOfWeek[i-1].DayOfWeek, merged.UsageByDayOfWeek[i].DayOfWeek)
		}
		for i := 1; i < len(merged.HourDayHeatmap); i++ {
			prev, next := merged.HourDayHeatmap[i-1], merged.HourDayHeatmap[i]
			if prev.DayOfWeek == next.DayOfWeek {
				require.LessOrEqual(t, prev.Hour, next.Hour)
				continue
			}
			require.Less(t, prev.DayOfWeek, next.DayOfWeek)
		}
	}
}
