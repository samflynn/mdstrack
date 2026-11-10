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
	MDSFolder string `json:"mds_folder"`
}

type mode int

const (
	setupMode mode = iota
	repositoriesMode
)

type model struct {
	mode         mode
	path         string
	repositories []string
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
	folder, err := loadFolder()
	if errors.Is(err, os.ErrNotExist) {
		return model{mode: setupMode}, nil
	}
	if err != nil {
		return model{}, fmt.Errorf("load mds configuration: %w", err)
	}

	repositories, err := findRepositories(folder)
	return model{mode: repositoriesMode, path: folder, repositories: repositories, err: err}, nil
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

	if err := saveFolder(absoluteFolder); err != nil {
		m.err = fmt.Errorf("save configuration: %w", err)
		return m, nil
	}
	repositories, err := findRepositories(absoluteFolder)
	m.mode, m.path, m.repositories, m.err = repositoriesMode, absoluteFolder, repositories, err
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
		availableRows := max(1, m.contentHeight()-6)
		visible := min(len(m.repositories), availableRows)
		for _, repository := range m.repositories[:visible] {
			lines = append(lines, selectedStyle.Render("• ")+truncate(repository, max(1, width-2)))
		}
		if remaining := len(m.repositories) - visible; remaining > 0 {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("… %d more repositories", remaining)))
		}
	}
	lines = append(lines, "", mutedStyle.Render(truncate(fmt.Sprintf("%d repositories · q to quit", len(m.repositories)), width)))
	return m.fullScreen(lines)
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

func loadFolder() (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var stored config
	if err := json.Unmarshal(data, &stored); err != nil {
		return "", err
	}
	if stored.MDSFolder == "" {
		return "", errors.New("configuration has no MDS folder")
	}
	return stored.MDSFolder, nil
}

func saveFolder(folder string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(config{MDSFolder: folder})
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
