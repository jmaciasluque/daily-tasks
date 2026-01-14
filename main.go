package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type Task struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Duration int    `json:"duration"`
	Status   string `json:"status"` // "todo" or "done"
	Order    int    `json:"order"`
}

type Data struct {
	LastReset string `json:"last_reset"`
	NextID    int    `json:"next_id"`
	Tasks     []Task `json:"tasks"`
	ThemeIndex int   `json:"theme_index"`
}

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
}

func (t taskItem) Title() string       { return fmt.Sprintf("%s • %dm", t.title, t.duration) }
func (t taskItem) Description() string { return "" }
func (t taskItem) FilterValue() string { return t.title }

type tickMsg time.Time

type model struct {
	data        Data
	dataPath    string
	lists       [2]list.Model
	focused     int
	width       int
	height      int
	mode        mode
	titleInput  textinput.Model
	durInput    textinput.Model
	editID      int
	errMsg      string
	statusMsg   string
	lastChecked string
	history     []Data
}

type Theme struct {
	Name       string
	Bg         string
	PanelBg    string
	Text       string
	Muted      string
	Border     string
	FocusBorder string
	FocusBg    string
	Accent     string
}

var themes = []Theme{
	{Name: "Charcoal", Bg: "#111111", PanelBg: "#1A1A1A", Text: "#E5E7EB", Muted: "#9CA3AF", Border: "#2A2A2A", FocusBorder: "#4B5563", FocusBg: "#1F2937", Accent: "#F59E0B"},
	{Name: "Sand", Bg: "#F6F1E7", PanelBg: "#FFF8EE", Text: "#3B2F2F", Muted: "#8C7B6B", Border: "#D6C7B2", FocusBorder: "#B08968", FocusBg: "#F1E2C8", Accent: "#B45309"},
	{Name: "Mint", Bg: "#0F1E1B", PanelBg: "#14312C", Text: "#D1FAE5", Muted: "#7BB9A5", Border: "#1F3F37", FocusBorder: "#34D399", FocusBg: "#1B4B3F", Accent: "#10B981"},
	{Name: "Ocean", Bg: "#0B1C2C", PanelBg: "#11243A", Text: "#DDEBFF", Muted: "#7AA3C4", Border: "#1E3650", FocusBorder: "#3B82F6", FocusBg: "#162E4A", Accent: "#38BDF8"},
	{Name: "Ember", Bg: "#1B0E0E", PanelBg: "#2A1414", Text: "#FEE2E2", Muted: "#FCA5A5", Border: "#3B1C1C", FocusBorder: "#F87171", FocusBg: "#3A1C1C", Accent: "#FB923C"},
	{Name: "Mono Light", Bg: "#F4F4F5", PanelBg: "#FFFFFF", Text: "#111827", Muted: "#6B7280", Border: "#D1D5DB", FocusBorder: "#111827", FocusBg: "#E5E7EB", Accent: "#0EA5E9"},
	{Name: "Solarized Dark", Bg: "#002B36", PanelBg: "#073642", Text: "#EEE8D5", Muted: "#93A1A1", Border: "#0B3B45", FocusBorder: "#268BD2", FocusBg: "#0B3B45", Accent: "#2AA198"},
	{Name: "Solarized Light", Bg: "#FDF6E3", PanelBg: "#FFF8DC", Text: "#586E75", Muted: "#93A1A1", Border: "#E7DEC3", FocusBorder: "#268BD2", FocusBg: "#EEE8D5", Accent: "#B58900"},
	{Name: "Forest", Bg: "#0F1A12", PanelBg: "#16261B", Text: "#E2F5E7", Muted: "#88A08E", Border: "#1C2F22", FocusBorder: "#22C55E", FocusBg: "#1E3A2A", Accent: "#84CC16"},
	{Name: "Plum", Bg: "#1A0F1F", PanelBg: "#2A1630", Text: "#F3E8FF", Muted: "#C4B5FD", Border: "#3B1F44", FocusBorder: "#A78BFA", FocusBg: "#3A2046", Accent: "#F472B6"},
	{Name: "Slate", Bg: "#0F172A", PanelBg: "#111827", Text: "#E5E7EB", Muted: "#9CA3AF", Border: "#1F2937", FocusBorder: "#94A3B8", FocusBg: "#1F2937", Accent: "#38BDF8"},
	{Name: "Coral", Bg: "#2A1410", PanelBg: "#3B1D17", Text: "#FFE4E6", Muted: "#FCA5A5", Border: "#4A2420", FocusBorder: "#FB7185", FocusBg: "#4A241E", Accent: "#FDBA74"},
	{Name: "Meadow", Bg: "#F1FAF3", PanelBg: "#FFFFFF", Text: "#1F2937", Muted: "#6B7280", Border: "#CDE7D4", FocusBorder: "#22C55E", FocusBg: "#E8F7EC", Accent: "#16A34A"},
	{Name: "Cobalt", Bg: "#0A0F2D", PanelBg: "#0F173B", Text: "#DDE2FF", Muted: "#8AA2FF", Border: "#1B255C", FocusBorder: "#6366F1", FocusBg: "#1C2452", Accent: "#60A5FA"},
	{Name: "Amber", Bg: "#1F1600", PanelBg: "#2A1E00", Text: "#FEF3C7", Muted: "#FCD34D", Border: "#3A2A00", FocusBorder: "#F59E0B", FocusBg: "#3A2A00", Accent: "#FBBF24"},
	{Name: "Paper", Bg: "#FAF7F0", PanelBg: "#FFFFFF", Text: "#2F2A24", Muted: "#8B8175", Border: "#E5DED5", FocusBorder: "#9A6F3A", FocusBg: "#EFE4D6", Accent: "#C2410C"},
	{Name: "Ice", Bg: "#0B1418", PanelBg: "#112027", Text: "#E6F4F1", Muted: "#8FB7B0", Border: "#1C2F36", FocusBorder: "#5EEAD4", FocusBg: "#1B2D33", Accent: "#2DD4BF"},
	{Name: "Lavender", Bg: "#201626", PanelBg: "#2B1F33", Text: "#F5E9FF", Muted: "#C4B5FD", Border: "#3A2B45", FocusBorder: "#C084FC", FocusBg: "#3B2A49", Accent: "#A855F7"},
	{Name: "Rose", Bg: "#2A0E1C", PanelBg: "#3A1327", Text: "#FFE4E6", Muted: "#FDA4AF", Border: "#4A1A32", FocusBorder: "#FB7185", FocusBg: "#4A1A32", Accent: "#F43F5E"},
	{Name: "Citrus", Bg: "#0F1405", PanelBg: "#1A220A", Text: "#ECFCCB", Muted: "#BEF264", Border: "#26300F", FocusBorder: "#A3E635", FocusBg: "#2A3412", Accent: "#FACC15"},
	{Name: "Steel", Bg: "#111214", PanelBg: "#1A1C1F", Text: "#E5E7EB", Muted: "#9CA3AF", Border: "#2A2D32", FocusBorder: "#7C8AA6", FocusBg: "#22252B", Accent: "#60A5FA"},
	{Name: "Redwood", Bg: "#20110E", PanelBg: "#2B1612", Text: "#FFE4E1", Muted: "#D6A2A0", Border: "#3A1C16", FocusBorder: "#C97C5D", FocusBg: "#3B1E18", Accent: "#F97316"},
	{Name: "Lagoon", Bg: "#061A1A", PanelBg: "#0B2626", Text: "#D1FAE5", Muted: "#7BC4B8", Border: "#123737", FocusBorder: "#2DD4BF", FocusBg: "#123737", Accent: "#14B8A6"},
	{Name: "Sunrise", Bg: "#2A1506", PanelBg: "#3B1E09", Text: "#FFE8D6", Muted: "#FDBA74", Border: "#4A2710", FocusBorder: "#FB923C", FocusBg: "#4A2710", Accent: "#F97316"},
	{Name: "Graphite", Bg: "#0B0B0C", PanelBg: "#141416", Text: "#F3F4F6", Muted: "#A1A1AA", Border: "#27272A", FocusBorder: "#52525B", FocusBg: "#1F1F23", Accent: "#22D3EE"},
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		os.Exit(1)
	}
	dataPath := filepath.Join(homeDir, "Nextcloud", ".daily-tasks.json")
	data, err := loadData(dataPath)
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

