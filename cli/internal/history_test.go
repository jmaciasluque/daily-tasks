package internal

import (
	"testing"
	"time"
)

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

func TestSaveDataWithHistoryRecordsDetailedEventsAndDeadlines(t *testing.T) {
	path := t.TempDir() + "/tasks.json"
	before := Data{
		LastReset: "2026-04-09",
		NextID:    4,
		Tasks: []Task{
			{ID: 1, Title: "Plan day", Duration: 30, Status: "todo", Order: 1, Deadline: "09:00"},
			{ID: 2, Title: "Review", Duration: 20, Status: "done", Order: 1, Deadline: "15:00"},
			{ID: 3, Title: "Archive old plan", Duration: 10, Status: "skipped", Order: 1, Deadline: "18:00"},
		},
	}
	after := CloneData(before)
	after.Tasks[0].Status = "done"
	after.Tasks[1].Title = "Review notes"
	after.Tasks[1].Duration = 25
	after.Tasks[1].Deadline = "16:30"
	after.Tasks = append(after.Tasks[:2], Task{ID: 4, Title: "Write summary", Duration: 45, Status: "todo", Order: 2, Deadline: "11:15"})
	after.NextID = 5

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

	snapshotByID := map[int]TaskSnapshot{}
	for _, snapshot := range history.Days[0].Tasks {
		snapshotByID[snapshot.ID] = snapshot
	}
	if len(snapshotByID) != 3 {
		t.Fatalf("expected 3 tasks in latest snapshot, got %+v", history.Days[0].Tasks)
	}
	if snapshotByID[1].Status != "done" || snapshotByID[1].Deadline != "09:00" {
		t.Fatalf("expected task 1 snapshot to keep done status and deadline, got %+v", snapshotByID[1])
	}
	if snapshotByID[2].Title != "Review notes" || snapshotByID[2].Duration != 25 || snapshotByID[2].Deadline != "16:30" {
		t.Fatalf("expected task 2 snapshot to keep updated fields, got %+v", snapshotByID[2])
	}
	if _, ok := snapshotByID[3]; ok {
		t.Fatalf("deleted task should not be in latest snapshot: %+v", history.Days[0].Tasks)
	}
	if snapshotByID[4].Status != "todo" || snapshotByID[4].Deadline != "11:15" {
		t.Fatalf("expected task 4 snapshot to keep todo status and deadline, got %+v", snapshotByID[4])
	}

	if len(history.Events) != 4 {
		t.Fatalf("expected 4 history events, got %+v", history.Events)
	}
	findEvent := func(taskID int, eventType string) HistoryEvent {
		t.Helper()
		for _, event := range history.Events {
			if event.TaskID == taskID && event.Type == eventType {
				return event
			}
		}
		t.Fatalf("missing %s event for task %d in %+v", eventType, taskID, history.Events)
		return HistoryEvent{}
	}

	statusEvent := findEvent(1, "status_changed")
	if statusEvent.Date != "2026-04-09" || statusEvent.FromStatus != "todo" || statusEvent.ToStatus != "done" ||
		statusEvent.Duration != 30 || statusEvent.Deadline != "09:00" {
		t.Fatalf("unexpected status event: %+v", statusEvent)
	}

	updateEvent := findEvent(2, "task_updated")
	if updateEvent.Date != "2026-04-09" || updateEvent.Title != "Review notes" || updateEvent.ToStatus != "done" ||
		updateEvent.Duration != 25 || updateEvent.Deadline != "16:30" {
		t.Fatalf("unexpected update event: %+v", updateEvent)
	}

	deleteEvent := findEvent(3, "task_deleted")
	if deleteEvent.Date != "2026-04-09" || deleteEvent.Title != "Archive old plan" || deleteEvent.FromStatus != "skipped" ||
		deleteEvent.Duration != 10 || deleteEvent.Deadline != "18:00" {
		t.Fatalf("unexpected delete event: %+v", deleteEvent)
	}

	addEvent := findEvent(4, "task_added")
	if addEvent.Date != "2026-04-09" || addEvent.Title != "Write summary" || addEvent.ToStatus != "todo" ||
		addEvent.Duration != 45 || addEvent.Deadline != "11:15" {
		t.Fatalf("unexpected add event: %+v", addEvent)
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

func TestSnapshotFiltersInvisibleTasks(t *testing.T) {
	// 2026-04-07 is a Tuesday (weekday 2)
	tasks := []Task{
		{ID: 1, Title: "Daily", Duration: 10, Status: "done"},
		{ID: 2, Title: "MWF only", Duration: 20, Status: "todo", Visibility: []int{1, 3, 5}},
		{ID: 3, Title: "Tue/Thu", Duration: 15, Status: "done", Visibility: []int{2, 4}},
	}

	snapshots := snapshotsForVisibleTasks(tasks, "2026-04-07")
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 visible snapshots on Tuesday, got %d", len(snapshots))
	}
	// Should include task 1 (daily) and task 3 (Tue/Thu)
	ids := map[int]bool{}
	for _, s := range snapshots {
		ids[s.ID] = true
	}
	if !ids[1] || !ids[3] {
		t.Errorf("unexpected snapshot IDs: %v", snapshots)
	}
	if ids[2] {
		t.Error("MWF task should not appear on Tuesday")
	}
}

func TestSnapshotVisibilityPreservesField(t *testing.T) {
	tasks := []Task{
		{ID: 1, Title: "MWF", Duration: 10, Status: "todo", Visibility: []int{1, 3, 5}},
	}
	// Monday
	snapshots := snapshotsForVisibleTasks(tasks, "2026-04-06")
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot on Monday, got %d", len(snapshots))
	}
	if len(snapshots[0].Visibility) != 3 {
		t.Errorf("expected visibility to be preserved in snapshot, got %v", snapshots[0].Visibility)
	}
}

