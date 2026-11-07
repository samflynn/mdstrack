package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	dashboard screen = iota
	search
)

type model struct {
	screen        screen
	width, height int
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.screen = search
		case "esc":
			m.screen = dashboard
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.screen == search {
		return m.searchView()
	}
	return m.dashboardView()
}

var (
	brand      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	kicker     = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	title      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	muted      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	key        = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	searchCard = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Padding(1, 2)
	dimCard    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(1, 2)
)

func (m model) dashboardView() string {
	width := m.contentWidth()
	line := muted.Render(strings.Repeat("─", width))
	header := lipgloss.JoinHorizontal(lipgloss.Top, brand.Render("mds"), muted.Render("  ·  your work, quietly organized"))
	search := searchCard.Width(width - 6).Render(key.Render("/") + "  " + title.Render("Search") + "\n" + muted.Render("Find a repository, assignment, or anything else when you are ready."))
	status := dimCard.Width(width - 6).Render(kicker.Render("VERSION 0.01") + "\n" + title.Render("A calm place to begin.") + "\n" + muted.Render("Your dashboard will grow here, one useful thing at a time."))
	content := strings.Join([]string{header, "", line, "", search, "", status}, "\n")
	return m.frame(content, key.Render("/")+muted.Render(" open search"))
}

func (m model) searchView() string {
	width := m.contentWidth()
	header := lipgloss.JoinHorizontal(lipgloss.Top, brand.Render("mds"), muted.Render("  ·  search"))
	blank := dimCard.Width(width - 6).Render(kicker.Render("SEARCH") + "\n" + title.Render("Nothing here yet.") + "\n" + muted.Render("This page is intentionally blank in v0.01."))
	return m.frame(header+"\n\n"+blank, key.Render("Esc")+muted.Render(" dashboard"))
}

func (m model) frame(content, footer string) string {
	if m.height == 0 {
		return content + "\n"
	}
	padding := max(1, (m.height-lipgloss.Height(content)-2)/2)
	left := max(2, (m.width-m.contentWidth())/2)
	return strings.Repeat("\n", padding) + lipgloss.NewStyle().PaddingLeft(left).Render(content) + "\n\n" + muted.Render(footer) + "\n"
}

func (m model) contentWidth() int {
	if m.width <= 0 {
		return 64
	}
	return min(72, max(36, m.width-8))
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

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