func newModel(data Data, path string) model {
	todoList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	doneList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)

	for _, l := range []*list.Model{&todoList, &doneList} {
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

	m := model{
		data:       data,
		dataPath:   path,
		lists:      [2]list.Model{todoList, doneList},
		focused:    0,
		mode:       modeNormal,
		titleInput: title,
		durInput:   dur,
		width:      80,
		height:     24,
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
	case "tab", "shift+tab":
		if m.focused == 0 {
			m.focused = 1
		} else {
			m.focused = 0
		}
		return m, nil
	case "h":
		m.focused = 0
		return m, nil
	case "l":
		m.focused = 1
		return m, nil
	case "a":
		m.mode = modeAdd
		m.errMsg = ""
		m.titleInput.SetValue("")
		m.durInput.SetValue("")
		m.titleInput.Focus()
		m.durInput.Blur()
		return m, nil
	case "t":
		m.data.ThemeIndex = (m.data.ThemeIndex + 1) % len(themes)
		m.applyTheme()
		_ = saveData(m.dataPath, m.data)
		return m, nil
	case "u":
		if m.undo() {
			m.applyTheme()
			m.syncLists()
			_ = saveData(m.dataPath, m.data)
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
		m.titleInput.Focus()
		m.durInput.Blur()
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
		t.Order = m.nextOrder(t.Status)
		m.syncLists()
		_ = saveData(m.dataPath, m.data)
		return m, nil
	case "J":
		m.pushHistory()
		if ok, newIdx := m.moveTask(1); ok {
			m.syncLists()
			m.lists[m.focused].Select(newIdx)
			_ = saveData(m.dataPath, m.data)
		}
		return m, nil
	case "K":
		m.pushHistory()
		if ok, newIdx := m.moveTask(-1); ok {
			m.syncLists()
			m.lists[m.focused].Select(newIdx)
			_ = saveData(m.dataPath, m.data)
		}
		return m, nil
	case "H":
		m.pushHistory()
		if ok, newIdx := m.moveTaskToOther(); ok {
			m.focused = 0
			m.syncLists()
			m.lists[m.focused].Select(newIdx)
			_ = saveData(m.dataPath, m.data)
		}
		return m, nil
	case "L":
		m.pushHistory()
		if ok, newIdx := m.moveTaskToOther(); ok {
			m.focused = 1
			m.syncLists()
			m.lists[m.focused].Select(newIdx)
			_ = saveData(m.dataPath, m.data)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.lists[m.focused], cmd = m.lists[m.focused].Update(msg)
	return m, cmd
}

func (m model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.errMsg = ""
		return m, nil
	case "tab", "shift+tab":
		if m.titleInput.Focused() {
			m.titleInput.Blur()
			m.durInput.Focus()
		} else {
			m.durInput.Blur()
			m.titleInput.Focus()
		}
		return m, nil
	case "enter":
		if m.titleInput.Focused() {
			m.titleInput.Blur()
			m.durInput.Focus()
			return m, nil
		}

		title := strings.TrimSpace(m.titleInput.Value())
		dur, err := parseDuration(m.durInput.Value())
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		if title == "" {
			m.errMsg = "Title cannot be empty"
			return m, nil
		}
		m.pushHistory()
		if m.mode == modeAdd {
			m.data.Tasks = append(m.data.Tasks, Task{
				ID:       m.data.NextID,
				Title:    title,
				Duration: dur,
				Status:   "todo",
				Order:    m.nextOrder("todo"),
			})
			m.data.NextID++
		} else {
			t := m.findTask(m.editID)
			if t != nil {
				t.Title = title
				t.Duration = dur
			}
		}
		m.mode = modeNormal
		m.errMsg = ""
		m.syncLists()
		_ = saveData(m.dataPath, m.data)
		return m, nil
	}

	var cmd tea.Cmd
	if m.titleInput.Focused() {
		m.titleInput, cmd = m.titleInput.Update(msg)
	} else {
		m.durInput, cmd = m.durInput.Update(msg)
	}
	return m, cmd
}

func (m model) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.deleteTask(m.editID)
		m.mode = modeNormal
		m.errMsg = ""
		m.syncLists()
		_ = saveData(m.dataPath, m.data)
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

	cols := lipgloss.JoinHorizontal(lipgloss.Top, m.renderList(0), m.renderList(1))
	footer := m.renderFooter()
	modal := m.renderModal()

	content := lipgloss.NewStyle().
		Padding(1, 2, 1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, cols, footer))

	if modal != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, modal)
	}

	bgStyle := lipgloss.NewStyle().Background(lipgloss.Color(theme.Bg))
	canvas := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Left,
		lipgloss.Top,
		content,
		lipgloss.WithWhitespaceBackground(lipgloss.Color(theme.Bg)),
	)
	return bgStyle.Render(canvas)
}

