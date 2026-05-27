package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"daily-tasks/internal"
)

// parseVisibility parses a comma-separated list of day names or numbers into
// weekday integers (0=Sun..6=Sat). Returns nil for empty input (= every day).
func parseVisibility(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	dayNames := map[string]int{
		"sun": 0, "sunday": 0,
		"mon": 1, "monday": 1,
		"tue": 2, "tuesday": 2,
		"wed": 3, "wednesday": 3,
		"thu": 4, "thursday": 4,
		"fri": 5, "friday": 5,
		"sat": 6, "saturday": 6,
	}
	seen := map[int]bool{}
	var days []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}
		if d, ok := dayNames[part]; ok {
			if !seen[d] {
				seen[d] = true
				days = append(days, d)
			}
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 6 {
			return nil, fmt.Errorf("invalid day %q: use 0-6 or day names (mon,tue,...)", part)
		}
		if !seen[n] {
			seen[n] = true
			days = append(days, n)
		}
	}
	if len(days) == 0 {
		return nil, nil
	}
	sort.Ints(days)
	return days, nil
}

func parseDeadline(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return "", errors.New("deadline must be in HH:MM format")
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return "", errors.New("deadline must be a valid time in HH:MM format")
	}
	return s, nil
}

func runNonTUI(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		return true, nil
	case "-v", "--version", "version":
		fmt.Println(internal.Version)
		return true, nil
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "list", "ls":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runList(cmdArgs)
	case "add":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runAdd(cmdArgs)
	case "done":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runMove(cmdArgs, "done")
	case "todo":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runMove(cmdArgs, "todo")
	case "skip":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runSkip(cmdArgs)
	case "delete", "del", "rm":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runDelete(cmdArgs)
	case "sync":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runSync(cmdArgs)
	case "push":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runPush(cmdArgs)
	case "stats":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runStats(cmdArgs)
	case "setup":
		return true, runSetup(cmdArgs)
	case "login":
		return true, runLogin(cmdArgs)
	case "logout":
		return true, runLogout(cmdArgs)
	case "web":
		return true, runWeb(cmdArgs)
	case "edit":
		if err := requireConfiguredBackend(); err != nil {
			return true, err
		}
		return true, runEdit(cmdArgs)
	default:
		return true, fmt.Errorf("unknown command: %s", cmd)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, "daily-tasks %s\n", internal.Version)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage: daily-tasks [command] [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  list, ls         List tasks")
	fmt.Fprintln(w, "  add              Add a task")
	fmt.Fprintln(w, "  done             Mark a task as done")
	fmt.Fprintln(w, "  skip             Mark a task as skipped")
	fmt.Fprintln(w, "  todo             Mark a task as todo")
	fmt.Fprintln(w, "  delete, del, rm  Delete a task")
	fmt.Fprintln(w, "  edit             Edit a task's fields")
	fmt.Fprintln(w, "  sync             Sync with Nextcloud")
	fmt.Fprintln(w, "  push             Force push local data")
	fmt.Fprintln(w, "  stats            Show historical stats")
	fmt.Fprintln(w, "  setup            Choose backend and connect Nextcloud")
	fmt.Fprintln(w, "  login            Configure hosted backend token")
	fmt.Fprintln(w, "  logout           Delete hosted backend token")
	fmt.Fprintln(w, "  web              Serve the local web app")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run with --help after a command to see command options.")
}

func printListUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks list [--status todo|done|skipped|all]")
}

func printAddUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks add --title \"Task title\" --duration 15 [--deadline HH:MM] [--visibility mon,wed,fri] [--status todo|done]")
}

func printMoveUsage(w io.Writer, status string) {
	fmt.Fprintf(w, "Usage: daily-tasks %s <id> [--id <id>]\\n", status)
}

func printSkipUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks skip <id> [--id <id>]")
}

func printDeleteUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks delete <id> [--id <id>]")
}

func printEditUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks edit <id> [--id <id>] [--title \"Task title\"] [--duration 15] [--deadline HH:MM] [--visibility mon,wed,fri] [--status todo|done|skipped]")
	fmt.Fprintln(w, "All flags except --id are optional. Only provided fields are updated.")
	fmt.Fprintln(w, "Use --visibility \"\" to clear day restrictions (visible every day).")
}

func printSyncUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks sync")
}

func printPushUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks push")
}

func printStatsUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks stats [--period 7d|30d|90d|365d] [--from YYYY-MM-DD --to YYYY-MM-DD]")
}

func printWebUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks web [--listen 127.0.0.1:8421] [--open=false]")
}

func printSetupUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks setup")
}

func printLoginUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks login [--provider google|facebook] [--api-url URL] [--token JWT]")
	fmt.Fprintln(w, "Without --token, opens the hosted OAuth flow and captures the callback on localhost.")
	fmt.Fprintln(w, "Use --token only for manual/bootstrap flows.")
}

func printLogoutUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks logout")
}

func runList(args []string) error {
	if wantsHelp(args) {
		printListUsage(os.Stdout)
		return nil
	}
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	status := fs.String("status", "all", "todo|done|all")
	if err := fs.Parse(args); err != nil {
		return err
	}

	data, _, reset, err := loadDataAndReset()
	if err != nil {
		return err
	}
	statusValue := strings.ToLower(strings.TrimSpace(*status))
	if statusValue == "" {
		return errors.New("status must be one of todo, done, all")
	}

	today := time.Now().Weekday()
	visibleOrdered := func(status string) []*internal.Task {
		var result []*internal.Task
		for _, t := range internal.OrderedTasks(&data, status) {
			if t.IsVisibleOn(today) {
				result = append(result, t)
			}
		}
		return result
	}

	switch statusValue {
	case "todo":
		printTaskGroup("TODO", visibleOrdered("todo"))
	case "done":
		printTaskGroup("DONE", visibleOrdered("done"))
	case "skipped":
		printTaskGroup("SKIPPED", visibleOrdered("skipped"))
	case "all":
		printTaskGroup("TODO", visibleOrdered("todo"))
		fmt.Println("")
		printTaskGroup("DONE", visibleOrdered("done"))
		fmt.Println("")
		printTaskGroup("SKIPPED", visibleOrdered("skipped"))
	default:
		return errors.New("status must be one of todo, done, skipped, all")
	}

	_ = reset
	return nil
}

func runAdd(args []string) error {
	if wantsHelp(args) {
		printAddUsage(os.Stdout)
		return nil
	}
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	title := fs.String("title", "", "task title")
	duration := fs.Int("duration", 0, "duration in minutes")
	deadline := fs.String("deadline", "", "daily reminder time in HH:MM format")
	visibility := fs.String("visibility", "", "visible days: mon,tue,wed or 0-6 (empty=every day)")
	status := fs.String("status", "todo", "todo|done")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*title) == "" {
		return errors.New("title is required")
	}
	if *duration <= 0 {
		return errors.New("duration must be a positive integer")
	}
	statusValue := strings.ToLower(strings.TrimSpace(*status))
	if statusValue != "todo" && statusValue != "done" {
		return errors.New("status must be todo or done")
	}
	deadlineValue, err := parseDeadline(*deadline)
	if err != nil {
		return err
	}
	visibilityValue, err := parseVisibility(*visibility)
	if err != nil {
		return err
	}

	data, path, _, err := loadDataAndReset()
	if err != nil {
		return err
	}

	newTask := internal.Task{
		ID:         data.NextID,
		Title:      strings.TrimSpace(*title),
		Duration:   *duration,
		Status:     statusValue,
		Order:      internal.NextOrder(&data, statusValue),
		Deadline:   deadlineValue,
		Visibility: visibilityValue,
	}
	before := internal.CloneData(data)
	data.Tasks = append(data.Tasks, newTask)
	data.NextID++

	if err := internal.SaveDataWithHistory(path, before, data); err != nil {
		return err
	}
	fmt.Printf("Added task #%d\n", newTask.ID)
	return nil
}

