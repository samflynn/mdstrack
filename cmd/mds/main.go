package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const applicationName = "mds"
const defaultGitHubHost = "github.ubc.ca"
const commitGoal = 3
const commitProgressWidth = 12
const defaultRefreshIntervalMinutes = 5

type config struct {
	MDSFolder              string          `json:"mds_folder"`
	GitHubUsername         string          `json:"github_username"`
	GitHubProfileName      string          `json:"github_profile_name"`
	GitHubHost             string          `json:"github_host"`
	DeadlineOverride       string          `json:"deadline_override,omitempty"`
	RefreshIntervalMinutes int             `json:"refresh_interval_minutes"`
	Favorites              map[string]bool `json:"favorites"`
	Collapsed              map[string]bool `json:"collapsed"`
	FavoritesTitle         string          `json:"favorites_title"`
}

type mode int

const (
	setupMode mode = iota
	repositoriesMode
	settingsMode
	searchMode
)

type commitCountsMsg struct {
	counts       map[string]int
	errors       map[string]error
	repositories []string
}

type commitSpinnerTickMsg struct{}

type refreshTickMsg struct{}

type workspaceRefreshMsg struct {
	repositories []string
	err          error
	refreshedAt  time.Time
}

type repositoryActionMsg struct {
	err error
}

type githubSearchResultsMsg struct {
	repositories []githubSearchRepository
	err          error
}

type clonedRepositoriesMsg struct {
	repositories map[string]bool
}

type cloneRepositoryMsg struct {
	repository string
	path       string
	err        error
}

type githubSearchRepository struct {
	FullName string `json:"full_name"`
}

type repositoryListRow struct {
	group      string
	repository string
	index      int
	key        string
	header     bool
}

type model struct {
	mode                   mode
	path                   string
	githubUsername         string
	githubProfileName      string
	githubHost             string
	deadlineOverride       string
	refreshIntervalMinutes int
	lastRefresh            time.Time
	nextRefresh            time.Time
	setupStep              int
	settingsCursor         int
	editingSetting         string
	settingInput           string
	repositories           []string
	favorites              map[string]bool
	commitCounts           map[string]int
	commitErrors           map[string]error
	loadingCommits         bool
	spinnerFrame           int
	collapsed              map[string]bool
	favoritesTitle         string
	editingTitle           bool
	titleInput             string
	githubActions          bool
	editorActions          bool
	searchChooser          bool
	localSearchActive      bool
	localSearch            string
	helpVisible            bool
	actionStatus           string
	githubSearch           string
	searchResults          []githubSearchRepository
	searchCursor           int
	searchLoading          bool
	searchError            error
	hideCloned             bool
	clonedRepositories     map[string]bool
	cloneConfirmation      string
	cloneStatus            string
	cursor                 int
	err                    error
	width                  int
	height                 int
}

var (
	titleStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	mutedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	borderStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2)
	promptStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	selectedStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	groupStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Italic(true)
	favoriteStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Italic(true).Bold(true)
	cursorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	selectedRowStyle    = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	railStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("235")).Padding(1, 2)
	railActiveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Background(lipgloss.Color("237")).Bold(true).PaddingLeft(1)
	dialogStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("141")).Padding(1, 2)
	commitSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
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
	now := time.Now()
	refreshInterval := configuredRefreshInterval(stored.RefreshIntervalMinutes)
	return model{mode: repositoriesMode, path: stored.MDSFolder, githubUsername: stored.GitHubUsername, githubProfileName: stored.GitHubProfileName, githubHost: configuredGitHubHost(stored.GitHubHost), deadlineOverride: stored.DeadlineOverride, refreshIntervalMinutes: refreshInterval, lastRefresh: now, nextRefresh: now.Add(time.Duration(refreshInterval) * time.Minute), repositories: repositories, favorites: stored.Favorites, commitCounts: make(map[string]int), commitErrors: make(map[string]error), loadingCommits: hasTrackedRepositories(stored.Favorites), collapsed: stored.Collapsed, favoritesTitle: stored.FavoritesTitle, err: err}, nil
}

