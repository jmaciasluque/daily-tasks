package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const historyVersion = 1

type TaskSnapshot struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Duration   int    `json:"duration"`
	Status     string `json:"status"`
	Deadline   string `json:"deadline,omitempty"`
	Visibility []int  `json:"visibility,omitempty"`
}

type HistoryDay struct {
	Date      string         `json:"date"`
	UpdatedAt int64          `json:"updated_at,omitempty"`
	Tasks     []TaskSnapshot `json:"tasks"`
}

type HistoryEvent struct {
	Timestamp  int64  `json:"timestamp"`
	Date       string `json:"date"`
	Type       string `json:"type"`
	TaskID     int    `json:"task_id"`
	Title      string `json:"title"`
	FromStatus string `json:"from_status,omitempty"`
	ToStatus   string `json:"to_status,omitempty"`
	Duration   int    `json:"duration,omitempty"`
	Deadline   string `json:"deadline,omitempty"`
}

type History struct {
	Version   int            `json:"version"`
	UpdatedAt int64          `json:"updated_at,omitempty"`
	Days      []HistoryDay   `json:"days,omitempty"`
	Events    []HistoryEvent `json:"events,omitempty"`
}

type DailyStats struct {
	Date            string  `json:"date"`
	TaskCount       int     `json:"task_count"`
	TodoCount       int     `json:"todo_count"`
	DoneCount       int     `json:"done_count"`
	SkippedCount    int     `json:"skipped_count"`
	TodoDuration    int     `json:"todo_duration"`
	DoneDuration    int     `json:"done_duration"`
	SkippedDuration int     `json:"skipped_duration"`
	CompletionRate  float64 `json:"completion_rate"`
}

type TaskFrequencyStats struct {
	TaskID          int     `json:"task_id"`
	Title           string  `json:"title"`
	RecordedDays    int     `json:"recorded_days"`
	TodoDays        int     `json:"todo_days"`
	DoneDays        int     `json:"done_days"`
	SkippedDays     int     `json:"skipped_days"`
	CompletionRate  float64 `json:"completion_rate"`
	TotalDuration   int     `json:"total_duration"`
	DoneDuration    int     `json:"done_duration"`
	SkippedDuration int     `json:"skipped_duration"`
}

type StatsSummary struct {
	From            string               `json:"from"`
	To              string               `json:"to"`
	RecordedDays    int                  `json:"recorded_days"`
	TaskCount       int                  `json:"task_count"`
	TodoCount       int                  `json:"todo_count"`
	DoneCount       int                  `json:"done_count"`
	SkippedCount    int                  `json:"skipped_count"`
	TodoDuration    int                  `json:"todo_duration"`
	DoneDuration    int                  `json:"done_duration"`
	SkippedDuration int                  `json:"skipped_duration"`
	CompletionRate  float64              `json:"completion_rate"`
	Daily           []DailyStats         `json:"daily"`
	Tasks           []TaskFrequencyStats `json:"tasks"`
}

func HistoryPath(dataPath string) string {
	ext := filepath.Ext(dataPath)
	if ext == "" {
		return dataPath + ".history.json"
	}
	base := strings.TrimSuffix(dataPath, ext)
	return base + ".history" + ext
}

func LoadHistory(dataPath string) (History, error) {
	path := HistoryPath(dataPath)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return History{Version: historyVersion}, nil
		}
		return History{}, err
	}

	var history History
	if err := json.Unmarshal(b, &history); err != nil {
		return History{}, err
	}
	if history.Version == 0 {
		history.Version = historyVersion
	}
	sortHistory(&history)
	return history, nil
}

func SaveHistory(dataPath string, history History) error {
	history.Version = historyVersion
	history.UpdatedAt = time.Now().UnixMilli()
	sortHistory(&history)

	b, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(HistoryPath(dataPath), b, 0o600)
}

