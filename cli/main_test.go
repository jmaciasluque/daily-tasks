package main

import (
	"strings"
	"testing"
	"time"

	"daily-tasks/internal"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestOverlayCenteredPlacesOverlayInMiddle(t *testing.T) {
	base := strings.Join([]string{
		"............",
		"............",
		"............",
		"............",
		"............",
	}, "\n")
	overlay := strings.Join([]string{
		"ABCD",
		"EFGH",
	}, "\n")

	got := overlayCentered(base, overlay, 12, 5)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}

	if !strings.Contains(lines[1], "ABCD") {
		t.Fatalf("expected overlay first row in line 2, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "EFGH") {
		t.Fatalf("expected overlay second row in line 3, got %q", lines[2])
	}
}

func TestOverlayCenteredPreservesANSIStyledBase(t *testing.T) {
	base := strings.Join([]string{
		"\x1b[31mHello World\x1b[0m",
		"\x1b[32mHello World\x1b[0m",
		"\x1b[34mHello World\x1b[0m",
	}, "\n")
	overlay := "BOX"

	got := overlayCentered(base, overlay, 11, 3)
	stripped := ansi.Strip(got)
	lines := strings.Split(stripped, "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[1], "BOX") {
		t.Fatalf("expected overlay to appear on middle line, got %q", lines[1])
	}
	if !strings.Contains(lines[0], "Hello World") || !strings.Contains(lines[2], "Hello World") {
		t.Fatalf("expected non-overlay lines to remain readable, got %#v", lines)
	}
}

func TestViewRendersEditOverlay(t *testing.T) {
	data := internal.Data{
		LastReset: "2026-04-14",
		NextID:    2,
		Tasks: []internal.Task{
			{ID: 1, Title: "Meditate", Duration: 5, Status: "todo", Order: 1, Deadline: "06:40"},
		},
	}

	m := newModel(data, "/tmp/tasks.json")
	m.width = 100
	m.height = 30
	m.resizeLists()
	m.syncLists()
	m.mode = modeEdit
	m.editID = 1
	m.titleInput.SetValue("Meditate")
	m.durInput.SetValue("5")
	m.deadlineInput.SetValue("06:40")

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Edit task") {
		t.Fatal("expected edit modal title in view output")
	}
	if !strings.Contains(view, "Meditate") {
		t.Fatal("expected task title in edit modal output")
	}
	if !strings.Contains(view, "To Do") {
		t.Fatal("expected task list to remain visible behind overlay")
	}
}

func TestViewRendersStatsScreenAndVersion(t *testing.T) {
	data := internal.Data{
		LastReset: "2026-04-14",
		NextID:    2,
		Tasks: []internal.Task{
			{ID: 1, Title: "Meditate", Duration: 5, Status: "done", Order: 1, Deadline: "06:40"},
		},
	}

	m := newModel(data, t.TempDir()+"/tasks.json")
	m.width = 120
	m.height = 32
	m.screen = screenStats
	m.refreshStats()

	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Daily Tasks v") {
		t.Fatal("expected version in TUI header")
	}
	if !strings.Contains(view, "Stats") {
		t.Fatal("expected stats screen title in view output")
	}
	if !strings.Contains(view, "version: ") {
		t.Fatal("expected version in footer output")
	}
	if !strings.Contains(view, "Recorded Days") {
		t.Fatal("expected stats cards in view output")
	}
}

func TestCycleStatsPeriodWraps(t *testing.T) {
	m := newModel(internal.Data{LastReset: "2026-04-14", NextID: 1}, t.TempDir()+"/tasks.json")
	m.statsPeriod = 0
	m.cycleStatsPeriod(-1)
	if m.statsPeriod != len(statsPeriods)-1 {
		t.Fatalf("expected wrap to last period, got %d", m.statsPeriod)
	}
	m.cycleStatsPeriod(1)
	if m.statsPeriod != 0 {
		t.Fatalf("expected wrap back to first period, got %d", m.statsPeriod)
	}
}

func TestCompletionStatusMessageIncludesPercent(t *testing.T) {
	got := completionStatusMessage(1, 4)
	if !strings.Contains(got, "1/4") {
		t.Fatalf("expected count in message, got %q", got)
	}
	if !strings.Contains(got, "25%") {
		t.Fatalf("expected percent in message, got %q", got)
	}
}

func TestCompletionStatusMessageAllDone(t *testing.T) {
	got := completionStatusMessage(3, 3)
	if !strings.Contains(got, "100%") {
		t.Fatalf("expected 100 percent in message, got %q", got)
	}
	if !strings.Contains(got, "All tasks done today") {
		t.Fatalf("expected all-done message, got %q", got)
	}
}

func TestEnterOnTodoSetsMotivationalStatus(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	data := internal.Data{
		LastReset: today,
		NextID:    3,
		Tasks: []internal.Task{
			{ID: 1, Title: "A", Duration: 10, Status: "todo", Order: 1},
			{ID: 2, Title: "B", Duration: 20, Status: "todo", Order: 2},
		},
	}

	m := newModel(data, t.TempDir()+"/tasks.json")
	updatedModel, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	updated := updatedModel.(model)

	task := internal.FindTask(&updated.data, 1)
	if task == nil || task.Status != "done" {
		t.Fatalf("expected selected task to be marked done, got %#v", task)
	}
	if !strings.Contains(updated.statusMsg, "50%") {
		t.Fatalf("expected motivational message with percent, got %q", updated.statusMsg)
	}
}

func TestAllTasksViewCanEditHiddenTask(t *testing.T) {
	today := int(time.Now().Weekday())
	hiddenDay := (today + 1) % 7
	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    3,
		Tasks: []internal.Task{
			{ID: 1, Title: "Visible", Duration: 10, Status: "todo", Order: 1},
			{ID: 2, Title: "Hidden", Duration: 20, Status: "todo", Order: 2, Visibility: []int{hiddenDay}},
		},
	}

	m := newModel(data, t.TempDir()+"/tasks.json")
	if got := len(m.lists[0].Items()); got != 1 {
		t.Fatalf("expected only visible task in default view, got %d", got)
	}

	updatedModel, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	updated := updatedModel.(model)
	if got := len(updated.lists[0].Items()); got != 2 {
		t.Fatalf("expected hidden task in all-tasks view, got %d items", got)
	}

	updated.lists[0].Select(1)
	editModel, _ := updated.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	edit := editModel.(model)
	if edit.mode != modeEdit || edit.editID != 2 {
		t.Fatalf("expected hidden task to open for edit, mode=%v editID=%d", edit.mode, edit.editID)
	}
}

func TestAllTasksViewBlocksStatusMutationForHiddenTask(t *testing.T) {
	today := int(time.Now().Weekday())
	hiddenDay := (today + 1) % 7
	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks: []internal.Task{
			{ID: 1, Title: "Hidden", Duration: 20, Status: "todo", Order: 1, Visibility: []int{hiddenDay}},
		},
	}

	m := newModel(data, t.TempDir()+"/tasks.json")
	updatedModel, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	updated := updatedModel.(model)
	updated.lists[0].Select(0)

	blockedModel, _ := updated.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	blocked := blockedModel.(model)
	task := internal.FindTask(&blocked.data, 1)
	if task == nil || task.Status != "todo" {
		t.Fatalf("expected hidden task status to stay todo, got %#v", task)
	}
	if !strings.Contains(blocked.statusMsg, "Hidden tasks can be edited or deleted") {
		t.Fatalf("expected hidden-task guard message, got %q", blocked.statusMsg)
	}
}
