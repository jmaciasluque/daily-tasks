package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"daily-tasks/internal"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type mode int

const (
	modeNormal mode = iota
	modeAdd
	modeEdit
	modeDeleteConfirm
)

type screen int

const (
	screenTasks screen = iota
	screenStats
)

var statsPeriods = []string{"7d", "30d", "90d", "365d"}

type taskItem struct {
	id          int
	title       string
	duration    int
	deadline    string
	hiddenToday bool
	visibility  []int
}

func (t taskItem) Title() string {
	s := fmt.Sprintf("%s • %dm", t.title, t.duration)
	if t.deadline != "" {
		indicator := internal.DeadlineIndicator(t.deadline, time.Now())
		if indicator != "" {
			s += fmt.Sprintf(" • ⏰ %s (%s)", t.deadline, indicator)
		} else {
			s += fmt.Sprintf(" • ⏰ %s", t.deadline)
		}
	}
	if t.hiddenToday {
		s += fmt.Sprintf(" • hidden today (%s)", formatVisibility(t.visibility))
	}
	return s
}
func (t taskItem) Description() string { return "" }
func (t taskItem) FilterValue() string { return t.title }

// separatorItem renders as a visual AM/PM group divider in the list.
type separatorItem struct {
	label string
}

func (s separatorItem) Title() string       { return s.label }
func (s separatorItem) Description() string { return "" }
func (s separatorItem) FilterValue() string { return "" }

// taskDelegate wraps DefaultDelegate to render separator items as muted dividers.
type taskDelegate struct {
	list.DefaultDelegate
	separatorStyle lipgloss.Style
}

func (d taskDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if sep, ok := item.(separatorItem); ok {
		fmt.Fprint(w, d.separatorStyle.Render(sep.label))
		return
	}
	d.DefaultDelegate.Render(w, m, index, item)
}

type tickMsg time.Time

type syncResultMsg struct {
	result internal.SyncStateResult
}

type pushResultMsg struct {
	err error
}

// colStatus maps TUI column index to task status
var colStatus = [3]string{"todo", "done", "skipped"}

var dayAbbrev = [7]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}

// formatVisibility converts a visibility slice to a human-readable string.
func formatVisibility(days []int) string {
	if len(days) == 0 {
		return ""
	}
	names := make([]string, len(days))
	for i, d := range days {
		if d >= 0 && d <= 6 {
			names[i] = dayAbbrev[d]
		} else {
			names[i] = strconv.Itoa(d)
		}
	}
	return strings.Join(names, ",")
}

type model struct {
	data            internal.Data
	dataPath        string
	lists           [3]list.Model
	focused         int
	width           int
	height          int
	mode            mode
	titleInput      textinput.Model
	durInput        textinput.Model
	deadlineInput   textinput.Model
	visibilityInput textinput.Model
	editID          int
	errMsg          string
	statusMsg       string
	lastChecked     string
	history         []internal.Data
	screen          screen
	statsPeriod     int
	statsSummary    internal.StatsSummary
	statsErr        string
	showAllTasks    bool
}

func main() {
	if handled, err := runNonTUI(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			printUsage(os.Stderr)
			os.Exit(1)
		}
		return
	}

	if err := ensureConfiguredBackendInteractive(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	dataPath, err := internal.DefaultDataPath()
	if err != nil {
		fmt.Println("Error finding data path:", err)
		os.Exit(1)
	}

	data, err := internal.LoadData(dataPath)
	if err != nil {
		fmt.Println("Error loading data:", err)
		os.Exit(1)
	}

	m := newModel(data, dataPath)
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

func newModel(data internal.Data, path string) model {
	todoList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	doneList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	skippedList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)

	for _, l := range []*list.Model{&todoList, &doneList, &skippedList} {
		l.SetShowFilter(false)
		l.SetShowHelp(false)
		l.SetShowStatusBar(false)
		l.SetFilteringEnabled(false)
		l.SetShowTitle(false)
	}

	title := textinput.New()
	title.Placeholder = "Task title"
	title.Focus()
	title.CharLimit = 120

	dur := textinput.New()
	dur.Placeholder = "Duration (minutes)"
	dur.CharLimit = 4

	deadline := textinput.New()
	deadline.Placeholder = "Deadline HH:MM (optional)"
	deadline.CharLimit = 5

	visibility := textinput.New()
	visibility.Placeholder = "Days: mon,wed,fri (empty=every day)"
	visibility.CharLimit = 50

	m := model{
		data:            data,
		dataPath:        path,
		lists:           [3]list.Model{todoList, doneList, skippedList},
		focused:         0,
		mode:            modeNormal,
		screen:          screenTasks,
		statsPeriod:     1,
		titleInput:      title,
		durInput:        dur,
		deadlineInput:   deadline,
		visibilityInput: visibility,
		width:           80,
		height:          24,
	}
	m.ensureReset()
	m.refreshStats()
	m.applyTheme()
	m.resizeLists()
	m.syncLists()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), textinput.Blink)
}

