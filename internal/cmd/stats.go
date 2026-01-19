package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/spf13/cobra"
)

func init() {
	statsCmd.Flags().Bool("json", false, "Output stats as JSON instead of opening HTML")
	statsCmd.Flags().Bool("no-open", false, "Generate HTML but don't open it in browser")
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show usage statistics",
	Long:  "Generate and display usage statistics including token usage, costs, and activity patterns",
	RunE:  runStats,
}

// Stats holds all the statistics data.
type Stats struct {
	GeneratedAt       time.Time          `json:"generated_at"`
	Total             TotalStats         `json:"total"`
	UsageByDay        []DailyUsage       `json:"usage_by_day"`
	UsageByModel      []ModelUsage       `json:"usage_by_model"`
	UsageByHour       []HourlyUsage      `json:"usage_by_hour"`
	UsageByDayOfWeek  []DayOfWeekUsage   `json:"usage_by_day_of_week"`
	RecentActivity    []DailyActivity    `json:"recent_activity"`
	AvgResponseTimeMs float64            `json:"avg_response_time_ms"`
	ToolUsage         []ToolUsage        `json:"tool_usage"`
	HourDayHeatmap    []HourDayHeatmapPt `json:"hour_day_heatmap"`
}

type TotalStats struct {
	TotalSessions         int64   `json:"total_sessions"`
	TotalPromptTokens     int64   `json:"total_prompt_tokens"`
	TotalCompletionTokens int64   `json:"total_completion_tokens"`
	TotalTokens           int64   `json:"total_tokens"`
	TotalCost             float64 `json:"total_cost"`
	TotalMessages         int64   `json:"total_messages"`
	AvgTokensPerSession   float64 `json:"avg_tokens_per_session"`
	AvgMessagesPerSession float64 `json:"avg_messages_per_session"`
}

type DailyUsage struct {
	Day              string  `json:"day"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	SessionCount     int64   `json:"session_count"`
}

type ModelUsage struct {
	Model        string `json:"model"`
	Provider     string `json:"provider"`
	MessageCount int64  `json:"message_count"`
}

type HourlyUsage struct {
	Hour         int   `json:"hour"`
	SessionCount int64 `json:"session_count"`
}

type DayOfWeekUsage struct {
	DayOfWeek        int    `json:"day_of_week"`
	DayName          string `json:"day_name"`
	SessionCount     int64  `json:"session_count"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

type DailyActivity struct {
	Day          string  `json:"day"`
	SessionCount int64   `json:"session_count"`
	TotalTokens  int64   `json:"total_tokens"`
	Cost         float64 `json:"cost"`
}

type ToolUsage struct {
	ToolName  string `json:"tool_name"`
	CallCount int64  `json:"call_count"`
}

type HourDayHeatmapPt struct {
	DayOfWeek    int   `json:"day_of_week"`
	Hour         int   `json:"hour"`
	SessionCount int64 `json:"session_count"`
}

func runStats(cmd *cobra.Command, _ []string) error {
	outputJSON, _ := cmd.Flags().GetBool("json")
	noOpen, _ := cmd.Flags().GetBool("no-open")
	dataDir, _ := cmd.Flags().GetString("data-dir")
	ctx := cmd.Context()

	if dataDir == "" {
		cfg, err := config.Init("", "", false)
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}
		dataDir = cfg.Options.DataDirectory
	}

	conn, err := db.Connect(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

	stats, err := gatherStats(ctx, conn)
	if err != nil {
		return fmt.Errorf("failed to gather stats: %w", err)
	}

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}

	htmlPath := filepath.Join(dataDir, "stats.html")
	if err := generateHTML(stats, htmlPath); err != nil {
		return fmt.Errorf("failed to generate HTML: %w", err)
	}

	fmt.Printf("Stats generated: %s\n", htmlPath)

	if !noOpen {
		if err := openBrowser(htmlPath); err != nil {
			fmt.Printf("Could not open browser: %v\n", err)
			fmt.Println("Please open the file manually.")
		}
	}

	return nil
}