func TestStatsWithVisibility(t *testing.T) {
	// Simulate: task 1 is daily, task 2 is MWF only
	// Day 1: Tuesday 2026-04-07 — only task 1 visible
	// Day 2: Wednesday 2026-04-08 — both tasks visible
	path := t.TempDir() + "/tasks.json"

	// Tuesday: both tasks exist but task 2 has MWF visibility
	beforeTue := Data{
		LastReset: "2026-04-07",
		Tasks: []Task{
			{ID: 1, Title: "Daily", Duration: 10, Status: "done"},
			{ID: 2, Title: "MWF", Duration: 20, Status: "todo", Visibility: []int{1, 3, 5}},
		},
	}
	afterTue := CloneData(beforeTue)
	if err := SaveDataWithHistory(path, beforeTue, afterTue); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wednesday: both visible, both done
	beforeWed := Data{
		LastReset: "2026-04-08",
		Tasks: []Task{
			{ID: 1, Title: "Daily", Duration: 10, Status: "done"},
			{ID: 2, Title: "MWF", Duration: 20, Status: "done", Visibility: []int{1, 3, 5}},
		},
	}
	afterWed := CloneData(beforeWed)
	if err := SaveDataWithHistory(path, beforeWed, afterWed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("failed to load history: %v", err)
	}

	// Tuesday snapshot should only have task 1
	var tueDaySnap *HistoryDay
	var wedDaySnap *HistoryDay
	for i := range history.Days {
		switch history.Days[i].Date {
		case "2026-04-07":
			tueDaySnap = &history.Days[i]
		case "2026-04-08":
			wedDaySnap = &history.Days[i]
		}
	}

	if tueDaySnap == nil || wedDaySnap == nil {
		t.Fatalf("expected both days in history, got %+v", history.Days)
	}
	if len(tueDaySnap.Tasks) != 1 {
		t.Errorf("Tuesday should have 1 visible task, got %d", len(tueDaySnap.Tasks))
	}
	if len(wedDaySnap.Tasks) != 2 {
		t.Errorf("Wednesday should have 2 visible tasks, got %d", len(wedDaySnap.Tasks))
	}

	// Stats should reflect correct completion rates
	stats := AggregateStats(history, "2026-04-07", "2026-04-08")

	// Tuesday: 1 task, 1 done => 100%; Wednesday: 2 tasks, 2 done => 100%
	if stats.Daily[0].TaskCount != 1 {
		t.Errorf("Tuesday should count 1 task, got %d", stats.Daily[0].TaskCount)
	}
	if stats.Daily[1].TaskCount != 2 {
		t.Errorf("Wednesday should count 2 tasks, got %d", stats.Daily[1].TaskCount)
	}
	if stats.CompletionRate != 1.0 {
		t.Errorf("expected 100%% completion rate, got %.2f", stats.CompletionRate)
	}

	_ = time.Wednesday // ensure time import is used
}

