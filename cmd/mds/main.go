package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type screen int

const (
	dashboard screen = iota
	search
)

type model struct {
	screen screen
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
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
		return "Search\n\nEsc dashboard · q quit\n"
	}
	return "Dashboard\n\n/ search · q quit\n"
}

func main() {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
