package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"daily-tasks/internal"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeNormal mode = iota
	modeAdd
	modeEdit
	modeDeleteConfirm
)

type taskItem struct {
	id       int
	title    string
	duration int
	deadline string
}

func (t taskItem) Title() string {
	s := fmt.Sprintf("%s • %dm", t.title, t.duration)
	if t.deadline != "" {
		s += fmt.Sprintf(" • ⏰ %s", t.deadline)
	}
	return s
}
func (t taskItem) Description() string { return "" }
func (t taskItem) FilterValue() string { return t.title }

type tickMsg time.Time

type syncResultMsg struct {
	result internal.SyncResult
}

type pushResultMsg struct {
	err error
}

// colStatus maps TUI column index to task status
var colStatus = [3]string{"todo", "done", "skipped"}

type model struct {
	data          internal.Data
	dataPath      string
	lists         [3]list.Model
	focused       int
	width         int
	height        int
	mode          mode
	titleInput    textinput.Model
	durInput      textinput.Model
	deadlineInput textinput.Model
	editID        int
	errMsg        string
	statusMsg     string
	lastChecked   string
	history       []internal.Data
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

	m := model{
		data:          data,
		dataPath:      path,
		lists:         [3]list.Model{todoList, doneList, skippedList},
		focused:       0,
		mode:          modeNormal,
		titleInput:    title,
		durInput:      dur,
		deadlineInput: deadline,
		width:         80,
		height:        24,
	}
	m.ensureReset()
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
		return m, tick()
	case syncResultMsg:
		if msg.result.Action == "error" {
			m.statusMsg = fmt.Sprintf("Sync failed: %s", msg.result.Message)
			return m, nil
		}
		m.data = internal.NormalizeData(msg.result.Data)
		m.ensureReset()
		m.applyTheme()
		m.syncLists()
		_ = internal.SaveData(m.dataPath, m.data)
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
		m.focused = (m.focused + 1) % 3
		return m, nil
	case "shift+tab":
		m.focused = (m.focused + 2) % 3
		return m, nil
	case "h":
		m.focused = (m.focused + 2) % 3
		return m, nil
	case "l":
		m.focused = (m.focused + 1) % 3
		return m, nil
	case "a":
		m.mode = modeAdd
		m.errMsg = ""
		m.titleInput.SetValue("")
		m.durInput.SetValue("")
		m.deadlineInput.SetValue("")
		m.titleInput.Focus()
		m.durInput.Blur()
		m.deadlineInput.Blur()
		return m, nil
	case "r":
		if !m.reloadFromDisk("") {
			return m, nil
		}
		m.statusMsg = "Syncing from Nextcloud..."
		return m, syncRemoteCmd(m.data)
	case "p":
		if !m.reloadFromDisk("") {
			return m, nil
		}
		m.statusMsg = "Pushing to Nextcloud..."
		return m, pushRemoteCmd(m.data)
	case "R":
		m.reloadFromDisk("Reloaded local data.")
		return m, nil
	case "t":
		m.data.ThemeIndex = (m.data.ThemeIndex + 1) % internal.ThemeCount()
		m.applyTheme()
		_ = internal.SaveData(m.dataPath, m.data)
		return m, nil
	case "u":
		if m.undo() {
			m.applyTheme()
			m.syncLists()
			_ = internal.SaveData(m.dataPath, m.data)
		}
		return m, nil
	case "e":
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
		m.titleInput.Focus()
		m.durInput.Blur()
		m.deadlineInput.Blur()
		return m, nil
	case "d":
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
		t := m.selectedTask()
		if t == nil || t.Status != "todo" {
			return m, nil
		}
		m.pushHistory()
		t.Status = "skipped"
		t.Order = internal.NextOrder(&m.data, "skipped")
		m.syncLists()
		_ = internal.SaveData(m.dataPath, m.data)
		return m, nil
	case "enter", " ":
		t := m.selectedTask()
		if t == nil {
			return m, nil
		}
		m.pushHistory()
		if t.Status == "todo" {
			t.Status = "done"
		} else {
			t.Status = "todo"
		}
		t.Order = internal.NextOrder(&m.data, t.Status)
		m.syncLists()
		_ = internal.SaveData(m.dataPath, m.data)
		return m, nil
	case "J":
		m.pushHistory()
		if ok, newIdx := m.moveTask(1); ok {
			m.syncLists()
			m.lists[m.focused].Select(newIdx)
			_ = internal.SaveData(m.dataPath, m.data)
		}
		return m, nil
	case "K":
		m.pushHistory()
		if ok, newIdx := m.moveTask(-1); ok {
			m.syncLists()
			m.lists[m.focused].Select(newIdx)
			_ = internal.SaveData(m.dataPath, m.data)
		}
		return m, nil
	case "H":
		m.pushHistory()
		oldIdx := m.lists[m.focused].Index()
		if ok, _ := m.moveTaskToCol((m.focused + 2) % 3); ok {
			m.syncLists()
			m.lists[m.focused].Select(internal.ClampIndex(oldIdx, len(m.lists[m.focused].Items())))
			_ = internal.SaveData(m.dataPath, m.data)
		}
		return m, nil
	case "L":
		m.pushHistory()
		oldIdx := m.lists[m.focused].Index()
		if ok, _ := m.moveTaskToCol((m.focused + 1) % 3); ok {
			m.syncLists()
			m.lists[m.focused].Select(internal.ClampIndex(oldIdx, len(m.lists[m.focused].Items())))
			_ = internal.SaveData(m.dataPath, m.data)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.lists[m.focused], cmd = m.lists[m.focused].Update(msg)
	return m, cmd
}

func (m model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	inputs := []*textinput.Model{&m.titleInput, &m.durInput, &m.deadlineInput}

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
		m.pushHistory()
		if m.mode == modeAdd {
			m.data.Tasks = append(m.data.Tasks, internal.Task{
				ID:       m.data.NextID,
				Title:    title,
				Duration: dur,
				Status:   "todo",
				Order:    internal.NextOrder(&m.data, "todo"),
				Deadline: deadlineVal,
			})
			m.data.NextID++
		} else {
			t := internal.FindTask(&m.data, m.editID)
			if t != nil {
				t.Title = title
				t.Duration = dur
				t.Deadline = deadlineVal
			}
		}
		m.mode = modeNormal
		m.errMsg = ""
		m.syncLists()
		_ = internal.SaveData(m.dataPath, m.data)
		return m, nil
	}

	var cmd tea.Cmd
	if m.titleInput.Focused() {
		m.titleInput, cmd = m.titleInput.Update(msg)
	} else if m.durInput.Focused() {
		m.durInput, cmd = m.durInput.Update(msg)
	} else {
		m.deadlineInput, cmd = m.deadlineInput.Update(msg)
	}
	return m, cmd
}