func tick() tea.Cmd {
	return tea.Tick(time.Minute, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeLists()
		return m, nil
	case tickMsg:
		m.ensureReset()
		m.refreshStats()
		return m, tick()
	case syncResultMsg:
		if msg.result.Action == "error" {
			m.statusMsg = fmt.Sprintf("Sync failed: %s", msg.result.Message)
			return m, nil
		}
		m.data = internal.NormalizeData(msg.result.Data)
		m.ensureReset()
		m.refreshStats()
		m.applyTheme()
		m.syncLists()
		_ = internal.SaveData(m.dataPath, m.data)
		_ = internal.SaveHistory(m.dataPath, msg.result.History)
		m.statusMsg = msg.result.Message
		return m, nil
	case pushResultMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Push failed: %s", msg.err)
		} else {
			m.statusMsg = "Pushed to Nextcloud."
		}
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case modeNormal:
			return m.updateNormal(msg)
		case modeAdd, modeEdit:
			return m.updateEdit(msg)
		case modeDeleteConfirm:
			return m.updateDeleteConfirm(msg)
		}
	}

	var cmd tea.Cmd
	m.lists[m.focused], cmd = m.lists[m.focused].Update(msg)
	return m, cmd
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab":
		if m.screen == screenStats {
			return m, nil
		}
		m.focused = (m.focused + 1) % 3
		return m, nil
	case "shift+tab":
		if m.screen == screenStats {
			return m, nil
		}
		m.focused = (m.focused + 2) % 3
		return m, nil
	case "h":
		if m.screen == screenStats {
			m.cycleStatsPeriod(-1)
			return m, nil
		}
		m.focused = (m.focused + 2) % 3
		return m, nil
	case "l":
		if m.screen == screenStats {
			m.cycleStatsPeriod(1)
			return m, nil
		}
		m.focused = (m.focused + 1) % 3
		return m, nil
	case "g":
		if m.screen == screenTasks {
			m.screen = screenStats
			m.refreshStats()
		} else {
			m.screen = screenTasks
		}
		return m, nil
	case "v":
		if m.screen != screenTasks {
			return m, nil
		}
		m.showAllTasks = !m.showAllTasks
		if m.showAllTasks {
			m.statusMsg = "Showing all tasks; hidden tasks can be edited or deleted."
		} else {
			m.statusMsg = "Showing tasks visible today."
		}
		m.syncLists()
		return m, nil
	case "[":
		if m.screen == screenStats {
			m.cycleStatsPeriod(-1)
		}
		return m, nil
	case "]":
		if m.screen == screenStats {
			m.cycleStatsPeriod(1)
		}
		return m, nil
	case "a":
		if m.screen != screenTasks {
			return m, nil
		}
		m.mode = modeAdd
		m.errMsg = ""
		m.titleInput.SetValue("")
		m.durInput.SetValue("")
		m.deadlineInput.SetValue("")
		m.visibilityInput.SetValue("")
		m.titleInput.Focus()
		m.durInput.Blur()
		m.deadlineInput.Blur()
		m.visibilityInput.Blur()
		return m, nil
	case "r":
		if !m.reloadFromDisk("") {
			return m, nil
		}
		m.statusMsg = "Syncing from Nextcloud..."
		return m, syncRemoteCmd(m.dataPath, m.data)
	case "p":
		if !m.reloadFromDisk("") {
			return m, nil
		}
		m.statusMsg = "Pushing to Nextcloud..."
		return m, pushRemoteCmd(m.dataPath, m.data)
	case "R":
		m.reloadFromDisk("Reloaded local data.")
		return m, nil
	case "t":
		before := internal.CloneData(m.data)
		m.data.ThemeIndex = (m.data.ThemeIndex + 1) % internal.ThemeCount()
		m.applyTheme()
		_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		return m, nil
	case "u":
		before := internal.CloneData(m.data)
		if m.undo() {
			m.applyTheme()
			m.syncLists()
			_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		}
		return m, nil
	case "e":
		if m.screen != screenTasks {
			return m, nil
		}
		t := m.selectedTask()
		if t == nil {
			return m, nil
		}
		m.mode = modeEdit
		m.errMsg = ""
		m.editID = t.ID
		m.titleInput.SetValue(t.Title)
		m.durInput.SetValue(strconv.Itoa(t.Duration))
		m.deadlineInput.SetValue(t.Deadline)
		m.visibilityInput.SetValue(formatVisibility(t.Visibility))
		m.titleInput.Focus()
		m.durInput.Blur()
		m.deadlineInput.Blur()
		m.visibilityInput.Blur()
		return m, nil
	case "d":
		if m.screen != screenTasks {
			return m, nil
		}
		t := m.selectedTask()
		if t == nil {
			return m, nil
		}
		m.pushHistory()
		m.mode = modeDeleteConfirm
		m.errMsg = ""
		m.editID = t.ID
		return m, nil
	case "s":
		if m.screen != screenTasks {
			return m, nil
		}
		t := m.selectedTask()
		if t == nil || t.Status != "todo" {
			return m, nil
		}
		if m.selectedTaskHiddenToday() {
			m.statusMsg = "Hidden tasks can be edited or deleted from All view."
			return m, nil
		}
		m.pushHistory()
		before := internal.CloneData(m.data)
		t.Status = "skipped"
		t.Order = internal.NextOrder(&m.data, "skipped")
		m.refreshStats()
		m.syncLists()
		_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		return m, nil
	case "enter", " ":
		if m.screen != screenTasks {
			return m, nil
		}
		t := m.selectedTask()
		if t == nil {
			return m, nil
		}
		if m.selectedTaskHiddenToday() {
			m.statusMsg = "Hidden tasks can be edited or deleted from All view."
			return m, nil
		}
		m.pushHistory()
		before := internal.CloneData(m.data)
		if t.Status == "todo" {
			t.Status = "done"
			doneCount, totalCount := m.todayCompletionProgress()
			m.statusMsg = completionStatusMessage(doneCount, totalCount)
		} else {
			t.Status = "todo"
		}
		t.Order = internal.NextOrder(&m.data, t.Status)
		m.refreshStats()
		m.syncLists()
		_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		return m, nil
	case "J":
		if m.screen != screenTasks {
			return m, nil
		}
		if m.selectedTaskHiddenToday() {
			m.statusMsg = "Hidden tasks can be edited or deleted from All view."
			return m, nil
		}
		m.pushHistory()
		before := internal.CloneData(m.data)
		if ok, taskIdx := m.moveTask(1); ok {
			m.refreshStats()
			m.syncLists()
			m.lists[m.focused].Select(taskIndexToListIndex(m.lists[m.focused].Items(), taskIdx))
			_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		}
		return m, nil
	case "K":
		if m.screen != screenTasks {
			return m, nil
		}
		if m.selectedTaskHiddenToday() {
			m.statusMsg = "Hidden tasks can be edited or deleted from All view."
			return m, nil
		}
		m.pushHistory()
		before := internal.CloneData(m.data)
		if ok, taskIdx := m.moveTask(-1); ok {
			m.refreshStats()
			m.syncLists()
			m.lists[m.focused].Select(taskIndexToListIndex(m.lists[m.focused].Items(), taskIdx))
			_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		}
		return m, nil
	case "H":
		if m.screen != screenTasks {
			return m, nil
		}
		if m.selectedTaskHiddenToday() {
			m.statusMsg = "Hidden tasks can be edited or deleted from All view."
			return m, nil
		}
		m.pushHistory()
		before := internal.CloneData(m.data)
		oldIdx := m.lists[m.focused].Index()
		if ok, _ := m.moveTaskToCol((m.focused + 2) % 3); ok {
			m.refreshStats()
			m.syncLists()
			m.lists[m.focused].Select(internal.ClampIndex(oldIdx, len(m.lists[m.focused].Items())))
			_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		}
		return m, nil
	case "L":
		if m.screen != screenTasks {
			return m, nil
		}
		if m.selectedTaskHiddenToday() {
			m.statusMsg = "Hidden tasks can be edited or deleted from All view."
			return m, nil
		}
		m.pushHistory()
		before := internal.CloneData(m.data)
		oldIdx := m.lists[m.focused].Index()
		if ok, _ := m.moveTaskToCol((m.focused + 1) % 3); ok {
			m.refreshStats()
			m.syncLists()
			m.lists[m.focused].Select(internal.ClampIndex(oldIdx, len(m.lists[m.focused].Items())))
			_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.lists[m.focused], cmd = m.lists[m.focused].Update(msg)
	return m, cmd
}

