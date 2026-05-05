package cmd

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/projects"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

//go:embed stats/index.html
var statsTemplate string

//go:embed stats/index.css
var statsCSS string

//go:embed stats/index.js
var statsJS string

//go:embed stats/header.svg
var headerSVG string

//go:embed stats/heartbit.svg
var heartbitSVG string

//go:embed stats/footer.svg
var footerSVG string

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show usage statistics",
	Long:  "Generate and display usage statistics including token usage, costs, and activity patterns",
	RunE:  runStats,
}

func init() {
	statsCmd.Flags().String("crawl-dir", "", "Crawl a directory recursively for all crush projects and aggregate stats")
	statsCmd.Flags().Bool("all", false, "Aggregate stats from all known projects (from projects.json)")
}

// Day names for day of week statistics.
var dayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

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

// ProjectStats associates stats with a project path.
type ProjectStats struct {
	ProjectPath string `json:"project_path"`
	Stats       *Stats `json:"stats"`
}

func runStats(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	dataDir, _ := cmd.Flags().GetString("data-dir")
	crawlDir, _ := cmd.Flags().GetString("crawl-dir")
	useAll, _ := cmd.Flags().GetBool("all")

	var projectStats []ProjectStats
	var err error

	switch {
	case crawlDir != "":
		projectStats, err = crawlForStats(ctx, crawlDir)
		if err != nil {
			return fmt.Errorf("failed to crawl for stats: %w", err)
		}
	case useAll:
		projectStats, err = gatherStatsFromProjects(ctx)
		if err != nil {
			return fmt.Errorf("failed to gather stats from projects: %w", err)
		}
	default:
		cfg, err := config.Init("", dataDir, false)
		if err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}
		if dataDir == "" {
			dataDir = cfg.Config().Options.DataDirectory
		}
		if shouldEnableMetrics(cfg.Config()) {
			event.Init()
		}

		event.StatsViewed()

		conn, err := db.Connect(ctx, dataDir)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer conn.Close()

		stats, err := gatherStats(ctx, conn)
		if err != nil {
			return fmt.Errorf("failed to gather stats: %w", err)
		}

		projectStats = []ProjectStats{{ProjectPath: "", Stats: stats}}
	}

	if len(projectStats) == 0 {
		return fmt.Errorf("no data available: no projects found")
	}

	// Merge stats from all projects.
	mergedStats := mergeStats(projectStats)

	if mergedStats.Total.TotalSessions == 0 {
		return fmt.Errorf("no data available: no sessions found in database")
	}

	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	username := currentUser.Username

	var projName string
	switch {
	case crawlDir != "":
		projName = crawlDir
	case useAll:
		projName = "all projects"
	default:
		project, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		projName = strings.Replace(project, currentUser.HomeDir, "~", 1)
	}

	outputDataDir := dataDir
	if outputDataDir == "" {
		cfg, err := config.Init("", "", false)
		if err == nil {
			outputDataDir = cfg.Config().Options.DataDirectory
		}
	}
	if outputDataDir == "" {
		outputDataDir = ".crush"
	}

	htmlPath := filepath.Join(outputDataDir, "stats/index.html")
	if err := generateHTML(mergedStats, projectStats, projName, username, htmlPath); err != nil {
		return fmt.Errorf("failed to generate HTML: %w", err)
	}

	fmt.Printf("Stats generated: %s\n", htmlPath)

	if err := browser.OpenFile(htmlPath); err != nil {
		fmt.Printf("Could not open browser: %v\n", err)
		fmt.Println("Please open the file manually.")
	}

	return nil
}

// crawlForStats crawls a directory recursively looking for .crush/crush.db files.
func crawlForStats(ctx context.Context, rootDir string) ([]ProjectStats, error) {
	var dbPaths []struct {
		dbPath     string
		projectDir string
	}

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors (permission denied, etc.)
		}

		// Skip common ignored directories early
		if d.IsDir() && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}

		// Look for .crush/crush.db pattern
		if !d.IsDir() && d.Name() == "crush.db" {
			dir := filepath.Dir(path)
			if filepath.Base(dir) == ".crush" {
				projectDir := filepath.Dir(dir)
				dbPaths = append(dbPaths, struct {
					dbPath     string
					projectDir string
				}{dbPath: path, projectDir: projectDir})
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return gatherStatsFromDBPaths(ctx, dbPaths)
}

// shouldSkipDir returns true for directories that should be skipped during crawling.
func shouldSkipDir(name string) bool {
	skipDirs := map[string]bool{
		".git":             true,
		".svn":             true,
		".hg":              true,
		"node_modules":     true,
		"vendor":           true,
		"dist":             true,
		"build":            true,
		"target":           true,
		".idea":            true,
		".vscode":          true,
		"__pycache__":      true,
		"bin":              true,
		"obj":              true,
		"out":              true,
		"coverage":         true,
		"logs":             true,
		"generated":        true,
		"bower_components": true,
		"jspm_packages":    true,
		".cache":           true,
		".npm":             true,
		".cargo":           true,
		"Library":          true,
		"Applications":     true,
		"System":           true,
	}
	return skipDirs[name]
}

// gatherStatsFromProjects gathers stats from all known projects in projects.json.
func gatherStatsFromProjects(ctx context.Context) ([]ProjectStats, error) {
	projectList, err := projects.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load projects: %w", err)
	}

	var dbPaths []struct {
		dbPath     string
		projectDir string
	}

	for _, p := range projectList.Projects {
		dbPath := filepath.Join(p.DataDir, "crush.db")
		if _, err := os.Stat(dbPath); err == nil {
			dbPaths = append(dbPaths, struct {
				dbPath     string
				projectDir string
			}{dbPath: dbPath, projectDir: p.Path})
		}
	}

	return gatherStatsFromDBPaths(ctx, dbPaths)
}

