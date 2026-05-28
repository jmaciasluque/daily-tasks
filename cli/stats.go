package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"daily-tasks/internal"

	"github.com/charmbracelet/lipgloss"
)

// ── Styles ────────────────────────────────────────────────────────────────

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D4AA"))
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	greenStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF66"))
	redStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	yellowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFDD00"))
	orangeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8800"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	upStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF66"))
	downStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444"))
	flatStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	fireStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6600"))
)

// renderStats produces the full enriched stats output.
func renderStats(stats internal.StatsSummary, streaks []internal.TaskStreak, trend internal.WeeklyTrend) string {
	var b strings.Builder

	// Header (newlines go *outside* styled strings to avoid ANSI padding)
	durStr := formatDuration(stats.DoneDuration)
	b.WriteString(headerStyle.Render(" 📊 STATS · " + stats.From + " → " + stats.To))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("    " + fmt.Sprintf("%d days · %d tasks · %s done", stats.RecordedDays, stats.TaskCount, durStr)))
	b.WriteString("\n\n")

	// Overview
	b.WriteString(renderOverview(stats))
	b.WriteString("\n")

	// Weekly trend
	b.WriteString(renderTrend(trend))
	b.WriteString("\n")

	// Heatmap
	if len(stats.Daily) > 0 {
		b.WriteString(renderHeatmap(stats.Daily))
		b.WriteString("\n")
	}

	// Streaks
	if len(streaks) > 0 {
		b.WriteString(renderStreaks(streaks))
		b.WriteString("\n")
	}

	// Weak spots
	if len(stats.Tasks) > 0 {
		b.WriteString(renderWeakSpots(stats.Tasks))
		b.WriteString("\n")
	}

	// Per-task detail
	b.WriteString(renderTaskDetail(stats.Tasks, streaks))

	return b.String()
}

// ── Sections ──────────────────────────────────────────────────────────────

func renderOverview(stats internal.StatsSummary) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("📋 OVERVIEW"))
	b.WriteString("\n")

	rate := stats.CompletionRate * 100

	// Bar
	barW := 20
	filled := int(math.Round(rate / 100.0 * float64(barW)))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)

	rateColor := greenStyle
	if rate < 60 {
		rateColor = redStyle
	} else if rate < 80 {
		rateColor = yellowStyle
	}
	b.WriteString("  " + rateColor.Render(fmt.Sprintf("%.0f%%", rate)) + " ")
	b.WriteString(rateColor.Render(bar) + " ")
	b.WriteString(dimStyle.Render(fmt.Sprintf("(%d%% done)", int(rate))))
	b.WriteString("\n")
	b.WriteString("  " + greenStyle.Render(fmt.Sprintf("✅ %d done", stats.DoneCount)) + "  ")
	b.WriteString(dimStyle.Render("(" + formatDuration(stats.DoneDuration) + ")"))
	b.WriteString("\n")
	b.WriteString("  " + yellowStyle.Render(fmt.Sprintf("⏭️  %d skipped", stats.SkippedCount)) + "  ")
	b.WriteString(dimStyle.Render("(" + formatDuration(stats.SkippedDuration) + ")"))
	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render(fmt.Sprintf("📝 %d todo", stats.TodoCount)) + "  ")
	b.WriteString(dimStyle.Render("(" + formatDuration(stats.TodoDuration) + ")"))
	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render(fmt.Sprintf("📆 %d recorded days", stats.RecordedDays)))
	b.WriteString("\n")

	// Daily average
	if stats.RecordedDays > 0 {
		avgDone := float64(stats.DoneCount) / float64(stats.RecordedDays)
		avgDur := stats.DoneDuration / stats.RecordedDays
		b.WriteString("  " + dimStyle.Render(fmt.Sprintf("📊 %.1f tasks/day · %s/day avg", avgDone, formatDuration(avgDur))))
		b.WriteString("\n")
	}

	return b.String()
}

func renderTrend(trend internal.WeeklyTrend) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("📈 WEEK TREND"))
	b.WriteString("\n")

	if trend.ThisWeek.TotalTasks == 0 {
		b.WriteString("  " + dimStyle.Render("Not enough data yet."))
		b.WriteString("\n")
		return b.String()
	}

	thisRate := trend.ThisWeek.CompletionRate * 100
	b.WriteString("  " + greenStyle.Render(fmt.Sprintf("This week: %.0f%%", thisRate)) + "  ")
	b.WriteString(dimStyle.Render(fmt.Sprintf("(%d/%d done)", trend.ThisWeek.DoneTasks, trend.ThisWeek.TotalTasks)))
	b.WriteString("\n")

	if trend.LastWeek.TotalTasks > 0 {
		lastRate := trend.LastWeek.CompletionRate * 100
		b.WriteString("  " + dimStyle.Render(fmt.Sprintf("Last week: %.0f%%", lastRate)) + "  ")
		b.WriteString(dimStyle.Render(fmt.Sprintf("(%d/%d done)", trend.LastWeek.DoneTasks, trend.LastWeek.TotalTasks)))
		b.WriteString("\n")

		changeStr := fmt.Sprintf("%+.0f pp", trend.Change)
		var direction string
		var style lipgloss.Style
		switch trend.Direction {
		case 1:
			direction = "📈 improving"
			style = upStyle
		case -1:
			direction = "📉 declining"
			style = downStyle
		default:
			direction = "➡️  steady"
			style = flatStyle
		}
		b.WriteString("  " + style.Render(changeStr) + "  " + style.Render(direction))
		b.WriteString("\n")
	}

	return b.String()
}

