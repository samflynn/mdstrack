package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const applicationName = "mds"
const defaultGitHubHost = "github.ubc.ca"

type config struct {
	MDSFolder         string          `json:"mds_folder"`
	GitHubUsername    string          `json:"github_username"`
	GitHubProfileName string          `json:"github_profile_name"`
	GitHubHost        string          `json:"github_host"`
	Favorites         map[string]bool `json:"favorites"`
	Collapsed         map[string]bool `json:"collapsed"`
	FavoritesTitle    string          `json:"favorites_title"`
}

type mode int

const (
	setupMode mode = iota
	repositoriesMode
	settingsMode
)

type commitCountsMsg struct {
	counts map[string]int
	errors map[string]error
}

type repositoryListRow struct {
	group      string
	repository string
	index      int
	key        string
	header     bool
}

type model struct {
	mode              mode
	path              string
	githubUsername    string
	githubProfileName string
	githubHost        string
	setupStep         int
	settingsCursor    int
	editingSetting    string
	settingInput      string
	repositories      []string
	favorites         map[string]bool
	commitCounts      map[string]int
	commitErrors      map[string]error
	collapsed         map[string]bool
	favoritesTitle    string
	editingTitle      bool
	titleInput        string
	cursor            int
	err               error
	width             int
	height            int
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2)
	promptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	groupStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Italic(true)
	favoriteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Italic(true).Bold(true)
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
)

