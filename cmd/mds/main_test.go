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

func TestEnterTogglesRepositoryCheckbox(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{mode: repositoriesMode, path: "/tmp/mds", repositories: []string{"511/lab-one"}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.favorites["511/lab-one"] {
		t.Fatal("expected Enter to mark the selected repository as a favorite")
	}
	if !strings.Contains(m.repositoriesView(), "[X]") {
		t.Fatal("expected a checked marker in the repository view")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.favorites["511/lab-one"] {
		t.Fatal("expected a second Enter to clear the selected favorite")
	}
}

func TestFavoritePersistence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := config{MDSFolder: "/tmp/mds", Favorites: map[string]bool{"511/lab-one": true}}
	if err := saveConfig(want); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadConfig() = %#v, want %#v", got, want)
	}
}

func TestRepositoryCursorScrollsViewport(t *testing.T) {
	m := model{
		mode:         repositoriesMode,
		width:        60,
		height:       18,
		repositories: []string{"one", "two", "three", "four", "five", "six", "seven"},
	}
	for range 6 {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(model)
	}
	if got, want := m.cursor, 6; got != want {
		t.Fatalf("cursor = %d, want %d", got, want)
	}
	start, end := m.repositoryWindow(m.repositoryListRows())
	if start >= end {
		t.Fatalf("repositoryWindow() = (%d, %d), expected a non-empty window", start, end)
	}
	if !strings.Contains(m.repositoriesView(), "▸ [ ] seven") {
		t.Fatal("expected the selected repository to be visible")
	}
}

func TestFavoritingMovesCursorToNextRepository(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{
		mode:         repositoriesMode,
		path:         "/tmp/mds",
		repositories: []string{"511/lab-one", "511/lab-two", "523/lab-one"},
		favorites:    make(map[string]bool),
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.selectedRepositoryIndex(), 1; got != want {
		t.Fatalf("selection after favoriting = %d, want %d", got, want)
	}
	if !strings.Contains(m.repositoriesView(), "▸ [ ] lab-two") {
		t.Fatal("expected selection to advance to the next repository")
	}
}

func TestFavoriteOrderingDrivesCursorNavigation(t *testing.T) {
	m := model{
		mode:         repositoriesMode,
		repositories: []string{"511/lab-one", "511/lab-two", "523/lab-one"},
		favorites:    map[string]bool{"523/lab-one": true},
	}
	if got, want := m.selectedRepositoryIndex(), 2; got != want {
		t.Fatalf("initial selection index = %d, want %d", got, want)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	if got, want := m.selectedRepositoryIndex(), 0; got != want {
		t.Fatalf("selection after down = %d, want %d", got, want)
	}
}

func TestFavoriteRepositoriesArePinnedAndGrouped(t *testing.T) {
	m := model{
		repositories: []string{"511/lab-one", "511/lab-two", "523/lab-one"},
		favorites:    map[string]bool{"511/lab-one": true, "511/lab-two": true},
	}
	rows := m.repositoryListRows()
	if got, want := rows[0].group, "FAVORITES"; got != want {
		t.Fatalf("first group = %q, want %q", got, want)
	}
	if got, want := rows[1].group, "511"; got != want {
		t.Fatalf("favorite group = %q, want %q", got, want)
	}
	if got, want := rows[2].repository, "lab-one"; got != want {
		t.Fatalf("first favorite = %q, want %q", got, want)
	}
	if got, want := rows[3].repository, "lab-two"; got != want {
		t.Fatalf("second favorite = %q, want %q", got, want)
	}
}

func TestRepositoryListRowsGroupsByFirstFolder(t *testing.T) {
	m := model{repositories: []string{"511/lab-one", "511/lab-two", "523/lab-one", "testing"}}
	rows := m.repositoryListRows()
	groups := []string{}
	for _, row := range rows {
		if row.index < 0 {
			groups = append(groups, row.group)
		}
	}
	want := []string{"511", "523", "ROOT"}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %v, want %v", groups, want)
	}
	if got, want := rows[1].repository, "lab-one"; got != want {
		t.Fatalf("first grouped repository = %q, want %q", got, want)
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
	if !strings.Contains(view, "of ") {
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
