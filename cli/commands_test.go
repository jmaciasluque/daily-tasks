package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"daily-tasks/internal"
)

// setupCLIEnv configures DAILY_TASKS_PATH and DAILY_TASKS_CONFIG to isolated
// temp files so CLI runner tests never touch real user state.
func setupCLIEnv(t *testing.T) (dataPath, configPath string) {
	t.Helper()
	dir := t.TempDir()
	dataPath = dir + "/tasks.json"
	configPath = dir + "/config.json"
	t.Setenv("DAILY_TASKS_PATH", dataPath)
	t.Setenv("DAILY_TASKS_CONFIG", configPath)
	if err := internal.SaveAppConfig(configPath, internal.AppConfig{Backend: internal.BackendLocal}); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}
	return dataPath, configPath
}

// ---------------------------------------------------------------------------
// parseVisibility
// ---------------------------------------------------------------------------

func TestLoginTokenCommandStoresTokenAndHostedConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config.json"
	tokenPath := dir + "/token"
	t.Setenv("DAILY_TASKS_CONFIG", configPath)
	t.Setenv("DAILY_TASKS_TOKEN", tokenPath)

	handled, err := runNonTUI([]string{"login", "--token", "jwt-token", "--api-url", "https://api.example.com/"})
	if err != nil {
		t.Fatalf("login command: %v", err)
	}
	if !handled {
		t.Fatal("login command was not handled")
	}

	cfg, err := internal.LoadAppConfig(configPath)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	if cfg.Backend != internal.BackendHosted || cfg.Hosted.APIURL != "https://api.example.com" {
		t.Fatalf("config = %+v", cfg)
	}
	token, err := internal.LoadHostedToken(tokenPath)
	if err != nil {
		t.Fatalf("LoadHostedToken: %v", err)
	}
	if token != "jwt-token" {
		t.Fatalf("token = %q", token)
	}
}

func TestLogoutDeletesHostedToken(t *testing.T) {
	dir := t.TempDir()
	configPath := dir + "/config.json"
	tokenPath := dir + "/token"
	t.Setenv("DAILY_TASKS_CONFIG", configPath)
	t.Setenv("DAILY_TASKS_TOKEN", tokenPath)
	if err := internal.SaveHostedToken(tokenPath, "jwt-token"); err != nil {
		t.Fatalf("SaveHostedToken: %v", err)
	}

	handled, err := runNonTUI([]string{"logout"})
	if err != nil {
		t.Fatalf("logout command: %v", err)
	}
	if !handled {
		t.Fatal("logout command was not handled")
	}
	if _, err := os.Stat(tokenPath); !os.IsNotExist(err) {
		t.Fatalf("expected token to be deleted, stat err=%v", err)
	}
}

func TestParseVisibilityEmpty(t *testing.T) {
	got, err := parseVisibility("")
	if err != nil || got != nil {
		t.Fatalf("expected nil,nil for empty input, got %v, %v", got, err)
	}
}