func (m model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	inputs := []*textinput.Model{&m.titleInput, &m.durInput, &m.deadlineInput, &m.visibilityInput}

	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.errMsg = ""
		return m, nil
	case "tab":
		for i, inp := range inputs {
			if inp.Focused() {
				inp.Blur()
				inputs[(i+1)%len(inputs)].Focus()
				break
			}
		}
		return m, nil
	case "shift+tab":
		for i, inp := range inputs {
			if inp.Focused() {
				inp.Blur()
				inputs[(i+len(inputs)-1)%len(inputs)].Focus()
				break
			}
		}
		return m, nil
	case "enter":
		if m.titleInput.Focused() {
			m.titleInput.Blur()
			m.durInput.Focus()
			return m, nil
		}
		if m.durInput.Focused() {
			m.durInput.Blur()
			m.deadlineInput.Focus()
			return m, nil
		}
		if m.deadlineInput.Focused() {
			m.deadlineInput.Blur()
			m.visibilityInput.Focus()
			return m, nil
		}

		title := strings.TrimSpace(m.titleInput.Value())
		dur, err := internal.ParseDuration(m.durInput.Value())
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		if title == "" {
			m.errMsg = "Title cannot be empty"
			return m, nil
		}
		deadlineVal, err := parseDeadline(m.deadlineInput.Value())
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		visibilityVal, err := parseVisibility(m.visibilityInput.Value())
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.pushHistory()
		before := internal.CloneData(m.data)
		if m.mode == modeAdd {
			m.data.Tasks = append(m.data.Tasks, internal.Task{
				ID:         m.data.NextID,
				Title:      title,
				Duration:   dur,
				Status:     "todo",
				Order:      internal.NextOrder(&m.data, "todo"),
				Deadline:   deadlineVal,
				Visibility: visibilityVal,
			})
			m.data.NextID++
		} else {
			t := internal.FindTask(&m.data, m.editID)
			if t != nil {
				t.Title = title
				t.Duration = dur
				t.Deadline = deadlineVal
				t.Visibility = visibilityVal
			}
		}
		m.mode = modeNormal
		m.errMsg = ""
		m.refreshStats()
		m.syncLists()
		_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		return m, nil
	}

	var cmd tea.Cmd
	if m.titleInput.Focused() {
		m.titleInput, cmd = m.titleInput.Update(msg)
	} else if m.durInput.Focused() {
		m.durInput, cmd = m.durInput.Update(msg)
	} else if m.deadlineInput.Focused() {
		m.deadlineInput, cmd = m.deadlineInput.Update(msg)
	} else {
		m.visibilityInput, cmd = m.visibilityInput.Update(msg)
	}
	return m, cmd
}

