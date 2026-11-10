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
	canvas = lipgloss.Color("235")
	panel  = lipgloss.Color("236")
	border = lipgloss.Color("240")
	ink    = lipgloss.Color("255")
	muted  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	plum   = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Bold(true)
	cyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	amber  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	key    = lipgloss.NewStyle().Foreground(lipgloss.Color("141")).Background(lipgloss.Color("238")).Padding(0, 1).Bold(true)
	pane   = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(border).Background(panel).Padding(1, 1)
)

func (m model) dashboardView() string {
	width := m.contentWidth()
	topBar := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(
		muted.Render("Help: Press ") + key.Render("?") + muted.Render(" or ") + key.Render("Esc") + muted.Render(" to close   │   ") + green.Render("mds · Terminal Workspace Tracker"))
	header := topBar + "\n\n" + lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(
		lipgloss.NewStyle().Bold(true).Foreground(ink).Render("M D S T R A C K")) + "\n" +
		lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Foreground(lipgloss.Color("78")).Render("Terminal Workspace Tracker")

	sidebarContent :=
		cyan.Bold(true).Render("WEATHER [YVR]") + "\n" +
			cyan.Render("  \\  /   11°C") + "\n" +
			green.Render("_ /\"\\_  light rain") + "\n" +
			muted.Render("  /  \\  feels 9°") + "\n" +
			muted.Render("wind 14 km/h") + "\n\n" +
			muted.Render("───────────────────") + "\n" +
			plum.Render("TODAY · MON NOV 3") + "\n" +
			cyan.Render("09:30") + " DSCI 571\n" +
			muted.Render("12:00") + " TA Office Hours\n" +
			green.Render("14:00") + " lootbox work\n" +
			amber.Render("23:59") + " DSCI-571 Quiz\n\n" +
			green.Render("● READY_WAIT_EVENT")

	mainWidth := max(42, width-27)
	toolbar := plum.Render("▣") + " " + key.Render("TTY: 52 × 24") + "  " + green.Render("● 12s auto-refresh") + "\n" +
		muted.Render("Workspace: ~/Documents/MDS   git: smudgal@student.ubc.ca   gh: ✓ authenticated")
	week := lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(ink).Padding(0, 1).Width(mainWidth - 4).Render(
		green.Render("❯") + " MDS · Week 5 — Unsupervised Learning" + "  " + amber.Render("due Sun Nov 8 23:59 [3d left]"))
	rows := strings.Join([]string{
		cyan.Render("  lootbox-infinite") + "    " + muted.Render("feat/rarity-tiers · 2h ago") + "  " + green.Render("███ 3/3"),
		lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(ink).Padding(0, 1).Render("▸ yt-cli-go             main · 1d ago · ● dirty · ↑1    ") + cyan.Render("██░ 2/3"),
		cyan.Render("  eink-calendar-app") + "   " + muted.Render("main · 4d ago · ↑2") + "           " + muted.Render("░░░ 0/3"),
		cyan.Render("  DSCI-571") + "           " + muted.Render("main · 2h ago · (primary clone)") + "  " + cyan.Render("██░ 2/3"),
		cyan.Render("  DSCI-571 (retry)") + "   " + muted.Render("main · 6d ago · (alternate path)") + " " + muted.Render("░░░ 0/3"),
	}, "\n")
	workspace := pane.Width(mainWidth).Render(toolbar + "\n\n" + week + "\n\n" + plum.Render("TRACKED REPOSITORIES (3/WEEK GOAL)") + "\n" + rows + "\n\n" +
		green.Render("✓ 2 done this week") + "\n" + muted.Render("  ✓ building-diorama                         3/3\n  ✓ stat-545-hw3                            0/3") + "\n\n" +
		inkStyle("3/7 targets met") + muted.Render(" · ") + cyan.Render("10 commits logged"))
	sidebar := pane.Width(23).Height(max(1, lipgloss.Height(workspace)-2)).Render(sidebarContent)
	content := header + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", workspace)
	footer := plum.Render("mds:~$ ") + key.Render("q") + muted.Render(" quit  ") + key.Render("/") + muted.Render(" fuzzy search")
	return m.frame(content, footer)
}

func (m model) searchView() string {
	width := m.contentWidth()
	blank := pane.Width(width).Render(plum.Render("┌─ SEARCH ───────────────────────────┐") + "\n\n" + inkStyle("Nothing here yet.") + "\n" + muted.Render("This page is intentionally blank in v0.01."))
	return m.frame(blank, key.Render("Esc")+muted.Render(" dashboard"))
}

func (m model) frame(content, footer string) string {
	if m.height == 0 {
		return content + "\n"
	}
	padding := max(0, (m.height-lipgloss.Height(content)-2)/2)
	left := max(1, (m.width-m.contentWidth())/2)
	return strings.Repeat("\n", padding) + lipgloss.NewStyle().Background(canvas).PaddingLeft(left).Render(content) + "\n" + muted.Render(footer) + "\n"
}

func (m model) contentWidth() int {
	if m.width <= 0 {
		return 92
	}
	return min(108, max(52, m.width-4))
}

func inkStyle(value string) string {
	return lipgloss.NewStyle().Foreground(ink).Bold(true).Render(value)
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