func HistoryWithCurrentSnapshot(history History, data Data, now int64) History {
	normalized := NormalizeData(data)
	if now == 0 {
		now = time.Now().UnixMilli()
	}
	if history.Version == 0 {
		history.Version = historyVersion
	}
	if upsertHistoryDay(&history, normalized, now) {
		history.UpdatedAt = now
	}
	sortHistory(&history)
	return history
}

func HistoryContentEqual(a, b History) bool {
	if a.Version == 0 {
		a.Version = historyVersion
	}
	if b.Version == 0 {
		b.Version = historyVersion
	}
	sortHistory(&a)
	sortHistory(&b)
	if a.Version != b.Version || len(a.Days) != len(b.Days) || len(a.Events) != len(b.Events) {
		return false
	}
	for i := range a.Days {
		if !historyDayEqual(a.Days[i], b.Days[i]) {
			return false
		}
	}
	for i := range a.Events {
		if a.Events[i] != b.Events[i] {
			return false
		}
	}
	return true
}

func MergeHistories(local, remote History) History {
	if local.Version == 0 {
		local.Version = historyVersion
	}
	if remote.Version == 0 {
		remote.Version = historyVersion
	}
	merged := History{
		Version: max(local.Version, remote.Version),
	}

	dayMap := map[string]HistoryDay{}
	for _, day := range append(local.Days, remote.Days...) {
		existing, ok := dayMap[day.Date]
		if !ok || day.UpdatedAt > existing.UpdatedAt || (day.UpdatedAt == existing.UpdatedAt && len(day.Tasks) >= len(existing.Tasks)) {
			copyDay := HistoryDay{
				Date:      day.Date,
				UpdatedAt: day.UpdatedAt,
				Tasks:     append([]TaskSnapshot(nil), day.Tasks...),
			}
			dayMap[day.Date] = copyDay
		}
	}
	for _, day := range dayMap {
		merged.Days = append(merged.Days, day)
	}

	eventMap := map[string]HistoryEvent{}
	for _, event := range append(local.Events, remote.Events...) {
		key := strings.Join([]string{
			strconv.FormatInt(event.Timestamp, 10),
			event.Date,
			event.Type,
			strconv.Itoa(event.TaskID),
			event.Title,
			event.FromStatus,
			event.ToStatus,
			strconv.Itoa(event.Duration),
			event.Deadline,
		}, "|")
		if _, ok := eventMap[key]; !ok {
			eventMap[key] = event
		}
	}
	for _, event := range eventMap {
		merged.Events = append(merged.Events, event)
	}

	if local.UpdatedAt > merged.UpdatedAt {
		merged.UpdatedAt = local.UpdatedAt
	}
	if remote.UpdatedAt > merged.UpdatedAt {
		merged.UpdatedAt = remote.UpdatedAt
	}
	sortHistory(&merged)
	return merged
}

func EnsureHistorySnapshot(dataPath string, data Data) error {
	history, err := LoadHistory(dataPath)
	if err != nil {
		return err
	}
	if upsertHistoryDay(&history, NormalizeData(data), time.Now().UnixMilli()) {
		return SaveHistory(dataPath, history)
	}
	return nil
}

func SaveDataWithHistory(dataPath string, before, after Data) error {
	normalizedBefore := NormalizeData(before)
	normalizedAfter := NormalizeData(after)

	if err := SaveData(dataPath, normalizedAfter); err != nil {
		return err
	}

	history, err := LoadHistory(dataPath)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	initialEventCount := len(history.Events)
	changed := false
	if normalizedBefore.LastReset != "" {
		changed = upsertHistoryDay(&history, normalizedBefore, now) || changed
	}
	appendHistoryEvents(&history, normalizedBefore, normalizedAfter, now)
	changed = upsertHistoryDay(&history, normalizedAfter, now) || changed

	if !changed && len(history.Events) == initialEventCount {
		return nil
	}
	return SaveHistory(dataPath, history)
}

func BuildStats(dataPath string, current Data, from, to string) (StatsSummary, error) {
	history, err := LoadHistory(dataPath)
	if err != nil {
		return StatsSummary{}, err
	}

	withSnapshot := HistoryWithCurrentSnapshot(history, current, time.Now().UnixMilli())
	if !HistoryContentEqual(withSnapshot, history) {
		if err := SaveHistory(dataPath, withSnapshot); err != nil {
			return StatsSummary{}, err
		}
	}

	return AggregateStats(withSnapshot, from, to), nil
}