func TestParseVisibilityDayNames(t *testing.T) {
	cases := []struct {
		input string
		want  []int
	}{
		{"mon", []int{1}},
		{"sun", []int{0}},
		{"sat", []int{6}},
		{"Mon,Wed,Fri", []int{1, 3, 5}},
		{"monday,tuesday", []int{1, 2}},
		{"Sunday,Saturday", []int{0, 6}},
	}
	for _, tc := range cases {
		got, err := parseVisibility(tc.input)
		if err != nil {
			t.Fatalf("parseVisibility(%q) unexpected error: %v", tc.input, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("parseVisibility(%q) = %v, want %v", tc.input, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseVisibility(%q)[%d] = %d, want %d", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestParseVisibilityNumbers(t *testing.T) {
	got, err := parseVisibility("0,1,6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 6 {
		t.Fatalf("unexpected result: %v", got)
	}
}

func TestParseVisibilityDuplicatesDeduped(t *testing.T) {
	got, err := parseVisibility("mon,mon,monday")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("expected dedup to [1], got %v", got)
	}
}

func TestParseVisibilityInvalidName(t *testing.T) {
	_, err := parseVisibility("badday")
	if err == nil {
		t.Fatal("expected error for invalid day name")
	}
}

func TestParseVisibilityOutOfRange(t *testing.T) {
	_, err := parseVisibility("7")
	if err == nil {
		t.Fatal("expected error for day 7")
	}
}

func TestParseVisibilityNegative(t *testing.T) {
	_, err := parseVisibility("-1")
	if err == nil {
		t.Fatal("expected error for negative day")
	}
}

func TestParseVisibilityAllWhitespace(t *testing.T) {
	got, err := parseVisibility("   ")
	if err != nil || got != nil {
		t.Fatalf("expected nil,nil for whitespace-only input, got %v, %v", got, err)
	}
}

func TestParseVisibilitySorted(t *testing.T) {
	got, err := parseVisibility("fri,mon,wed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Fatalf("expected sorted [1,3,5], got %v", got)
	}
}

// ---------------------------------------------------------------------------
// parseDeadline
// ---------------------------------------------------------------------------

func TestParseDeadlineEmpty(t *testing.T) {
	got, err := parseDeadline("")
	if err != nil || got != "" {
		t.Fatalf("expected empty string for empty input, got %q, %v", got, err)
	}
}

func TestParseDeadlineValid(t *testing.T) {
	cases := []string{"00:00", "06:30", "12:00", "23:59", "09:05"}
	for _, tc := range cases {
		got, err := parseDeadline(tc)
		if err != nil {
			t.Fatalf("parseDeadline(%q) unexpected error: %v", tc, err)
		}
		if got != tc {
			t.Errorf("parseDeadline(%q) = %q, want %q", tc, got, tc)
		}
	}
}

func TestParseDeadlineInvalidFormat(t *testing.T) {
	cases := []string{"1:00", "10:0", "10-30", "ab:cd", "25:00", "00:60", "10:61"}
	for _, tc := range cases {
		_, err := parseDeadline(tc)
		if err == nil {
			t.Errorf("parseDeadline(%q) expected error, got nil", tc)
		}
	}
}

func TestParseDeadlineWhitespace(t *testing.T) {
	got, err := parseDeadline("  09:00  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "09:00" {
		t.Errorf("expected trimmed value %q, got %q", "09:00", got)
	}
}

// ---------------------------------------------------------------------------
// wantsHelp
// ---------------------------------------------------------------------------

func TestWantsHelpTrue(t *testing.T) {
	if !wantsHelp([]string{"-h"}) {
		t.Error("expected true for -h")
	}
	if !wantsHelp([]string{"--help"}) {
		t.Error("expected true for --help")
	}
	if !wantsHelp([]string{"foo", "--help", "bar"}) {
		t.Error("expected true when --help appears anywhere")
	}
}

func TestWantsHelpFalse(t *testing.T) {
	if wantsHelp([]string{}) {
		t.Error("expected false for empty args")
	}
	if wantsHelp([]string{"add", "--title", "Task"}) {
		t.Error("expected false for normal args")
	}
}

// ---------------------------------------------------------------------------
// parseStatsPeriod
// ---------------------------------------------------------------------------

func TestParseStatsPeriod(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"7d", 7},
		{"30d", 30},
		{"", 30},
		{"90d", 90},
		{"365d", 365},
	}
	for _, tc := range cases {
		got, err := parseStatsPeriod(tc.input)
		if err != nil {
			t.Fatalf("parseStatsPeriod(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("parseStatsPeriod(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestParseStatsPeriodInvalid(t *testing.T) {
	_, err := parseStatsPeriod("14d")
	if err == nil {
		t.Error("expected error for unsupported period")
	}
}

// ---------------------------------------------------------------------------
// resolveStatsRange
// ---------------------------------------------------------------------------

func TestResolveStatsRangeExplicit(t *testing.T) {
	from, to, err := resolveStatsRange("30d", "2026-01-01", "2026-01-31")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if from != "2026-01-01" || to != "2026-01-31" {
		t.Errorf("unexpected result: from=%q to=%q", from, to)
	}
}

func TestResolveStatsRangeExplicitMissingTo(t *testing.T) {
	_, _, err := resolveStatsRange("30d", "2026-01-01", "")
	if err == nil {
		t.Error("expected error when only from is given")
	}
}

func TestResolveStatsRangeExplicitMissingFrom(t *testing.T) {
	_, _, err := resolveStatsRange("30d", "", "2026-01-31")
	if err == nil {
		t.Error("expected error when only to is given")
	}
}

func TestResolveStatsRangeExplicitInvalidFrom(t *testing.T) {
	_, _, err := resolveStatsRange("30d", "not-a-date", "2026-01-31")
	if err == nil {
		t.Error("expected error for invalid from date")
	}
}

func TestResolveStatsRangeExplicitInvalidTo(t *testing.T) {
	_, _, err := resolveStatsRange("30d", "2026-01-01", "baddate")
	if err == nil {
		t.Error("expected error for invalid to date")
	}
}

func TestResolveStatsRangeExplicitFromAfterTo(t *testing.T) {
	_, _, err := resolveStatsRange("30d", "2026-02-01", "2026-01-01")
	if err == nil {
		t.Error("expected error when from > to")
	}
}

func TestResolveStatsRangePeriod(t *testing.T) {
	from, to, err := resolveStatsRange("7d", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	if to != today {
		t.Errorf("expected to=%q, got %q", today, to)
	}
	expected := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	if from != expected {
		t.Errorf("expected from=%q, got %q", expected, from)
	}
}

func TestResolveStatsRangeInvalidPeriod(t *testing.T) {
	_, _, err := resolveStatsRange("2d", "", "")
	if err == nil {
		t.Error("expected error for unsupported period")
	}
}

// ---------------------------------------------------------------------------
// parseID
// ---------------------------------------------------------------------------

func TestParseIDFromFlag(t *testing.T) {
	got, err := parseID(3, []string{})
	if err != nil || got != 3 {
		t.Fatalf("expected id=3, got %d, %v", got, err)
	}
}

func TestParseIDFromArgs(t *testing.T) {
	got, err := parseID(0, []string{"5"})
	if err != nil || got != 5 {
		t.Fatalf("expected id=5, got %d, %v", got, err)
	}
}

func TestParseIDMissing(t *testing.T) {
	_, err := parseID(0, []string{})
	if err == nil {
		t.Fatal("expected error when no id provided")
	}
}

func TestParseIDNonNumeric(t *testing.T) {
	_, err := parseID(0, []string{"abc"})
	if err == nil {
		t.Fatal("expected error for non-numeric arg")
	}
}

func TestParseIDZeroArg(t *testing.T) {
	_, err := parseID(0, []string{"0"})
	if err == nil {
		t.Fatal("expected error for id=0 arg")
	}
}

func TestParseIDNegativeArg(t *testing.T) {
	_, err := parseID(0, []string{"-1"})
	if err == nil {
		t.Fatal("expected error for negative id arg")
	}
}

// ---------------------------------------------------------------------------
// printTaskGroup
// ---------------------------------------------------------------------------

func TestPrintTaskGroupOutput(t *testing.T) {
	tasks := []*internal.Task{
		{ID: 1, Title: "Task A", Duration: 30, Status: "todo"},
		{ID: 2, Title: "Task B", Duration: 15, Status: "done", Deadline: "09:00"},
	}
	// Capture stdout
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTaskGroup("TODO", tasks)

	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "TODO (2)") {
		t.Errorf("expected group label with count, got: %q", out)
	}
	if !strings.Contains(out, "#1") || !strings.Contains(out, "Task A") {
		t.Errorf("expected task A in output, got: %q", out)
	}
	if !strings.Contains(out, "09:00") {
		t.Errorf("expected deadline in output, got: %q", out)
	}
}

func TestPrintTaskGroupEmpty(t *testing.T) {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTaskGroup("DONE", []*internal.Task{})

	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "DONE (0)") {
		t.Errorf("expected empty group label, got: %q", out)
	}
}

// ---------------------------------------------------------------------------
// runAdd / runList / runMove / runSkip / runDelete — integration with temp file
// ---------------------------------------------------------------------------

func TestRunAddAndList(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	if err := runAdd([]string{"--title", "Morning run", "--duration", "30"}); err != nil {
		t.Fatalf("runAdd failed: %v", err)
	}

	data, err := internal.LoadData(dataPath)
	if err != nil {
		t.Fatalf("failed to load data after add: %v", err)
	}
	if len(data.Tasks) != 1 || data.Tasks[0].Title != "Morning run" {
		t.Fatalf("unexpected tasks after add: %+v", data.Tasks)
	}
	if data.Tasks[0].Duration != 30 {
		t.Fatalf("expected duration 30, got %d", data.Tasks[0].Duration)
	}
}

func TestRunAddWithDeadlineAndVisibility(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	if err := runAdd([]string{"--title", "Read", "--duration", "20", "--deadline", "08:00", "--visibility", "mon,wed,fri"}); err != nil {
		t.Fatalf("runAdd failed: %v", err)
	}

	data, err := internal.LoadData(dataPath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if len(data.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(data.Tasks))
	}
	task := data.Tasks[0]
	if task.Deadline != "08:00" {
		t.Errorf("expected deadline 08:00, got %q", task.Deadline)
	}
	if len(task.Visibility) != 3 {
		t.Errorf("expected 3 visibility days, got %v", task.Visibility)
	}
}

func TestRunAddMissingTitle(t *testing.T) {
	setupCLIEnv(t)
	err := runAdd([]string{"--duration", "10"})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestRunAddMissingDuration(t *testing.T) {
	setupCLIEnv(t)
	err := runAdd([]string{"--title", "Test"})
	if err == nil {
		t.Fatal("expected error for missing duration")
	}
}

func TestRunAddInvalidDeadline(t *testing.T) {
	setupCLIEnv(t)
	err := runAdd([]string{"--title", "T", "--duration", "5", "--deadline", "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid deadline")
	}
}

func TestRunAddInvalidStatus(t *testing.T) {
	setupCLIEnv(t)
	err := runAdd([]string{"--title", "T", "--duration", "5", "--status", "skipped"})
	if err == nil {
		t.Fatal("expected error for invalid status in add")
	}
}

func TestRunListNoError(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	// Seed some tasks
	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    3,
		Tasks: []internal.Task{
			{ID: 1, Title: "Task 1", Duration: 10, Status: "todo", Order: 1},
			{ID: 2, Title: "Task 2", Duration: 20, Status: "done", Order: 1},
		},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("failed to seed data: %v", err)
	}

	for _, status := range []string{"todo", "done", "skipped", "all"} {
		if err := runList([]string{"--status", status}); err != nil {
			t.Errorf("runList(--status %s) unexpected error: %v", status, err)
		}
	}
}

func TestRunListInvalidStatus(t *testing.T) {
	setupCLIEnv(t)
	err := runList([]string{"--status", "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestRunMoveTaskDone(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Run", Duration: 30, Status: "todo", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	if err := runMove([]string{"1"}, "done"); err != nil {
		t.Fatalf("runMove to done failed: %v", err)
	}

	got, _ := internal.LoadData(dataPath)
	task := internal.FindTask(&got, 1)
	if task == nil || task.Status != "done" {
		t.Fatalf("expected task status done, got %+v", task)
	}
}

func TestRunMoveAlreadySameStatus(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Run", Duration: 30, Status: "done", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	// Should succeed (no-op) without error
	if err := runMove([]string{"1"}, "done"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunMoveTaskNotFound(t *testing.T) {
	setupCLIEnv(t)
	err := runMove([]string{"99"}, "done")
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestRunSkip(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Read", Duration: 20, Status: "todo", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	if err := runSkip([]string{"1"}); err != nil {
		t.Fatalf("runSkip failed: %v", err)
	}

	got, _ := internal.LoadData(dataPath)
	task := internal.FindTask(&got, 1)
	if task == nil || task.Status != "skipped" {
		t.Fatalf("expected skipped status, got %+v", task)
	}
}

func TestRunSkipAlreadySkipped(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Read", Duration: 20, Status: "skipped", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	if err := runSkip([]string{"1"}); err != nil {
		t.Fatalf("runSkip already-skipped should not error: %v", err)
	}
}

func TestRunSkipNotFound(t *testing.T) {
	setupCLIEnv(t)
	err := runSkip([]string{"99"})
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestRunDelete(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Delete me", Duration: 5, Status: "todo", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	if err := runDelete([]string{"1"}); err != nil {
		t.Fatalf("runDelete failed: %v", err)
	}

	got, _ := internal.LoadData(dataPath)
	if internal.FindTask(&got, 1) != nil {
		t.Fatal("expected task to be deleted")
	}
}

func TestRunDeleteNotFound(t *testing.T) {
	setupCLIEnv(t)
	err := runDelete([]string{"42"})
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestRunDeleteMissingID(t *testing.T) {
	setupCLIEnv(t)
	err := runDelete([]string{})
	if err == nil {
		t.Fatal("expected error when no id provided")
	}
}

// ---------------------------------------------------------------------------
// wantsHelp triggers for CLI runners
// ---------------------------------------------------------------------------

func TestRunListHelp(t *testing.T) {
	err := runList([]string{"--help"})
	if err != nil {
		t.Fatalf("runList --help should not error, got: %v", err)
	}
}

func TestRunAddHelp(t *testing.T) {
	err := runAdd([]string{"--help"})
	if err != nil {
		t.Fatalf("runAdd --help should not error, got: %v", err)
	}
}

func TestRunMoveHelp(t *testing.T) {
	err := runMove([]string{"--help"}, "done")
	if err != nil {
		t.Fatalf("runMove --help should not error, got: %v", err)
	}
}

func TestRunSkipHelp(t *testing.T) {
	err := runSkip([]string{"--help"})
	if err != nil {
		t.Fatalf("runSkip --help should not error, got: %v", err)
	}
}

func TestRunDeleteHelp(t *testing.T) {
	err := runDelete([]string{"--help"})
	if err != nil {
		t.Fatalf("runDelete --help should not error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// printUsage helpers
// ---------------------------------------------------------------------------

func TestPrintUsageHelpers(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	if !strings.Contains(buf.String(), "daily-tasks") {
		t.Error("printUsage should mention daily-tasks")
	}

	buf.Reset()
	printListUsage(&buf)
	if buf.Len() == 0 {
		t.Error("printListUsage should write something")
	}

	buf.Reset()
	printAddUsage(&buf)
	if buf.Len() == 0 {
		t.Error("printAddUsage should write something")
	}

	buf.Reset()
	printMoveUsage(&buf, "done")
	if buf.Len() == 0 {
		t.Error("printMoveUsage should write something")
	}

	buf.Reset()
	printSkipUsage(&buf)
	if buf.Len() == 0 {
		t.Error("printSkipUsage should write something")
	}

	buf.Reset()
	printDeleteUsage(&buf)
	if buf.Len() == 0 {
		t.Error("printDeleteUsage should write something")
	}

	buf.Reset()
	printSyncUsage(&buf)
	if buf.Len() == 0 {
		t.Error("printSyncUsage should write something")
	}

	buf.Reset()
	printPushUsage(&buf)
	if buf.Len() == 0 {
		t.Error("printPushUsage should write something")
	}

	buf.Reset()
	printStatsUsage(&buf)
	if buf.Len() == 0 {
		t.Error("printStatsUsage should write something")
	}

	buf.Reset()
	printWebUsage(&buf)
	if buf.Len() == 0 {
		t.Error("printWebUsage should write something")
	}

	buf.Reset()
	printSetupUsage(&buf)
	if buf.Len() == 0 {
		t.Error("printSetupUsage should write something")
	}
}

// ---------------------------------------------------------------------------
// runStats (with seeded history)
// ---------------------------------------------------------------------------

func TestRunStats(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	before := internal.Data{
		LastReset: "2026-04-07",
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Meditate", Duration: 10, Status: "done", Order: 1}},
	}
	after := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Meditate", Duration: 10, Status: "todo", Order: 1}},
	}
	if err := internal.SaveDataWithHistory(dataPath, before, after); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	from := "2026-04-07"
	to := time.Now().Format("2006-01-02")
	if err := runStats([]string{"--from", from, "--to", to}); err != nil {
		t.Fatalf("runStats failed: %v", err)
	}
}

func TestRunStatsHelp(t *testing.T) {
	err := runStats([]string{"--help"})
	if err != nil {
		t.Fatalf("runStats --help should not error: %v", err)
	}
}

func TestRunStatsInvalidPeriod(t *testing.T) {
	setupCLIEnv(t)
	err := runStats([]string{"--period", "999d"})
	if err == nil {
		t.Fatal("expected error for invalid period")
	}
}

// ---------------------------------------------------------------------------
// loadDataAndReset
// ---------------------------------------------------------------------------

func TestLoadDataAndReset(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	// File doesn't exist yet — should return fresh empty data
	data, path, _, err := loadDataAndReset()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != dataPath {
		t.Errorf("expected path %q, got %q", dataPath, path)
	}
	if data.NextID != 1 {
		t.Errorf("expected NextID=1, got %d", data.NextID)
	}
}

func TestLoadDataAndResetTrigger(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	// Create data with old LastReset to trigger reset
	old := internal.Data{
		LastReset: "2020-01-01",
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Old", Duration: 10, Status: "done", Order: 1}},
	}
	if err := internal.SaveData(dataPath, old); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	data, _, reset, err := loadDataAndReset()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reset {
		t.Error("expected reset=true for old LastReset")
	}
	task := internal.FindTask(&data, 1)
	if task == nil || task.Status != "todo" {
		t.Fatalf("expected task reset to todo, got %+v", task)
	}
}

// ---------------------------------------------------------------------------
// runNonTUI helpers
// ---------------------------------------------------------------------------

func TestRunNonTUIEmpty(t *testing.T) {
	handled, err := runNonTUI([]string{})
	if handled || err != nil {
		t.Errorf("expected (false, nil) for empty args, got (%v, %v)", handled, err)
	}
}

func TestRunNonTUIVersion(t *testing.T) {
	handled, err := runNonTUI([]string{"--version"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for --version, got (%v, %v)", handled, err)
	}
	handled, err = runNonTUI([]string{"-v"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for -v, got (%v, %v)", handled, err)
	}
}

func TestRunNonTUIHelp(t *testing.T) {
	handled, err := runNonTUI([]string{"-h"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for -h, got (%v, %v)", handled, err)
	}
	handled, err = runNonTUI([]string{"help"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for help, got (%v, %v)", handled, err)
	}
}

func TestRunNonTUIUnknownCommand(t *testing.T) {
	handled, err := runNonTUI([]string{"nosuchcmd"})
	if !handled {
		t.Error("expected handled=true for unknown command")
	}
	if err == nil || !strings.Contains(err.Error(), "nosuchcmd") {
		t.Errorf("expected error mentioning command, got: %v", err)
	}
}

func TestRunNonTUIAddRequiresBackend(t *testing.T) {
	// Config path pointing at a nonexistent file — backend not configured
	t.Setenv("DAILY_TASKS_CONFIG", t.TempDir()+"/missing-config.json")
	handled, err := runNonTUI([]string{"add", "--title", "T", "--duration", "5"})
	if !handled {
		t.Error("expected handled=true")
	}
	if err == nil {
		t.Error("expected error when backend not configured")
	}
	_ = fmt.Sprintf("%v", err) // use err
}

// ---------------------------------------------------------------------------
// runNonTUI — dispatch to CLI runners with configured backend
// ---------------------------------------------------------------------------

func TestRunNonTUIListSuccess(t *testing.T) {
	setupCLIEnv(t)
	handled, err := runNonTUI([]string{"list"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for list, got (%v, %v)", handled, err)
	}
}

func TestRunNonTUILsAlias(t *testing.T) {
	setupCLIEnv(t)
	handled, err := runNonTUI([]string{"ls"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for ls, got (%v, %v)", handled, err)
	}
}

func TestRunNonTUIAddSuccess(t *testing.T) {
	setupCLIEnv(t)
	handled, err := runNonTUI([]string{"add", "--title", "Task X", "--duration", "10"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for add, got (%v, %v)", handled, err)
	}
}

func TestRunNonTUIDoneAndTodo(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "X", Duration: 5, Status: "todo", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	handled, err := runNonTUI([]string{"done", "1"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for done, got (%v, %v)", handled, err)
	}

	handled, err = runNonTUI([]string{"todo", "1"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for todo, got (%v, %v)", handled, err)
	}
}

func TestRunNonTUISkipAndDelete(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	// Two tasks so we can skip one and delete another
	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    3,
		Tasks: []internal.Task{
			{ID: 1, Title: "Skip me", Duration: 5, Status: "todo", Order: 1},
			{ID: 2, Title: "Delete me", Duration: 10, Status: "todo", Order: 2},
		},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed error: %v", err)
	}

	handled, err := runNonTUI([]string{"skip", "1"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for skip, got (%v, %v)", handled, err)
	}

	handled, err = runNonTUI([]string{"delete", "2"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for delete, got (%v, %v)", handled, err)
	}

	handled, err = runNonTUI([]string{"rm", "1"})
	// task 1 still exists (was only skipped)
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for rm, got (%v, %v)", handled, err)
	}
}

func TestRunNonTUIDelAlias(t *testing.T) {
	dataPath, _ := setupCLIEnv(t)

	data := internal.Data{
		LastReset: time.Now().Format("2006-01-02"),
		NextID:    2,
		Tasks:     []internal.Task{{ID: 1, Title: "Del me", Duration: 5, Status: "todo", Order: 1}},
	}
	if err := internal.SaveData(dataPath, data); err != nil {
		t.Fatalf("seed: %v", err)
	}

	handled, err := runNonTUI([]string{"del", "1"})
	if !handled || err != nil {
		t.Errorf("expected (true, nil) for del, got (%v, %v)", handled, err)
	}
}