func (m model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		before := internal.CloneData(m.data)
		internal.DeleteTask(&m.data, m.editID)
		m.mode = modeNormal
		m.errMsg = ""
		m.refreshStats()
		m.syncLists()
		_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
		return m, nil
	case "n", "N", "esc":
		m.mode = modeNormal
		m.errMsg = ""
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	theme := m.currentTheme()
	modal := m.renderModal()
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.Text)).
		Render(fmt.Sprintf("Daily Tasks v%s", internal.Version))

	body := m.renderTasksView()
	if m.screen == screenStats {
		body = m.renderStatsView()
	}
	footer := m.renderFooter()

	content := lipgloss.NewStyle().
		Padding(1, 2, 1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))

	out := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Left,
		lipgloss.Top,
		content,
		lipgloss.WithWhitespaceBackground(lipgloss.Color(theme.Bg)),
		lipgloss.WithWhitespaceForeground(lipgloss.Color(theme.Bg)),
	)
	// Hard-cap output to terminal height to prevent any overflow
	if lines := strings.SplitN(out, "\n", m.height+1); len(lines) > m.height {
		out = strings.Join(lines[:m.height], "\n")
	}
	if modal != "" {
		out = overlayCentered(out, modal, m.width, m.height)
	}
	return out
}

func overlayCentered(base, overlay string, width, height int) string {
	baseLines := normalizeLines(base, width, height)
	overlayWidth := max(1, lipgloss.Width(overlay))
	overlayLines := normalizeLines(overlay, overlayWidth, lipgloss.Height(overlay))
	if len(overlayLines) == 0 {
		return strings.Join(baseLines, "\n")
	}

	startRow := max(0, (height-len(overlayLines))/2)
	startCol := max(0, (width-overlayWidth)/2)

	for row, overlayLine := range overlayLines {
		targetRow := startRow + row
		if targetRow < 0 || targetRow >= len(baseLines) {
			continue
		}
		prefix := ansi.Cut(baseLines[targetRow], 0, startCol)
		suffix := ansi.Cut(baseLines[targetRow], startCol+overlayWidth, width)
		baseLines[targetRow] = prefix + overlayLine + suffix
	}

	return strings.Join(baseLines, "\n")
}

func normalizeLines(s string, width, height int) []string {
	if width <= 0 {
		width = 1
	}
	lines := strings.Split(s, "\n")
	if height > 0 && len(lines) < height {
		for len(lines) < height {
			lines = append(lines, "")
		}
	}

	normalized := make([]string, len(lines))
	for i, line := range lines {
		truncated := ansi.Truncate(line, width, "")
		lineWidth := ansi.StringWidth(truncated)
		if lineWidth < width {
			truncated += strings.Repeat(" ", width-lineWidth)
		}
		normalized[i] = truncated
	}
	return normalized
}

func (m model) renderList(idx int) string {
	theme := m.currentTheme()
	listWidth := m.lists[idx].Width()

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.Text)).
		Background(lipgloss.Color(theme.PanelBg)).
		Padding(1, 2).
		Width(listWidth)

	focusedTitleStyle := titleStyle.Copy().
		Background(lipgloss.Color(theme.FocusBg))

	titles := [3]string{"To Do", "Done", "Skipped"}
	title := titles[idx]
	if idx == m.focused && m.mode == modeNormal {
		title = focusedTitleStyle.Render(title)
	} else {
		title = titleStyle.Render(title)
	}

	listView := m.lists[idx].View()
	borderColor := lipgloss.Color(theme.Border)
	if idx == m.focused && m.mode == modeNormal {
		borderColor = lipgloss.Color(theme.FocusBorder)
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Background(lipgloss.Color(theme.PanelBg)).
		Padding(1, 1).
		Width(listWidth).
		Height(m.lists[idx].Height() + 3).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, listView))
	return box
}