func gatherStats(ctx context.Context, conn *sql.DB) (*Stats, error) {
	queries := db.New(conn)

	stats := &Stats{
		GeneratedAt: time.Now(),
	}

	// Total stats.
	total, err := queries.GetTotalStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("get total stats: %w", err)
	}
	stats.Total = TotalStats{
		TotalSessions:         total.TotalSessions,
		TotalPromptTokens:     toInt64(total.TotalPromptTokens),
		TotalCompletionTokens: toInt64(total.TotalCompletionTokens),
		TotalTokens:           toInt64(total.TotalPromptTokens) + toInt64(total.TotalCompletionTokens),
		TotalCost:             toFloat64(total.TotalCost),
		TotalMessages:         toInt64(total.TotalMessages),
		AvgTokensPerSession:   toFloat64(total.AvgTokensPerSession),
		AvgMessagesPerSession: toFloat64(total.AvgMessagesPerSession),
	}

	// Usage by day.
	dailyUsage, err := queries.GetUsageByDay(ctx)
	if err != nil {
		return nil, fmt.Errorf("get usage by day: %w", err)
	}
	for _, d := range dailyUsage {
		prompt := nullFloat64ToInt64(d.PromptTokens)
		completion := nullFloat64ToInt64(d.CompletionTokens)
		stats.UsageByDay = append(stats.UsageByDay, DailyUsage{
			Day:              fmt.Sprintf("%v", d.Day),
			PromptTokens:     prompt,
			CompletionTokens: completion,
			TotalTokens:      prompt + completion,
			Cost:             d.Cost.Float64,
			SessionCount:     d.SessionCount,
		})
	}

	// Usage by model.
	modelUsage, err := queries.GetUsageByModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("get usage by model: %w", err)
	}
	for _, m := range modelUsage {
		stats.UsageByModel = append(stats.UsageByModel, ModelUsage{
			Model:        m.Model,
			Provider:     m.Provider,
			MessageCount: m.MessageCount,
		})
	}

	// Usage by hour.
	hourlyUsage, err := queries.GetUsageByHour(ctx)
	if err != nil {
		return nil, fmt.Errorf("get usage by hour: %w", err)
	}
	for _, h := range hourlyUsage {
		stats.UsageByHour = append(stats.UsageByHour, HourlyUsage{
			Hour:         int(h.Hour),
			SessionCount: h.SessionCount,
		})
	}

	// Usage by day of week.
	dowUsage, err := queries.GetUsageByDayOfWeek(ctx)
	if err != nil {
		return nil, fmt.Errorf("get usage by day of week: %w", err)
	}
	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	for _, d := range dowUsage {
		stats.UsageByDayOfWeek = append(stats.UsageByDayOfWeek, DayOfWeekUsage{
			DayOfWeek:        int(d.DayOfWeek),
			DayName:          dayNames[d.DayOfWeek],
			SessionCount:     d.SessionCount,
			PromptTokens:     nullFloat64ToInt64(d.PromptTokens),
			CompletionTokens: nullFloat64ToInt64(d.CompletionTokens),
		})
	}

	// Recent activity (last 30 days).
	recent, err := queries.GetRecentActivity(ctx)
	if err != nil {
		return nil, fmt.Errorf("get recent activity: %w", err)
	}
	for _, r := range recent {
		stats.RecentActivity = append(stats.RecentActivity, DailyActivity{
			Day:          fmt.Sprintf("%v", r.Day),
			SessionCount: r.SessionCount,
			TotalTokens:  nullFloat64ToInt64(r.TotalTokens),
			Cost:         r.Cost.Float64,
		})
	}

	// Average response time.
	avgResp, err := queries.GetAverageResponseTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("get average response time: %w", err)
	}
	stats.AvgResponseTimeMs = toFloat64(avgResp) * 1000

	// Tool usage.
	toolUsage, err := queries.GetToolUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("get tool usage: %w", err)
	}
	for _, t := range toolUsage {
		if name, ok := t.ToolName.(string); ok && name != "" {
			stats.ToolUsage = append(stats.ToolUsage, ToolUsage{
				ToolName:  name,
				CallCount: t.CallCount,
			})
		}
	}

	// Hour/day heatmap.
	heatmap, err := queries.GetHourDayHeatmap(ctx)
	if err != nil {
		return nil, fmt.Errorf("get hour day heatmap: %w", err)
	}
	for _, h := range heatmap {
		stats.HourDayHeatmap = append(stats.HourDayHeatmap, HourDayHeatmapPt{
			DayOfWeek:    int(h.DayOfWeek),
			Hour:         int(h.Hour),
			SessionCount: h.SessionCount,
		})
	}

	return stats, nil
}