func (m model) renderList(idx int) string {
	theme := m.currentTheme()
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.Text)).
		Padding(1, 2).
		Height(3)
	focusedTitleStyle := titleStyle.Copy().
		Background(lipgloss.Color(theme.FocusBg)).
		Foreground(lipgloss.Color(theme.Text))

	title := "To Do"
	if idx == 1 {
		title = "Done"
	}
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
		Width(m.lists[idx].Width()).
		Height(m.lists[idx].Height()+2).
		Render(lipgloss.JoinVertical(lipgloss.Left, title, listView))
	return box
}

func (m model) renderFooter() string {
	if m.mode != modeNormal {
		return ""
	}
	theme := m.currentTheme()
	help := fmt.Sprintf("a:add  e:edit  d:delete  space:move  shift+k/j:reorder  t:theme (%s)  tab:switch  q:quit", theme.Name)
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		PaddingTop(1).
		Render(help)
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
		"enter:save  esc:cancel  tab:switch",
	}
	if m.errMsg != "" {
		lines = append(lines, "", errStyle.Render(m.errMsg))
	}
	return strings.Join(lines, "\n")
}

func (m *model) resizeLists() {
	gap := 2
	usableWidth := m.width - gap - 6
	if usableWidth < 40 {
		usableWidth = 40
	}
	listWidth := usableWidth / 2
	listHeight := m.height - 10
	if listHeight < 5 {
		listHeight = 5
	}
	for i := range m.lists {
		m.lists[i].SetSize(listWidth, listHeight)
	}
	m.applyTheme()
}