func TestDailyResetHistoryIgnoresHiddenTasks(t *testing.T) {
	path := t.TempDir() + "/tasks.json"
	today := time.Now().Format("2006-01-02")
	todayWeekday := int(time.Now().Weekday())
	hiddenDay := (todayWeekday + 1) % 7
	before := Data{
		LastReset: "2020-01-01",
		Tasks: []Task{
			{ID: 1, Title: "Visible", Duration: 10, Status: "done", Order: 1},
			{ID: 2, Title: "Hidden", Duration: 20, Status: "done", Order: 2, Visibility: []int{hiddenDay}},
		},
	}
	after := CloneData(before)
	if !ResetIfNewDay(&after) {
		t.Fatal("expected reset")
	}
	if err := SaveDataWithHistory(path, before, after); err != nil {
		t.Fatalf("failed to save data with history: %v", err)
	}

	history, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("failed to load history: %v", err)
	}
	var todaySnapshot *HistoryDay
	for i := range history.Days {
		if history.Days[i].Date == today {
			todaySnapshot = &history.Days[i]
			break
		}
	}
	if todaySnapshot == nil {
		t.Fatalf("expected history snapshot for %s, got %+v", today, history.Days)
	}
	if len(todaySnapshot.Tasks) != 1 || todaySnapshot.Tasks[0].ID != 1 || todaySnapshot.Tasks[0].Status != "todo" {
		t.Fatalf("expected only visible task reset in today's snapshot, got %+v", todaySnapshot.Tasks)
	}

	if len(history.Events) != 1 {
		t.Fatalf("expected one reset status event, got %+v", history.Events)
	}
	event := history.Events[0]
	if event.TaskID != 1 || event.Type != "status_changed" || event.FromStatus != "done" || event.ToStatus != "todo" {
		t.Fatalf("unexpected reset event: %+v", event)
	}
}

// ---------------------------------------------------------------------------
// HistoryWithCurrentSnapshot
// ---------------------------------------------------------------------------

func TestHistoryWithCurrentSnapshotAddsDay(t *testing.T) {
	data := Data{
		LastReset: "2026-05-01",
		NextID:    2,
		Tasks: []Task{
			{ID: 1, Title: "Morning run", Duration: 30, Status: "done", Order: 1},
		},
	}
	history := History{Version: 1}
	now := int64(1000000000000)

	got := HistoryWithCurrentSnapshot(history, data, now)
	if len(got.Days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(got.Days))
	}
	if got.Days[0].Date != "2026-05-01" {
		t.Errorf("unexpected date: %q", got.Days[0].Date)
	}
	if len(got.Days[0].Tasks) != 1 || got.Days[0].Tasks[0].Title != "Morning run" {
		t.Errorf("unexpected tasks: %+v", got.Days[0].Tasks)
	}
}

func TestHistoryWithCurrentSnapshotUpdatesExistingDay(t *testing.T) {
	data := Data{
		LastReset: "2026-05-01",
		NextID:    2,
		Tasks: []Task{
			{ID: 1, Title: "Read", Duration: 20, Status: "done", Order: 1},
		},
	}
	history := History{
		Version: 1,
		Days: []HistoryDay{
			{
				Date:      "2026-05-01",
				UpdatedAt: 500,
				Tasks:     []TaskSnapshot{{ID: 1, Title: "Read", Duration: 20, Status: "todo"}},
			},
		},
	}

	got := HistoryWithCurrentSnapshot(history, data, 1000)
	if len(got.Days) != 1 {
		t.Fatalf("expected still 1 day, got %d", len(got.Days))
	}
	if got.Days[0].Tasks[0].Status != "done" {
		t.Errorf("expected updated status, got %q", got.Days[0].Tasks[0].Status)
	}
}

func TestHistoryWithCurrentSnapshotZeroTimestamp(t *testing.T) {
	data := Data{LastReset: "2026-05-02", NextID: 1}
	history := History{}
	// now=0 means use real time
	got := HistoryWithCurrentSnapshot(history, data, 0)
	if got.Version != historyVersion {
		t.Errorf("expected version %d, got %d", historyVersion, got.Version)
	}
}

// ---------------------------------------------------------------------------
// HistoryContentEqual
// ---------------------------------------------------------------------------

func TestHistoryContentEqualIdentical(t *testing.T) {
	h := History{
		Version: 1,
		Days: []HistoryDay{
			{Date: "2026-05-01", Tasks: []TaskSnapshot{{ID: 1, Title: "T", Duration: 10, Status: "done"}}},
		},
	}
	if !HistoryContentEqual(h, h) {
		t.Error("identical histories should be equal")
	}
}