func toInt64(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	case int:
		return int64(val)
	default:
		return 0
	}
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	default:
		return 0
	}
}

func nullFloat64ToInt64(n sql.NullFloat64) int64 {
	if n.Valid {
		return int64(n.Float64)
	}
	return 0
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func generateHTML(stats *Stats, path string) error {
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Crush Usage Statistics</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        :root {
            --bg: #1a1b26;
            --bg-secondary: #24283b;
            --text: #c0caf5;
            --text-muted: #565f89;
            --accent: #7aa2f7;
            --accent2: #bb9af7;
            --accent3: #7dcfff;
            --accent4: #9ece6a;
            --border: #414868;
        }
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            padding: 2rem;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        h1 {
            font-size: 2.5rem;
            margin-bottom: 0.5rem;
            color: var(--accent);
        }
        .subtitle {
            color: var(--text-muted);
            margin-bottom: 2rem;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 1.5rem;
        }
        .stat-card h3 {
            font-size: 0.875rem;
            color: var(--text-muted);
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 0.5rem;
        }
        .stat-card .value {
            font-size: 2rem;
            font-weight: 700;
            color: var(--accent);
        }
        .stat-card .value.cost {
            color: var(--accent4);
        }
        .charts-grid {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 1.5rem;
            margin-bottom: 2rem;
        }
        @media (max-width: 1024px) {
            .charts-grid {
                grid-template-columns: 1fr;
            }
        }
        .chart-card {
            background: var(--bg-secondary);
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 1.5rem;
        }
        .chart-card.full-width {
            grid-column: 1 / -1;
        }
        .chart-card h2 {
            font-size: 1.25rem;
            margin-bottom: 1rem;
            color: var(--text);
        }
        .chart-container {
            position: relative;
            height: 300px;
        }
        .chart-container.tall {
            height: 400px;
        }
        table {
            width: 100%%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        th, td {
            text-align: left;
            padding: 0.75rem;
            border-bottom: 1px solid var(--border);
        }
        th {
            color: var(--text-muted);
            font-weight: 500;
            font-size: 0.875rem;
        }
        td {
            font-family: 'SF Mono', Consolas, monospace;
        }
        .model-tag {
            background: var(--bg);
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.875rem;
        }
        .footer {
            text-align: center;
            color: var(--text-muted);
            margin-top: 2rem;
            padding-top: 2rem;
            border-top: 1px solid var(--border);
        }

    </style>
</head>
<body>
    <div class="container">
        <h1>Crush Usage Statistics</h1>
        <p class="subtitle">Generated on <span id="generated-at"></span></p>

        <div class="stats-grid">
            <div class="stat-card">
                <h3>Total Sessions</h3>
                <div class="value" id="total-sessions"></div>
            </div>
            <div class="stat-card">
                <h3>Total Messages</h3>
                <div class="value" id="total-messages"></div>
            </div>
            <div class="stat-card">
                <h3>Total Tokens</h3>
                <div class="value" id="total-tokens"></div>
            </div>
            <div class="stat-card">
                <h3>Total Cost</h3>
                <div class="value cost" id="total-cost"></div>
            </div>
            <div class="stat-card">
                <h3>Avg Tokens/Session</h3>
                <div class="value" id="avg-tokens"></div>
            </div>
            <div class="stat-card">
                <h3>Avg Response Time</h3>
                <div class="value" id="avg-response"></div>
            </div>
        </div>

        <div class="charts-grid">
            <div class="chart-card full-width">
                <h2>Activity (Last 30 Days)</h2>
                <div class="chart-container tall">
                    <canvas id="recentActivityChart"></canvas>
                </div>
            </div>

            <div class="chart-card full-width">
                <h2>Activity Heatmap (Hour × Day of Week)</h2>
                <div class="chart-container">
                    <canvas id="heatmapChart"></canvas>
                </div>
            </div>

            <div class="chart-card">
                <h2>Tool Usage</h2>
                <div class="chart-container">
                    <canvas id="toolChart"></canvas>
                </div>
            </div>

            <div class="chart-card">
                <h2>Token Distribution</h2>
                <div class="chart-container">
                    <canvas id="tokenPieChart"></canvas>
                </div>
            </div>

            <div class="chart-card full-width">
                <h2>Usage by Model</h2>
                <div class="chart-container tall">
                    <canvas id="modelChart"></canvas>
                </div>
            </div>
        </div>

        <div class="chart-card">
            <h2>Daily Usage History</h2>
            <table id="daily-table">
                <thead>
                    <tr>
                        <th>Date</th>
                        <th>Sessions</th>
                        <th>Prompt Tokens</th>
                        <th>Completion Tokens</th>
                        <th>Total Tokens</th>
                        <th>Cost</th>
                    </tr>
                </thead>
                <tbody></tbody>
            </table>
        </div>

        <div class="footer">
            <p>Generated by Crush</p>
        </div>
    </div>

    <script>
        const stats = %s;

        // Helper functions
        function formatNumber(n) {
            return new Intl.NumberFormat().format(Math.round(n));
        }

        function formatCost(n) {
            return '$' + n.toFixed(4);
        }

        function formatTime(ms) {
            if (ms < 1000) return Math.round(ms) + 'ms';
            return (ms / 1000).toFixed(1) + 's';
        }

        // Populate summary cards
        document.getElementById('generated-at').textContent = new Date(stats.generated_at).toLocaleString();
        document.getElementById('total-sessions').textContent = formatNumber(stats.total.total_sessions);
        document.getElementById('total-messages').textContent = formatNumber(stats.total.total_messages);
        document.getElementById('total-tokens').textContent = formatNumber(stats.total.total_tokens);
        document.getElementById('total-cost').textContent = formatCost(stats.total.total_cost);
        document.getElementById('avg-tokens').textContent = formatNumber(stats.total.avg_tokens_per_session);
        document.getElementById('avg-response').textContent = formatTime(stats.avg_response_time_ms);

        // Chart defaults
        Chart.defaults.color = '#c0caf5';
        Chart.defaults.borderColor = '#414868';

        // Recent Activity Chart
        if (stats.recent_activity && stats.recent_activity.length > 0) {
            new Chart(document.getElementById('recentActivityChart'), {
                type: 'bar',
                data: {
                    labels: stats.recent_activity.map(d => d.day),
                    datasets: [{
                        label: 'Sessions',
                        data: stats.recent_activity.map(d => d.session_count),
                        backgroundColor: '#7aa2f7',
                        borderRadius: 4,
                        yAxisID: 'y'
                    }, {
                        label: 'Tokens (K)',
                        data: stats.recent_activity.map(d => d.total_tokens / 1000),
                        type: 'line',
                        borderColor: '#bb9af7',
                        backgroundColor: 'transparent',
                        tension: 0.3,
                        yAxisID: 'y1'
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    interaction: { mode: 'index', intersect: false },
                    scales: {
                        y: { position: 'left', title: { display: true, text: 'Sessions' } },
                        y1: { position: 'right', title: { display: true, text: 'Tokens (K)' }, grid: { drawOnChartArea: false } }
                    }
                }
            });
        }

        // Heatmap (Hour × Day of Week) - Bubble Chart
        const dayLabels = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
        
        if (stats.hour_day_heatmap && stats.hour_day_heatmap.length > 0) {
            let maxCount = Math.max(...stats.hour_day_heatmap.map(h => h.session_count));
            if (maxCount === 0) maxCount = 1;
            const scaleFactor = 20 / Math.sqrt(maxCount);
            
            new Chart(document.getElementById('heatmapChart'), {
                type: 'bubble',
                data: {
                    datasets: [{
                        label: 'Sessions',
                        data: stats.hour_day_heatmap.filter(h => h.session_count > 0).map(h => ({
                            x: h.hour,
                            y: h.day_of_week,
                            r: Math.sqrt(h.session_count) * scaleFactor,
                            count: h.session_count
                        })),
                        backgroundColor: 'rgba(122, 162, 247, 0.6)',
                        borderColor: 'rgba(122, 162, 247, 1)',
                        borderWidth: 1
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        x: {
                            min: 0,
                            max: 23,
                            grid: { display: false },
                            title: { display: true, text: 'Hour of Day' },
                            ticks: { stepSize: 1, callback: v => Number.isInteger(v) ? v : '' }
                        },
                        y: {
                            min: 0,
                            max: 6,
                            reverse: true,
                            grid: { display: false },
                            title: { display: true, text: 'Day of Week' },
                            ticks: { stepSize: 1, callback: v => dayLabels[v] || '' }
                        }
                    },
                    plugins: {
                        legend: { display: false },
                        tooltip: {
                            callbacks: {
                                label: ctx => dayLabels[ctx.raw.y] + ' ' + ctx.raw.x + ':00 - ' + ctx.raw.count + ' sessions'
                            }
                        }
                    }
                }
            });
        }

        // Tool Usage Chart
        if (stats.tool_usage && stats.tool_usage.length > 0) {
            const toolColors = ['#7aa2f7', '#bb9af7', '#7dcfff', '#9ece6a', '#f7768e', '#e0af68', '#73daca', '#ff9e64', '#c0caf5', '#565f89'];
            new Chart(document.getElementById('toolChart'), {
                type: 'bar',
                data: {
                    labels: stats.tool_usage.slice(0, 15).map(t => t.tool_name),
                    datasets: [{
                        label: 'Calls',
                        data: stats.tool_usage.slice(0, 15).map(t => t.call_count),
                        backgroundColor: toolColors,
                        borderRadius: 4
                    }]
                },
                options: {
                    indexAxis: 'y',
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false } }
                }
            });
        }

        // Token Distribution Pie
        new Chart(document.getElementById('tokenPieChart'), {
            type: 'doughnut',
            data: {
                labels: ['Prompt Tokens', 'Completion Tokens'],
                datasets: [{
                    data: [stats.total.total_prompt_tokens, stats.total.total_completion_tokens],
                    backgroundColor: ['#7aa2f7', '#bb9af7']
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { position: 'bottom' }
                }
            }
        });

        // Model Usage Chart (horizontal bar)
        if (stats.usage_by_model && stats.usage_by_model.length > 0) {
            const modelColors = ['#7aa2f7', '#bb9af7', '#7dcfff', '#9ece6a', '#f7768e', '#e0af68', '#73daca', '#ff9e64', '#c0caf5', '#565f89'];
            new Chart(document.getElementById('modelChart'), {
                type: 'bar',
                data: {
                    labels: stats.usage_by_model.map(m => m.model + ' (' + m.provider + ')'),
                    datasets: [{
                        label: 'Messages',
                        data: stats.usage_by_model.map(m => m.message_count),
                        backgroundColor: modelColors.slice(0, stats.usage_by_model.length),
                        borderRadius: 4
                    }]
                },
                options: {
                    indexAxis: 'y',
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: { legend: { display: false } }
                }
            });
        }

        // Daily Usage Table
        const tableBody = document.querySelector('#daily-table tbody');
        stats.usage_by_day?.slice(0, 30).forEach(d => {
            const row = document.createElement('tr');
            row.innerHTML = '<td>' + d.day + '</td>' +
                '<td>' + d.session_count + '</td>' +
                '<td>' + formatNumber(d.prompt_tokens) + '</td>' +
                '<td>' + formatNumber(d.completion_tokens) + '</td>' +
                '<td>' + formatNumber(d.total_tokens) + '</td>' +
                '<td>' + formatCost(d.cost) + '</td>';
            tableBody.appendChild(row);
        });
    </script>
</body>
</html>
`, string(statsJSON))

	return os.WriteFile(path, []byte(html), 0o644)
}
