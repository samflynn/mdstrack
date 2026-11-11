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

func TestSettingsUpdatesGitHubProfileNameAndHost(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{mode: repositoriesMode, path: t.TempDir(), githubUsername: "smudgal", githubProfileName: "Old Name", githubHost: "github.ubc.ca", favorites: map[string]bool{}, collapsed: map[string]bool{}}
	m.settingsCursor = 2
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.editingSetting, "profile-name"; got != want {
		t.Fatalf("editing setting = %q, want %q", got, want)
	}
	m.settingInput = ""
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("Shrey")},
		{Type: tea.KeySpace, Runes: []rune{' '}},
		{Type: tea.KeyRunes, Runes: []rune("Mudgal")},
	} {
		updated, _ = m.Update(key)
		m = updated.(model)
	}
	if got, want := m.settingInput, "Shrey Mudgal"; got != want {
		t.Fatalf("typed profile display name = %q, want %q", got, want)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	m.settingsCursor = 3
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	m.settingInput = "https://github.ubc.ca/"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.githubProfileName, "Shrey Mudgal"; got != want {
		t.Fatalf("GitHub profile display name = %q, want %q", got, want)
	}
	if got, want := m.githubHost, "github.ubc.ca"; got != want {
		t.Fatalf("GitHub host = %q, want %q", got, want)
	}
	stored, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := stored.GitHubProfileName, "Shrey Mudgal"; got != want {
		t.Fatalf("stored profile display name = %q, want %q", got, want)
	}
	if got, want := stored.GitHubHost, "github.ubc.ca"; got != want {
		t.Fatalf("stored GitHub host = %q, want %q", got, want)
	}
}

func TestSetupPromptsForProfileNameAndGitHubHost(t *testing.T) {
	m := model{mode: setupMode, setupStep: 2}
	if !strings.Contains(m.setupView(), "GitHub profile display name:") {
		t.Fatal("expected setup to request the GitHub profile display name")
	}
	m.setupStep = 3
	m.githubHost = defaultGitHubHost
	if !strings.Contains(m.setupView(), "GitHub host:") {
		t.Fatal("expected setup to request the GitHub host")
	}
}

func TestTrackedRepositoryRendersCommitError(t *testing.T) {
	m := model{mode: repositoriesMode, width: 100, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{"511/lab-one": true}, commitErrors: map[string]error{"511/lab-one": errors.New("gh auth login required")}, collapsed: map[string]bool{}}
	view := m.repositoriesView()
	if !strings.Contains(view, "GH ERROR") || !strings.Contains(view, "gh auth login required") {
		t.Fatalf("expected the gh error to be visible, got %q", view)
	}
}