// gatherStatsFromDBPaths gathers stats from a list of database paths in parallel.
func gatherStatsFromDBPaths(ctx context.Context, dbPaths []struct {
	dbPath     string
	projectDir string
},
) ([]ProjectStats, error) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []ProjectStats
	)

	sem := make(chan struct{}, 10) // Limit concurrent DB connections

	for _, item := range dbPaths {
		wg.Add(1)
		go func(dbPath, projectDir string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			conn, err := db.ConnectReadOnly(ctx, dbPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: skipping %s: %v\n", dbPath, err)
				return
			}
			defer conn.Close()

			stats, err := gatherStats(ctx, conn)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: failed to gather stats from %s: %v\n", dbPath, err)
				return
			}

			mu.Lock()
			results = append(results, ProjectStats{
				ProjectPath: projectDir,
				Stats:       stats,
			})
			mu.Unlock()
		}(item.dbPath, item.projectDir)
	}

	wg.Wait()

	return results, nil
}

// mergeStats combines stats from multiple projects into a single Stats struct.
func mergeStats(projectStats []ProjectStats) *Stats {
	merged := &Stats{
		GeneratedAt: time.Now().UTC(),
	}

	// Maps for aggregating unique entries.
	dailyUsageMap := make(map[string]DailyUsage)
	modelUsageMap := make(map[string]ModelUsage)
	hourlyUsageMap := make(map[int]HourlyUsage)
	dayOfWeekMap := make(map[int]DayOfWeekUsage)
	recentActivityMap := make(map[string]DailyActivity)
	toolUsageMap := make(map[string]ToolUsage)
	heatmapMap := make(map[string]HourDayHeatmapPt) // key: "day-hour"

	var totalResponseTimeMs float64
	var responseTimeCount int64

	for _, ps := range projectStats {
		s := ps.Stats

		// Aggregate total stats.
		merged.Total.TotalSessions += s.Total.TotalSessions
		merged.Total.TotalPromptTokens += s.Total.TotalPromptTokens
		merged.Total.TotalCompletionTokens += s.Total.TotalCompletionTokens
		merged.Total.TotalTokens += s.Total.TotalTokens
		merged.Total.TotalCost += s.Total.TotalCost
		merged.Total.TotalMessages += s.Total.TotalMessages

		// Aggregate daily usage.
		for _, d := range s.UsageByDay {
			existing := dailyUsageMap[d.Day]
			existing.Day = d.Day
			existing.PromptTokens += d.PromptTokens
			existing.CompletionTokens += d.CompletionTokens
			existing.TotalTokens += d.TotalTokens
			existing.Cost += d.Cost
			existing.SessionCount += d.SessionCount
			dailyUsageMap[d.Day] = existing
		}

		// Aggregate model usage.
		for _, m := range s.UsageByModel {
			key := m.Model + "|" + m.Provider
			existing := modelUsageMap[key]
			existing.Model = m.Model
			existing.Provider = m.Provider
			existing.MessageCount += m.MessageCount
			modelUsageMap[key] = existing
		}

		// Aggregate hourly usage.
		for _, h := range s.UsageByHour {
			existing := hourlyUsageMap[h.Hour]
			existing.Hour = h.Hour
			existing.SessionCount += h.SessionCount
			hourlyUsageMap[h.Hour] = existing
		}

		// Aggregate day of week usage.
		for _, d := range s.UsageByDayOfWeek {
			existing := dayOfWeekMap[d.DayOfWeek]
			existing.DayOfWeek = d.DayOfWeek
			existing.DayName = d.DayName
			existing.SessionCount += d.SessionCount
			existing.PromptTokens += d.PromptTokens
			existing.CompletionTokens += d.CompletionTokens
			dayOfWeekMap[d.DayOfWeek] = existing
		}

		// Aggregate recent activity.
		for _, r := range s.RecentActivity {
			existing := recentActivityMap[r.Day]
			existing.Day = r.Day
			existing.SessionCount += r.SessionCount
			existing.TotalTokens += r.TotalTokens
			existing.Cost += r.Cost
			recentActivityMap[r.Day] = existing
		}

		// Aggregate tool usage (normalize to lowercase for consistency).
		for _, t := range s.ToolUsage {
			toolName := strings.ToLower(t.ToolName)
			existing := toolUsageMap[toolName]
			existing.ToolName = toolName
			existing.CallCount += t.CallCount
			toolUsageMap[toolName] = existing
		}

		// Aggregate heatmap.
		for _, h := range s.HourDayHeatmap {
			key := fmt.Sprintf("%d-%d", h.DayOfWeek, h.Hour)
			existing := heatmapMap[key]
			existing.DayOfWeek = h.DayOfWeek
			existing.Hour = h.Hour
			existing.SessionCount += h.SessionCount
			heatmapMap[key] = existing
		}

		// Accumulate response time for averaging.
		if s.AvgResponseTimeMs > 0 {
			totalResponseTimeMs += s.AvgResponseTimeMs * float64(s.Total.TotalMessages)
			responseTimeCount += s.Total.TotalMessages
		}
	}

	// Calculate averages.
	if merged.Total.TotalSessions > 0 {
		merged.Total.AvgTokensPerSession = float64(merged.Total.TotalTokens) / float64(merged.Total.TotalSessions)
		merged.Total.AvgMessagesPerSession = float64(merged.Total.TotalMessages) / float64(merged.Total.TotalSessions)
	}

	if responseTimeCount > 0 {
		merged.AvgResponseTimeMs = totalResponseTimeMs / float64(responseTimeCount)
	}

	// Convert maps back to slices.
	for _, d := range dailyUsageMap {
		merged.UsageByDay = append(merged.UsageByDay, d)
	}
	for _, m := range modelUsageMap {
		merged.UsageByModel = append(merged.UsageByModel, m)
	}
	for _, h := range hourlyUsageMap {
		merged.UsageByHour = append(merged.UsageByHour, h)
	}
	for _, d := range dayOfWeekMap {
		merged.UsageByDayOfWeek = append(merged.UsageByDayOfWeek, d)
	}
	for _, r := range recentActivityMap {
		merged.RecentActivity = append(merged.RecentActivity, r)
	}
	for _, t := range toolUsageMap {
		merged.ToolUsage = append(merged.ToolUsage, t)
	}
	for _, h := range heatmapMap {
		merged.HourDayHeatmap = append(merged.HourDayHeatmap, h)
	}

	// Sort slices by count (descending).
	sort.Slice(merged.UsageByModel, func(i, j int) bool {
		return merged.UsageByModel[i].MessageCount > merged.UsageByModel[j].MessageCount
	})
	sort.Slice(merged.ToolUsage, func(i, j int) bool {
		return merged.ToolUsage[i].CallCount > merged.ToolUsage[j].CallCount
	})

	return merged
}

