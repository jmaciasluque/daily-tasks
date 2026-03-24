package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"daily-tasks/internal"
)

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
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "list", "ls":
		return true, runList(cmdArgs)
	case "add":
		return true, runAdd(cmdArgs)
	case "done":
		return true, runMove(cmdArgs, "done")
	case "todo":
		return true, runMove(cmdArgs, "todo")
	case "skip":
		return true, runSkip(cmdArgs)
	case "delete", "del", "rm":
		return true, runDelete(cmdArgs)
	case "sync":
		return true, runSync(cmdArgs)
	case "push":
		return true, runPush(cmdArgs)
	default:
		return true, fmt.Errorf("unknown command: %s", cmd)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks [command] [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  list, ls         List tasks")
	fmt.Fprintln(w, "  add              Add a task")
	fmt.Fprintln(w, "  done             Mark a task as done")
	fmt.Fprintln(w, "  skip             Mark a task as skipped")
	fmt.Fprintln(w, "  todo             Mark a task as todo")
	fmt.Fprintln(w, "  delete, del, rm  Delete a task")
	fmt.Fprintln(w, "  sync             Sync with Nextcloud")
	fmt.Fprintln(w, "  push             Force push local data")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run with --help after a command to see command options.")
}

func printListUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks list [--status todo|done|skipped|all]")
}

func printAddUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks add --title \"Task title\" --duration 15 [--deadline HH:MM] [--status todo|done]")
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

func printSyncUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks sync")
}

func printPushUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks push")
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

	data, path, reset, err := loadDataAndReset()
	if err != nil {
		return err
	}
	statusValue := strings.ToLower(strings.TrimSpace(*status))
	if statusValue == "" {
		return errors.New("status must be one of todo, done, all")
	}

	switch statusValue {
	case "todo":
		printTaskGroup("TODO", internal.OrderedTasks(&data, "todo"))
	case "done":
		printTaskGroup("DONE", internal.OrderedTasks(&data, "done"))
	case "skipped":
		printTaskGroup("SKIPPED", internal.OrderedTasks(&data, "skipped"))
	case "all":
		printTaskGroup("TODO", internal.OrderedTasks(&data, "todo"))
		fmt.Println("")
		printTaskGroup("DONE", internal.OrderedTasks(&data, "done"))
		fmt.Println("")
		printTaskGroup("SKIPPED", internal.OrderedTasks(&data, "skipped"))
	default:
		return errors.New("status must be one of todo, done, skipped, all")
	}

	if reset {
		if err := internal.SaveData(path, data); err != nil {
			return err
		}
	}
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

	data, path, _, err := loadDataAndReset()
	if err != nil {
		return err
	}

	newTask := internal.Task{
		ID:       data.NextID,
		Title:    strings.TrimSpace(*title),
		Duration: *duration,
		Status:   statusValue,
		Order:    internal.NextOrder(&data, statusValue),
		Deadline: deadlineValue,
	}
	data.Tasks = append(data.Tasks, newTask)
	data.NextID++

	if err := internal.SaveData(path, data); err != nil {
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

	task.Status = status
	task.Order = internal.NextOrder(&data, status)

	if err := internal.SaveData(path, data); err != nil {
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

	task.Status = "skipped"
	task.Order = internal.NextOrder(&data, "skipped")

	if err := internal.SaveData(path, data); err != nil {
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
	internal.DeleteTask(&data, id)

	if err := internal.SaveData(path, data); err != nil {
		return err
	}
	fmt.Printf("Deleted task #%d\n", id)
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
	if reset {
		if err := internal.SaveData(path, data); err != nil {
			return err
		}
	}

	settings, err := internal.LoadWebDAVSettings()
	if err != nil {
		return err
	}

	result := internal.SyncWithRemote(settings, data)
	if result.Action == "error" {
		return errors.New(result.Message)
	}

	data = internal.NormalizeData(result.Data)
	if err := internal.SaveData(path, data); err != nil {
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
	if reset {
		if err := internal.SaveData(path, data); err != nil {
			return err
		}
	}

	settings, err := internal.LoadWebDAVSettings()
	if err != nil {
		return err
	}
	if err := internal.PushRemoteData(settings, data); err != nil {
		return err
	}
	fmt.Println("Pushed local data")
	return nil
}

func loadDataAndReset() (internal.Data, string, bool, error) {
	path, err := internal.DefaultDataPath()
	if err != nil {
		return internal.Data{}, "", false, err
	}

	data, err := internal.LoadData(path)
	if err != nil {
		return internal.Data{}, "", false, err
	}

	reset := internal.ResetIfNewDay(&data)
	return data, path, reset, nil
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