func (m model) Init() tea.Cmd {
	if m.mode != repositoriesMode {
		return nil
	}
	return tea.Batch(loadTrackedCommitCounts(m.path, m.repositories, m.favorites, m.githubUsername, m.githubProfileName, m.githubHost), nextCommitSpinnerTick(), nextRefreshTick())
}

func (m *model) startCommitCountLoad(repositories []string) tea.Cmd {
	m.loadingCommits = len(repositories) > 0
	if !m.loadingCommits {
		return nil
	}
	return tea.Batch(loadTrackedCommitCounts(m.path, repositories, m.favorites, m.githubUsername, m.githubProfileName, m.githubHost), nextCommitSpinnerTick())
}

func hasTrackedRepositories(favorites map[string]bool) bool {
	for _, tracked := range favorites {
		if tracked {
			return true
		}
	}
	return false
}

func nextRefreshTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m model) refreshWorkspace() tea.Cmd {
	return func() tea.Msg {
		repositories, err := findRepositories(m.path)
		return workspaceRefreshMsg{repositories: repositories, err: err, refreshedAt: time.Now()}
	}
}

func nextCommitSpinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return commitSpinnerTickMsg{}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case commitCountsMsg:
		for _, repository := range msg.repositories {
			delete(m.commitCounts, repository)
			delete(m.commitErrors, repository)
		}
		for repository, count := range msg.counts {
			m.commitCounts[repository] = count
		}
		for repository, err := range msg.errors {
			m.commitErrors[repository] = err
		}
		m.loadingCommits = false
	case commitSpinnerTickMsg:
		if m.loadingCommits {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(commitSpinnerFrames)
			return m, nextCommitSpinnerTick()
		}
	case refreshTickMsg:
		if m.mode == repositoriesMode && !m.nextRefresh.After(time.Now()) {
			return m, m.refreshWorkspace()
		}
		return m, nextRefreshTick()
	case workspaceRefreshMsg:
		m.lastRefresh = msg.refreshedAt
		m.nextRefresh = msg.refreshedAt.Add(time.Duration(m.refreshIntervalMinutes) * time.Minute)
		if msg.err != nil {
			m.err = fmt.Errorf("refresh repositories: %w", msg.err)
			return m, nextRefreshTick()
		}
		m.repositories, m.err = msg.repositories, nil
		return m, tea.Batch(loadTrackedCommitCounts(m.path, m.repositories, m.favorites, m.githubUsername, m.githubProfileName, m.githubHost), findClonedRepositories(m.path, m.repositories, m.githubHost), nextRefreshTick())
	case repositoryActionMsg:
		if msg.err != nil {
			m.actionStatus = "Action failed: " + msg.err.Error()
		}
	case githubSearchResultsMsg:
		m.searchResults, m.searchError, m.searchLoading = msg.repositories, msg.err, false
	case clonedRepositoriesMsg:
		m.clonedRepositories = msg.repositories
	case cloneRepositoryMsg:
		if msg.err != nil {
			m.cloneStatus = "Clone failed: " + msg.err.Error()
			break
		}
		m.cloneStatus = "Cloned " + msg.repository
		if m.clonedRepositories == nil {
			m.clonedRepositories = make(map[string]bool)
		}
		m.clonedRepositories[msg.repository] = true
		m.repositories, m.err = findRepositories(m.path)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || (msg.String() == "q" && m.mode != setupMode && !m.editingTitle && m.editingSetting == "") {
			return m, tea.Quit
		}
		if m.helpVisible {
			if msg.String() == "esc" || msg.String() == "?" {
				m.helpVisible = false
			}
			return m, nil
		}
		if m.mode == settingsMode {
			return m.updateSettings(msg)
		}
		if m.mode == searchMode {
			return m.updateSearch(msg)
		}
		if m.mode == repositoriesMode {
			if m.searchChooser {
				return m.updateSearchChooser(msg)
			}
			if m.editorActions {
				return m.updateEditorActions(msg)
			}
			if m.githubActions {
				return m.updateGitHubActions(msg)
			}
			if m.localSearchActive {
				switch msg.String() {
				case "esc":
					m.localSearch, m.localSearchActive, m.cursor = "", false, 0
					return m, nil
				case "backspace", "ctrl+h":
					m.localSearch, m.cursor = removeLastRune(m.localSearch), 0
					return m, nil
				default:
					if isTextInputKey(msg) {
						m.localSearch, m.cursor = m.localSearch+string(msg.Runes), 0
						return m, nil
					}
				}
			}
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
					if isTextInputKey(msg) {
						m.titleInput += string(msg.Runes)
					}
				}
				return m, nil
			}

			rows := m.repositoryListRows()
			switch msg.String() {
			case "?":
				m.helpVisible = true
			case "/":
				m.searchChooser, m.actionStatus = true, ""
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
			case "o":
				if _, ok := m.selectedRepository(rows); ok {
					m.editorActions, m.actionStatus = true, ""
				}
			case "g":
				if _, ok := m.selectedRepository(rows); ok {
					m.githubActions, m.actionStatus = true, ""
				}
			case "enter":
				if len(rows) > 0 {
					repository, tracked := m.activateRow(rows[m.cursor])
					if tracked {
						return m, m.startCommitCountLoad([]string{repository})
					}
					return m, nil
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
			if isTextInputKey(msg) {
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
	if err := saveConfig(config{MDSFolder: folder, GitHubUsername: username, GitHubProfileName: profileName, GitHubHost: host, RefreshIntervalMinutes: defaultRefreshIntervalMinutes, Favorites: make(map[string]bool), Collapsed: make(map[string]bool)}); err != nil {
		m.err = fmt.Errorf("save configuration: %w", err)
		return m, nil
	}
	repositories, err := findRepositories(folder)
	now := time.Now()
	m.mode, m.path, m.githubUsername, m.githubProfileName, m.githubHost, m.refreshIntervalMinutes, m.lastRefresh, m.nextRefresh, m.repositories, m.favorites, m.collapsed, m.err = repositoriesMode, folder, username, profileName, host, defaultRefreshIntervalMinutes, now, now.Add(defaultRefreshIntervalMinutes*time.Minute), repositories, make(map[string]bool), make(map[string]bool), err
	return m, nil
}

func (m model) View() string {
	switch m.mode {
	case setupMode:
		return m.setupView()
	case settingsMode:
		return m.settingsView()
	case searchMode:
		return m.searchView()
	default:
		if m.helpVisible {
			return m.helpView()
		}
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
	lines := []string{m.screenTitle("First-time setup"), mutedStyle.Render("Connect your workspace and GitHub identity."), "", promptStyle.Render(fmt.Sprintf("STEP %d OF 4", m.setupStep+1)), "", label, promptStyle.Render("> " + truncateLeft(value, max(1, width-3)) + "█")}
	if m.err != nil {
		lines = append(lines, "", errorStyle.Render(truncate("Error: "+m.err.Error(), width)))
	}
	lines = append(lines, "", mutedStyle.Render(truncate("Enter to continue · q or Ctrl+C to quit", width)))
	return m.fullScreen(lines)
}

func (m model) effectiveDeadline(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if m.deadlineOverride != "" {
		override, err := time.ParseInLocation("2006-01-02", m.deadlineOverride, now.Location())
		if err == nil && !override.Before(today) {
			return override
		}
	}
	return nextSaturday(today)
}

func nextSaturday(day time.Time) time.Time {
	return day.AddDate(0, 0, (int(time.Saturday)-int(day.Weekday())+7)%7)
}

func (m model) settingsView() string {
	width := m.contentWidth()
	values := settingsValues(m)
	lines := []string{m.screenTitle("Settings"), mutedStyle.Render("Workspace defaults and refresh behavior."), ""}
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

func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.cloneConfirmation != "" {
		switch msg.String() {
		case "y":
			repository := m.cloneConfirmation
			m.cloneConfirmation = ""
			return m, cloneGitHubRepository(m.path, m.githubHost, repository)
		case "n", "esc":
			m.cloneConfirmation = ""
		}
		return m, nil
	}
	if m.searchLoading {
		if msg.String() == "esc" {
			m.mode = repositoriesMode
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.mode = repositoriesMode
	case "up", "k":
		m.searchCursor = max(0, m.searchCursor-1)
	case "down", "j":
		m.searchCursor = min(max(0, len(m.filteredSearchResults())-1), m.searchCursor+1)
	case "h":
		m.hideCloned = !m.hideCloned
		m.searchCursor = 0
	case "enter":
		results := m.filteredSearchResults()
		if m.searchCursor < len(results) {
			m.cloneConfirmation, m.cloneStatus = results[m.searchCursor].FullName, ""
		}
	case "backspace", "ctrl+h":
		m.githubSearch = removeLastRune(m.githubSearch)
		m.searchCursor = 0
	default:
		if isTextInputKey(msg) {
			m.githubSearch += string(msg.Runes)
			m.searchCursor = 0
		}
	}
	return m, nil
}

func (m model) searchView() string {
	width := m.contentWidth()
	lines := []string{m.screenTitle("GitHub search"), mutedStyle.Render("Accessible repositories on " + m.githubHost), "", promptStyle.Render("/ " + truncateLeft(m.githubSearch, max(1, width-3)) + "█")}
	if m.searchLoading {
		lines = append(lines, "", mutedStyle.Render("Loading accessible repositories..."))
	} else if m.searchError != nil {
		lines = append(lines, "", errorStyle.Render(truncate("Unable to load repositories: "+m.searchError.Error(), width)))
	} else {
		results := m.filteredSearchResults()
		visible := max(1, m.contentHeight()-8)
		start := max(0, min(m.searchCursor, max(0, len(results)-visible)))
		end := min(len(results), start+visible)
		for index, repository := range results[start:end] {
			prefix := "  "
			style := mutedStyle
			if start+index == m.searchCursor {
				prefix, style = "▸ ", cursorStyle
			}
			lines = append(lines, style.Render(prefix+truncate(repository.FullName, width-2)))
		}
		if len(results) == 0 {
			lines = append(lines, "", mutedStyle.Render("No accessible repositories match your search."))
		}
	}
	hideClonedLabel := "h hide cloned"
	if m.hideCloned {
		hideClonedLabel = "h show cloned"
	}
	footer := "Type to filter · ↑/↓ move · Enter clone · " + hideClonedLabel + " · Esc back · q quit"
	if m.cloneConfirmation != "" {
		footer = "Clone " + m.cloneConfirmation + " into workspace? y yes · n no"
	} else if m.cloneStatus != "" {
		footer = m.cloneStatus
	}
	lines = append(lines, "", mutedStyle.Render(truncate(footer, width)))
	return m.fullScreen(lines)
}

func (m model) filteredSearchResults() []githubSearchRepository {
	query := strings.ToLower(strings.TrimSpace(m.githubSearch))
	results := make([]githubSearchRepository, 0, len(m.searchResults))
	for _, repository := range m.searchResults {
		if m.hideCloned && m.clonedRepositories[repository.FullName] {
			continue
		}
		if strings.Contains(strings.ToLower(repository.FullName), query) {
			results = append(results, repository)
		}
	}
	return results
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
		{key: "deadline", label: "Deadline", value: m.deadlineDisplayValue(time.Now())},
		{key: "refresh-interval", label: "Refresh interval", value: fmt.Sprintf("%d minutes", m.refreshIntervalMinutes)},
	}
}

func (m model) deadlineDisplayValue(now time.Time) string {
	value := m.effectiveDeadline(now).Format("2006-01-02")
	if m.deadlineOverride != "" {
		override, err := time.ParseInLocation("2006-01-02", m.deadlineOverride, now.Location())
		if err == nil && !override.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())) {
			return value + " (override)"
		}
	}
	return value + " (Saturday default)"
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
			if isTextInputKey(msg) {
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
		if setting.key == "deadline" {
			m.settingInput = m.deadlineOverride
		} else if setting.key == "refresh-interval" {
			m.settingInput = fmt.Sprintf("%d", m.refreshIntervalMinutes)
		}
	}
	return m, nil
}

func (m model) saveSetting() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.settingInput)
	if value == "" && m.editingSetting != "deadline" {
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
	} else if m.editingSetting == "deadline" {
		if value != "" {
			deadline, err := time.ParseInLocation("2006-01-02", value, time.Local)
			if err != nil {
				m.err = errors.New("deadline must use YYYY-MM-DD")
				return m, nil
			}
			m.deadlineOverride = deadline.Format("2006-01-02")
		} else {
			m.deadlineOverride = ""
		}
	} else if m.editingSetting == "refresh-interval" {
		interval, err := parseRefreshInterval(value)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.refreshIntervalMinutes = interval
		m.nextRefresh = time.Now().Add(time.Duration(interval) * time.Minute)
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

func (m model) updateSearchChooser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchChooser = false
	case "l":
		m.searchChooser, m.localSearchActive, m.localSearch, m.cursor = false, true, "", 0
	case "g":
		m.searchChooser = false
		m.mode, m.githubSearch, m.searchCursor, m.searchError = searchMode, "", 0, nil
		m.hideCloned, m.searchLoading = true, true
		return m, tea.Batch(loadGitHubRepositories(m.githubHost), findClonedRepositories(m.path, m.repositories, m.githubHost))
	}
	return m, nil
}

