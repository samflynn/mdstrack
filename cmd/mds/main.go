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
	MDSFolder string          `json:"mds_folder"`
	Favorites map[string]bool `json:"favorites"`
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
}

type model struct {
	mode         mode
	path         string
	repositories []string
	favorites    map[string]bool
	cursor       int
	err          error
	width        int
	height       int
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	borderStyle   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2)
	promptStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
	groupStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
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
	return model{mode: repositoriesMode, path: stored.MDSFolder, repositories: repositories, favorites: stored.Favorites, err: err}, nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
		if m.mode == repositoriesMode {
			indexes := m.repositoryIndexes()
			switch msg.String() {
			case "up", "k":
				m.cursor = max(0, m.cursor-1)
			case "down", "j":
				m.cursor = min(max(0, len(indexes)-1), m.cursor+1)
			case "enter":
				if len(indexes) > 0 {
					if m.favorites == nil {
						m.favorites = make(map[string]bool)
					}
					repository := m.repositories[indexes[m.cursor]]
					m.favorites[repository] = !m.favorites[repository]
					if err := saveConfig(config{MDSFolder: m.path, Favorites: m.favorites}); err != nil {
						m.favorites[repository] = !m.favorites[repository]
						m.err = fmt.Errorf("save favorites: %w", err)
					} else {
						m.cursor = m.repositoryCursor(repository)
					}
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

	if err := saveConfig(config{MDSFolder: absoluteFolder, Favorites: make(map[string]bool)}); err != nil {
		m.err = fmt.Errorf("save configuration: %w", err)
		return m, nil
	}
	repositories, err := findRepositories(absoluteFolder)
	m.mode, m.path, m.repositories, m.favorites, m.err = repositoriesMode, absoluteFolder, repositories, make(map[string]bool), err
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
		for _, row := range rows[start:end] {
			if row.index < 0 {
				lines = append(lines, groupStyle.Render("[ "+truncate(row.group, width-4)+" ]"))
				continue
			}
			name := truncate(row.repository, max(1, width-5))
			marker := "[ ]"
			if m.favorites[m.repositories[row.index]] {
				marker = "[X]"
			}
			if row.index == m.selectedRepositoryIndex() {
				lines = append(lines, cursorStyle.Render("▸ "+marker+" "+name))
				continue
			}
			lines = append(lines, selectedStyle.Render(marker+" ")+name)
		}
		if start > 0 || end < len(rows) {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d–%d of %d", start+1, end, len(rows))))
		}
	}
	footer := fmt.Sprintf("%d repos · ↑/↓ move · Enter favorite · q quit", len(m.repositories))
	lines = append(lines, "", mutedStyle.Render(truncate(footer, width)))
	return m.fullScreen(lines)
}

func (m model) repositoryIndexes() []int {
	favorites := make([]int, 0, len(m.repositories))
	regular := make([]int, 0, len(m.repositories))
	for index, repository := range m.repositories {
		if m.favorites[repository] {
			favorites = append(favorites, index)
			continue
		}
		regular = append(regular, index)
	}
	return append(favorites, regular...)
}

func (m model) selectedRepositoryIndex() int {
	indexes := m.repositoryIndexes()
	if len(indexes) == 0 {
		return -1
	}
	return indexes[min(m.cursor, len(indexes)-1)]
}

func (m model) repositoryCursor(repository string) int {
	for cursor, index := range m.repositoryIndexes() {
		if m.repositories[index] == repository {
			return cursor
		}
	}
	return 0
}

func (m model) repositoryListRows() []repositoryListRow {
	indexes := m.repositoryIndexes()
	favoriteCount := 0
	for _, index := range indexes {
		if !m.favorites[m.repositories[index]] {
			break
		}
		favoriteCount++
	}
	favorites := indexes[:favoriteCount]
	regular := indexes[favoriteCount:]

	rows := make([]repositoryListRow, 0, len(m.repositories)+len(favorites)+2)
	if len(favorites) > 0 {
		rows = append(rows, repositoryListRow{group: "FAVORITES", index: -1})
		rows = appendGroupedRepositoryRows(rows, m.repositories, favorites)
	}
	return appendGroupedRepositoryRows(rows, m.repositories, regular)
}

func appendGroupedRepositoryRows(rows []repositoryListRow, repositories []string, indexes []int) []repositoryListRow {
	previousGroup := ""
	for _, index := range indexes {
		group, name := repositoryGroup(repositories[index])
		if group != previousGroup {
			rows = append(rows, repositoryListRow{group: group, index: -1})
			previousGroup = group
		}
		rows = append(rows, repositoryListRow{repository: name, index: index})
	}
	return rows
}

func (m model) repositoryWindow(rows []repositoryListRow) (int, int) {
	visible := max(1, m.contentHeight()-6)
	cursorRow := 0
	for index, row := range rows {
		if row.index == m.selectedRepositoryIndex() {
			cursorRow = index
			break
		}
	}
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
