package internal

import "testing"

func TestHistoryPath(t *testing.T) {
	if got := HistoryPath("/tmp/tasks.json"); got != "/tmp/tasks.history.json" {
		t.Fatalf("expected history sibling path, got %q", got)
	}
	if got := HistoryPath("/tmp/tasks"); got != "/tmp/tasks.history.json" {
		t.Fatalf("expected history suffix path, got %q", got)
	}
}

func TestSaveDataWithHistoryRecordsSnapshotsAndEvents(t *testing.T) {
	path := t.TempDir() + "/tasks.json"
	before := Data{
		LastReset: "2026-04-07",
		NextID:    2,
		Tasks: []Task{
			{ID: 1, Title: "Workout", Duration: 30, Status: "todo", Order: 1},
		},
	}
	after := CloneData(before)
	after.Tasks[0].Status = "done"
	after.Tasks = append(after.Tasks, Task{ID: 2, Title: "Read", Duration: 20, Status: "todo", Order: 2})
	after.NextID = 3

	if err := SaveDataWithHistory(path, before, after); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("failed to load history: %v", err)
	}
	if len(history.Days) != 1 {
		t.Fatalf("expected 1 recorded day, got %d", len(history.Days))
	}
	if len(history.Days[0].Tasks) != 2 {
		t.Fatalf("expected 2 tasks in daily snapshot, got %d", len(history.Days[0].Tasks))
	}
	if history.Days[0].Tasks[0].Status != "done" {
		t.Fatalf("expected latest snapshot to keep updated status, got %q", history.Days[0].Tasks[0].Status)
	}
	if len(history.Events) != 2 {
		t.Fatalf("expected 2 history events, got %d", len(history.Events))
	}
}

func TestSaveDataWithHistoryPreservesPreResetDay(t *testing.T) {
	path := t.TempDir() + "/tasks.json"
	before := Data{
		LastReset: "2026-04-07",
		Tasks: []Task{
			{ID: 1, Title: "Workout", Duration: 30, Status: "done", Order: 1},
			{ID: 2, Title: "Read", Duration: 20, Status: "skipped", Order: 1},
		},
	}
	after := Data{
		LastReset: "2026-04-08",
		Tasks: []Task{
			{ID: 1, Title: "Workout", Duration: 30, Status: "todo", Order: 1},
			{ID: 2, Title: "Read", Duration: 20, Status: "todo", Order: 2},
		},
	}

	if err := SaveDataWithHistory(path, before, after); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("failed to load history: %v", err)
	}
	if len(history.Days) != 2 {
		t.Fatalf("expected 2 recorded days, got %d", len(history.Days))
	}
	if history.Days[0].Date != "2026-04-07" || history.Days[1].Date != "2026-04-08" {
		t.Fatalf("unexpected recorded dates: %+v", history.Days)
	}
	if history.Days[0].Tasks[0].Status != "done" || history.Days[0].Tasks[1].Status != "skipped" {
		t.Fatalf("expected pre-reset statuses to be preserved, got %+v", history.Days[0].Tasks)
	}
	if history.Days[1].Tasks[0].Status != "todo" || history.Days[1].Tasks[1].Status != "todo" {
		t.Fatalf("expected reset day statuses to be todo, got %+v", history.Days[1].Tasks)
	}
}

func TestAggregateStats(t *testing.T) {
	history := History{
		Version: 1,
		Days: []HistoryDay{
			{
				Date: "2026-04-07",
				Tasks: []TaskSnapshot{
					{ID: 1, Title: "Workout", Duration: 30, Status: "done"},
					{ID: 2, Title: "Read", Duration: 20, Status: "todo"},
				},
			},
			{
				Date: "2026-04-08",
				Tasks: []TaskSnapshot{
					{ID: 1, Title: "Workout", Duration: 30, Status: "done"},
					{ID: 2, Title: "Read", Duration: 20, Status: "skipped"},
				},
			},
		},
	}

	stats := AggregateStats(history, "2026-04-07", "2026-04-08")
	if stats.RecordedDays != 2 {
		t.Fatalf("expected 2 recorded days, got %d", stats.RecordedDays)
	}
	if stats.DoneCount != 2 || stats.TodoCount != 1 || stats.SkippedCount != 1 {
		t.Fatalf("unexpected aggregate counts: %+v", stats)
	}
	if stats.DoneDuration != 60 || stats.TodoDuration != 20 || stats.SkippedDuration != 20 {
		t.Fatalf("unexpected aggregate durations: %+v", stats)
	}
	if len(stats.Tasks) != 2 {
		t.Fatalf("expected 2 task frequency rows, got %d", len(stats.Tasks))
	}
	if stats.Tasks[0].Title != "Workout" || stats.Tasks[0].DoneDays != 2 {
		t.Fatalf("expected workout to be top completed task, got %+v", stats.Tasks[0])
	}
}