func (m model) renderFooter() string {
	if m.mode != modeNormal {
		return ""
	}
	theme := m.currentTheme()
	scope := "today"
	if m.showAllTasks {
		scope = "all"
	}
	help := fmt.Sprintf("g:stats  v:view(%s)  [:prev  ]:next  a:add  e:edit  d:delete  s:skip  space:toggle  J/K:reorder  H/L:move  R:reload  r:sync  p:push  t:theme (%s)  tab:switch  q:quit", scope, theme.Name)
	if m.screen == screenStats {
		help = fmt.Sprintf("g:tasks  [:prev  ]:next  h/l:range  t:theme (%s)  R:reload  r:sync  p:push  q:quit", theme.Name)
	}
	helpLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Render(help)
	pathLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Render(fmt.Sprintf("version: %s  data: %s", internal.Version, m.dataPath))
	if m.statusMsg == "" {
		return lipgloss.NewStyle().PaddingTop(1).Render(helpLine + "\n" + pathLine)
	}
	statusLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Accent)).
		Render(m.statusMsg)
	return lipgloss.NewStyle().PaddingTop(1).Render(helpLine + "\n" + statusLine + "\n" + pathLine)
}

func (m model) renderModal() string {
	theme := m.currentTheme()
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(theme.FocusBorder)).
		Background(lipgloss.Color(theme.PanelBg)).
		Padding(1, 2).
		Width(50)

	switch m.mode {
	case modeAdd:
		return modalStyle.Render(m.editView("Add task"))
	case modeEdit:
		return modalStyle.Render(m.editView("Edit task"))
	case modeDeleteConfirm:
		return modalStyle.Render("Delete this task? (y/n)")
	}
	return ""
}

func (m model) editView(title string) string {
	theme := m.currentTheme()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.Text))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent))

	lines := []string{
		titleStyle.Render(title),
		"",
		"Title:",
		m.titleInput.View(),
		"",
		"Duration (minutes):",
		m.durInput.View(),
		"",
		"Deadline (HH:MM, optional):",
		m.deadlineInput.View(),
		"",
		"Visibility (days, e.g. mon,wed,fri — empty=every day):",
		m.visibilityInput.View(),
		"",
		"enter:next  tab:switch  esc:cancel",
	}
	if m.errMsg != "" {
		lines = append(lines, "", errStyle.Render(m.errMsg))
	}
	return strings.Join(lines, "\n")
}

func (m *model) resizeLists() {
	gap := 4
	usableWidth := m.width - gap - 8
	if usableWidth < 60 {
		usableWidth = 60
	}
	listWidth := usableWidth / 3
	// Vertical budget: outer padding (2) + header (1) + box border (2) +
	// box padding (2) + column title with padding (3) + footer (measured)
	footerHeight := lipgloss.Height(m.renderFooter())
	listHeight := m.height - 10 - footerHeight
	if listHeight < 5 {
		listHeight = 5
	}
	for i := range m.lists {
		m.lists[i].SetSize(listWidth, listHeight)
	}
	m.applyTheme()
}

func (m *model) syncLists() {
	today := time.Now().Weekday()
	var todoItems, doneItems, skippedItems []list.Item
	prevGroup := ""
	for _, t := range m.orderedTasksForView("todo") {
		hiddenToday := !t.IsVisibleOn(today)
		if t.Deadline != "" {
			group := "AM"
			if !internal.IsAM(t.Deadline) {
				group = "PM"
			}
			if group != prevGroup {
				todoItems = append(todoItems, separatorItem{label: "── " + group + " ──"})
				prevGroup = group
			}
		}
		todoItems = append(todoItems, taskItem{id: t.ID, title: t.Title, duration: t.Duration, deadline: t.Deadline, hiddenToday: hiddenToday, visibility: t.Visibility})
	}
	for _, t := range m.orderedTasksForView("done") {
		hiddenToday := !t.IsVisibleOn(today)
		doneItems = append(doneItems, taskItem{id: t.ID, title: t.Title, duration: t.Duration, deadline: t.Deadline, hiddenToday: hiddenToday, visibility: t.Visibility})
	}
	for _, t := range m.orderedTasksForView("skipped") {
		hiddenToday := !t.IsVisibleOn(today)
		skippedItems = append(skippedItems, taskItem{id: t.ID, title: t.Title, duration: t.Duration, deadline: t.Deadline, hiddenToday: hiddenToday, visibility: t.Visibility})
	}
	m.lists[0].SetItems(todoItems)
	m.lists[1].SetItems(doneItems)
	m.lists[2].SetItems(skippedItems)
}

