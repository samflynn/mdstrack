package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const target = 3

type Action int

const (
	ActionAdd Action = iota
	ActionRemove
	ActionDone
	ActionClone
	ActionOpen
)

type Repo struct {
	ID, Name, Path, Remote, URL, Branch string
	YourCommits                         int
	Dirty, Done, Assigned, RemoteOnly   bool
}
type Snapshot struct {
	Workspace, Week, GitHubHost, GitHubUser, GitHubState string
	Deadline                                             *time.Time
	Repos                                                []Repo
}
type refreshMsg struct {
	snapshot Snapshot
	err      error
}
type actionMsg struct{ err error }
type view int

const (
	dashboard view = iota
	browse
	detail
	search
)

type model struct {
	snapshot        Snapshot
	selected, width int
	current         view
	query           string
	err             error
	refresh         func() (Snapshot, error)
	action          func(Action, Repo) error
}

func Run(snapshot Snapshot, refresh func() (Snapshot, error), action func(Action, Repo) error) error {
	_, err := tea.NewProgram(model{snapshot: snapshot, refresh: refresh, action: action}, tea.WithAltScreen()).Run()
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
	case actionMsg:
		m.err = msg.err
		if msg.err == nil {
			return m, m.refreshCmd()
		}
	case tea.KeyMsg:
		key := msg.String()
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
		if key == "esc" {
			m.current = dashboard
			m.query = ""
			m.selected = 0
			return m, nil
		}
		if m.current == search {
			if key == "backspace" {
				if len(m.query) > 0 {
					m.query = m.query[:len(m.query)-1]
				}
				m.clamp()
				return m, nil
			}
			if len(key) == 1 {
				m.query += key
				m.clamp()
				return m, nil
			}
		}
		switch key {
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.rows())-1 {
				m.selected++
			}
		case "r":
			return m, m.refreshCmd()
		case "b":
			m.current = browse
			m.selected = 0
		case "/":
			m.current = search
			m.selected = 0
		case "l", "right":
			if len(m.rows()) > 0 {
				m.current = detail
			}
		case "a":
			return m, m.actionCmd(ActionAdd)
		case "d":
			return m, m.actionCmd(ActionRemove)
		case " ":
			return m, m.actionCmd(ActionDone)
		case "c":
			return m, m.actionCmd(ActionClone)
		case "enter":
			return m, m.actionCmd(ActionOpen)
		}
	}
	return m, nil
}
func (m model) refreshCmd() tea.Cmd {
	return func() tea.Msg { s, e := m.refresh(); return refreshMsg{s, e} }
}
func (m model) actionCmd(action Action) tea.Cmd {
	rows := m.rows()
	if len(rows) == 0 {
		return nil
	}
	repo := rows[m.selected]
	return func() tea.Msg { return actionMsg{m.action(action, repo)} }
}
func (m *model) clamp() {
	n := len(m.rows())
	if n == 0 {
		m.selected = 0
	} else if m.selected >= n {
		m.selected = n - 1
	}
}
func (m model) rows() []Repo {
	var rows []Repo
	for _, repo := range m.snapshot.Repos {
		switch m.current {
		case dashboard:
			if repo.Assigned && !repo.Done {
				rows = append(rows, repo)
			}
		case browse:
			rows = append(rows, repo)
		case search:
			if strings.Contains(strings.ToLower(repo.Name+" "+repo.Remote), strings.ToLower(m.query)) {
				rows = append(rows, repo)
			}
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
	selectStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	title := "Dashboard"
	if m.current == browse {
		title = "Browse"
	}
	if m.current == detail {
		title = "Detail"
	}
	if m.current == search {
		title = "Search: " + m.query
	}
	deadline := "no deadline"
	if m.snapshot.Deadline != nil {
		deadline = deadlineLabel(*m.snapshot.Deadline, time.Now())
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", header.Render(fmt.Sprintf("%s · %s · %s    %s", m.snapshot.Workspace, m.snapshot.Week, title, deadline)))
	b.WriteString(strings.Repeat("─", max(40, m.width-2)) + "\n")
	rows := m.rows()
	if len(rows) == 0 {
		b.WriteString(muted.Render("No repositories here. Press b to browse, / to search, or r to refresh.") + "\n")
	}
	if m.current == detail && len(rows) > 0 {
		repo := rows[m.selected]
		b.WriteString(selectStyle.Render(repo.Name) + "\n")
		fmt.Fprintf(&b, "Path: %s\nRemote: %s\nBranch: %s\nYour weekly commits: %d/%d\n", value(repo.Path, "not cloned"), value(repo.Remote, "none"), value(repo.Branch, "n/a"), repo.YourCommits, target)
		b.WriteString(muted.Render("Enter opens folder · c clones · d removes · Esc back") + "\n")
	} else {
		for i, repo := range rows {
			prefix := "  "
			if i == m.selected {
				prefix = "▸ "
			}
			tag := ""
			if repo.Assigned {
				tag = " ★"
			}
			if repo.RemoteOnly {
				tag += " remote"
			}
			line := fmt.Sprintf("%s%-24s %s %d/%d%s", prefix, repo.Name, bar(repo.YourCommits), repo.YourCommits, target, tag)
			if i == m.selected {
				line = selectStyle.Render(line)
			}
			b.WriteString(line + "\n")
			detail := repo.Branch
			if detail == "" {
				detail = repo.Remote
			}
			if repo.Dirty {
				detail += " · dirty"
			}
			b.WriteString(muted.Render("    "+value(detail, "not cloned")) + "\n")
		}
	}
	done := 0
	for _, repo := range m.snapshot.Repos {
		if repo.Assigned && repo.Done {
			done++
		}
	}
	if m.current == dashboard && done > 0 {
		b.WriteString(muted.Render(fmt.Sprintf("✓ %d done · b browse to reopen", done)) + "\n")
	}
	b.WriteString(strings.Repeat("─", max(40, m.width-2)) + "\n")
	b.WriteString(muted.Render("b browse · / search · a add · d remove · space done · c clone · enter open · l detail · r refresh · q quit"))
	b.WriteString("\n" + muted.Render("GitHub: "+m.snapshot.GitHubUser+" @ "+m.snapshot.GitHubHost+" · "+m.snapshot.GitHubState))
	if m.err != nil {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(m.err.Error()))
	}
	return b.String() + "\n"
}
func value(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
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