func TestGitHubActionsMenuOpensForSelectedRepository(t *testing.T) {
	m := model{mode: repositoriesMode, width: 80, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(model)
	if !m.githubActions {
		t.Fatal("expected g to open the GitHub actions menu for a repository")
	}
	if view := m.repositoriesView(); !strings.Contains(view, "GitHub actions") || !strings.Contains(view, "o open URL") || strings.Contains(view, "Copy GitHub URL") {
		t.Fatalf("expected compact GitHub actions footer in view, got %q", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(model).githubActions {
		t.Fatal("expected Esc to close the GitHub actions menu")
	}
}

func TestGitHubActionsMenuDoesNotOpenForGroupHeader(t *testing.T) {
	m := model{mode: repositoriesMode, repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if updated.(model).githubActions {
		t.Fatal("expected g on a group header to leave the GitHub actions menu closed")
	}
}

func TestGitHubCommitCountMatchesLoginOrDisplayName(t *testing.T) {
	commits := []githubCommit{
		{Author: &struct {
			Login string `json:"login"`
		}{Login: "smudgal"}},
		{Commit: struct {
			Author struct {
				Name string `json:"name"`
			} `json:"author"`
		}{Author: struct {
			Name string `json:"name"`
		}{Name: "Shrey Mudgal"}}},
		{Author: &struct {
			Login string `json:"login"`
		}{Login: "another-user"}},
	}
	count := 0
	for _, commit := range commits {
		if commitMatchesUser(commit, "smudgal", "Shrey Mudgal") {
			count++
		}
	}
	if got, want := count, 2; got != want {
		t.Fatalf("matched commits = %d, want %d", got, want)
	}
}

func TestTrackedRepositoryRendersLoadingSpinner(t *testing.T) {
	m := model{mode: repositoriesMode, width: 80, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{"511/lab-one": true}, commitCounts: map[string]int{}, commitErrors: map[string]error{}, loadingCommits: true, collapsed: map[string]bool{}}
	if !strings.Contains(m.repositoriesView(), "Loading") {
		t.Fatal("expected tracked repository to render a loading indicator")
	}
	updated, _ := m.Update(commitCountsMsg{counts: map[string]int{"511/lab-one": 0}, errors: map[string]error{}})
	m = updated.(model)
	if m.loadingCommits {
		t.Fatal("expected completed commit lookup to stop loading")
	}
	if strings.Contains(m.repositoriesView(), "Loading") {
		t.Fatal("expected completed commit lookup to remove the loading indicator")
	}
}

func TestCommitCountUpdatePreservesUnrelatedTrackedCounts(t *testing.T) {
	m := model{
		mode:         repositoriesMode,
		commitCounts: map[string]int{"511/old": 8},
		commitErrors: map[string]error{},
	}
	updated, _ := m.Update(commitCountsMsg{
		counts:       map[string]int{"511/new": 3},
		errors:       map[string]error{},
		repositories: []string{"511/new"},
	})
	m = updated.(model)
	if got, want := m.commitCounts["511/old"], 8; got != want {
		t.Fatalf("existing count = %d, want %d", got, want)
	}
	if got, want := m.commitCounts["511/new"], 3; got != want {
		t.Fatalf("new count = %d, want %d", got, want)
	}
}

func TestUntrackingRepositoryClearsOnlyItsCommitState(t *testing.T) {
	m := model{
		mode:         repositoriesMode,
		repositories: []string{"511/first", "511/second"},
		favorites:    map[string]bool{"511/first": true, "511/second": true},
		commitCounts: map[string]int{"511/first": 2, "511/second": 5},
		commitErrors: map[string]error{},
		collapsed:    map[string]bool{},
	}
	if repository, tracked := m.activateRow(repositoryListRow{index: 0}); repository != "511/first" || tracked {
		t.Fatalf("untracking result = (%q, %t), want (511/first, false)", repository, tracked)
	}
	if _, ok := m.commitCounts["511/first"]; ok {
		t.Fatal("expected untracked repository count to be cleared")
	}
	if got, want := m.commitCounts["511/second"], 5; got != want {
		t.Fatalf("unrelated count = %d, want %d", got, want)
	}
}

func TestCommitProgressBarCapsAtThreeSegments(t *testing.T) {
	for _, test := range []struct {
		count int
		want  string
	}{
		{count: 0, want: "░░░░░░░░░░░░"},
		{count: 1, want: "████░░░░░░░░"},
		{count: 2, want: "████████░░░░"},
		{count: 3, want: "████████████"},
		{count: 5, want: "████████████"},
	} {
		if got := commitProgressBar(test.count); got != test.want {
			t.Errorf("commitProgressBar(%d) = %q, want %q", test.count, got, test.want)
		}
	}
}

func TestTrackedRepositoryRendersCommitProgress(t *testing.T) {
	m := model{mode: repositoriesMode, width: 80, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{"511/lab-one": true}, commitCounts: map[string]int{"511/lab-one": 5}, collapsed: map[string]bool{}}
	if view := m.repositoriesView(); !strings.Contains(view, "5/3") || strings.Index(view, "████████████") > strings.Index(view, "5/3") {
		t.Fatal("expected tracked repository to render its GitHub commit progress")
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
	want := config{MDSFolder: "/tmp/mds", GitHubProfileName: "Shrey Mudgal", GitHubHost: "github.ubc.ca", Favorites: map[string]bool{"511/lab-one": true}, Collapsed: map[string]bool{"favorites": true}, FavoritesTitle: "priority"}
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