func gatherStats(ctx context.Context, conn *sql.DB) (*Stats, error) {
	queries := db.New(conn)

	stats := &Stats{
		GeneratedAt: time.Now().UTC(),
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
	for _, d := range dowUsage {
		stats.UsageByDayOfWeek = append(stats.UsageByDayOfWeek, DayOfWeekUsage{
			DayOfWeek:        int(d.DayOfWeek),
			DayName:          dayNames[int(d.DayOfWeek)],
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

func generateHTML(stats *Stats, projectStats []ProjectStats, projName, username, path string) error {
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	projectStatsJSON, err := json.Marshal(projectStats)
	if err != nil {
		return err
	}

	tmpl, err := template.New("stats").Parse(statsTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	data := struct {
		StatsJSON        template.JS
		ProjectStatsJSON template.JS
		CSS              template.CSS
		JS               template.JS
		Header           template.HTML
		Heartbit         template.HTML
		Footer           template.HTML
		GeneratedAt      string
		ProjectName      string
		Username         string
	}{
		StatsJSON:        template.JS(statsJSON),
		ProjectStatsJSON: template.JS(projectStatsJSON),
		CSS:              template.CSS(statsCSS),
		JS:               template.JS(statsJS),
		Header:           template.HTML(headerSVG),
		Heartbit:         template.HTML(heartbitSVG),
		Footer:           template.HTML(footerSVG),
		GeneratedAt:      stats.GeneratedAt.Format("2006-01-02"),
		ProjectName:      projName,
		Username:         username,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	return os.WriteFile(path, buf.Bytes(), 0o644)
}
