package main

import (
	"strings"
	"testing"

	"daily-tasks/internal"

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
