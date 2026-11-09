package main

import (
	"testing"
	"time"

	"daily-tasks/internal"
)

func TestRunEditMultipleFlags(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Old", Duration: 10, Status: "todo", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := runEdit([]string{"--id", "1", "--title", "New", "--duration", "25", "--deadline", "18:00", "--visibility", "mon,wed,fri"}); err != nil {
		t.Fatalf("runEdit: %v", err)
	}

	got, _ := internal.LoadData(dataPath)
	task := internal.FindTask(&got, 1)
	if task.Title != "New" {
		t.Fatalf("title: got %q", task.Title)
	}
	if task.Duration != 25 {
		t.Fatalf("duration: got %d", task.Duration)
	}
	if task.Deadline != "18:00" {
		t.Fatalf("deadline: got %q", task.Deadline)
	}
	if len(task.Visibility) != 3 || task.Visibility[0] != 1 {
		t.Fatalf("visibility: got %v", task.Visibility)
	}
}

func TestRunEditVisibilityClear(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Gym", Duration: 30, Status: "todo", Order: 1, Visibility: []int{1, 3, 5}}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := runEdit([]string{"--id", "1", "--visibility", ""}); err != nil {
		t.Fatalf("runEdit: %v", err)
	}

	got, _ := internal.LoadData(dataPath)
	task := internal.FindTask(&got, 1)
	if len(task.Visibility) != 0 {
		t.Fatalf("expected visibility cleared, got %v", task.Visibility)
	}
}

func TestRunEditStatus(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Task", Duration: 5, Status: "todo", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := runEdit([]string{"--id", "1", "--status", "done"}); err != nil {
		t.Fatalf("runEdit: %v", err)
	}

	got, _ := internal.LoadData(dataPath)
	task := internal.FindTask(&got, 1)
	if task.Status != "done" {
		t.Fatalf("expected status done, got %q", task.Status)
	}
}

func TestRunEditNoFlags(t *testing.T) {
	setupCLIEnv(t)
	err := runEdit([]string{"--id", "1"})
	if err == nil {
		t.Fatal("expected error for no update flags")
	}
}

func TestRunEditNotFound(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Existing", Duration: 5, Status: "todo", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := runEdit([]string{"--id", "99", "--title", "Nope"})
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestRunEditInvalidStatus(t *testing.T) {
	setupCLIEnv(t)
	err := runEdit([]string{"--id", "1", "--status", "unknown"})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}