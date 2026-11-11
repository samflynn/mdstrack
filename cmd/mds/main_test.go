package main

import (
	"errors"
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
	if got, want := updated.(model).path, "/Users/penrose/Documents/MDS"; got != want {
		t.Fatalf("setup path = %q, want %q", got, want)
	}
}

func TestSetupPromptsForGitHubUsernameAfterWorkspace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	workspace := t.TempDir()
	m := model{mode: setupMode, path: workspace}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.setupStep, 1; got != want {
		t.Fatalf("setup step = %d, want %d", got, want)
	}
	if !strings.Contains(m.setupView(), "GitHub username:") {
		t.Fatal("expected the second setup prompt to request a GitHub username")
	}
}

func TestSettingsUpdatesGitHubUsername(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{mode: repositoriesMode, path: t.TempDir(), githubUsername: "old-user", favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(model)
	if m.mode != settingsMode {
		t.Fatal("expected s to open settings")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.editingSetting != "github" {
		t.Fatalf("editing setting = %q, want github", m.editingSetting)
	}
	m.settingInput = "new-user"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.githubUsername, "new-user"; got != want {
		t.Fatalf("GitHub username = %q, want %q", got, want)
	}
	stored, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := stored.GitHubUsername, "new-user"; got != want {
		t.Fatalf("stored GitHub username = %q, want %q", got, want)
	}
}

func TestTrackedRepositoryRendersCommitError(t *testing.T) {
	m := model{mode: repositoriesMode, width: 100, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{"511/lab-one": true}, commitErrors: map[string]error{"511/lab-one": errors.New("gh auth login required")}, collapsed: map[string]bool{}}
	view := m.repositoriesView()
	if !strings.Contains(view, "GH ERROR") || !strings.Contains(view, "gh auth login required") {
		t.Fatalf("expected the gh error to be visible, got %q", view)
	}
}

func TestTrackedRepositoryRendersCommitCount(t *testing.T) {
	m := model{mode: repositoriesMode, width: 80, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{"511/lab-one": true}, commitCounts: map[string]int{"511/lab-one": 14}, collapsed: map[string]bool{}}
	if !strings.Contains(m.repositoriesView(), "14 commits") {
		t.Fatal("expected tracked repository to render its GitHub commit count")
	}
}

func TestGitHubRepositoryParsesSupportedOrigins(t *testing.T) {
	for _, remote := range []string{"git@github.com:BagToad/mdstrack.git", "https://github.com/BagToad/mdstrack.git", "ssh://git@github.com/BagToad/mdstrack", "https://github.ubc.ca/mds-2026-27/DSCI_511_worksheet1_smudgal.git"} {
		host, owner, repository, ok := githubRepository(remote)
		if !ok || owner == "" || repository == "" || host == "" {
			t.Fatalf("githubRepository(%q) = (%q, %q, %q, %t)", remote, host, owner, repository, ok)
		}
	}
	if _, _, _, ok := githubRepository("https://gitlab.com/BagToad/mdstrack.git"); ok {
		t.Fatal("expected non-GitHub remote to be rejected")
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

func TestHeadersAreSelectableAndCollapseGroups(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{mode: repositoriesMode, path: "/tmp/mds", repositories: []string{"511/lab-one", "511/lab-two"}, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	rows := m.repositoryListRows()
	if !rows[0].header || rows[0].key != "group:511" {
		t.Fatalf("first row = %#v, want the 511 group header", rows[0])
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.collapsed["group:511"] {
		t.Fatal("expected Enter on a group header to collapse it")
	}
	if len(m.repositoryListRows()) != 1 {
		t.Fatal("expected the collapsed group to hide its repositories")
	}
}

func TestFavoritesArePinnedAndCanCollapse(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{mode: repositoriesMode, path: "/tmp/mds", repositories: []string{"511/lab-one", "511/lab-two", "523/lab-one"}, favorites: map[string]bool{"511/lab-one": true, "511/lab-two": true}, collapsed: map[string]bool{}}
	rows := m.repositoryListRows()
	if !rows[0].header || rows[0].key != "favorites" {
		t.Fatalf("first row = %#v, want the favorites header", rows[0])
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.collapsed["favorites"] {
		t.Fatal("expected Enter on favorites to collapse it")
	}
	if strings.Contains(m.repositoriesView(), "[X]") {
		t.Fatal("expected collapsed favorites to hide favorite repositories")
	}
}

func TestFavoritesTitleCanBeEditedAndPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{mode: repositoriesMode, path: "/tmp/mds", repositories: []string{"511/lab-one"}, favorites: map[string]bool{"511/lab-one": true}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(model)
	if !m.editingTitle {
		t.Fatal("expected e on the favorites header to start editing")
	}
	m.titleInput = "priority"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.favoritesDisplayTitle(), "PRIORITY (TRACKING)"; got != want {
		t.Fatalf("favorites title = %q, want %q", got, want)
	}
	stored, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := stored.FavoritesTitle, "priority"; got != want {
		t.Fatalf("stored title = %q, want %q", got, want)
	}
}

func TestFavoritePersistence(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	want := config{MDSFolder: "/tmp/mds", Favorites: map[string]bool{"511/lab-one": true}, Collapsed: map[string]bool{"favorites": true}, FavoritesTitle: "priority"}
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

func TestFindRepositoriesFindsAndSortsNestedRepositories(t *testing.T) {
	root := t.TempDir()
	for _, repository := range []string{"zeta", "course/week-one/project"} {
		if err := os.MkdirAll(filepath.Join(root, repository, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
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