func (m *model) orderedTasksForView(status string) []*internal.Task {
	today := time.Now().Weekday()
	var tasks []*internal.Task
	for _, task := range internal.OrderedTasks(&m.data, status) {
		if m.showAllTasks || task.IsVisibleOn(today) {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (m model) renderTasksView() string {
	return lipgloss.JoinHorizontal(lipgloss.Top, m.renderList(0), m.renderList(1), m.renderList(2))
}

func (m model) renderStatsView() string {
	theme := m.currentTheme()
	panelWidth := max(60, m.width-8)
	if panelWidth > m.width-4 {
		panelWidth = m.width - 4
	}
	innerWidth := max(40, panelWidth-8)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Border)).
		Background(lipgloss.Color(theme.PanelBg)).
		Padding(1, 2).
		Width(panelWidth)

	period := statsPeriods[m.statsPeriod]
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.Text)).
		Background(lipgloss.Color(theme.PanelBg)).
		Width(innerWidth).
		Render(fmt.Sprintf("Stats  %s  (%s to %s)", period, m.statsSummary.From, m.statsSummary.To))
	divider := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Border)).
		Background(lipgloss.Color(theme.PanelBg)).
		Render(strings.Repeat("─", innerWidth))

	if m.statsErr != "" {
		return boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			title,
			divider,
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Render(m.statsErr),
		))
	}

	summary := lipgloss.NewStyle().
		Width(innerWidth).
		Background(lipgloss.Color(theme.PanelBg)).
		Foreground(lipgloss.Color(theme.Text)).
		Render(strings.Join([]string{
			renderMetric("Recorded Days", fmt.Sprintf("%d", m.statsSummary.RecordedDays)),
			renderMetric("Completion", fmt.Sprintf("%.0f%%", m.statsSummary.CompletionRate*100)),
			renderMetric("Done Time", formatMinutes(m.statsSummary.DoneDuration)),
			renderMetric("Snapshots", fmt.Sprintf("%d", m.statsSummary.TaskCount)),
		}, "   "))

	dailyLines := []string{"Daily activity"}
	for _, day := range tailDaily(m.statsSummary.Daily, 7) {
		dailyLines = append(dailyLines, fmt.Sprintf("%s %s D:%d S:%d T:%d",
			day.Date,
			barForDay(day, 18),
			day.DoneCount,
			day.SkippedCount,
			day.TodoCount,
		))
	}
	if len(m.statsSummary.Daily) == 0 {
		dailyLines = append(dailyLines, "No history recorded yet.")
	}

	taskLines := []string{"Top tasks"}
	for _, task := range topTasks(m.statsSummary.Tasks, 5) {
		taskLines = append(taskLines, fmt.Sprintf("%s  done %d/%d  skipped %d  %s",
			task.Title,
			task.DoneDays,
			task.RecordedDays,
			task.SkippedDays,
			formatMinutes(task.DoneDuration),
		))
	}
	if len(m.statsSummary.Tasks) == 0 {
		taskLines = append(taskLines, "No task history recorded yet.")
	}

	dailyBlock := lipgloss.NewStyle().
		Width(innerWidth).
		Background(lipgloss.Color(theme.PanelBg)).
		Foreground(lipgloss.Color(theme.Text)).
		Render(strings.Join(dailyLines, "\n"))

	taskBlock := lipgloss.NewStyle().
		Width(innerWidth).
		Background(lipgloss.Color(theme.PanelBg)).
		Foreground(lipgloss.Color(theme.Text)).
		Render(strings.Join(taskLines, "\n"))

	ranges := lipgloss.NewStyle().
		Width(innerWidth).
		Background(lipgloss.Color(theme.PanelBg)).
		Foreground(lipgloss.Color(theme.Muted)).
		Render("Ranges: 7d / 30d / 90d / 365d  with [ ] or h/l")

	return boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		title,
		divider,
		"",
		summary,
		"",
		dailyBlock,
		"",
		taskBlock,
		"",
		ranges,
	))
}

func renderMetric(label, value string) string {
	return fmt.Sprintf("%s %s", label+":", value)
}

func (m *model) todayCompletionProgress() (doneCount int, totalCount int) {
	today := time.Now().Weekday()
	for _, task := range m.data.Tasks {
		if !task.IsVisibleOn(today) {
			continue
		}
		totalCount++
		if task.Status == "done" {
			doneCount++
		}
	}
	return doneCount, totalCount
}

func completionStatusMessage(doneCount, totalCount int) string {
	if totalCount <= 0 {
		return "Task completed."
	}
	percent := doneCount * 100 / totalCount
	switch {
	case doneCount >= totalCount:
		return fmt.Sprintf("🎉 All tasks done today: %d/%d (100%%). Amazing work!", doneCount, totalCount)
	case percent >= 75:
		return fmt.Sprintf("Great momentum: %d/%d done (%d%%).", doneCount, totalCount, percent)
	case percent >= 50:
		return fmt.Sprintf("Nice progress: %d/%d done (%d%%). Keep going!", doneCount, totalCount, percent)
	default:
		return fmt.Sprintf("Good start: %d/%d done (%d%%).", doneCount, totalCount, percent)
	}
}