func renderHeatmap(dailies []internal.DailyStats) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("📅 CALENDAR HEATMAP"))
	b.WriteString("\n")

	// Sort ascending by date
	sorted := make([]internal.DailyStats, len(dailies))
	copy(sorted, dailies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date < sorted[j].Date
	})

	// Group into weeks (Mon-Sun)
	type cell struct {
		date string
		rate float64
	}
	var weeks [][]cell
	var currentWeek []cell
	for _, d := range sorted {
		t, err := time.Parse("2006-01-02", d.Date)
		if err != nil {
			continue
		}
		dow := t.Weekday()
		if dow == time.Monday && len(currentWeek) > 0 {
			weeks = append(weeks, currentWeek)
			currentWeek = nil
		}
		if len(weeks) == 0 && len(currentWeek) == 0 {
			for d := time.Monday; d < dow; d++ {
				currentWeek = append(currentWeek, cell{rate: -1})
			}
		}
		rate := 0.0
		if d.TaskCount > 0 {
			rate = float64(d.DoneCount) / float64(d.TaskCount)
		}
		currentWeek = append(currentWeek, cell{date: d.Date, rate: rate})
	}
	for len(currentWeek) < 7 {
		currentWeek = append(currentWeek, cell{rate: -1})
	}
	if len(currentWeek) > 0 {
		weeks = append(weeks, currentWeek)
	}

	// Limit to last 8 weeks
	maxWeeks := 8
	if len(weeks) > maxWeeks {
		weeks = weeks[len(weeks)-maxWeeks:]
	}

	// Day headers
	b.WriteString("     ")
	dayNames := []string{"Lu", "Ma", "Mi", "Ju", "Vi", "Sá", "Do"}
	for _, dn := range dayNames {
		b.WriteString(" " + dn + " ")
	}
	b.WriteString("\n")

	// Week rows
	for _, week := range weeks {
		b.WriteString("  ")
		for _, c := range week {
			if c.date == "" || c.rate < 0 {
				b.WriteString("   ")
				continue
			}
			block := internal.DayBlock(c.rate)
			if c.rate >= 0.8 {
				b.WriteString(greenStyle.Render(" " + block + " "))
			} else if c.rate >= 0.5 {
				b.WriteString(yellowStyle.Render(" " + block + " "))
			} else if c.rate > 0 {
				b.WriteString(redStyle.Render(" " + block + " "))
			} else {
				b.WriteString(dimStyle.Render(" · "))
			}
		}
		// Week rate
		var totalDone, totalTasks int
		for _, c := range week {
			if c.date == "" {
				continue
			}
			for _, d := range sorted {
				if d.Date == c.date {
					totalDone += d.DoneCount
					totalTasks += d.TaskCount
					break
				}
			}
		}
		if totalTasks > 0 {
			wr := float64(totalDone) / float64(totalTasks) * 100
			b.WriteString(" " + dimStyle.Render(fmt.Sprintf("%.0f%%", wr)))
		}
		b.WriteString("\n")
	}

	// Legend
	b.WriteString("  ")
	b.WriteString(greenStyle.Render(" ██ ≥80%") + yellowStyle.Render(" ▓▓ 50-79%") + redStyle.Render(" ░░ <50%") + dimStyle.Render(" · · none"))
	b.WriteString("\n")

	return b.String()
}

func renderStreaks(streaks []internal.TaskStreak) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("🔥 STREAKS"))
	b.WriteString("\n")

	limit := 15
	if len(streaks) < limit {
		limit = len(streaks)
	}

	// Header
	b.WriteString("  " + dimStyle.Render(fmt.Sprintf("%-28s %12s %12s %6s", "Task", "Current", "Best", "Done")))
	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render(strings.Repeat("─", 62)))
	b.WriteString("\n")

	for i := 0; i < limit; i++ {
		s := streaks[i]
		var statusIcon string
		var style lipgloss.Style

		switch {
		case s.Current >= 20:
			statusIcon = "🔥🔥"
			style = fireStyle
		case s.Current >= 10:
			statusIcon = "🔥"
			style = fireStyle
		case s.Current >= 5:
			statusIcon = "💪"
			style = greenStyle
		case s.Current >= 2:
			statusIcon = "👍"
			style = yellowStyle
		case s.Current == 0:
			statusIcon = "💀"
			style = redStyle
		default:
			statusIcon = "·"
			style = dimStyle
		}

		barLen := 6
		fill := 0
		if s.Longest > 0 {
			fill = int(math.Round(float64(s.Current) / float64(s.Longest) * float64(barLen)))
		}
		bar := strings.Repeat("█", fill) + strings.Repeat("░", barLen-fill)

		b.WriteString(fmt.Sprintf("  %s %-24s %s  %3dd  %s  %3d/%d\n",
			style.Render(statusIcon),
			truncate(s.Title, 24),
			streakStyle(s.Current).Render(bar),
			s.Current,
			dimStyle.Render(fmt.Sprintf("%3dd", s.Longest)),
			s.DoneDays, s.RecordedDays))
	}

	if len(streaks) > limit {
		b.WriteString("  " + dimStyle.Render(fmt.Sprintf("... and %d more", len(streaks)-limit)))
		b.WriteString("\n")
	}

	return b.String()
}

