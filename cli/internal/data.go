package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Task represents a single task item
type Task struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Duration   int    `json:"duration"`
	Status     string `json:"status"` // "todo", "done", or "skipped"
	Order      int    `json:"order"`
	Deadline   string `json:"deadline,omitempty"`   // HH:MM format, optional daily reminder time
	Visibility []int  `json:"visibility,omitempty"` // days of the week (0=Sun..6=Sat); nil/empty = every day
}

// Data represents the complete task data structure
type Data struct {
	LastReset    string `json:"last_reset"`
	NextID       int    `json:"next_id"`
	Tasks        []Task `json:"tasks"`
	ThemeIndex   int    `json:"theme_index"`
	LastModified int64  `json:"last_modified,omitempty"`
}

func normalizeLastModified(ts int64) int64 {
	if ts > 0 && ts < 100000000000 {
		return ts * 1000
	}
	return ts
}

// DefaultDataPath returns the default path for the data file
func DefaultDataPath() (string, error) {
	// Check environment variable first
	if envPath := os.Getenv("DAILY_TASKS_PATH"); envPath != "" {
		return envPath, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "Nextcloud", ".daily-tasks.json"), nil
}

// LoadData reads and parses the data file from the given path
func LoadData(path string) (Data, error) {
	today := time.Now().Format("2006-01-02")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Data{LastReset: today, NextID: 1, LastModified: 0}, nil
		}
		return Data{}, err
	}

	var data Data
	if err := json.Unmarshal(b, &data); err != nil {
		return Data{}, err
	}
	return NormalizeData(data), nil
}

// SaveData writes the data to the given path
func SaveData(path string, data Data) error {
	data.LastModified = time.Now().UnixMilli()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// NormalizeData ensures all fields have valid values
func NormalizeData(data Data) Data {
	if data.LastReset == "" {
		data.LastReset = time.Now().Format("2006-01-02")
	}
	if data.NextID == 0 {
		data.NextID = 1
	}
	if data.ThemeIndex < 0 || data.ThemeIndex >= ThemeCount() {
		data.ThemeIndex = 0
	}
	data.LastModified = normalizeLastModified(data.LastModified)
	AssignMissingOrders(&data)
	return data
}

// AssignMissingOrders assigns order values to tasks that don't have them
func AssignMissingOrders(data *Data) {
	maxTodo := 0
	maxDone := 0
	maxSkipped := 0
	for i := range data.Tasks {
		t := &data.Tasks[i]
		if t.Order != 0 {
			if t.Status == "done" && t.Order > maxDone {
				maxDone = t.Order
			} else if t.Status == "skipped" && t.Order > maxSkipped {
				maxSkipped = t.Order
			} else if t.Status == "todo" && t.Order > maxTodo {
				maxTodo = t.Order
			}
			continue
		}
		if t.Status == "done" {
			maxDone++
			t.Order = maxDone
		} else if t.Status == "skipped" {
			maxSkipped++
			t.Order = maxSkipped
		} else {
			maxTodo++
			t.Order = maxTodo
		}
	}
}

// CloneData creates a deep copy of the data
func CloneData(d Data) Data {
	out := d
	out.Tasks = make([]Task, len(d.Tasks))
	for i, t := range d.Tasks {
		out.Tasks[i] = t
		if len(t.Visibility) > 0 {
			out.Tasks[i].Visibility = make([]int, len(t.Visibility))
			copy(out.Tasks[i].Visibility, t.Visibility)
		}
	}
	return out
}

// OrderedTasks returns tasks with the given status, sorted by order
func OrderedTasks(data *Data, status string) []*Task {
	var tasks []*Task
	for i := range data.Tasks {
		if data.Tasks[i].Status == status {
			tasks = append(tasks, &data.Tasks[i])
		}
	}
	SortTasks(tasks)
	return tasks
}

// SortTasks sorts tasks by deadline time first, then by order.
// Tasks with a deadline are sorted chronologically before tasks without one.
func SortTasks(tasks []*Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		di, dj := tasks[i].Deadline, tasks[j].Deadline
		if di != dj {
			if di == "" {
				return false // no deadline sorts after
			}
			if dj == "" {
				return true // has deadline sorts before
			}
			return di < dj
		}
		if tasks[i].Order == tasks[j].Order {
			return tasks[i].ID < tasks[j].ID
		}
		return tasks[i].Order < tasks[j].Order
	})
}