func main() {
	initial, err := newModel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if _, err := tea.NewProgram(initial, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newModel() (model, error) {
	stored, err := loadConfig()
	if errors.Is(err, os.ErrNotExist) {
		return model{mode: setupMode}, nil
	}
	if err != nil {
		return model{}, fmt.Errorf("load mds configuration: %w", err)
	}

	repositories, err := findRepositories(stored.MDSFolder)
	return model{mode: repositoriesMode, path: stored.MDSFolder, githubUsername: stored.GitHubUsername, githubProfileName: stored.GitHubProfileName, githubHost: configuredGitHubHost(stored.GitHubHost), repositories: repositories, favorites: stored.Favorites, commitCounts: make(map[string]int), commitErrors: make(map[string]error), collapsed: stored.Collapsed, favoritesTitle: stored.FavoritesTitle, err: err}, nil
}

func (m model) Init() tea.Cmd {
	if m.mode != repositoriesMode {
		return nil
	}
	return loadTrackedCommitCounts(m.path, m.repositories, m.favorites, m.githubUsername, m.githubProfileName, m.githubHost)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case commitCountsMsg:
		m.commitCounts, m.commitErrors = msg.counts, msg.errors
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || (msg.String() == "q" && m.mode != setupMode && !m.editingTitle && m.editingSetting == "") {
			return m, tea.Quit
		}
		if m.mode == settingsMode {
			return m.updateSettings(msg)
		}
		if m.mode == repositoriesMode {
			if m.editingTitle {
				switch msg.String() {
				case "enter":
					m.favoritesTitle = strings.TrimSpace(m.titleInput)
					m.editingTitle = false
					m.savePreferences()
				case "esc":
					m.editingTitle = false
				case "backspace", "ctrl+h":
					if len(m.titleInput) > 0 {
						m.titleInput = m.titleInput[:len(m.titleInput)-1]
					}
				default:
					if msg.Type == tea.KeyRunes {
						m.titleInput += string(msg.Runes)
					}
				}
				return m, nil
			}

			rows := m.repositoryListRows()
			switch msg.String() {
			case "s":
				m.mode, m.err = settingsMode, nil
			case "up", "k":
				m.cursor = m.moveCursor(rows, -1)
			case "down", "j":
				m.cursor = m.moveCursor(rows, 1)
			case "e":
				if len(rows) > 0 && rows[m.cursor].key == "favorites" {
					m.editingTitle, m.titleInput = true, m.favoritesDisplayTitle()
				}
			case "enter":
				if len(rows) > 0 {
					m.activateRow(rows[m.cursor])
					return m, loadTrackedCommitCounts(m.path, m.repositories, m.favorites, m.githubUsername, m.githubProfileName, m.githubHost)
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "enter":
			return m.advanceSetup()
		case "backspace", "ctrl+h":
			if m.setupStep == 0 && len(m.path) > 0 {
				m.path = m.path[:len(m.path)-1]
			}
			switch m.setupStep {
			case 1:
				m.githubUsername = removeLastRune(m.githubUsername)
			case 2:
				m.githubProfileName = removeLastRune(m.githubProfileName)
			case 3:
				m.githubHost = removeLastRune(m.githubHost)
			}
		default:
			if msg.Type == tea.KeyRunes {
				switch m.setupStep {
				case 0:
					m.path += string(msg.Runes)
				case 1:
					m.githubUsername += string(msg.Runes)
				case 2:
					m.githubProfileName += string(msg.Runes)
				case 3:
					m.githubHost += string(msg.Runes)
				}
			}
		}
	}
	return m, nil
}

func (m model) advanceSetup() (tea.Model, tea.Cmd) {
	if m.setupStep == 0 {
		folder := strings.TrimSpace(m.path)
		if folder == "" {
			m.err = errors.New("enter the path to your UBC MDS folder")
			return m, nil
		}
		absoluteFolder, err := filepath.Abs(filepath.Clean(folder))
		if err != nil {
			m.err = fmt.Errorf("resolve folder: %w", err)
			return m, nil
		}
		info, err := os.Stat(absoluteFolder)
		if err != nil {
			m.err = fmt.Errorf("open folder: %w", err)
			return m, nil
		}
		if !info.IsDir() {
			m.err = errors.New("the MDS path must be a folder")
			return m, nil
		}
		m.path, m.setupStep, m.err = absoluteFolder, 1, nil
		return m, nil
	}

	if m.setupStep == 1 && strings.TrimSpace(m.githubUsername) == "" {
		m.err = errors.New("enter your GitHub username")
		return m, nil
	}
	if m.setupStep == 2 && strings.TrimSpace(m.githubProfileName) == "" {
		m.err = errors.New("enter your GitHub profile display name")
		return m, nil
	}
	if m.setupStep < 3 {
		m.setupStep++
		m.err = nil
		if m.setupStep == 3 && m.githubHost == "" {
			m.githubHost = defaultGitHubHost
		}
		return m, nil
	}
	folder := strings.TrimSpace(m.path)
	username := strings.TrimSpace(m.githubUsername)
	profileName := strings.TrimSpace(m.githubProfileName)
	host := normalizeGitHubHost(m.githubHost)
	if host == "" {
		m.err = errors.New("enter your GitHub host")
		return m, nil
	}
	if err := saveConfig(config{MDSFolder: folder, GitHubUsername: username, GitHubProfileName: profileName, GitHubHost: host, Favorites: make(map[string]bool), Collapsed: make(map[string]bool)}); err != nil {
		m.err = fmt.Errorf("save configuration: %w", err)
		return m, nil
	}
	repositories, err := findRepositories(folder)
	m.mode, m.path, m.githubUsername, m.githubProfileName, m.githubHost, m.repositories, m.favorites, m.collapsed, m.err = repositoriesMode, folder, username, profileName, host, repositories, make(map[string]bool), make(map[string]bool), err
	return m, nil
}

func (m model) View() string {
	switch m.mode {
	case setupMode:
		return m.setupView()
	case settingsMode:
		return m.settingsView()
	default:
		return m.repositoriesView()
	}
}

func (m model) setupView() string {
	width := m.contentWidth()
	label, value := "Location of UBC MDS folder:", m.path
	if m.setupStep == 1 {
		label, value = "GitHub username:", m.githubUsername
	} else if m.setupStep == 2 {
		label, value = "GitHub profile display name:", m.githubProfileName
	} else if m.setupStep == 3 {
		label, value = "GitHub host:", m.githubHost
	}
	lines := []string{titleStyle.Render("mds"), mutedStyle.Render("UBC MDS repository tracker"), "", "First-time setup", "", label, promptStyle.Render("> " + truncateLeft(value, max(1, width-3)) + "█")}
	if m.err != nil {
		lines = append(lines, "", errorStyle.Render(truncate("Error: "+m.err.Error(), width)))
	}
	lines = append(lines, "", mutedStyle.Render(truncate("Enter to continue · q or Ctrl+C to quit", width)))
	return m.fullScreen(lines)
}

func (m model) settingsView() string {
	width := m.contentWidth()
	values := settingsValues(m)
	lines := []string{titleStyle.Render("mds") + "  " + mutedStyle.Render("Settings"), ""}
	for index, setting := range values {
		prefix := "  "
		if index == m.settingsCursor {
			prefix = "▸ "
		}
		style := mutedStyle
		if index == m.settingsCursor {
			style = cursorStyle
		}
		lines = append(lines, style.Render(prefix+truncate(setting.label+": "+setting.value, width)))
	}
	if m.editingSetting != "" {
		lines = append(lines, "", promptStyle.Render("New value: "+truncateLeft(m.settingInput, max(1, width-12))+"█"), mutedStyle.Render("Enter save · Esc cancel"))
	}
	if m.err != nil {
		lines = append(lines, "", errorStyle.Render(truncate("Error: "+m.err.Error(), width)))
	}
	lines = append(lines, "", mutedStyle.Render("↑/↓ select · Enter edit · Esc back · q quit"))
	return m.fullScreen(lines)
}

type settingValue struct {
	key   string
	label string
	value string
}

func settingsValues(m model) []settingValue {
	return []settingValue{
		{key: "workspace", label: "Workspace", value: m.path},
		{key: "github", label: "GitHub username", value: m.githubUsername},
		{key: "profile-name", label: "GitHub profile display name", value: m.githubProfileName},
		{key: "host", label: "GitHub host", value: m.githubHost},
	}
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editingSetting != "" {
		switch msg.String() {
		case "enter":
			return m.saveSetting()
		case "esc":
			m.editingSetting, m.settingInput = "", ""
		case "backspace", "ctrl+h":
			if len(m.settingInput) > 0 {
				m.settingInput = m.settingInput[:len(m.settingInput)-1]
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.settingInput += string(msg.Runes)
			}
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.mode, m.err = repositoriesMode, nil
	case "up", "k":
		m.settingsCursor = max(0, m.settingsCursor-1)
	case "down", "j":
		m.settingsCursor = min(len(settingsValues(m))-1, m.settingsCursor+1)
	case "enter":
		setting := settingsValues(m)[m.settingsCursor]
		m.editingSetting, m.settingInput = setting.key, setting.value
	}
	return m, nil
}

func (m model) saveSetting() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.settingInput)
	if value == "" {
		m.err = errors.New("a setting cannot be empty")
		return m, nil
	}
	if m.editingSetting == "workspace" {
		folder, err := filepath.Abs(filepath.Clean(value))
		if err != nil {
			m.err = fmt.Errorf("resolve folder: %w", err)
			return m, nil
		}
		info, err := os.Stat(folder)
		if err != nil {
			m.err = fmt.Errorf("open folder: %w", err)
			return m, nil
		}
		if !info.IsDir() {
			m.err = errors.New("the workspace path must be a folder")
			return m, nil
		}
		repositories, err := findRepositories(folder)
		if err != nil {
			m.err = fmt.Errorf("scan repositories: %w", err)
			return m, nil
		}
		m.path, m.repositories = folder, repositories
	} else if m.editingSetting == "github" {
		m.githubUsername = value
	} else if m.editingSetting == "profile-name" {
		m.githubProfileName = value
	} else {
		host := normalizeGitHubHost(value)
		if host == "" {
			m.err = errors.New("GitHub host cannot be empty")
			return m, nil
		}
		m.githubHost = host
	}
	m.editingSetting, m.settingInput, m.err = "", "", nil
	m.savePreferences()
	return m, nil
}

func (m model) repositoriesView() string {
	width := m.contentWidth()
	lines := []string{
		titleStyle.Render("mds") + "  " + mutedStyle.Render("UBC MDS repositories"),
		mutedStyle.Render(truncate(m.path, width)),
		"",
	}
	if m.err != nil {
		lines = append(lines, errorStyle.Render(truncate("Unable to scan repositories: "+m.err.Error(), width)))
	} else if len(m.repositories) == 0 {
		lines = append(lines, mutedStyle.Render("No Git repositories found in this folder."))
	} else {
		rows := m.repositoryListRows()
		start, end := m.repositoryWindow(rows)
		for index, row := range rows[start:end] {
			selected := start+index == m.cursor
			if row.key == "divider" {
				lines = append(lines, mutedStyle.Render(strings.Repeat("─", width)))
				continue
			}
			indent := ""
			if strings.HasPrefix(row.key, "favorite:") {
				indent = "  "
			}
			if row.header {
				prefix := "⌄ "
				if m.collapsed[row.key] {
					prefix = "› "
				}
				label := indent + prefix + truncate(strings.ToUpper(row.group), width-len(indent)-3)
				if selected {
					lines = append(lines, cursorStyle.Render("▸ "+label))
				} else if row.key == "favorites" {
					lines = append(lines, favoriteStyle.Render("★ "+label))
				} else {
					lines = append(lines, groupStyle.Render(label))
				}
				continue
			}
			marker := "[ ]"
			if m.favorites[m.repositories[row.index]] {
				marker = "[X]"
			}
			countText := ""
			if m.favorites[m.repositories[row.index]] {
				if count, ok := m.commitCounts[m.repositories[row.index]]; ok {
					countText = fmt.Sprintf("%d commits", count)
				} else if m.commitErrors[m.repositories[row.index]] != nil {
					countText = "GH ERROR"
				}
			}
			prefix := indent + marker + " "
			if selected {
				prefix = "▸ " + prefix
			}
			available := width - lipgloss.Width(prefix) - lipgloss.Width(countText)
			name := truncate(row.repository, max(1, available-1))
			rowText := prefix + name
			if countText != "" {
				rowText += strings.Repeat(" ", max(1, width-lipgloss.Width(rowText)-lipgloss.Width(countText))) + countText
			}
			if selected {
				lines = append(lines, cursorStyle.Render(rowText))
			} else {
				lines = append(lines, selectedStyle.Render(prefix)+name+mutedStyle.Render(strings.TrimPrefix(rowText, prefix+name)))
			}
		}
		if start > 0 || end < len(rows) {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d–%d of %d", start+1, end, len(rows))))
		}
	}
	if m.editingTitle {
		lines = append(lines, "", promptStyle.Render("Rename tracking: "+truncate(m.titleInput, max(1, width-17))+"█"), mutedStyle.Render("Enter save · Esc cancel"))
	}
	if repository, err := m.firstCommitError(); err != nil {
		lines = append(lines, "", errorStyle.Render(truncate(fmt.Sprintf("gh error (%s): %v", repository, err), width)))
	}
	footer := fmt.Sprintf("%d repos · ↑/↓ move · Enter track · e rename tracking · s settings · q quit", len(m.repositories))
	lines = append(lines, "", mutedStyle.Render(truncate(footer, width)))
	return m.fullScreen(lines)
}

func (m model) firstCommitError() (string, error) {
	for _, repository := range m.repositories {
		if err := m.commitErrors[repository]; err != nil {
			return repository, err
		}
	}
	return "", nil
}

func (m model) favoritesDisplayTitle() string {
	if m.favoritesTitle == "" {
		return "TRACKING"
	}
	return strings.ToUpper(m.favoritesTitle) + " (TRACKING)"
}

func (m *model) activateRow(row repositoryListRow) {
	if row.key == "divider" {
		return
	}
	if row.header {
		if m.collapsed == nil {
			m.collapsed = make(map[string]bool)
		}
		m.collapsed[row.key] = !m.collapsed[row.key]
		m.savePreferences()
		return
	}
	if m.favorites == nil {
		m.favorites = make(map[string]bool)
	}
	nextRepository := ""
	for _, next := range m.repositoryListRows()[m.cursor+1:] {
		if !next.header {
			nextRepository = m.repositories[next.index]
			break
		}
	}
	repository := m.repositories[row.index]
	m.favorites[repository] = !m.favorites[repository]
	m.savePreferences()
	if nextRepository != "" {
		m.cursor = m.rowCursorForRepository(nextRepository)
	} else {
		m.cursor = min(m.cursor, max(0, len(m.repositoryListRows())-1))
	}
}

func (m *model) savePreferences() {
	if err := saveConfig(config{MDSFolder: m.path, GitHubUsername: m.githubUsername, GitHubProfileName: m.githubProfileName, GitHubHost: m.githubHost, Favorites: m.favorites, Collapsed: m.collapsed, FavoritesTitle: m.favoritesTitle}); err != nil {
		m.err = fmt.Errorf("save preferences: %w", err)
	}
}

func (m model) rowCursorForRepository(repository string) int {
	for cursor, row := range m.repositoryListRows() {
		if !row.header && m.repositories[row.index] == repository {
			return cursor
		}
	}
	return 0
}

func (m model) repositoryListRows() []repositoryListRow {
	favorites, regular := []int{}, []int{}
	for index, repository := range m.repositories {
		if m.favorites[repository] {
			favorites = append(favorites, index)
		} else {
			regular = append(regular, index)
		}
	}
	rows := []repositoryListRow{}
	if len(favorites) > 0 {
		rows = append(rows, repositoryListRow{group: m.favoritesDisplayTitle(), key: "favorites", header: true})
		if !m.collapsed["favorites"] {
			rows = appendGroupedRepositoryRows(rows, m.repositories, favorites, "favorite:", m.collapsed)
		}
		if len(regular) > 0 {
			rows = append(rows, repositoryListRow{key: "divider"})
		}
	}
	return appendGroupedRepositoryRows(rows, m.repositories, regular, "group:", m.collapsed)
}

func appendGroupedRepositoryRows(rows []repositoryListRow, repositories []string, indexes []int, prefix string, collapsed map[string]bool) []repositoryListRow {
	previousGroup := ""
	for _, index := range indexes {
		group, name := repositoryGroup(repositories[index])
		key := prefix + group
		if group != previousGroup {
			rows = append(rows, repositoryListRow{group: group, key: key, header: true})
			previousGroup = group
		}
		if !collapsed[key] {
			rows = append(rows, repositoryListRow{repository: name, index: index, key: prefix + "repository"})
		}
	}
	return rows
}

func (m model) moveCursor(rows []repositoryListRow, direction int) int {
	cursor := m.cursor
	for candidate := cursor + direction; candidate >= 0 && candidate < len(rows); candidate += direction {
		if rows[candidate].key != "divider" {
			return candidate
		}
	}
	return cursor
}

func (m model) repositoryWindow(rows []repositoryListRow) (int, int) {
	visible := max(1, m.contentHeight()-6)
	cursorRow := min(m.cursor, max(0, len(rows)-1))
	start := max(0, cursorRow-visible+1)
	end := min(len(rows), start+visible)
	if end-start < visible {
		start = max(0, end-visible)
	}
	return start, end
}

func repositoryGroup(repository string) (string, string) {
	parts := strings.Split(filepath.ToSlash(repository), "/")
	if len(parts) == 1 {
		return "ROOT", parts[0]
	}
	return parts[0], strings.Join(parts[1:], "/")
}

func (m model) fullScreen(lines []string) string {
	width := m.viewportWidth()
	height := m.viewportHeight()
	content := strings.Join(lines, "\n")
	panel := borderStyle.Width(max(1, width-6)).Height(max(1, height-4)).Render(content)
	return "\n" + lipgloss.NewStyle().PaddingLeft(2).Render(panel) + "\n"
}

func (m model) viewportWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

func (m model) viewportHeight() int {
	if m.height > 0 {
		return m.height
	}
	return 24
}

func (m model) contentWidth() int {
	return max(1, m.viewportWidth()-10)
}

func (m model) contentHeight() int {
	return max(1, m.viewportHeight()-6)
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func truncateLeft(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[len(runes)-width:])
	}
	return "…" + string(runes[len(runes)-width+1:])
}

func findRepositories(root string) ([]string, error) {
	var repositories []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() || entry.Name() != ".git" {
			return nil
		}
		repository := filepath.Dir(path)
		relative, err := filepath.Rel(root, repository)
		if err != nil {
			return err
		}
		repositories = append(repositories, relative)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(repositories)
	return repositories, nil
}

func loadTrackedCommitCounts(workspace string, repositories []string, favorites map[string]bool, username, profileName, configuredHost string) tea.Cmd {
	return func() tea.Msg {
		counts := make(map[string]int)
		failures := make(map[string]error)
		if username == "" {
			return commitCountsMsg{counts: counts, errors: failures}
		}
		for _, repository := range repositories {
			if !favorites[repository] {
				continue
			}
			fullPath := filepath.Join(workspace, repository)
			remote, err := gitOriginURL(fullPath)
			if err != nil {
				failures[repository] = err
				continue
			}
			host, owner, name, ok := githubRepository(remote)
			if !ok {
				failures[repository] = errors.New("origin is not hosted on GitHub")
				continue
			}
			if !strings.EqualFold(host, configuredHost) {
				failures[repository] = fmt.Errorf("origin host %q does not match configured GitHub host %q", host, configuredHost)
				continue
			}
			count, err := githubCommitCount(configuredHost, owner, name, username, profileName)
			if err != nil {
				failures[repository] = err
				continue
			}
			counts[repository] = count
		}
		return commitCountsMsg{counts: counts, errors: failures}
	}
}

func gitOriginURL(repository string) (string, error) {
	output, err := exec.Command("git", "-C", repository, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return "", fmt.Errorf("read origin URL: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func githubRepository(remote string) (string, string, string, bool) {
	remote = strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	var host, path string
	switch {
	case strings.HasPrefix(remote, "git@"):
		parts := strings.SplitN(strings.TrimPrefix(remote, "git@"), ":", 2)
		if len(parts) != 2 {
			return "", "", "", false
		}
		host, path = parts[0], parts[1]
	case strings.HasPrefix(remote, "https://"):
		parts := strings.SplitN(strings.TrimPrefix(remote, "https://"), "/", 2)
		if len(parts) != 2 {
			return "", "", "", false
		}
		host, path = parts[0], parts[1]
	case strings.HasPrefix(remote, "ssh://git@"):
		parts := strings.SplitN(strings.TrimPrefix(remote, "ssh://git@"), "/", 2)
		if len(parts) != 2 {
			return "", "", "", false
		}
		host, path = parts[0], parts[1]
	default:
		return "", "", "", false
	}
	if !strings.Contains(host, "github") {
		return "", "", "", false
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	return host, parts[0], parts[1], true
}

type githubCommit struct {
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	Commit struct {
		Author struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commit"`
}

func githubCommitCount(host, owner, repository, username, displayName string) (int, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/commits?per_page=100", owner, repository)
	output, err := exec.Command("gh", "api", "--hostname", host, "--paginate", "--slurp", endpoint).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("query GitHub commits: %s", strings.TrimSpace(string(output)))
	}
	var pages [][]githubCommit
	if err := json.Unmarshal(output, &pages); err != nil {
		return 0, fmt.Errorf("decode GitHub commits: %w", err)
	}
	count := 0
	for _, commits := range pages {
		for _, commit := range commits {
			if commitMatchesUser(commit, username, displayName) {
				count++
			}
		}
	}
	return count, nil
}

func commitMatchesUser(commit githubCommit, username, displayName string) bool {
	if commit.Author != nil && strings.EqualFold(commit.Author.Login, username) {
		return true
	}
	return displayName != "" && strings.EqualFold(commit.Commit.Author.Name, displayName)
}

func configPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, applicationName, "config.json"), nil
}

func loadConfig() (config, error) {
	path, err := configPath()
	if err != nil {
		return config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}
	var stored config
	if err := json.Unmarshal(data, &stored); err != nil {
		return config{}, err
	}
	if stored.MDSFolder == "" {
		return config{}, errors.New("configuration has no MDS folder")
	}
	if stored.Favorites == nil {
		stored.Favorites = make(map[string]bool)
	}
	if stored.Collapsed == nil {
		stored.Collapsed = make(map[string]bool)
	}
	stored.GitHubHost = configuredGitHubHost(stored.GitHubHost)
	return stored, nil
}

func configuredGitHubHost(host string) string {
	if normalized := normalizeGitHubHost(host); normalized != "" {
		return normalized
	}
	return defaultGitHubHost
}

func normalizeGitHubHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	return strings.TrimSuffix(host, "/")
}

func removeLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}

func saveConfig(stored config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
