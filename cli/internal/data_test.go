package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultDataPath(t *testing.T) {
	// Test with environment variable
	t.Run("with env var", func(t *testing.T) {
		os.Setenv("DAILY_TASKS_PATH", "/custom/path/tasks.json")
		defer os.Unsetenv("DAILY_TASKS_PATH")

		path, err := DefaultDataPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "/custom/path/tasks.json" {
			t.Errorf("expected /custom/path/tasks.json, got %s", path)
		}
	})

	// Test without environment variable
	t.Run("without env var", func(t *testing.T) {
		os.Unsetenv("DAILY_TASKS_PATH")

		path, err := DefaultDataPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, "Nextcloud", ".daily-tasks.json")
		if path != expected {
			t.Errorf("expected %s, got %s", expected, path)
		}
	})
}

func TestLoadData(t *testing.T) {
	t.Run("non-existent file returns empty data", func(t *testing.T) {
		data, err := LoadData("/nonexistent/path/file.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.NextID != 1 {
			t.Errorf("expected NextID=1, got %d", data.NextID)
		}
		if len(data.Tasks) != 0 {
			t.Errorf("expected empty tasks, got %d", len(data.Tasks))
		}
	})

	t.Run("loads valid JSON file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.json")

		content := `{
			"last_reset": "2026-01-23",
			"next_id": 5,
			"tasks": [
				{"id": 1, "title": "Test Task", "duration": 10, "status": "todo", "order": 1}
			],
			"theme_index": 2,
			"last_modified": 1234567890
		}`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		data, err := LoadData(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data.NextID != 5 {
			t.Errorf("expected NextID=5, got %d", data.NextID)
		}
		if len(data.Tasks) != 1 {
			t.Errorf("expected 1 task, got %d", len(data.Tasks))
		}
		if data.Tasks[0].Title != "Test Task" {
			t.Errorf("expected title 'Test Task', got '%s'", data.Tasks[0].Title)
		}
	})
}

func TestSaveData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	data := Data{
		LastReset:  "2026-01-23",
		NextID:     3,
		Tasks:      []Task{{ID: 1, Title: "Test", Duration: 5, Status: "todo", Order: 1}},
		ThemeIndex: 1,
	}

	if err := SaveData(path, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists and can be loaded
	loaded, err := LoadData(path)
	if err != nil {
		t.Fatalf("failed to load saved data: %v", err)
	}
	if loaded.NextID != 3 {
		t.Errorf("expected NextID=3, got %d", loaded.NextID)
	}
	if loaded.LastModified == 0 {
		t.Error("expected LastModified to be set")
	}
}

func TestNormalizeData(t *testing.T) {
	t.Run("sets defaults for empty data", func(t *testing.T) {
		data := NormalizeData(Data{})

		if data.LastReset == "" {
			t.Error("expected LastReset to be set")
		}
		if data.NextID != 1 {
			t.Errorf("expected NextID=1, got %d", data.NextID)
		}
		if data.ThemeIndex != 0 {
			t.Errorf("expected ThemeIndex=0, got %d", data.ThemeIndex)
		}
	})

	t.Run("clamps invalid theme index", func(t *testing.T) {
		data := NormalizeData(Data{ThemeIndex: 9999})
		if data.ThemeIndex != 0 {
			t.Errorf("expected ThemeIndex=0, got %d", data.ThemeIndex)
		}

		data = NormalizeData(Data{ThemeIndex: -5})
		if data.ThemeIndex != 0 {
			t.Errorf("expected ThemeIndex=0, got %d", data.ThemeIndex)
		}
	})

	t.Run("converts second timestamps to milliseconds", func(t *testing.T) {
		data := NormalizeData(Data{LastModified: 1700000000})
		if data.LastModified != 1700000000000 {
			t.Errorf("expected LastModified to be normalized to milliseconds, got %d", data.LastModified)
		}
	})
}