func runMove(args []string, status string) error {
	if wantsHelp(args) {
		printMoveUsage(os.Stdout, status)
		return nil
	}
	fs := flag.NewFlagSet(status, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	idFlag := fs.Int("id", 0, "task id")
	if err := fs.Parse(args); err != nil {
		return err
	}

	id, err := parseID(*idFlag, fs.Args())
	if err != nil {
		return err
	}

	data, path, _, err := loadDataAndReset()
	if err != nil {
		return err
	}

	task := internal.FindTask(&data, id)
	if task == nil {
		return fmt.Errorf("task %d not found", id)
	}
	if task.Status == status {
		fmt.Printf("Task #%d already %s\n", id, status)
		return nil
	}

	before := internal.CloneData(data)
	task.Status = status
	task.Order = internal.NextOrder(&data, status)

	if err := internal.SaveDataWithHistory(path, before, data); err != nil {
		return err
	}
	fmt.Printf("Updated task #%d to %s\n", id, status)
	return nil
}

func runSkip(args []string) error {
	if wantsHelp(args) {
		printSkipUsage(os.Stdout)
		return nil
	}
	fs := flag.NewFlagSet("skip", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	idFlag := fs.Int("id", 0, "task id")
	if err := fs.Parse(args); err != nil {
		return err
	}

	id, err := parseID(*idFlag, fs.Args())
	if err != nil {
		return err
	}

	data, path, _, err := loadDataAndReset()
	if err != nil {
		return err
	}

	task := internal.FindTask(&data, id)
	if task == nil {
		return fmt.Errorf("task %d not found", id)
	}
	if task.Status == "skipped" {
		fmt.Printf("Task #%d already skipped\n", id)
		return nil
	}

	before := internal.CloneData(data)
	task.Status = "skipped"
	task.Order = internal.NextOrder(&data, "skipped")

	if err := internal.SaveDataWithHistory(path, before, data); err != nil {
		return err
	}
	fmt.Printf("Skipped task #%d\n", id)
	return nil
}

func runDelete(args []string) error {
	if wantsHelp(args) {
		printDeleteUsage(os.Stdout)
		return nil
	}
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	idFlag := fs.Int("id", 0, "task id")
	if err := fs.Parse(args); err != nil {
		return err
	}

	id, err := parseID(*idFlag, fs.Args())
	if err != nil {
		return err
	}

	data, path, _, err := loadDataAndReset()
	if err != nil {
		return err
	}

	if internal.FindTask(&data, id) == nil {
		return fmt.Errorf("task %d not found", id)
	}
	before := internal.CloneData(data)
	internal.DeleteTask(&data, id)

	if err := internal.SaveDataWithHistory(path, before, data); err != nil {
		return err
	}
	fmt.Printf("Deleted task #%d\n", id)
	return nil
}

func runEdit(args []string) error {
	if wantsHelp(args) {
		printEditUsage(os.Stdout)
		return nil
	}

	// Extract task ID from positional first arg if present before flag parsing
	// (flag parser stops at first non-flag, so we need to handle this upfront)
	var positionalID int
	var parsedID bool
	restArgs := args
	if len(args) > 0 && args[0][0] != '-' {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			positionalID = n
			parsedID = true
			restArgs = args[1:]
		}
	}

	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	idFlag := fs.Int("id", 0, "task id")
	title := fs.String("title", "", "new title")
	duration := fs.Int("duration", 0, "new duration in minutes")
	deadline := fs.String("deadline", "", "new deadline in HH:MM")
	visibility := fs.String("visibility", "", "new visible days: mon,tue,wed or 0-6 (empty=every day)")
	status := fs.String("status", "", "new status: todo|done|skipped")
	if err := fs.Parse(restArgs); err != nil {
		return err
	}

	// Track which flags were explicitly set
	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})

	// Resolve task ID: --id flag, positional first arg, or fs.Args() fallback
	id, err := parseID(*idFlag, fs.Args())
	if err != nil && parsedID {
		id = positionalID
		err = nil
	} else if err != nil {
		return err
	}
	if !parsedID && *idFlag == 0 && len(fs.Args()) == 0 {
		return errors.New("task id is required")
	}

	// Must have at least one update flag besides --id
	updatable := []string{"title", "duration", "deadline", "visibility", "status"}
	hasUpdates := false
	for _, name := range updatable {
		if visited[name] {
			hasUpdates = true
			break
		}
	}
	if !hasUpdates {
		printEditUsage(os.Stderr)
		return errors.New("at least one field to update is required (--title, --duration, --deadline, --visibility, --status)")
	}

	// Validate provided values
	if visited["title"] && strings.TrimSpace(*title) == "" {
		return errors.New("title cannot be empty")
	}
	if visited["duration"] && *duration <= 0 {
		return errors.New("duration must be a positive integer")
	}
	if visited["deadline"] {
		if _, err := parseDeadline(*deadline); err != nil {
			return err
		}
	}
	if visited["visibility"] {
		if _, err := parseVisibility(*visibility); err != nil {
			return err
		}
	}
	if visited["status"] {
		sv := strings.ToLower(strings.TrimSpace(*status))
		if sv != "todo" && sv != "done" && sv != "skipped" {
			return errors.New("status must be todo, done, or skipped")
		}
	}

	data, path, _, err := loadDataAndReset()
	if err != nil {
		return err
	}

	task := internal.FindTask(&data, id)
	if task == nil {
		return fmt.Errorf("task %d not found", id)
	}

	before := internal.CloneData(data)

	if visited["title"] {
		task.Title = strings.TrimSpace(*title)
	}
	if visited["duration"] {
		task.Duration = *duration
	}
	if visited["deadline"] {
		task.Deadline = strings.TrimSpace(*deadline)
	}
	if visited["visibility"] {
		v, _ := parseVisibility(*visibility)
		task.Visibility = v
	}
	if visited["status"] {
		sv := strings.ToLower(strings.TrimSpace(*status))
		task.Status = sv
		task.Order = internal.NextOrder(&data, sv)
	}

	if err := internal.SaveDataWithHistory(path, before, data); err != nil {
		return err
	}
	fmt.Printf("Updated task #%d\n", id)
	return nil
}

