package internal

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type setupStep int

const (
	setupStepBackend setupStep = iota
	setupStepServerURL
	setupStepUsername
	setupStepPassword
	setupStepConfirm
)

type setupSaveResultMsg struct {
	err error
}

type setupModel struct {
	configPath string

	step          setupStep
	backendChoice int
	serverInput   textinput.Model
	usernameInput textinput.Model
	passwordInput textinput.Model

	width  int
	height int
	errMsg string
	saving bool
	saved  bool
}

// RunSetupTUI runs the first-run backend setup flow and saves the result to the
// default config path.
func RunSetupTUI() error {
	configPath, err := DefaultConfigPath()
	if err != nil {
		return err
	}
	return runSetupTUIAtPath(configPath)
}

func runSetupTUIAtPath(configPath string) error {
	finalModel, err := tea.NewProgram(newSetupModel(configPath), tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	if m, ok := finalModel.(setupModel); ok && m.saved {
		return nil
	}
	return fmt.Errorf("%w: setup was cancelled", ErrBackendNotConfigured)
}

func newSetupModel(configPath string) setupModel {
	theme := GetTheme(0)
	inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Text))
	placeholderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent))

	serverInput := textinput.New()
	serverInput.Placeholder = "https://cloud.example.com"
	serverInput.CharLimit = 240
	serverInput.Width = 52
	serverInput.TextStyle = inputStyle
	serverInput.PlaceholderStyle = placeholderStyle
	serverInput.Cursor.Style = cursorStyle

	usernameInput := textinput.New()
	usernameInput.Placeholder = "nextcloud-user"
	usernameInput.CharLimit = 120
	usernameInput.Width = 52
	usernameInput.TextStyle = inputStyle
	usernameInput.PlaceholderStyle = placeholderStyle
	usernameInput.Cursor.Style = cursorStyle

	passwordInput := textinput.New()
	passwordInput.Placeholder = "app password"
	passwordInput.CharLimit = 240
	passwordInput.Width = 52
	passwordInput.EchoMode = textinput.EchoPassword
	passwordInput.EchoCharacter = '*'
	passwordInput.TextStyle = inputStyle
	passwordInput.PlaceholderStyle = placeholderStyle
	passwordInput.Cursor.Style = cursorStyle

	return setupModel{
		configPath:    configPath,
		step:          setupStepBackend,
		backendChoice: 0,
		serverInput:   serverInput,
		usernameInput: usernameInput,
		passwordInput: passwordInput,
		width:         80,
		height:        24,
	}
}

func (m setupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case setupSaveResultMsg:
		m.saving = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.saved = true
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.saving {
			return m, nil
		}
		switch m.step {
		case setupStepBackend:
			return m.updateSetupBackend(msg)
		case setupStepServerURL, setupStepUsername, setupStepPassword:
			return m.updateSetupInput(msg)
		case setupStepConfirm:
			return m.updateSetupConfirm(msg)
		}
	}
	return m, nil
}

func (m setupModel) updateSetupBackend(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return m, tea.Quit
	case "up", "k", "shift+tab":
		m.backendChoice = (m.backendChoice + 1) % 2
		m.errMsg = ""
		return m, nil
	case "down", "j", "tab":
		m.backendChoice = (m.backendChoice + 1) % 2
		m.errMsg = ""
		return m, nil
	case "1", "l":
		m.backendChoice = 0
		m.errMsg = ""
		return m, nil
	case "2", "n":
		m.backendChoice = 1
		m.errMsg = ""
		return m, nil
	case "enter", " ":
		m.errMsg = ""
		if m.selectedBackend() == BackendLocal {
			m.step = setupStepConfirm
			return m, nil
		}
		m.step = setupStepServerURL
		m.focusCurrentInput()
		return m, nil
	}
	return m, nil
}

func (m setupModel) updateSetupInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.errMsg = ""
		m.previousSetupStep()
		return m, nil
	case "enter":
		if err := m.validateCurrentInput(); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.errMsg = ""
		m.nextSetupStep()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.step {
	case setupStepServerURL:
		m.serverInput, cmd = m.serverInput.Update(msg)
	case setupStepUsername:
		m.usernameInput, cmd = m.usernameInput.Update(msg)
	case setupStepPassword:
		m.passwordInput, cmd = m.passwordInput.Update(msg)
	}
	return m, cmd
}