func (m model) updateEditorActions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.editorActions = false
		return m, nil
	}
	repository, ok := m.selectedRepository(m.repositoryListRows())
	if !ok {
		m.editorActions = false
		return m, nil
	}
	path := filepath.Join(m.path, repository)
	if msg.String() == "f" {
		m.editorActions = false
		return m, runRepositoryAction("open", path)
	}
	applications := map[string]string{"v": "Visual Studio Code", "p": "Positron"}
	application, ok := applications[msg.String()]
	if !ok {
		return m, nil
	}
	m.editorActions = false
	return m, openInApplication(application, path)
}

func (m model) updateGitHubActions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.githubActions = false
		return m, nil
	}
	repository, ok := m.selectedRepository(m.repositoryListRows())
	if !ok {
		m.githubActions = false
		return m, nil
	}
	url, err := m.githubURL(repository)
	if err != nil {
		m.actionStatus = "GitHub action unavailable: " + err.Error()
		return m, nil
	}
	switch msg.String() {
	case "o":
		m.githubActions = false
		return m, openURL(url)
	case "c":
		m.githubActions = false
		m.actionStatus = "Copied GitHub URL"
		return m, copyToClipboard(url)
	}
	return m, nil
}

func (m model) selectedRepository(rows []repositoryListRow) (string, bool) {
	if m.cursor < 0 || m.cursor >= len(rows) || rows[m.cursor].header || rows[m.cursor].key == "divider" {
		return "", false
	}
	return m.repositories[rows[m.cursor].index], true
}