func TestHistoryContentEqualDifferentDayCount(t *testing.T) {
	a := History{Version: 1, Days: []HistoryDay{{Date: "2026-05-01"}}}
	b := History{Version: 1}
	if HistoryContentEqual(a, b) {
		t.Error("histories with different day counts should not be equal")
	}
}

func TestHistoryContentEqualDifferentTaskStatus(t *testing.T) {
	day := func(status string) History {
		return History{
			Version: 1,
			Days: []HistoryDay{
				{Date: "2026-05-01", Tasks: []TaskSnapshot{{ID: 1, Title: "T", Duration: 10, Status: status}}},
			},
		}
	}
	if HistoryContentEqual(day("todo"), day("done")) {
		t.Error("histories with different task status should not be equal")
	}
}

func TestHistoryContentEqualDifferentEvents(t *testing.T) {
	base := History{Version: 1}
	withEvent := History{
		Version: 1,
		Events:  []HistoryEvent{{Timestamp: 1, Date: "2026-05-01", Type: "task_added", TaskID: 1}},
	}
	if HistoryContentEqual(base, withEvent) {
		t.Error("histories with different event counts should not be equal")
	}
}

func TestHistoryContentEqualVersionZeroNormalized(t *testing.T) {
	a := History{Version: 0}
	b := History{Version: 1}
	if !HistoryContentEqual(a, b) {
		t.Error("version 0 should be normalized to 1")
	}
}

// ---------------------------------------------------------------------------
// MergeHistories
// ---------------------------------------------------------------------------

func TestMergeHistoriesEmptyBoth(t *testing.T) {
	merged := MergeHistories(History{}, History{})
	if merged.Version != historyVersion {
		t.Errorf("expected version %d, got %d", historyVersion, merged.Version)
	}
	if len(merged.Days) != 0 || len(merged.Events) != 0 {
		t.Errorf("expected empty merge, got %+v", merged)
	}
}

func TestMergeHistoriesPicksNewerDay(t *testing.T) {
	local := History{
		Version: 1,
		Days: []HistoryDay{
			{Date: "2026-05-01", UpdatedAt: 100, Tasks: []TaskSnapshot{{ID: 1, Status: "todo"}}},
		},
	}
	remote := History{
		Version: 1,
		Days: []HistoryDay{
			{Date: "2026-05-01", UpdatedAt: 200, Tasks: []TaskSnapshot{{ID: 1, Status: "done"}}},
		},
	}

	merged := MergeHistories(local, remote)
	if len(merged.Days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(merged.Days))
	}
	if merged.Days[0].Tasks[0].Status != "done" {
		t.Errorf("expected remote (newer) day to win, got %q", merged.Days[0].Tasks[0].Status)
	}
}

func TestMergeHistoriesCombinesDifferentDays(t *testing.T) {
	local := History{
		Version: 1,
		Days:    []HistoryDay{{Date: "2026-05-01", Tasks: []TaskSnapshot{{ID: 1, Status: "done"}}}},
	}
	remote := History{
		Version: 1,
		Days:    []HistoryDay{{Date: "2026-05-02", Tasks: []TaskSnapshot{{ID: 1, Status: "todo"}}}},
	}

	merged := MergeHistories(local, remote)
	if len(merged.Days) != 2 {
		t.Fatalf("expected 2 merged days, got %d", len(merged.Days))
	}
}

func TestMergeHistoriesDeduplicatesEvents(t *testing.T) {
	event := HistoryEvent{
		Timestamp: 1000, Date: "2026-05-01", Type: "task_added",
		TaskID: 1, Title: "Task A", ToStatus: "todo", Duration: 10,
	}
	local := History{Version: 1, Events: []HistoryEvent{event}}
	remote := History{Version: 1, Events: []HistoryEvent{event}}

	merged := MergeHistories(local, remote)
	if len(merged.Events) != 1 {
		t.Errorf("expected deduplication to 1 event, got %d", len(merged.Events))
	}
}

func TestMergeHistoriesCombinesUniqueEvents(t *testing.T) {
	e1 := HistoryEvent{Timestamp: 1000, Date: "2026-05-01", Type: "task_added", TaskID: 1, Duration: 10}
	e2 := HistoryEvent{Timestamp: 2000, Date: "2026-05-01", Type: "status_changed", TaskID: 1, Duration: 10}
	local := History{Version: 1, Events: []HistoryEvent{e1}}
	remote := History{Version: 1, Events: []HistoryEvent{e2}}

	merged := MergeHistories(local, remote)
	if len(merged.Events) != 2 {
		t.Errorf("expected 2 unique events, got %d", len(merged.Events))
	}
}

