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
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Duration int    `json:"duration"`
	Status   string `json:"status"` // "todo", "done", or "skipped"
	Order    int    `json:"order"`
	Deadline string `json:"deadline,omitempty"` // HH:MM format, optional daily reminder time
}

// Data represents the complete task data structure
type Data struct {
	LastReset    string `json:"last_reset"`
	NextID       int    `json:"next_id"`
	Tasks        []Task `json:"tasks"`
	ThemeIndex   int    `json:"theme_index"`
	LastModified int64  `json:"last_modified,omitempty"`
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
			return Data{LastReset: today, NextID: 1, LastModified: time.Now().Unix()}, nil
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
	data.LastModified = time.Now().Unix()
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
	if data.LastModified == 0 {
		data.LastModified = time.Now().Unix()
	}
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
	copy(out.Tasks, d.Tasks)
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

// SortTasks sorts tasks by order, then by ID
func SortTasks(tasks []*Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
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
	data.LastModified = time.Now().Unix()
	return true
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