func (m model) githubURL(repository string) (string, error) {
	remote, err := gitOriginURL(filepath.Join(m.path, repository))
	if err != nil {
		return "", err
	}
	host, owner, name, ok := githubRepository(remote)
	if !ok {
		return "", errors.New("origin is not hosted on GitHub")
	}
	return fmt.Sprintf("https://%s/%s/%s", host, owner, name), nil
}

func openInApplication(application, path string) tea.Cmd {
	return func() tea.Msg {
		return repositoryActionMsg{err: exec.Command("open", "-a", application, path).Run()}
	}
}

func openURL(url string) tea.Cmd {
	return runRepositoryAction("open", url)
}

func copyToClipboard(value string) tea.Cmd {
	return func() tea.Msg {
		command := exec.Command("pbcopy")
		command.Stdin = strings.NewReader(value)
		return repositoryActionMsg{err: command.Run()}
	}
}

func runRepositoryAction(command string, argument string) tea.Cmd {
	return func() tea.Msg {
		return repositoryActionMsg{err: exec.Command(command, argument).Run()}
	}
}

func (m model) helpView() string {
	width := m.contentWidth()
	lines := []string{
		titleStyle.Render("mds") + mutedStyle.Render("  /  Keyboard shortcuts"),
		promptStyle.Render("NAVIGATE") + "   ↑/↓ or j/k move · Enter track",
		promptStyle.Render("OPEN") + "       o menu · v VS Code · p Positron · f Finder",
		promptStyle.Render("GITHUB") + "     g URL actions · o open · c copy",
		promptStyle.Render("SEARCH") + "     / chooser · l local · g GitHub",
		promptStyle.Render("MANAGE") + "     s settings · e rename Tracking",
		"",
		mutedStyle.Render("? or Esc close · q quit"),
	}
	if m.contentHeight() > 10 {
		lines = append([]string{lines[0], mutedStyle.Render("Move through your workspace without leaving the terminal."), ""}, lines[1:]...)
	}
	for index, line := range lines {
		lines[index] = truncate(line, width)
	}
	return m.fullScreen(lines)
}