func runSync(args []string) error {
	if wantsHelp(args) {
		printSyncUsage(os.Stdout)
		return nil
	}
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	data, path, reset, err := loadDataAndReset()
	if err != nil {
		return err
	}
	_ = reset

	backend, err := internal.LoadRemoteBackend()
	if err != nil {
		return err
	}
	if _, ok := backend.(*internal.WebDAVBackend); ok && internal.LocalPathInNextcloudSyncFolder(path) {
		fmt.Println("Desktop client is syncing this folder; skipped WebDAV sync")
		return nil
	}
	history, err := internal.LoadHistory(path)
	if err != nil {
		return err
	}

	result := internal.SyncStateWithRemote(backend, data, history)
	if result.Action == "error" {
		return errors.New(result.Message)
	}

	data = internal.NormalizeData(result.Data)
	if err := internal.SaveData(path, data); err != nil {
		return err
	}
	if err := internal.SaveHistory(path, result.History); err != nil {
		return err
	}
	fmt.Println(result.Message)
	return nil
}

func runPush(args []string) error {
	if wantsHelp(args) {
		printPushUsage(os.Stdout)
		return nil
	}
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	data, path, reset, err := loadDataAndReset()
	if err != nil {
		return err
	}
	_ = reset

	backend, err := internal.LoadRemoteBackend()
	if err != nil {
		return err
	}
	if _, ok := backend.(*internal.WebDAVBackend); ok && internal.LocalPathInNextcloudSyncFolder(path) {
		fmt.Println("Desktop client is syncing this folder; skipped WebDAV push")
		return nil
	}
	history, err := internal.LoadHistory(path)
	if err != nil {
		return err
	}
	if err := internal.PushRemoteState(backend, data, history); err != nil {
		return err
	}
	fmt.Println("Pushed local data")
	return nil
}