func (m *model) syncLists() {
	var todoItems []list.Item
	var doneItems []list.Item
	for _, t := range m.orderedTasks("todo") {
		todoItems = append(todoItems, taskItem{id: t.ID, title: t.Title, duration: t.Duration})
	}
	for _, t := range m.orderedTasks("done") {
		doneItems = append(doneItems, taskItem{id: t.ID, title: t.Title, duration: t.Duration})
	}
	m.lists[0].SetItems(todoItems)
	m.lists[1].SetItems(doneItems)
}

func (m *model) selectedTask() *Task {
	items := m.lists[m.focused].Items()
	if len(items) == 0 {
		return nil
	}
	item, ok := items[m.lists[m.focused].Index()].(taskItem)
	if !ok {
		return nil
	}
	return m.findTask(item.id)
}

func (m *model) findTask(id int) *Task {
	for i := range m.data.Tasks {
		if m.data.Tasks[i].ID == id {
			return &m.data.Tasks[i]
		}
	}
	return nil
}

func (m *model) deleteTask(id int) {
	for i := range m.data.Tasks {
		if m.data.Tasks[i].ID == id {
			m.data.Tasks = append(m.data.Tasks[:i], m.data.Tasks[i+1:]...)
			return
		}
	}
}

func (m *model) ensureReset() {
	today := time.Now().Format("2006-01-02")
	if m.data.LastReset != today {
		reset := append(m.orderedTasks("todo"), m.orderedTasks("done")...)
		for i, t := range reset {
			t.Status = "todo"
			t.Order = i + 1
		}
		m.data.LastReset = today
		m.syncLists()
		_ = saveData(m.dataPath, m.data)
	}
}