func AggregateStats(history History, from, to string) StatsSummary {
	sortHistory(&history)

	filtered := make([]HistoryDay, 0, len(history.Days))
	for _, day := range history.Days {
		if from != "" && day.Date < from {
			continue
		}
		if to != "" && day.Date > to {
			continue
		}
		filtered = append(filtered, day)
	}

	summary := StatsSummary{
		From:         from,
		To:           to,
		RecordedDays: len(filtered),
		Daily:        make([]DailyStats, 0, len(filtered)),
	}

	type taskAccumulator struct {
		TaskFrequencyStats
	}

	taskMap := map[int]*taskAccumulator{}
	for _, day := range filtered {
		daily := DailyStats{Date: day.Date}
		for _, task := range day.Tasks {
			daily.TaskCount++
			summary.TaskCount++

			acc, ok := taskMap[task.ID]
			if !ok {
				acc = &taskAccumulator{
					TaskFrequencyStats: TaskFrequencyStats{
						TaskID: task.ID,
						Title:  task.Title,
					},
				}
				taskMap[task.ID] = acc
			}
			acc.Title = task.Title
			acc.RecordedDays++
			acc.TotalDuration += task.Duration

			switch task.Status {
			case "done":
				daily.DoneCount++
				daily.DoneDuration += task.Duration
				summary.DoneCount++
				summary.DoneDuration += task.Duration
				acc.DoneDays++
				acc.DoneDuration += task.Duration
			case "skipped":
				daily.SkippedCount++
				daily.SkippedDuration += task.Duration
				summary.SkippedCount++
				summary.SkippedDuration += task.Duration
				acc.SkippedDays++
				acc.SkippedDuration += task.Duration
			default:
				daily.TodoCount++
				daily.TodoDuration += task.Duration
				summary.TodoCount++
				summary.TodoDuration += task.Duration
				acc.TodoDays++
			}
		}
		if daily.TaskCount > 0 {
			daily.CompletionRate = float64(daily.DoneCount) / float64(daily.TaskCount)
		}
		summary.Daily = append(summary.Daily, daily)
	}

	if summary.TaskCount > 0 {
		summary.CompletionRate = float64(summary.DoneCount) / float64(summary.TaskCount)
	}

	for _, acc := range taskMap {
		if acc.RecordedDays > 0 {
			acc.CompletionRate = float64(acc.DoneDays) / float64(acc.RecordedDays)
		}
		summary.Tasks = append(summary.Tasks, acc.TaskFrequencyStats)
	}

	sort.Slice(summary.Tasks, func(i, j int) bool {
		if summary.Tasks[i].DoneDays == summary.Tasks[j].DoneDays {
			if summary.Tasks[i].RecordedDays == summary.Tasks[j].RecordedDays {
				return summary.Tasks[i].TaskID < summary.Tasks[j].TaskID
			}
			return summary.Tasks[i].RecordedDays > summary.Tasks[j].RecordedDays
		}
		return summary.Tasks[i].DoneDays > summary.Tasks[j].DoneDays
	})

	return summary
}

func sortHistory(history *History) {
	sort.Slice(history.Days, func(i, j int) bool {
		return history.Days[i].Date < history.Days[j].Date
	})
	sort.Slice(history.Events, func(i, j int) bool {
		if history.Events[i].Timestamp == history.Events[j].Timestamp {
			if history.Events[i].TaskID == history.Events[j].TaskID {
				return history.Events[i].Type < history.Events[j].Type
			}
			return history.Events[i].TaskID < history.Events[j].TaskID
		}
		return history.Events[i].Timestamp < history.Events[j].Timestamp
	})
}