func (m model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		internal.DeleteTask(&m.data, m.editID)
		m.mode = modeNormal
		m.errMsg = ""
		m.syncLists()
		_ = internal.SaveData(m.dataPath, m.data)
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
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.Text)).
		Render("Daily Tasks")

	cols := lipgloss.JoinHorizontal(lipgloss.Top, m.renderList(0), m.renderList(1), m.renderList(2))
	footer := m.renderFooter()
	modal := m.renderModal()

	content := lipgloss.NewStyle().
		Padding(1, 2, 1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, cols, footer))

	if modal != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, modal)
	}

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
	return out
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
	help := fmt.Sprintf("a:add  e:edit  d:delete  s:skip  space:toggle  J/K:reorder  H/L:move  R:reload  r:sync  p:push  t:theme (%s)  tab:switch  q:quit", theme.Name)
	helpLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Render(help)
	pathLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Render(fmt.Sprintf("data: %s", m.dataPath))
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
	var todoItems, doneItems, skippedItems []list.Item
	for _, t := range internal.OrderedTasks(&m.data, "todo") {
		todoItems = append(todoItems, taskItem{id: t.ID, title: t.Title, duration: t.Duration, deadline: t.Deadline})
	}
	for _, t := range internal.OrderedTasks(&m.data, "done") {
		doneItems = append(doneItems, taskItem{id: t.ID, title: t.Title, duration: t.Duration, deadline: t.Deadline})
	}
	for _, t := range internal.OrderedTasks(&m.data, "skipped") {
		skippedItems = append(skippedItems, taskItem{id: t.ID, title: t.Title, duration: t.Duration, deadline: t.Deadline})
	}
	m.lists[0].SetItems(todoItems)
	m.lists[1].SetItems(doneItems)
	m.lists[2].SetItems(skippedItems)
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

func (m *model) ensureReset() {
	if internal.ResetIfNewDay(&m.data) {
		m.syncLists()
		_ = internal.SaveData(m.dataPath, m.data)
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
	if internal.ResetIfNewDay(&data) {
		if err := internal.SaveData(m.dataPath, data); err != nil {
			m.statusMsg = fmt.Sprintf("Reload failed: %s", err)
			return false
		}
	}

	m.data = data
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
	return true
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

		m.lists[i].SetDelegate(delegate)
		styles := m.lists[i].Styles
		styles.PaginationStyle = styles.PaginationStyle.Foreground(lipgloss.Color(theme.Muted))
		styles.HelpStyle = styles.HelpStyle.Foreground(lipgloss.Color(theme.Muted))
		m.lists[i].Styles = styles
	}
}

func (m *model) moveTask(delta int) (bool, int) {
	status := colStatus[m.focused]
	ordered := internal.OrderedTasks(&m.data, status)
	if len(ordered) == 0 {
		return false, 0
	}
	idx := m.lists[m.focused].Index()
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
	newIdx := m.indexInStatus(t.ID, newStatus)
	return true, newIdx
}

func (m *model) indexInStatus(id int, status string) int {
	ordered := internal.OrderedTasks(&m.data, status)
	for i := range ordered {
		if ordered[i].ID == id {
			return i
		}
	}
	return 0
}

func syncRemoteCmd(localData internal.Data) tea.Cmd {
	return func() tea.Msg {
		settings, err := internal.LoadWebDAVSettings()
		if err != nil {
			return syncResultMsg{result: internal.SyncResult{
				Data:    localData,
				Action:  "error",
				Message: err.Error(),
			}}
		}
		result := internal.SyncWithRemote(settings, localData)
		return syncResultMsg{result: result}
	}
}

func pushRemoteCmd(data internal.Data) tea.Cmd {
	return func() tea.Msg {
		settings, err := internal.LoadWebDAVSettings()
		if err != nil {
			return pushResultMsg{err: err}
		}
		err = internal.PushRemoteData(settings, data)
		return pushResultMsg{err: err}
	}
}