func runStats(args []string) error {
	if wantsHelp(args) {
		printStatsUsage(os.Stdout)
		return nil
	}
	fs := flag.NewFlagSet("stats", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	period := fs.String("period", "30d", "7d|30d|90d|365d")
	from := fs.String("from", "", "start date (YYYY-MM-DD)")
	to := fs.String("to", "", "end date (YYYY-MM-DD)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	data, path, _, err := loadDataAndReset()
	if err != nil {
		return err
	}

	fromValue, toValue, err := resolveStatsRange(*period, *from, *to)
	if err != nil {
		return err
	}

	stats, err := internal.BuildStats(path, data, fromValue, toValue)
	if err != nil {
		return err
	}

	// Load raw history for streak computation
	history, err := internal.LoadHistory(path)
	if err != nil {
		// Non-fatal — streaks are a bonus
		history = internal.History{}
	}
	historyWithSnapshot := internal.HistoryWithCurrentSnapshot(history, data, 0)

	// Compute streaks and trends
	streaks := internal.ComputeTaskStreaks(stats.Daily, historyWithSnapshot, stats.Tasks, fromValue, toValue)
	trend := internal.ComputeWeeklyTrend(stats.Daily)

	// Render
	output := renderStats(stats, streaks, trend)
	fmt.Print(output)

	return nil
}

func runWeb(args []string) error {
	if wantsHelp(args) {
		printWebUsage(os.Stdout)
		return nil
	}
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	listen := fs.String("listen", "127.0.0.1:8421", "listen address")
	openBrowser := fs.Bool("open", true, "open the browser automatically")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return serveWebApp(*listen, *openBrowser)
}

func runSetup(args []string) error {
	if wantsHelp(args) {
		printSetupUsage(os.Stdout)
		return nil
	}
	if len(args) > 0 {
		return errors.New("setup does not take any arguments")
	}
	return internal.RunSetupTUI()
}

func runLogin(args []string) error {
	if wantsHelp(args) {
		printLoginUsage(os.Stdout)
		return nil
	}
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	provider := fs.String("provider", "facebook", "google|facebook")
	apiURL := fs.String("api-url", internal.DefaultHostedAPIURL, "hosted API URL")
	token := fs.String("token", "", "JWT returned by hosted login")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("login does not take positional arguments")
	}

	normalizedAPIURL := internal.NormalizeServerURL(*apiURL)
	if normalizedAPIURL == "" {
		normalizedAPIURL = internal.DefaultHostedAPIURL
	}
	providerValue := strings.ToLower(strings.TrimSpace(*provider))
	if providerValue != "google" && providerValue != "facebook" {
		return errors.New("provider must be google or facebook")
	}

	cfgPath, err := internal.DefaultConfigPath()
	if err != nil {
		return err
	}
	if err := internal.SaveAppConfig(cfgPath, internal.AppConfig{
		Backend: internal.BackendHosted,
		Hosted:  internal.HostedConfig{APIURL: normalizedAPIURL},
	}); err != nil {
		return err
	}

	tokenValue := strings.TrimSpace(*token)
	if tokenValue == "" {
		fmt.Printf("Opening hosted login in your browser (%s)...\n", providerValue)
		tokenValue, err = internal.RunHostedLogin(context.Background(), internal.HostedLoginOptions{
			APIURL:   normalizedAPIURL,
			Provider: providerValue,
			OpenBrowser: func(raw string) error {
				openBrowser(raw)
				return nil
			},
		})
		if err != nil {
			return err
		}
	}
	tokenPath, err := internal.DefaultHostedTokenPath()
	if err != nil {
		return err
	}
	if err := internal.SaveHostedToken(tokenPath, tokenValue); err != nil {
		return err
	}
	fmt.Println("Hosted backend configured")
	return nil
}

func runLogout(args []string) error {
	if wantsHelp(args) {
		printLogoutUsage(os.Stdout)
		return nil
	}
	if len(args) > 0 {
		return errors.New("logout does not take any arguments")
	}
	tokenPath, err := internal.DefaultHostedTokenPath()
	if err != nil {
		return err
	}
	if err := internal.DeleteHostedToken(tokenPath); err != nil {
		return err
	}
	fmt.Println("Logged out of hosted backend")
	return nil
}

func loadDataAndReset() (internal.Data, string, bool, error) {
	path, err := internal.DefaultDataPath()
	if err != nil {
		return internal.Data{}, "", false, err
	}

	// Pull from remote if newer (read-only GET, safe alongside desktop client)
	internal.PullDataIfRemoteNewer(path)

	data, err := internal.LoadData(path)
	if err != nil {
		return internal.Data{}, "", false, err
	}

	before := internal.CloneData(data)
	reset := internal.ResetIfNewDay(&data)
	if reset {
		if err := internal.SaveDataWithHistory(path, before, data); err != nil {
			return internal.Data{}, "", false, err
		}
	}
	return data, path, reset, nil
}

func resolveStatsRange(period, from, to string) (string, string, error) {
	if from != "" || to != "" {
		if from == "" || to == "" {
			return "", "", errors.New("from and to must be provided together")
		}
		if _, err := time.Parse("2006-01-02", from); err != nil {
			return "", "", errors.New("from must be in YYYY-MM-DD format")
		}
		if _, err := time.Parse("2006-01-02", to); err != nil {
			return "", "", errors.New("to must be in YYYY-MM-DD format")
		}
		if from > to {
			return "", "", errors.New("from must be on or before to")
		}
		return from, to, nil
	}

	days, err := parseStatsPeriod(period)
	if err != nil {
		return "", "", err
	}
	end := time.Now()
	start := end.AddDate(0, 0, -(days - 1))
	return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
}

func parseStatsPeriod(period string) (int, error) {
	switch strings.TrimSpace(period) {
	case "", "30d":
		return 30, nil
	case "7d":
		return 7, nil
	case "90d":
		return 90, nil
	case "365d":
		return 365, nil
	default:
		return 0, errors.New("period must be one of 7d, 30d, 90d, 365d")
	}
}

func parseID(id int, args []string) (int, error) {
	if id <= 0 {
		if len(args) == 0 {
			return 0, errors.New("task id is required")
		}
		parsed, err := strconv.Atoi(args[0])
		if err != nil || parsed <= 0 {
			return 0, errors.New("task id must be a positive integer")
		}
		id = parsed
	}
	return id, nil
}

func printTaskGroup(label string, tasks []*internal.Task) {
	fmt.Printf("%s (%d)\n", label, len(tasks))
	for _, task := range tasks {
		marker := "[ ]"
		if task.Status == "done" {
			marker = "[x]"
		} else if task.Status == "skipped" {
			marker = "[-]"
		}
		line := fmt.Sprintf("%s #%d %s (%dm)", marker, task.ID, task.Title, task.Duration)
		if task.Deadline != "" {
			line += fmt.Sprintf(" [%s]", task.Deadline)
		}
		fmt.Println(line)
	}
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}