func (m setupModel) updateSetupConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc", "b":
		m.errMsg = ""
		m.previousSetupStep()
		return m, nil
	case "enter", "y", "Y":
		cfg := m.setupConfig()
		if !IsBackendConfigured(cfg) {
			m.errMsg = "backend configuration is incomplete"
			return m, nil
		}
		m.saving = true
		m.errMsg = ""
		return m, saveSetupConfigCmd(m.configPath, cfg)
	}
	return m, nil
}

func (m *setupModel) focusCurrentInput() {
	m.serverInput.Blur()
	m.usernameInput.Blur()
	m.passwordInput.Blur()
	switch m.step {
	case setupStepServerURL:
		m.serverInput.Focus()
	case setupStepUsername:
		m.usernameInput.Focus()
	case setupStepPassword:
		m.passwordInput.Focus()
	}
}

func (m *setupModel) nextSetupStep() {
	switch m.step {
	case setupStepServerURL:
		m.step = setupStepUsername
	case setupStepUsername:
		m.step = setupStepPassword
	case setupStepPassword:
		m.step = setupStepConfirm
	}
	m.focusCurrentInput()
}

func (m *setupModel) previousSetupStep() {
	switch m.step {
	case setupStepServerURL:
		m.step = setupStepBackend
	case setupStepUsername:
		m.step = setupStepServerURL
	case setupStepPassword:
		m.step = setupStepUsername
	case setupStepConfirm:
		if m.selectedBackend() == BackendLocal {
			m.step = setupStepBackend
		} else {
			m.step = setupStepPassword
		}
	}
	m.focusCurrentInput()
}

func (m *setupModel) validateCurrentInput() error {
	switch m.step {
	case setupStepServerURL:
		value := NormalizeServerURL(m.serverInput.Value())
		if err := validateSetupServerURL(value); err != nil {
			return err
		}
		m.serverInput.SetValue(value)
	case setupStepUsername:
		value := strings.TrimSpace(m.usernameInput.Value())
		if value == "" {
			return errors.New("username is required")
		}
		m.usernameInput.SetValue(value)
	case setupStepPassword:
		value := strings.TrimSpace(m.passwordInput.Value())
		if value == "" {
			return errors.New("app password is required")
		}
		m.passwordInput.SetValue(value)
	}
	return nil
}

func validateSetupServerURL(raw string) error {
	if raw == "" {
		return errors.New("server URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("server URL must include http:// or https:// and a host")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("server URL must start with http:// or https://")
	}
	return nil
}

func (m setupModel) selectedBackend() BackendKind {
	if m.backendChoice == 1 {
		return BackendNextcloud
	}
	return BackendLocal
}

func (m setupModel) setupConfig() AppConfig {
	if m.selectedBackend() == BackendLocal {
		return AppConfig{Backend: BackendLocal}
	}
	return NormalizeAppConfig(AppConfig{
		Backend: BackendNextcloud,
		Nextcloud: NextcloudConfig{
			ServerURL:   m.serverInput.Value(),
			LoginName:   m.usernameInput.Value(),
			AppPassword: m.passwordInput.Value(),
		},
	})
}

func saveSetupConfigCmd(path string, cfg AppConfig) tea.Cmd {
	return func() tea.Msg {
		return setupSaveResultMsg{err: SaveAppConfig(path, cfg)}
	}
}

func (m setupModel) View() string {
	theme := GetTheme(0)
	panelWidth := setupPanelWidth(m.width)
	innerWidth := max(1, panelWidth-6)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(theme.Text)).
		Render("Daily Tasks Setup")
	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Render("Choose where Daily Tasks stores and syncs your data.")

	lines := []string{
		title,
		subtitle,
		"",
		m.renderSetupProgress(innerWidth),
		"",
		m.renderSetupBody(innerWidth),
	}
	if m.errMsg != "" {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Render(m.errMsg))
	}
	if m.saving {
		lines = append(lines, "", lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Render("Saving configuration..."))
	}
	lines = append(lines, "", m.renderSetupHelp())

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.FocusBorder)).
		Background(lipgloss.Color(theme.PanelBg)).
		Foreground(lipgloss.Color(theme.Text)).
		Padding(1, 2).
		Width(panelWidth).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	return lipgloss.Place(
		max(1, m.width),
		max(1, m.height),
		lipgloss.Center,
		lipgloss.Center,
		panel,
		lipgloss.WithWhitespaceBackground(lipgloss.Color(theme.Bg)),
		lipgloss.WithWhitespaceForeground(lipgloss.Color(theme.Bg)),
	)
}