func (m *model) selectedTask() *internal.Task {
	items := m.lists[m.focused].Items()
	if len(items) == 0 {
		return nil
	}
	item, ok := items[m.lists[m.focused].Index()].(taskItem)
	if !ok {
		return nil
	}
	return internal.FindTask(&m.data, item.id)
}

func (m *model) selectedTaskHiddenToday() bool {
	if !m.showAllTasks {
		return false
	}
	t := m.selectedTask()
	return t != nil && !t.IsVisibleToday()
}

func (m *model) ensureReset() {
	before := internal.CloneData(m.data)
	if internal.ResetIfNewDay(&m.data) {
		m.refreshStats()
		m.syncLists()
		_ = internal.SaveDataWithHistory(m.dataPath, before, m.data)
	}
}

func (m *model) reloadFromDisk(successMsg string) bool {
	indices := [3]int{
		m.lists[0].Index(),
		m.lists[1].Index(),
		m.lists[2].Index(),
	}

	data, err := internal.LoadData(m.dataPath)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Reload failed: %s", err)
		return false
	}
	before := internal.CloneData(data)
	if internal.ResetIfNewDay(&data) {
		if err := internal.SaveDataWithHistory(m.dataPath, before, data); err != nil {
			m.statusMsg = fmt.Sprintf("Reload failed: %s", err)
			return false
		}
	}

	m.data = data
	m.refreshStats()
	m.applyTheme()
	m.syncLists()
	for i := range m.lists {
		m.lists[i].Select(internal.ClampIndex(indices[i], len(m.lists[i].Items())))
	}
	if successMsg != "" {
		m.statusMsg = successMsg
	}
	return true
}

func (m *model) pushHistory() {
	const maxHistory = 100
	m.history = append(m.history, internal.CloneData(m.data))
	if len(m.history) > maxHistory {
		m.history = m.history[len(m.history)-maxHistory:]
	}
}

func (m *model) undo() bool {
	if len(m.history) == 0 {
		return false
	}
	last := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	m.data = internal.CloneData(last)
	m.refreshStats()
	return true
}

func (m *model) cycleStatsPeriod(delta int) {
	count := len(statsPeriods)
	m.statsPeriod = (m.statsPeriod + delta + count) % count
	m.refreshStats()
}

func (m *model) refreshStats() {
	from, to := tuiStatsRange(statsPeriods[m.statsPeriod])
	stats, err := internal.BuildStats(m.dataPath, m.data, from, to)
	if err != nil {
		m.statsErr = fmt.Sprintf("Stats failed: %s", err)
		m.statsSummary = internal.StatsSummary{From: from, To: to}
		return
	}
	m.statsErr = ""
	m.statsSummary = stats
}

func tuiStatsRange(period string) (string, string) {
	days := 30
	switch period {
	case "7d":
		days = 7
	case "90d":
		days = 90
	case "365d":
		days = 365
	}
	end := time.Now()
	start := end.AddDate(0, 0, -(days - 1))
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

func formatMinutes(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	rest := minutes % 60
	if rest == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, rest)
}

func tailDaily(days []internal.DailyStats, limit int) []internal.DailyStats {
	if len(days) <= limit {
		return days
	}
	return days[len(days)-limit:]
}

func topTasks(tasks []internal.TaskFrequencyStats, limit int) []internal.TaskFrequencyStats {
	if len(tasks) <= limit {
		return tasks
	}
	return tasks[:limit]
}

func barForDay(day internal.DailyStats, width int) string {
	total := day.TaskCount
	if total <= 0 || width <= 0 {
		return strings.Repeat(" ", width)
	}
	doneWidth := max(0, day.DoneCount*width/total)
	skippedWidth := max(0, day.SkippedCount*width/total)
	todoWidth := max(0, width-doneWidth-skippedWidth)
	if doneWidth+skippedWidth+todoWidth < width {
		todoWidth += width - (doneWidth + skippedWidth + todoWidth)
	}
	return strings.Repeat("#", doneWidth) + strings.Repeat("+", skippedWidth) + strings.Repeat(".", todoWidth)
}

func (m model) currentTheme() internal.Theme {
	return internal.GetTheme(m.data.ThemeIndex)
}

