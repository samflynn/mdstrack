package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSetupAcceptsPastedPath(t *testing.T) {
	m := model{mode: setupMode}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/Users/penrose/Documents/MDS"), Paste: true})

	got := updated.(model).path
	want := "/Users/penrose/Documents/MDS"
	if got != want {
		t.Fatalf("setup path = %q, want %q", got, want)
	}
}

func TestTruncateForTerminalWidth(t *testing.T) {
	if got, want := truncate("repository-name", 8), "reposit…"; got != want {
		t.Fatalf("truncate() = %q, want %q", got, want)
	}
	if got, want := truncateLeft("/Users/penrose/Documents/MDS", 12), "…cuments/MDS"; got != want {
		t.Fatalf("truncateLeft() = %q, want %q", got, want)
	}
}

func TestRepositoriesViewLimitsRowsToViewport(t *testing.T) {
	m := model{
		mode:         repositoriesMode,
		width:        60,
		height:       12,
		repositories: []string{"one", "two", "three", "four", "five", "six", "seven"},
	}
	view := m.repositoriesView()
	if !strings.Contains(view, "more repositories") {
		t.Fatalf("expected an overflow summary, got %q", view)
	}
}

func TestFindRepositoriesFindsAndSortsNestedRepositories(t *testing.T) {
	root := t.TempDir()
	for _, repository := range []string{"zeta", "course/week-one/project"} {
		if err := os.MkdirAll(filepath.Join(root, repository, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("not a repository"), 0o600); err != nil {
		t.Fatal(err)
	}

	repositories, err := findRepositories(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"course/week-one/project", "zeta"}
	if !reflect.DeepEqual(repositories, want) {
		t.Fatalf("findRepositories() = %v, want %v", repositories, want)
	}
}
