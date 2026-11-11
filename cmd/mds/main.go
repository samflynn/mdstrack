package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const applicationName = "mds"

type config struct {
	MDSFolder      string          `json:"mds_folder"`
	Favorites      map[string]bool `json:"favorites"`
	Collapsed      map[string]bool `json:"collapsed"`
	FavoritesTitle string          `json:"favorites_title"`
}

type mode int

const (
	setupMode mode = iota
	repositoriesMode
)

type repositoryListRow struct {
	group      string
	repository string
	index      int
	key        string
	header     bool
}

type model struct {
	mode           mode
	path           string
	repositories   []string
	favorites      map[string]bool
	collapsed      map[string]bool
	favoritesTitle string
	editingTitle   bool
	titleInput     string
	cursor         int
	err            error
	width          int
	height         int
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
	return model{mode: repositoriesMode, path: stored.MDSFolder, repositories: repositories, favorites: stored.Favorites, collapsed: stored.Collapsed, favoritesTitle: stored.FavoritesTitle, err: err}, nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || (msg.String() == "q" && !m.editingTitle) {
			return m, tea.Quit
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
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "enter":
			return m.saveSetup()
		case "backspace", "ctrl+h":
			if len(m.path) > 0 {
				m.path = m.path[:len(m.path)-1]
			}
		default:
			if msg.Type == tea.KeyRunes {
				m.path += string(msg.Runes)
			}
		}
	}
	return m, nil
}

func (m model) saveSetup() (tea.Model, tea.Cmd) {
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

	if err := saveConfig(config{MDSFolder: absoluteFolder, Favorites: make(map[string]bool), Collapsed: make(map[string]bool)}); err != nil {
		m.err = fmt.Errorf("save configuration: %w", err)
		return m, nil
	}
	repositories, err := findRepositories(absoluteFolder)
	m.mode, m.path, m.repositories, m.favorites, m.collapsed, m.err = repositoriesMode, absoluteFolder, repositories, make(map[string]bool), make(map[string]bool), err
	return m, nil
}

func (m model) View() string {
	if m.mode == setupMode {
		return m.setupView()
	}
	return m.repositoriesView()
}

func (m model) setupView() string {
	width := m.contentWidth()
	lines := []string{
		titleStyle.Render("mds"),
		mutedStyle.Render("UBC MDS repository tracker"),
		"",
		"First-time setup",
		"",
		"Location of UBC MDS folder:",
		promptStyle.Render("> " + truncateLeft(m.path, max(1, width-3)) + "█"),
	}
	if m.err != nil {
		lines = append(lines, "", errorStyle.Render(truncate("Error: "+m.err.Error(), width)))
	}
	lines = append(lines, "", mutedStyle.Render(truncate("Enter to continue · q or Ctrl+C to quit", width)))
	return m.fullScreen(lines)
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
			name := truncate(row.repository, max(1, width-len(indent)-5))
			marker := "[ ]"
			if m.favorites[m.repositories[row.index]] {
				marker = "[X]"
			}
			if selected {
				lines = append(lines, cursorStyle.Render("▸ "+indent+marker+" "+name))
			} else {
				lines = append(lines, selectedStyle.Render(indent+marker+" ")+name)
			}
		}
		if start > 0 || end < len(rows) {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d–%d of %d", start+1, end, len(rows))))
		}
	}
	if m.editingTitle {
		lines = append(lines, "", promptStyle.Render("Rename favorites: "+truncate(m.titleInput, max(1, width-18))+"█"), mutedStyle.Render("Enter save · Esc cancel"))
	}
	footer := fmt.Sprintf("%d repos · ↑/↓ move · Enter act · e rename favorites · q quit", len(m.repositories))
	lines = append(lines, "", mutedStyle.Render(truncate(footer, width)))
	return m.fullScreen(lines)
}

func (m model) favoritesDisplayTitle() string {
	if m.favoritesTitle == "" {
		return "FAVORITES"
	}
	return strings.ToUpper(m.favoritesTitle) + " (FAVORITES)"
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
	if err := saveConfig(config{MDSFolder: m.path, Favorites: m.favorites, Collapsed: m.collapsed, FavoritesTitle: m.favoritesTitle}); err != nil {
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
	return stored, nil
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