func TestAssignMissingOrders(t *testing.T) {
	data := Data{
		Tasks: []Task{
			{ID: 1, Title: "A", Status: "todo", Order: 0},
			{ID: 2, Title: "B", Status: "todo", Order: 0},
			{ID: 3, Title: "C", Status: "done", Order: 0},
		},
	}

	AssignMissingOrders(&data)

	// Check todo tasks got orders
	for _, task := range data.Tasks {
		if task.Order == 0 {
			t.Errorf("task %d should have non-zero order", task.ID)
		}
	}
}

func TestCloneData(t *testing.T) {
	original := Data{
		LastReset: "2026-01-23",
		NextID:    5,
		Tasks:     []Task{{ID: 1, Title: "Test", Duration: 5, Status: "todo", Order: 1}},
	}

	clone := CloneData(original)

	// Modify clone
	clone.Tasks[0].Title = "Modified"
	clone.NextID = 10

	// Original should be unchanged
	if original.Tasks[0].Title != "Test" {
		t.Error("original task was modified")
	}
	if original.NextID != 5 {
		t.Error("original NextID was modified")
	}
}

func TestOrderedTasks(t *testing.T) {
	t.Run("sorted by order when no deadlines", func(t *testing.T) {
		data := Data{
			Tasks: []Task{
				{ID: 1, Title: "A", Status: "todo", Order: 3},
				{ID: 2, Title: "B", Status: "done", Order: 1},
				{ID: 3, Title: "C", Status: "todo", Order: 1},
				{ID: 4, Title: "D", Status: "todo", Order: 2},
			},
		}

		todoTasks := OrderedTasks(&data, "todo")
		if len(todoTasks) != 3 {
			t.Fatalf("expected 3 todo tasks, got %d", len(todoTasks))
		}

		if todoTasks[0].ID != 3 || todoTasks[1].ID != 4 || todoTasks[2].ID != 1 {
			t.Errorf("tasks not sorted correctly: %v, %v, %v", todoTasks[0].ID, todoTasks[1].ID, todoTasks[2].ID)
		}

		doneTasks := OrderedTasks(&data, "done")
		if len(doneTasks) != 1 {
			t.Fatalf("expected 1 done task, got %d", len(doneTasks))
		}
	})

	t.Run("sorted by deadline time", func(t *testing.T) {
		data := Data{
			Tasks: []Task{
				{ID: 1, Title: "Late", Status: "todo", Order: 1, Deadline: "22:00"},
				{ID: 2, Title: "Early", Status: "todo", Order: 2, Deadline: "06:00"},
				{ID: 3, Title: "Mid", Status: "todo", Order: 3, Deadline: "12:00"},
			},
		}

		tasks := OrderedTasks(&data, "todo")
		if tasks[0].ID != 2 || tasks[1].ID != 3 || tasks[2].ID != 1 {
			t.Errorf("expected order [2,3,1] got [%d,%d,%d]", tasks[0].ID, tasks[1].ID, tasks[2].ID)
		}
	})

	t.Run("tasks with deadlines sort before tasks without", func(t *testing.T) {
		data := Data{
			Tasks: []Task{
				{ID: 1, Title: "No deadline", Status: "todo", Order: 1},
				{ID: 2, Title: "Has deadline", Status: "todo", Order: 2, Deadline: "08:00"},
			},
		}

		tasks := OrderedTasks(&data, "todo")
		if tasks[0].ID != 2 || tasks[1].ID != 1 {
			t.Errorf("expected task with deadline first, got [%d,%d]", tasks[0].ID, tasks[1].ID)
		}
	})

	t.Run("same deadline sorts by order", func(t *testing.T) {
		data := Data{
			Tasks: []Task{
				{ID: 1, Title: "Second", Status: "todo", Order: 2, Deadline: "08:00"},
				{ID: 2, Title: "First", Status: "todo", Order: 1, Deadline: "08:00"},
			},
		}

		tasks := OrderedTasks(&data, "todo")
		if tasks[0].ID != 2 || tasks[1].ID != 1 {
			t.Errorf("expected order [2,1] got [%d,%d]", tasks[0].ID, tasks[1].ID)
		}
	})
}