func setupPanelWidth(width int) int {
	if width <= 0 {
		return 72
	}
	panelWidth := width - 4
	if panelWidth > 72 {
		panelWidth = 72
	}
	if panelWidth < 44 {
		panelWidth = min(width, 44)
	}
	return max(1, panelWidth)
}

func (m setupModel) renderSetupProgress(width int) string {
	theme := GetTheme(0)
	steps := []string{"Backend"}
	if m.selectedBackend() == BackendNextcloud || m.step == setupStepBackend {
		steps = append(steps, "Server", "User", "Password")
	}
	steps = append(steps, "Confirm")

	current := m.progressIndex()
	parts := make([]string, len(steps))
	for i, step := range steps {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))
		if i == current {
			style = style.Foreground(lipgloss.Color(theme.Accent)).Bold(true)
		}
		parts[i] = style.Render(step)
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(parts, "  /  "))
}

func (m setupModel) progressIndex() int {
	switch m.step {
	case setupStepBackend:
		return 0
	case setupStepServerURL:
		return 1
	case setupStepUsername:
		return 2
	case setupStepPassword:
		return 3
	case setupStepConfirm:
		if m.selectedBackend() == BackendLocal {
			return 1
		}
		return 4
	default:
		return 0
	}
}

func (m setupModel) renderSetupBody(width int) string {
	switch m.step {
	case setupStepBackend:
		return m.renderBackendChoices(width)
	case setupStepServerURL:
		return m.renderInputStep("Nextcloud server URL", m.serverInput.View())
	case setupStepUsername:
		return m.renderInputStep("Nextcloud username", m.usernameInput.View())
	case setupStepPassword:
		return m.renderInputStep("Nextcloud app password", m.passwordInput.View())
	case setupStepConfirm:
		return m.renderSetupConfirm(width)
	default:
		return ""
	}
}

func (m setupModel) renderBackendChoices(width int) string {
	theme := GetTheme(0)
	lines := []string{"Choose a backend:"}
	choices := []struct {
		title string
		desc  string
	}{
		{title: "Local only", desc: "store data on this device without sync"},
		{title: "Nextcloud", desc: "sync through Nextcloud WebDAV"},
	}
	for i, choice := range choices {
		prefix := "  "
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Text)).
			Width(width)
		if i == m.backendChoice {
			prefix = "> "
			style = style.
				Bold(true).
				Background(lipgloss.Color(theme.FocusBg)).
				Foreground(lipgloss.Color(theme.Text))
		}
		lines = append(lines, style.Render(fmt.Sprintf("%s%s - %s", prefix, choice.title, choice.desc)))
	}
	return strings.Join(lines, "\n")
}

func (m setupModel) renderInputStep(label, input string) string {
	return strings.Join([]string{
		label,
		input,
	}, "\n")
}

func (m setupModel) renderSetupConfirm(width int) string {
	theme := GetTheme(0)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))
	lines := []string{"Confirm configuration:"}
	if m.selectedBackend() == BackendLocal {
		lines = append(lines, "Backend: Local only")
	} else {
		cfg := m.setupConfig()
		lines = append(lines,
			"Backend: Nextcloud",
			"Server: "+cfg.Nextcloud.ServerURL,
			"Username: "+cfg.Nextcloud.LoginName,
			"Remote path: "+cfg.Nextcloud.RemotePath,
			"App password: "+muted.Render("saved, not shown"),
		)
	}
	lines = append(lines, muted.Render("Config path: "+m.configPath))
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (m setupModel) renderSetupHelp() string {
	theme := GetTheme(0)
	help := "enter:next  esc:back  ctrl+c:cancel"
	switch m.step {
	case setupStepBackend:
		help = "up/down:choose  enter:next  q:cancel"
	case setupStepConfirm:
		help = "enter/y:save  esc/b:back  q:cancel"
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(theme.Muted)).
		Render(help)
}