func renderWeakSpots(tasks []internal.TaskFrequencyStats) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("🔻 WEAK SPOTS (lowest completion)"))
	b.WriteString("\n")

	sorted := make([]internal.TaskFrequencyStats, len(tasks))
	copy(sorted, tasks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CompletionRate < sorted[j].CompletionRate
	})

	limit := 8
	if len(sorted) < limit {
		limit = len(sorted)
	}

	b.WriteString("  " + dimStyle.Render(fmt.Sprintf("%-28s %8s %6s", "Task", "Rate", "Details")))
	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render(strings.Repeat("─", 52)))
	b.WriteString("\n")

	for i := 0; i < limit; i++ {
		t := sorted[i]
		rate := t.CompletionRate * 100
		var rateStyle lipgloss.Style
		var dot string
		switch {
		case rate >= 80:
			rateStyle = greenStyle
			dot = "🟢"
		case rate >= 50:
			rateStyle = yellowStyle
			dot = "🟡"
		case rate >= 25:
			rateStyle = orangeStyle
			dot = "🟠"
		default:
			rateStyle = redStyle
			dot = "🔴"
		}

		barW := 8
		filled := int(math.Round(rate / 100.0 * float64(barW)))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)

		b.WriteString(fmt.Sprintf("  %s %-24s %s %4.0f%%  %s  %s\n",
			dot,
			truncate(t.Title, 24),
			rateStyle.Render(bar),
			rate,
			greenStyle.Render(fmt.Sprintf("%d/%d", t.DoneDays, t.RecordedDays)),
			yellowStyle.Render(fmt.Sprintf("%d skipped", t.SkippedDays))))
	}

	return b.String()
}

func renderTaskDetail(tasks []internal.TaskFrequencyStats, streaks []internal.TaskStreak) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("📋 PER-TASK BREAKDOWN"))
	b.WriteString("\n")

	streakMap := map[int]internal.TaskStreak{}
	for _, s := range streaks {
		streakMap[s.TaskID] = s
	}

	sorted := make([]internal.TaskFrequencyStats, len(tasks))
	copy(sorted, tasks)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].DoneDays > sorted[j].DoneDays
	})

	b.WriteString("  " + dimStyle.Render(fmt.Sprintf("%-4s %-24s %8s %8s %8s %12s", "ID", "Task", "Done", "Skip", "Rate", "Streak")))
	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Render(strings.Repeat("─", 68)))
	b.WriteString("\n")

	for _, t := range sorted {
		rate := t.CompletionRate * 100
		var rateStr string
		switch {
		case rate >= 80:
			rateStr = greenStyle.Render(fmt.Sprintf("%.0f%%", rate))
		case rate >= 50:
			rateStr = yellowStyle.Render(fmt.Sprintf("%.0f%%", rate))
		default:
			rateStr = redStyle.Render(fmt.Sprintf("%.0f%%", rate))
		}

		streakStr := dimStyle.Render("  —")
		if s, ok := streakMap[t.TaskID]; ok {
			if s.Current > 0 {
				streakStr = fireStyle.Render(fmt.Sprintf("🔥 %dd", s.Current))
			} else {
				streakStr = dimStyle.Render("💀")
			}
		}

		b.WriteString(fmt.Sprintf("  %s %-24s %s %s %s %s\n",
			dimStyle.Render(fmt.Sprintf("#%-2d", t.TaskID)),
			truncate(t.Title, 24),
			greenStyle.Render(fmt.Sprintf("%3d/%d", t.DoneDays, t.RecordedDays)),
			yellowStyle.Render(fmt.Sprintf("%3d", t.SkippedDays)),
			rateStr,
			streakStr))
	}

	return b.String()
}

// ── Helpers ───────────────────────────────────────────────────────────────

func trendArrow(dir int) string {
	switch dir {
	case 1:
		return upStyle.Render("↗ improving")
	case -1:
		return downStyle.Render("↘ declining")
	default:
		return flatStyle.Render("→ steady")
	}
}

func streakStyle(n int) lipgloss.Style {
	switch {
	case n >= 10:
		return fireStyle
	case n >= 5:
		return greenStyle
	case n >= 2:
		return yellowStyle
	case n == 0:
		return redStyle
	default:
		return dimStyle
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func formatDuration(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	h := minutes / 60
	m := minutes % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}