func upsertHistoryDay(history *History, data Data, now int64) bool {
	day := HistoryDay{
		Date:      data.LastReset,
		UpdatedAt: now,
		Tasks:     snapshotsForVisibleTasks(data.Tasks, data.LastReset),
	}

	for i := range history.Days {
		if history.Days[i].Date == day.Date {
			if historyDayEqual(history.Days[i], day) {
				return false
			}
			history.Days[i] = day
			return true
		}
	}

	history.Days = append(history.Days, day)
	return true
}

func historyDayEqual(a, b HistoryDay) bool {
	if a.Date != b.Date || len(a.Tasks) != len(b.Tasks) {
		return false
	}
	for i := range a.Tasks {
		ta, tb := a.Tasks[i], b.Tasks[i]
		if ta.ID != tb.ID || ta.Title != tb.Title || ta.Duration != tb.Duration ||
			ta.Status != tb.Status || ta.Deadline != tb.Deadline {
			return false
		}
		if len(ta.Visibility) != len(tb.Visibility) {
			return false
		}
		for j := range ta.Visibility {
			if ta.Visibility[j] != tb.Visibility[j] {
				return false
			}
		}
	}
	return true
}

func snapshotsForTasks(tasks []Task) []TaskSnapshot {
	snapshots := make([]TaskSnapshot, 0, len(tasks))
	for _, task := range tasks {
		snapshots = append(snapshots, TaskSnapshot{
			ID:         task.ID,
			Title:      task.Title,
			Duration:   task.Duration,
			Status:     task.Status,
			Deadline:   task.Deadline,
			Visibility: task.Visibility,
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].ID < snapshots[j].ID
	})
	return snapshots
}

// snapshotsForVisibleTasks returns snapshots only for tasks visible on the
// given date string (YYYY-MM-DD). Falls back to all tasks on parse error.
func snapshotsForVisibleTasks(tasks []Task, date string) []TaskSnapshot {
	weekday, err := WeekdayFromDate(date)
	if err != nil {
		return snapshotsForTasks(tasks)
	}
	visible := VisibleTasksOn(tasks, weekday)
	return snapshotsForTasks(visible)
}

func appendHistoryEvents(history *History, before, after Data, now int64) {
	beforeMap := map[int]Task{}
	afterMap := map[int]Task{}

	for _, task := range before.Tasks {
		beforeMap[task.ID] = task
	}
	for _, task := range after.Tasks {
		afterMap[task.ID] = task
	}

	for id, previous := range beforeMap {
		next, ok := afterMap[id]
		if !ok {
			history.Events = append(history.Events, HistoryEvent{
				Timestamp:  now,
				Date:       before.LastReset,
				Type:       "task_deleted",
				TaskID:     previous.ID,
				Title:      previous.Title,
				FromStatus: previous.Status,
				Duration:   previous.Duration,
				Deadline:   previous.Deadline,
			})
			continue
		}

		if previous.Status != next.Status {
			history.Events = append(history.Events, HistoryEvent{
				Timestamp:  now,
				Date:       after.LastReset,
				Type:       "status_changed",
				TaskID:     next.ID,
				Title:      next.Title,
				FromStatus: previous.Status,
				ToStatus:   next.Status,
				Duration:   next.Duration,
				Deadline:   next.Deadline,
			})
		}

		if previous.Title != next.Title || previous.Duration != next.Duration || previous.Deadline != next.Deadline {
			history.Events = append(history.Events, HistoryEvent{
				Timestamp: now,
				Date:      after.LastReset,
				Type:      "task_updated",
				TaskID:    next.ID,
				Title:     next.Title,
				ToStatus:  next.Status,
				Duration:  next.Duration,
				Deadline:  next.Deadline,
			})
		}
	}

	for id, task := range afterMap {
		if _, ok := beforeMap[id]; ok {
			continue
		}
		history.Events = append(history.Events, HistoryEvent{
			Timestamp: now,
			Date:      after.LastReset,
			Type:      "task_added",
			TaskID:    task.ID,
			Title:     task.Title,
			ToStatus:  task.Status,
			Duration:  task.Duration,
			Deadline:  task.Deadline,
		})
	}
}