func (m *model) applyTheme() {
	theme := m.currentTheme()
	for i := range m.lists {
		width := m.lists[i].Width()
		delegate := list.NewDefaultDelegate()
		delegate.ShowDescription = false
		delegate.Styles.NormalTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Text)).
			Background(lipgloss.Color(theme.PanelBg)).
			Padding(0, 0, 0, 2).
			Width(width)
		delegate.Styles.SelectedTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Text)).
			Background(lipgloss.Color(theme.FocusBg)).
			Padding(0, 0, 0, 2).
			Width(width).
			Bold(true)
		delegate.Styles.DimmedTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Muted)).
			Background(lipgloss.Color(theme.PanelBg)).
			Padding(0, 0, 0, 2).
			Width(width)
		delegate.Styles.NormalDesc = delegate.Styles.NormalTitle
		delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle
		delegate.Styles.DimmedDesc = delegate.Styles.DimmedTitle
		delegate.Styles.FilterMatch = lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Accent)).
			Background(lipgloss.Color(theme.PanelBg))

		if i == 0 {
			m.lists[i].SetDelegate(taskDelegate{
				DefaultDelegate: delegate,
				separatorStyle: lipgloss.NewStyle().
					Foreground(lipgloss.Color(theme.Muted)).
					Padding(0, 0, 0, 2).
					Width(width),
			})
		} else {
			m.lists[i].SetDelegate(delegate)
		}
		styles := m.lists[i].Styles
		styles.PaginationStyle = styles.PaginationStyle.Foreground(lipgloss.Color(theme.Muted))
		styles.HelpStyle = styles.HelpStyle.Foreground(lipgloss.Color(theme.Muted))
		m.lists[i].Styles = styles
	}
}

// listIndexToTaskIndex converts a list index (which may include separator items)
// to a task-only index by counting only taskItem entries before it.
func listIndexToTaskIndex(items []list.Item, listIdx int) int {
	taskIdx := 0
	for i := 0; i < listIdx && i < len(items); i++ {
		if _, ok := items[i].(taskItem); ok {
			taskIdx++
		}
	}
	return taskIdx
}

// taskIndexToListIndex converts a task-only index to a list index by
// counting separator items before the Nth task.
func taskIndexToListIndex(items []list.Item, taskIdx int) int {
	count := 0
	for i, item := range items {
		if _, ok := item.(taskItem); ok {
			if count == taskIdx {
				return i
			}
			count++
		}
	}
	return 0
}

func (m *model) moveTask(delta int) (bool, int) {
	status := colStatus[m.focused]
	ordered := m.orderedTasksForView(status)
	if len(ordered) == 0 {
		return false, 0
	}
	items := m.lists[m.focused].Items()
	listIdx := m.lists[m.focused].Index()
	if _, ok := items[listIdx].(taskItem); !ok {
		return false, 0 // cursor is on a separator
	}
	idx := listIndexToTaskIndex(items, listIdx)
	if idx < 0 || idx >= len(ordered) {
		return false, 0
	}
	swapIdx := idx + delta
	if swapIdx < 0 || swapIdx >= len(ordered) {
		return false, 0
	}
	ordered[idx].Order, ordered[swapIdx].Order = ordered[swapIdx].Order, ordered[idx].Order
	return true, swapIdx
}

func (m *model) moveTaskToCol(targetCol int) (bool, int) {
	t := m.selectedTask()
	if t == nil {
		return false, 0
	}
	newStatus := colStatus[targetCol]
	t.Status = newStatus
	t.Order = internal.NextOrder(&m.data, newStatus)
	return true, 0
}

func syncRemoteCmd(dataPath string, localData internal.Data) tea.Cmd {
	return func() tea.Msg {
		backend, err := internal.LoadRemoteBackend()
		if err != nil {
			return syncResultMsg{result: internal.SyncStateResult{
				Data:    localData,
				Action:  "error",
				Message: err.Error(),
			}}
		}
		if _, ok := backend.(*internal.WebDAVBackend); ok && internal.LocalPathInNextcloudSyncFolder(dataPath) {
			return syncResultMsg{result: internal.SyncStateResult{
				Data:    localData,
				Action:  "in_sync",
				Message: "Desktop client is syncing this folder; skipped WebDAV",
			}}
		}
		history, historyErr := internal.LoadHistory(dataPath)
		if historyErr != nil {
			return syncResultMsg{result: internal.SyncStateResult{
				Data:    localData,
				Action:  "error",
				Message: historyErr.Error(),
			}}
		}
		result := internal.SyncStateWithRemote(backend, localData, history)
		return syncResultMsg{result: result}
	}
}

func pushRemoteCmd(dataPath string, data internal.Data) tea.Cmd {
	return func() tea.Msg {
		backend, err := internal.LoadRemoteBackend()
		if err != nil {
			return pushResultMsg{err: err}
		}
		if _, ok := backend.(*internal.WebDAVBackend); ok && internal.LocalPathInNextcloudSyncFolder(dataPath) {
			return pushResultMsg{err: internal.ErrWebDAVHandledByDesktopClient}
		}
		history, historyErr := internal.LoadHistory(dataPath)
		if historyErr != nil {
			return pushResultMsg{err: historyErr}
		}
		err = internal.PushRemoteState(backend, data, history)
		return pushResultMsg{err: err}
	}
}