func (m model) repositoriesView() string {
	if m.searchChooser {
		return m.dialogView("Search", "Choose where to search", []string{"l  Local workspace", "g  GitHub repositories"}, "Esc cancel")
	}
	if m.editorActions {
		return m.dialogView("Open", "Open the selected repository in", []string{"v VS Code", "p Positron", "f Finder"}, "Esc cancel")
	}
	if m.githubActions {
		return m.dialogView("GitHub actions", "GitHub actions · repository link actions", []string{"o open URL", "c copy URL"}, "Esc cancel")
	}
	width := m.contentWidth()
	lines := []string{
		m.screenTitle("Workspace"),
		mutedStyle.Render("Repository cockpit  ·  " + truncateLeft(m.path, max(1, width-23))),
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
					lines = append(lines, selectedRowStyle.Width(width).Render(cursorStyle.Render("▸ "+label)))
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
			deadlineText := ""
			nameStyle := lipgloss.NewStyle()
			if m.favorites[m.repositories[row.index]] {
				deadlineText = deadlineStatus(m.effectiveDeadline(time.Now()), time.Now())
				if count, ok := m.commitCounts[m.repositories[row.index]]; ok {
					countText = commitProgress(count)
					nameStyle = commitProgressStyle(count)
				} else if m.commitErrors[m.repositories[row.index]] != nil {
					countText = "GH ERROR"
					nameStyle = errorStyle
				} else if m.loadingCommits {
					countText = commitSpinnerFrames[m.spinnerFrame] + " Loading"
				}
			}
			prefix := indent + marker + " "
			if selected {
				prefix = "▸ " + prefix
			}
			rightText := deadlineText
			if countText != "" {
				if rightText != "" {
					rightText += "  "
				}
				rightText += countText
			}
			available := width - lipgloss.Width(prefix) - lipgloss.Width(rightText)
			name := truncate(row.repository, max(1, available-1))
			prefixText := selectedStyle.Render(prefix)
			if selected {
				prefixText = cursorStyle.Render(prefix)
			}
			rowText := prefixText + nameStyle.Render(name)
			if rightText != "" {
				rowText += strings.Repeat(" ", max(1, width-lipgloss.Width(rowText)-lipgloss.Width(rightText))) + rightText
			}
			if selected {
				rowText = selectedRowStyle.Width(width).Render(rowText)
			}
			lines = append(lines, rowText)
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
	footer := fmt.Sprintf("%d repos · ↑↓ · Enter · / search · o open · g GitHub · ? help", len(m.repositories))
	if m.localSearchActive {
		footer = "/ " + m.localSearch + "█ · ↑/↓ move · Enter track · Esc clear search"
	}
	if m.actionStatus != "" {
		footer = m.actionStatus
	}
	if !m.localSearchActive && m.actionStatus == "" {
		status := m.refreshStatus(time.Now())
		footerWidth := max(1, width-lipgloss.Width(status)-2)
		footer = truncate(footer, footerWidth) + strings.Repeat(" ", max(1, width-lipgloss.Width(truncate(footer, footerWidth))-lipgloss.Width(status))) + status
	}
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

func (m *model) activateRow(row repositoryListRow) (string, bool) {
	if row.key == "divider" {
		return "", false
	}
	if row.header {
		if m.collapsed == nil {
			m.collapsed = make(map[string]bool)
		}
		m.collapsed[row.key] = !m.collapsed[row.key]
		m.savePreferences()
		return "", false
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
	tracked := m.favorites[repository]
	if !tracked {
		delete(m.commitCounts, repository)
		delete(m.commitErrors, repository)
	}
	m.savePreferences()
	if nextRepository != "" {
		m.cursor = m.rowCursorForRepository(nextRepository)
	} else {
		m.cursor = min(m.cursor, max(0, len(m.repositoryListRows())-1))
	}
	return repository, tracked
}

func (m *model) savePreferences() {
	if err := saveConfig(config{MDSFolder: m.path, GitHubUsername: m.githubUsername, GitHubProfileName: m.githubProfileName, GitHubHost: m.githubHost, DeadlineOverride: m.deadlineOverride, RefreshIntervalMinutes: m.refreshIntervalMinutes, Favorites: m.favorites, Collapsed: m.collapsed, FavoritesTitle: m.favoritesTitle}); err != nil {
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
		if m.localSearch != "" && !strings.Contains(strings.ToLower(repository), strings.ToLower(m.localSearch)) {
			continue
		}
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

func (m model) screenTitle(section string) string {
	return titleStyle.Render("mds") + mutedStyle.Render("  /  ") + promptStyle.Render(strings.ToUpper(section))
}

func (m model) dialogView(title, description string, options []string, footer string) string {
	width := min(46, max(24, m.contentWidth()-8))
	lines := []string{titleStyle.Render(strings.ToUpper(title)), mutedStyle.Render(truncate(description, width)), ""}
	for _, option := range options {
		lines = append(lines, promptStyle.Render(truncate(option, width)))
	}
	lines = append(lines, "", mutedStyle.Render(truncate(footer, width)))
	card := dialogStyle.Width(width).Render(strings.Join(lines, "\n"))
	availableHeight := max(1, m.contentHeight())
	paddingTop := max(0, (availableHeight-lipgloss.Height(card))/2)
	return m.fullScreen([]string{strings.Repeat("\n", paddingTop) + card})
}

func (m model) fullScreen(lines []string) string {
	width := m.viewportWidth()
	height := m.viewportHeight()
	content := strings.Join(lines, "\n")
	if width >= 100 {
		railWidth := 24
		panelWidth := max(1, width-railWidth-9)
		panel := borderStyle.Width(panelWidth).Height(max(1, height-4)).Render(content)
		rail := m.navigationRail(railWidth, height-4)
		return "\n" + lipgloss.NewStyle().PaddingLeft(2).Render(lipgloss.JoinHorizontal(lipgloss.Top, rail, " ", panel)) + "\n"
	}
	panel := borderStyle.Width(max(1, width-6)).Height(max(1, height-4)).Render(content)
	return "\n" + lipgloss.NewStyle().PaddingLeft(2).Render(panel) + "\n"
}

func (m model) navigationRail(width, height int) string {
	active := "WORKSPACE"
	if m.mode == settingsMode {
		active = "SETTINGS"
	} else if m.mode == searchMode {
		active = "GITHUB SEARCH"
	}
	items := []string{"WORKSPACE", "GITHUB SEARCH", "SETTINGS"}
	lines := []string{titleStyle.Render("MDS"), mutedStyle.Render("WORKSPACE"), ""}
	for _, item := range items {
		if item == active {
			lines = append(lines, railActiveStyle.Width(width-4).Render("● "+item))
		} else {
			lines = append(lines, mutedStyle.Render(item))
		}
	}
	lines = append(lines, "", mutedStyle.Render("STATUS"), promptStyle.Render(fmt.Sprintf("%d tracked", len(m.favorites))), mutedStyle.Render(fmt.Sprintf("%d repos", len(m.repositories))))
	if m.mode == repositoriesMode {
		lines = append(lines, "", mutedStyle.Render(truncate(m.refreshStatus(time.Now()), width-4)))
	}
	content := strings.Join(lines, "\n")
	return railStyle.Width(width - 4).Height(max(1, height)).Render(content)
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

func commitProgress(count int) string {
	style := commitProgressStyle(count)
	return style.Render(commitProgressBar(count)) + " " + style.Render(fmt.Sprintf("%d/%d", count, commitGoal))
}

func deadlineStatus(deadline, now time.Time) string {
	deadlineDay := time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 0, 0, 0, 0, deadline.Location())
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := int(deadlineDay.Sub(nowDay).Hours() / 24)
	switch {
	case days == 0:
		return promptStyle.Render("due today")
	case days == 1:
		return promptStyle.Render("1 day left")
	case days > 1:
		return promptStyle.Render(fmt.Sprintf("%d days left", days))
	case days == -1:
		return errorStyle.Render("1 day overdue")
	default:
		return errorStyle.Render(fmt.Sprintf("%d days overdue", -days))
	}
}

func commitProgressBar(count int) string {
	filled := min(max(count, 0), commitGoal) * commitProgressWidth / commitGoal
	return strings.Repeat("█", filled) + strings.Repeat("░", commitProgressWidth-filled)
}

func commitProgressStyle(count int) lipgloss.Style {
	switch {
	case count >= commitGoal:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	case count == 2:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	}
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
			return commitCountsMsg{counts: counts, errors: failures, repositories: repositories}
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
		return commitCountsMsg{counts: counts, errors: failures, repositories: repositories}
	}
}

func loadGitHubRepositories(host string) tea.Cmd {
	return func() tea.Msg {
		output, err := exec.Command("gh", "api", "--hostname", host, "--paginate", "--slurp", "user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=full_name").CombinedOutput()
		if err != nil {
			return githubSearchResultsMsg{err: fmt.Errorf("query GitHub repositories: %s", strings.TrimSpace(string(output)))}
		}
		var pages [][]githubSearchRepository
		if err := json.Unmarshal(output, &pages); err != nil {
			return githubSearchResultsMsg{err: fmt.Errorf("decode GitHub repositories: %w", err)}
		}
		var repositories []githubSearchRepository
		for _, page := range pages {
			repositories = append(repositories, page...)
		}
		sort.Slice(repositories, func(left, right int) bool {
			return strings.ToLower(repositories[left].FullName) < strings.ToLower(repositories[right].FullName)
		})
		return githubSearchResultsMsg{repositories: repositories}
	}
}

func findClonedRepositories(workspace string, repositories []string, configuredHost string) tea.Cmd {
	return func() tea.Msg {
		cloned := make(map[string]bool)
		for _, repository := range repositories {
			remote, err := gitOriginURL(filepath.Join(workspace, repository))
			if err != nil {
				continue
			}
			host, owner, name, ok := githubRepository(remote)
			if ok && strings.EqualFold(host, configuredHost) {
				cloned[owner+"/"+name] = true
			}
		}
		return clonedRepositoriesMsg{repositories: cloned}
	}
}

func cloneGitHubRepository(workspace, host, repository string) tea.Cmd {
	return func() tea.Msg {
		name := filepath.Base(repository)
		destination := filepath.Join(workspace, name)
		if _, err := os.Stat(destination); err == nil {
			return cloneRepositoryMsg{repository: repository, path: destination, err: fmt.Errorf("destination %q already exists", name)}
		} else if !errors.Is(err, os.ErrNotExist) {
			return cloneRepositoryMsg{repository: repository, path: destination, err: fmt.Errorf("inspect clone destination: %w", err)}
		}
		url := fmt.Sprintf("https://%s/%s.git", host, repository)
		output, err := exec.Command("git", "clone", url, destination).CombinedOutput()
		if err != nil {
			return cloneRepositoryMsg{repository: repository, path: destination, err: fmt.Errorf("git clone: %s", strings.TrimSpace(string(output)))}
		}
		return cloneRepositoryMsg{repository: repository, path: destination}
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

func configuredRefreshInterval(interval int) int {
	if interval > 0 {
		return interval
	}
	return defaultRefreshIntervalMinutes
}

func parseRefreshInterval(value string) (int, error) {
	interval, err := strconv.Atoi(value)
	if err != nil || interval <= 0 {
		return 0, errors.New("refresh interval must be a positive number of minutes")
	}
	return interval, nil
}

func (m model) refreshStatus(now time.Time) string {
	if m.lastRefresh.IsZero() || m.nextRefresh.IsZero() {
		return "Refresh pending"
	}
	remaining := max(0, int(m.nextRefresh.Sub(now).Seconds()))
	return fmt.Sprintf("Last %s · Next %d:%02d", m.lastRefresh.Format("15:04"), remaining/60, remaining%60)
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

func isTextInputKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace
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