func TestNextOrder(t *testing.T) {
	data := Data{
		Tasks: []Task{
			{ID: 1, Status: "todo", Order: 3},
			{ID: 2, Status: "todo", Order: 5},
			{ID: 3, Status: "done", Order: 2},
		},
	}

	if next := NextOrder(&data, "todo"); next != 6 {
		t.Errorf("expected 6, got %d", next)
	}
	if next := NextOrder(&data, "done"); next != 3 {
		t.Errorf("expected 3, got %d", next)
	}
}

func TestFindTask(t *testing.T) {
	data := Data{
		Tasks: []Task{
			{ID: 1, Title: "First"},
			{ID: 2, Title: "Second"},
		},
	}

	task := FindTask(&data, 2)
	if task == nil {
		t.Fatal("expected to find task")
	}
	if task.Title != "Second" {
		t.Errorf("expected 'Second', got '%s'", task.Title)
	}

	if found := FindTask(&data, 999); found != nil {
		t.Error("expected nil for non-existent task")
	}
}

func TestDeleteTask(t *testing.T) {
	data := Data{
		Tasks: []Task{
			{ID: 1, Title: "First"},
			{ID: 2, Title: "Second"},
			{ID: 3, Title: "Third"},
		},
	}

	DeleteTask(&data, 2)

	if len(data.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(data.Tasks))
	}
	if FindTask(&data, 2) != nil {
		t.Error("task 2 should have been deleted")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"5", 5, false},
		{"10", 10, false},
		{" 15 ", 15, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-5", 0, true},
		{"0", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseDuration(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDuration(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestResetIfNewDay(t *testing.T) {
	t.Run("same day no reset", func(t *testing.T) {
		today := time.Now().Format("2006-01-02")
		data := Data{
			LastReset: today,
			Tasks: []Task{
				{ID: 1, Status: "done", Order: 1},
			},
		}

		changed := ResetIfNewDay(&data)
		if changed {
			t.Error("should not have changed on same day")
		}
		if data.Tasks[0].Status != "done" {
			t.Error("task status should not have changed")
		}
	})

	t.Run("new day resets tasks", func(t *testing.T) {
		data := Data{
			LastReset: "2020-01-01",
			Tasks: []Task{
				{ID: 1, Status: "done", Order: 1},
				{ID: 2, Status: "todo", Order: 1},
			},
		}

		changed := ResetIfNewDay(&data)
		if !changed {
			t.Error("should have changed on new day")
		}
		for _, task := range data.Tasks {
			if task.Status != "todo" {
				t.Errorf("task %d should be todo, got %s", task.ID, task.Status)
			}
		}
	})
}

func TestIsAM(t *testing.T) {
	tests := []struct {
		deadline string
		want     bool
	}{
		{"06:00", true},
		{"11:59", true},
		{"00:00", true},
		{"12:00", false},
		{"13:30", false},
		{"23:59", false},
		{"", false},
		{"bad", false},
	}
	for _, tt := range tests {
		if got := IsAM(tt.deadline); got != tt.want {
			t.Errorf("IsAM(%q) = %v, want %v", tt.deadline, got, tt.want)
		}
	}
}

func TestDeadlineIndicator(t *testing.T) {
	now := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		deadline string
		want     string
	}{
		{"empty", "", ""},
		{"invalid", "bad", ""},
		{"in 2h", "12:00", "in 2h"},
		{"in 2h 30m", "12:30", "in 2h 30m"},
		{"in 45m", "10:45", "in 45m"},
		{"in 5m", "10:05", "in 5m"},
		{"now", "10:00", "now"},
		{"30m ago", "09:30", "30m ago"},
		{"1h 30m ago", "08:30", "1h 30m ago"},
		{"3h ago", "07:00", "3h ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeadlineIndicator(tt.deadline, now)
			if got != tt.want {
				t.Errorf("DeadlineIndicator(%q, 10:00) = %q, want %q", tt.deadline, got, tt.want)
			}
		})
	}
}

func TestIsVisibleOn(t *testing.T) {
	t.Run("nil visibility means every day", func(t *testing.T) {
		task := Task{ID: 1, Title: "Daily"}
		for d := time.Sunday; d <= time.Saturday; d++ {
			if !task.IsVisibleOn(d) {
				t.Errorf("expected visible on %s", d)
			}
		}
	})

	t.Run("empty visibility means every day", func(t *testing.T) {
		task := Task{ID: 1, Title: "Daily", Visibility: []int{}}
		if !task.IsVisibleOn(time.Monday) {
			t.Error("expected visible on Monday")
		}
	})

	t.Run("specific days", func(t *testing.T) {
		task := Task{ID: 1, Title: "MWF", Visibility: []int{1, 3, 5}} // Mon, Wed, Fri
		if !task.IsVisibleOn(time.Monday) {
			t.Error("expected visible on Monday")
		}
		if !task.IsVisibleOn(time.Wednesday) {
			t.Error("expected visible on Wednesday")
		}
		if task.IsVisibleOn(time.Tuesday) {
			t.Error("expected NOT visible on Tuesday")
		}
		if task.IsVisibleOn(time.Sunday) {
			t.Error("expected NOT visible on Sunday")
		}
	})
}

func TestVisibleTasksOn(t *testing.T) {
	tasks := []Task{
		{ID: 1, Title: "Daily", Visibility: nil},
		{ID: 2, Title: "MWF", Visibility: []int{1, 3, 5}},
		{ID: 3, Title: "Weekends", Visibility: []int{0, 6}},
	}

	mon := VisibleTasksOn(tasks, time.Monday)
	if len(mon) != 2 {
		t.Fatalf("expected 2 visible on Monday, got %d", len(mon))
	}
	if mon[0].ID != 1 || mon[1].ID != 2 {
		t.Errorf("unexpected Monday tasks: %v", mon)
	}

	sun := VisibleTasksOn(tasks, time.Sunday)
	if len(sun) != 2 {
		t.Fatalf("expected 2 visible on Sunday, got %d", len(sun))
	}
	if sun[0].ID != 1 || sun[1].ID != 3 {
		t.Errorf("unexpected Sunday tasks: %v", sun)
	}
}

func TestWeekdayFromDate(t *testing.T) {
	// 2026-04-15 is a Wednesday
	wd, err := WeekdayFromDate("2026-04-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wd != time.Wednesday {
		t.Errorf("expected Wednesday, got %s", wd)
	}

	_, err = WeekdayFromDate("bad-date")
	if err == nil {
		t.Error("expected error for bad date")
	}
}

func TestCloneDataPreservesVisibility(t *testing.T) {
	original := Data{
		LastReset: "2026-01-23",
		NextID:    2,
		Tasks: []Task{
			{ID: 1, Title: "MWF", Duration: 5, Status: "todo", Order: 1, Visibility: []int{1, 3, 5}},
		},
	}

	clone := CloneData(original)
	clone.Tasks[0].Visibility[0] = 0

	if original.Tasks[0].Visibility[0] != 1 {
		t.Error("original visibility was modified through clone")
	}
}

func TestClampIndex(t *testing.T) {
	tests := []struct {
		idx, length, want int
	}{
		{0, 5, 0},
		{2, 5, 2},
		{4, 5, 4},
		{5, 5, 4},
		{10, 5, 4},
		{-1, 5, 0},
		{0, 0, 0},
		{5, 0, 0},
	}

	for _, tt := range tests {
		got := ClampIndex(tt.idx, tt.length)
		if got != tt.want {
			t.Errorf("ClampIndex(%d, %d) = %d, want %d", tt.idx, tt.length, got, tt.want)
		}
	}
}
