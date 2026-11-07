package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const target = 3

type Repo struct {
	Name, Branch string
	YourCommits  int
	Dirty, Done  bool
}
type Snapshot struct {
	Workspace, Week string
	Deadline        *time.Time
	Repos           []Repo
}
type refreshMsg struct {
	snapshot Snapshot
	err      error
}
type mutationMsg struct{ err error }
type model struct {
	snapshot        Snapshot
	selected, width int
	err             error
	refresh         func() (Snapshot, error)
	toggleDone      func(string) error
}

func Run(snapshot Snapshot, refresh func() (Snapshot, error), toggleDone func(string) error) error {
	_, err := tea.NewProgram(model{snapshot: snapshot, refresh: refresh, toggleDone: toggleDone}, tea.WithAltScreen()).Run()
	return err
}
func (m model) Init() tea.Cmd { return nil }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case refreshMsg:
		m.err = msg.err
		if msg.err == nil {
			m.snapshot = msg.snapshot
			m.clamp()
		}
	case mutationMsg:
		m.err = msg.err
		if msg.err == nil {
			return m, m.refreshCmd()
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.visible())-1 {
				m.selected++
			}
		case "r":
			return m, m.refreshCmd()
		case " ":
			rows := m.visible()
			if len(rows) == 0 {
				return m, nil
			}
			name := rows[m.selected].Name
			return m, func() tea.Msg { return mutationMsg{err: m.toggleDone(name)} }
		}
	}
	return m, nil
}
func (m *model) clamp() {
	if n := len(m.visible()); n == 0 {
		m.selected = 0
	} else if m.selected >= n {
		m.selected = n - 1
	}
}
func (m model) refreshCmd() tea.Cmd {
	return func() tea.Msg { s, e := m.refresh(); return refreshMsg{snapshot: s, err: e} }
}
func (m model) visible() []Repo {
	rows := make([]Repo, 0, len(m.snapshot.Repos))
	for _, repo := range m.snapshot.Repos {
		if !repo.Done {
			rows = append(rows, repo)
		}
	}
	return rows
}
func (m model) View() string {
	if m.width == 0 {
		return "Loading mds…"
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selected := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	deadline := "no deadline"
	if m.snapshot.Deadline != nil {
		deadline = deadlineLabel(*m.snapshot.Deadline, time.Now())
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", header.Render(fmt.Sprintf("%s · %s    %s", m.snapshot.Workspace, m.snapshot.Week, deadline)))
	b.WriteString(strings.Repeat("─", max(40, m.width-2)) + "\n")
	rows := m.visible()
	done := len(m.snapshot.Repos) - len(rows)
	if len(rows) == 0 {
		b.WriteString(muted.Render("No unfinished assigned repos. Press r to refresh.") + "\n")
	}
	for i, repo := range rows {
		prefix := "  "
		if i == m.selected {
			prefix = "▸ "
		}
		line := fmt.Sprintf("%s%-24s %s %d/%d", prefix, repo.Name, bar(repo.YourCommits), repo.YourCommits, target)
		if i == m.selected {
			line = selected.Render(line)
		}
		b.WriteString(line + "\n")
		detail := repo.Branch
		if repo.Dirty {
			detail += " · dirty"
		}
		b.WriteString(muted.Render("    "+detail) + "\n")
	}
	if done > 0 {
		b.WriteString(muted.Render(fmt.Sprintf("✓ %d done (space on a row marks it done)", done)) + "\n")
	}
	b.WriteString(strings.Repeat("─", max(40, m.width-2)) + "\n")
	b.WriteString(muted.Render("↑↓ select · space done · r refresh · q quit"))
	if m.err != nil {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(m.err.Error()))
	}
	return b.String() + "\n"
}
func bar(n int) string {
	if n > target {
		n = target
	}
	return strings.Repeat("█", n) + strings.Repeat("░", target-n)
}
func deadlineLabel(deadline, now time.Time) string {
	d := deadline.Sub(now)
	if d < 0 {
		return fmt.Sprintf("%dd over", int(-d.Hours()/24)+1)
	}
	if d < 24*time.Hour {
		return "DUE TODAY"
	}
	return fmt.Sprintf("%dd left", int(d.Hours()/24)+1)
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