// NextOrder returns the next available order value for a status
func NextOrder(data *Data, status string) int {
	maxOrder := 0
	for i := range data.Tasks {
		if data.Tasks[i].Status == status && data.Tasks[i].Order > maxOrder {
			maxOrder = data.Tasks[i].Order
		}
	}
	return maxOrder + 1
}

// FindTask returns a pointer to the task with the given ID
func FindTask(data *Data, id int) *Task {
	for i := range data.Tasks {
		if data.Tasks[i].ID == id {
			return &data.Tasks[i]
		}
	}
	return nil
}

// DeleteTask removes the task with the given ID
func DeleteTask(data *Data, id int) {
	for i := range data.Tasks {
		if data.Tasks[i].ID == id {
			data.Tasks = append(data.Tasks[:i], data.Tasks[i+1:]...)
			return
		}
	}
}

// ParseDuration parses a duration string into minutes
func ParseDuration(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("Duration cannot be empty")
	}
	d, err := strconv.Atoi(s)
	if err != nil || d <= 0 {
		return 0, errors.New("Duration must be a positive integer")
	}
	return d, nil
}

// ResetIfNewDay checks if we need to reset tasks for a new day
func ResetIfNewDay(data *Data) bool {
	today := time.Now().Format("2006-01-02")
	if data.LastReset == today {
		return false
	}

	// Move all tasks (todo, done, and skipped) back to todo
	reset := append(OrderedTasks(data, "todo"), OrderedTasks(data, "done")...)
	reset = append(reset, OrderedTasks(data, "skipped")...)
	for i, t := range reset {
		t.Status = "todo"
		t.Order = i + 1
	}
	data.LastReset = today
	data.LastModified = time.Now().UnixMilli()
	return true
}

// IsAM returns true if the deadline hour is before noon (AM).
func IsAM(deadline string) bool {
	if len(deadline) < 5 {
		return false
	}
	h, err := strconv.Atoi(deadline[:2])
	if err != nil {
		return false
	}
	return h < 12
}

// DeadlineIndicator returns a human-readable time-remaining or overdue string
// relative to now. Returns "" for empty or invalid deadlines.
func DeadlineIndicator(deadline string, now time.Time) string {
	if deadline == "" || len(deadline) < 5 {
		return ""
	}
	parts := strings.SplitN(deadline, ":", 2)
	if len(parts) != 2 {
		return ""
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return ""
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	diff := target.Sub(now)

	if diff > -time.Minute && diff < time.Minute {
		return "now"
	}
	if diff > 0 {
		return "in " + formatDuration(diff)
	}
	return formatDuration(-diff) + " ago"
}

func formatDuration(d time.Duration) string {
	totalMin := int(d.Minutes())
	if totalMin < 60 {
		return strconv.Itoa(totalMin) + "m"
	}
	h := totalMin / 60
	m := totalMin % 60
	if m == 0 {
		return strconv.Itoa(h) + "h"
	}
	return strconv.Itoa(h) + "h " + strconv.Itoa(m) + "m"
}

// IsVisibleOn returns true if the task should appear on the given weekday.
// An empty or nil Visibility slice means the task is visible every day.
func (t *Task) IsVisibleOn(weekday time.Weekday) bool {
	if len(t.Visibility) == 0 {
		return true
	}
	day := int(weekday)
	for _, v := range t.Visibility {
		if v == day {
			return true
		}
	}
	return false
}

// IsVisibleToday returns true if the task should appear today.
func (t *Task) IsVisibleToday() bool {
	return t.IsVisibleOn(time.Now().Weekday())
}

// VisibleTasksOn returns only tasks visible on the given weekday.
func VisibleTasksOn(tasks []Task, weekday time.Weekday) []Task {
	var result []Task
	for _, t := range tasks {
		if t.IsVisibleOn(weekday) {
			result = append(result, t)
		}
	}
	return result
}

// WeekdayFromDate parses a YYYY-MM-DD date and returns its weekday.
func WeekdayFromDate(date string) (time.Weekday, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return 0, err
	}
	return t.Weekday(), nil
}

// ClampIndex clamps an index to valid range
func ClampIndex(idx, length int) int {
	if length <= 0 {
		return 0
	}
	if idx < 0 {
		return 0
	}
	if idx >= length {
		return length - 1
	}
	return idx
}