func TestMergeHistoriesTakesMaxVersion(t *testing.T) {
	local := History{Version: 1}
	remote := History{Version: 2}
	merged := MergeHistories(local, remote)
	if merged.Version != 2 {
		t.Errorf("expected version 2, got %d", merged.Version)
	}
}

func TestMergeHistoriesTakesMaxUpdatedAt(t *testing.T) {
	local := History{Version: 1, UpdatedAt: 500}
	remote := History{Version: 1, UpdatedAt: 999}
	merged := MergeHistories(local, remote)
	if merged.UpdatedAt != 999 {
		t.Errorf("expected UpdatedAt 999, got %d", merged.UpdatedAt)
	}
}

// ---------------------------------------------------------------------------
// EnsureHistorySnapshot
// ---------------------------------------------------------------------------

func TestEnsureHistorySnapshotCreatesSnapshot(t *testing.T) {
	path := t.TempDir() + "/tasks.json"
	data := Data{
		LastReset: "2026-05-03",
		NextID:    2,
		Tasks:     []Task{{ID: 1, Title: "Walk", Duration: 15, Status: "done", Order: 1}},
	}

	if err := EnsureHistorySnapshot(path, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("failed to load history: %v", err)
	}
	if len(history.Days) != 1 || history.Days[0].Date != "2026-05-03" {
		t.Fatalf("expected snapshot for 2026-05-03, got %+v", history.Days)
	}
}

func TestEnsureHistorySnapshotNoSaveWhenUnchanged(t *testing.T) {
	path := t.TempDir() + "/tasks.json"
	data := Data{
		LastReset: "2026-05-03",
		NextID:    2,
		Tasks:     []Task{{ID: 1, Title: "Walk", Duration: 15, Status: "done", Order: 1}},
	}

	// First call creates snapshot
	if err := EnsureHistorySnapshot(path, data); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	h1, _ := LoadHistory(path)

	// Second call with same data should be a no-op (UpdatedAt not bumped)
	if err := EnsureHistorySnapshot(path, data); err != nil {
		t.Fatalf("second call error: %v", err)
	}
	h2, _ := LoadHistory(path)

	if h1.UpdatedAt != h2.UpdatedAt {
		t.Errorf("expected no-op on unchanged data, but UpdatedAt changed %d -> %d", h1.UpdatedAt, h2.UpdatedAt)
	}
}

// ---------------------------------------------------------------------------
// BuildStats
// ---------------------------------------------------------------------------

func TestBuildStats(t *testing.T) {
	path := t.TempDir() + "/tasks.json"

	before := Data{
		LastReset: "2026-04-10",
		NextID:    3,
		Tasks: []Task{
			{ID: 1, Title: "Run", Duration: 30, Status: "done", Order: 1},
			{ID: 2, Title: "Read", Duration: 20, Status: "skipped", Order: 1},
		},
	}
	after := Data{
		LastReset: "2026-04-11",
		NextID:    3,
		Tasks: []Task{
			{ID: 1, Title: "Run", Duration: 30, Status: "todo", Order: 1},
			{ID: 2, Title: "Read", Duration: 20, Status: "todo", Order: 2},
		},
	}
	if err := SaveDataWithHistory(path, before, after); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	stats, err := BuildStats(path, after, "2026-04-10", "2026-04-11")
	if err != nil {
		t.Fatalf("BuildStats error: %v", err)
	}
	if stats.RecordedDays < 1 {
		t.Errorf("expected at least 1 recorded day, got %d", stats.RecordedDays)
	}
	if stats.DoneCount < 1 {
		t.Errorf("expected at least 1 done count, got %d", stats.DoneCount)
	}
	if stats.From != "2026-04-10" || stats.To != "2026-04-11" {
		t.Errorf("unexpected from/to: %q / %q", stats.From, stats.To)
	}
}

func TestBuildStatsCreatesSnapshotForCurrentData(t *testing.T) {
	path := t.TempDir() + "/tasks.json"
	today := time.Now().Format("2006-01-02")

	current := Data{
		LastReset: today,
		NextID:    2,
		Tasks:     []Task{{ID: 1, Title: "Meditate", Duration: 10, Status: "done", Order: 1}},
	}

	stats, err := BuildStats(path, current, today, today)
	if err != nil {
		t.Fatalf("BuildStats error: %v", err)
	}
	if stats.RecordedDays != 1 {
		t.Errorf("expected 1 recorded day for today, got %d", stats.RecordedDays)
	}
}