func parseDuration(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("Duration cannot be empty")
	}
	d, err := strconv.Atoi(s)
	if err != nil || d <= 0 {
		return 0, errors.New("Duration must be a positive integer")
	}
	return d, nil
}

func loadData(path string) (Data, error) {
	today := time.Now().Format("2006-01-02")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Data{LastReset: today, NextID: 1}, nil
		}
		return Data{}, err
	}

	var data Data
	if err := json.Unmarshal(b, &data); err != nil {
		return Data{}, err
	}
	if data.LastReset == "" {
		data.LastReset = today
	}
	if data.NextID == 0 {
		data.NextID = 1
	}
	if data.ThemeIndex < 0 || data.ThemeIndex >= len(themes) {
		data.ThemeIndex = 0
	}
	assignMissingOrders(&data)
	return data, nil
}

func (m model) currentTheme() Theme {
	idx := m.data.ThemeIndex
	if idx < 0 || idx >= len(themes) {
		idx = 0
	}
	return themes[idx]
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

func (m *model) pushHistory() {
	const maxHistory = 100
	m.history = append(m.history, cloneData(m.data))
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
	m.data = cloneData(last)
	return true
}

func cloneData(d Data) Data {
	out := d
	out.Tasks = make([]Task, len(d.Tasks))
	copy(out.Tasks, d.Tasks)
	return out
}

func saveData(path string, data Data) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func (m *model) orderedTasks(status string) []*Task {
	var tasks []*Task
	for i := range m.data.Tasks {
		if m.data.Tasks[i].Status == status {
			tasks = append(tasks, &m.data.Tasks[i])
		}
	}
	sortTasks(tasks)
	return tasks
}

func (m *model) nextOrder(status string) int {
	maxOrder := 0
	for i := range m.data.Tasks {
		if m.data.Tasks[i].Status == status && m.data.Tasks[i].Order > maxOrder {
			maxOrder = m.data.Tasks[i].Order
		}
	}
	return maxOrder + 1
}

func (m *model) moveTask(delta int) (bool, int) {
	status := "todo"
	if m.focused == 1 {
		status = "done"
	}
	ordered := m.orderedTasks(status)
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

func (m *model) moveTaskToOther() (bool, int) {
	t := m.selectedTask()
	if t == nil {
		return false, 0
	}
	if t.Status == "todo" {
		t.Status = "done"
	} else {
		t.Status = "todo"
	}
	t.Order = m.nextOrder(t.Status)
	newIdx := m.indexInStatus(t.ID, t.Status)
	return true, newIdx
}

func (m *model) indexInStatus(id int, status string) int {
	ordered := m.orderedTasks(status)
	for i := range ordered {
		if ordered[i].ID == id {
			return i
		}
	}
	return 0
}

func sortTasks(tasks []*Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Order == tasks[j].Order {
			return tasks[i].ID < tasks[j].ID
		}
		return tasks[i].Order < tasks[j].Order
	})
}

func assignMissingOrders(data *Data) {
	maxTodo := 0
	maxDone := 0
	for i := range data.Tasks {
		t := &data.Tasks[i]
		if t.Order != 0 {
			if t.Status == "done" && t.Order > maxDone {
				maxDone = t.Order
			} else if t.Status != "done" && t.Order > maxTodo {
				maxTodo = t.Order
			}
			continue
		}
		if t.Status == "done" {
			maxDone++
			t.Order = maxDone
		} else {
			maxTodo++
			t.Order = maxTodo
		}
	}
}
