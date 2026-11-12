package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestRefreshIntervalSettingAndStatus(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	now := time.Date(2026, time.November, 11, 12, 0, 0, 0, time.UTC)
	m := model{mode: settingsMode, path: t.TempDir(), githubUsername: "smudgal", githubProfileName: "Shrey Mudgal", githubHost: "github.ubc.ca", refreshIntervalMinutes: defaultRefreshIntervalMinutes, favorites: map[string]bool{}, collapsed: map[string]bool{}, settingsCursor: 5, lastRefresh: now, nextRefresh: now.Add(5 * time.Minute)}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.editingSetting, "refresh-interval"; got != want {
		t.Fatalf("editing setting = %q, want %q", got, want)
	}
	m.settingInput = "10"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.refreshIntervalMinutes, 10; got != want {
		t.Fatalf("refresh interval = %d, want %d", got, want)
	}
	m.lastRefresh, m.nextRefresh = now, now.Add(10*time.Minute)
	if got := m.refreshStatus(now); !strings.Contains(got, "Last 12:00") || !strings.Contains(got, "Next 10:00") {
		t.Fatalf("refresh status = %q", got)
	}
	if _, err := parseRefreshInterval("0"); err == nil {
		t.Fatal("expected zero refresh interval to be rejected")
	}
}

func TestDeadlineUsesOverrideUntilItPasses(t *testing.T) {
	now := time.Date(2026, time.November, 11, 12, 0, 0, 0, time.UTC)
	m := model{deadlineOverride: "2026-11-13"}
	if got, want := m.effectiveDeadline(now).Format("2006-01-02"), "2026-11-13"; got != want {
		t.Fatalf("override deadline = %s, want %s", got, want)
	}
	m.deadlineOverride = "2026-11-10"
	if got, want := m.effectiveDeadline(now).Format("2006-01-02"), "2026-11-14"; got != want {
		t.Fatalf("expired override deadline = %s, want next Saturday %s", got, want)
	}
}

func TestDeadlineStatusUsesCalendarDays(t *testing.T) {
	now := time.Date(2026, time.November, 11, 23, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		deadline time.Time
		want     string
	}{
		{deadline: time.Date(2026, time.November, 14, 1, 0, 0, 0, time.UTC), want: "3 days left"},
		{deadline: time.Date(2026, time.November, 12, 0, 0, 0, 0, time.UTC), want: "1 day left"},
		{deadline: time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC), want: "due today"},
		{deadline: time.Date(2026, time.November, 9, 0, 0, 0, 0, time.UTC), want: "2 days overdue"},
	} {
		if got := deadlineStatus(test.deadline, now); !strings.Contains(got, test.want) {
			t.Errorf("deadlineStatus(%s) = %q, want %q", test.deadline, got, test.want)
		}
	}
}

func TestSettingsSavesAndClearsDeadlineOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{mode: settingsMode, path: t.TempDir(), githubUsername: "smudgal", githubProfileName: "Shrey Mudgal", githubHost: "github.ubc.ca", favorites: map[string]bool{}, collapsed: map[string]bool{}, settingsCursor: 4}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.editingSetting, "deadline"; got != want {
		t.Fatalf("editing setting = %q, want %q", got, want)
	}
	m.settingInput = "2026-11-13"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.deadlineOverride, "2026-11-13"; got != want {
		t.Fatalf("deadline override = %q, want %q", got, want)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	m.settingInput = ""
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.deadlineOverride != "" {
		t.Fatalf("deadline override = %q, want cleared override", m.deadlineOverride)
	}
	stored, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if stored.DeadlineOverride != "" {
		t.Fatalf("stored deadline override = %q, want cleared override", stored.DeadlineOverride)
	}
}

func TestTrackedRepositoryRendersCommitError(t *testing.T) {
	m := model{mode: repositoriesMode, width: 100, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{"511/lab-one": true}, commitErrors: map[string]error{"511/lab-one": errors.New("gh auth login required")}, collapsed: map[string]bool{}}
	view := m.repositoriesView()
	if !strings.Contains(view, "GH ERROR") || !strings.Contains(view, "gh auth login required") {
		t.Fatalf("expected the gh error to be visible, got %q", view)
	}
}

func TestRepositoryFooterIncludesSearchShortcut(t *testing.T) {
	now := time.Now()
	m := model{mode: repositoriesMode, width: 100, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}, lastRefresh: now, nextRefresh: now.Add(5 * time.Minute)}
	if view := m.repositoriesView(); !strings.Contains(view, "/ search") || !strings.Contains(view, "? help") || !strings.Contains(view, "Last ") || !strings.Contains(view, "Next ") {
		t.Fatalf("expected repository footer to include search, help, and refresh shortcuts, got %q", view)
	}
}

func TestHelpOverlayIsHeightCappedAndDismissible(t *testing.T) {
	m := model{mode: repositoriesMode, width: 60, height: 16, repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(model)
	if !m.helpVisible {
		t.Fatal("expected question mark to open help")
	}
	if view := m.View(); !strings.Contains(view, "Keyboard shortcuts") || lipgloss.Height(view) != m.viewportHeight() {
		t.Fatalf("expected height-capped help overlay, got %q", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(model).helpVisible {
		t.Fatal("expected Esc to close help")
	}
}

func TestRepositoryViewFitsCommonTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{{60, 16}, {80, 20}, {120, 40}} {
		m := model{mode: repositoriesMode, width: size.width, height: size.height, repositories: []string{"511/a-very-long-repository-name-that-must-truncate-safely", "511/lab-two", "521/lab-three"}, favorites: map[string]bool{"511/lab-two": true}, commitCounts: map[string]int{"511/lab-two": 2}, collapsed: map[string]bool{}, lastRefresh: time.Now(), nextRefresh: time.Now().Add(5 * time.Minute)}
		view := m.repositoriesView()
		if got := lipgloss.Height(view); got != size.height {
			t.Errorf("%dx%d view height = %d, want %d", size.width, size.height, got, size.height)
		}
		for _, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > size.width {
				t.Errorf("%dx%d line width = %d, exceeds %d: %q", size.width, size.height, got, size.width, line)
			}
		}
	}
}

func TestWideRepositoryViewIncludesNavigationRail(t *testing.T) {
	m := model{mode: repositoriesMode, width: 120, height: 40, repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}, lastRefresh: time.Now(), nextRefresh: time.Now().Add(5 * time.Minute)}
	view := m.repositoriesView()
	for _, text := range []string{"WORKSPACE", "GITHUB SEARCH", "SETTINGS", "Repository cockpit"} {
		if !strings.Contains(view, text) {
			t.Fatalf("wide workspace view is missing %q: %q", text, view)
		}
	}
	if got := lipgloss.Height(view); got != m.viewportHeight() {
		t.Fatalf("wide workspace view height = %d, want %d", got, m.viewportHeight())
	}
}

func TestWideTrackedRowsFitWorkspacePanel(t *testing.T) {
	m := model{mode: repositoriesMode, width: 120, height: 40, repositories: []string{"511/lab-one"}, favorites: map[string]bool{"511/lab-one": true}, commitCounts: map[string]int{"511/lab-one": 3}, collapsed: map[string]bool{}, lastRefresh: time.Now(), nextRefresh: time.Now().Add(5 * time.Minute)}
	view := m.repositoriesView()
	if strings.Contains(view, "\n  │  days left") {
		t.Fatalf("wide tracked row wrapped its deadline outside the workspace panel: %q", view)
	}
	if !strings.Contains(view, "days left") || !strings.Contains(view, "3/3") {
		t.Fatalf("wide tracked row lost deadline or progress information: %q", view)
	}
}

func TestRepositoryActionMenusRenderAsCards(t *testing.T) {
	m := model{mode: repositoriesMode, width: 80, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}, editorActions: true}
	view := m.repositoriesView()
	for _, text := range []string{"OPEN", "Open the selected repository in", "v VS Code", "f Finder"} {
		if !strings.Contains(view, text) {
			t.Fatalf("open action card is missing %q: %q", text, view)
		}
	}
	if got := lipgloss.Height(view); got != m.viewportHeight() {
		t.Fatalf("open action card height = %d, want %d", got, m.viewportHeight())
	}
}

func TestHomeSearchFiltersRepositoriesAndKeepsTrackingAction(t *testing.T) {
	m := model{mode: repositoriesMode, width: 80, height: 20, repositories: []string{"511/lab-one", "511/worksheet-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(model)
	if !m.searchChooser {
		t.Fatal("expected slash to open the search chooser")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("worksheet")})
	m = updated.(model)
	rows := m.repositoryListRows()
	if !m.localSearchActive || len(rows) != 2 || rows[1].repository != "worksheet-one" {
		t.Fatalf("local search rows = %#v, want worksheet repository only", rows)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.favorites["511/worksheet-one"] {
		t.Fatal("expected Enter in local search to track the selected repository")
	}
}

func TestEditorActionsMenuOpensForSelectedRepository(t *testing.T) {
	m := model{mode: repositoriesMode, width: 80, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = updated.(model)
	if !m.editorActions {
		t.Fatal("expected o to open editor actions")
	}
	if view := m.repositoriesView(); !strings.Contains(view, "v VS Code") || !strings.Contains(view, "p Positron") || !strings.Contains(view, "f Finder") || strings.Contains(view, "RStudio") || lipgloss.Height(view) != m.viewportHeight() {
		t.Fatalf("expected terminal-safe editor actions footer, got %q", view)
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

func TestGitHubActionsMenuDoesNotOverflowTerminal(t *testing.T) {
	repositories := make([]string, 20)
	for index := range repositories {
		repositories[index] = "511/repository-" + string(rune('a'+index))
	}
	m := model{mode: repositoriesMode, width: 80, height: 20, repositories: repositories, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(model)
	if got, want := lipgloss.Height(m.repositoriesView()), m.viewportHeight(); got != want {
		t.Fatalf("GitHub actions view height = %d, want terminal height %d", got, want)
	}
}

func TestSearchOpensAndFiltersAccessibleRepositories(t *testing.T) {
	m := model{mode: repositoriesMode, width: 80, height: 20, githubHost: "github.ubc.ca", searchResults: []githubSearchRepository{{FullName: "mds-2026-27/DSCI_511_lab1"}, {FullName: "mds-2026-27/DSCI_521_lab0"}, {FullName: "other/course"}}, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(model)
	if m.mode != searchMode || !m.searchLoading || !m.hideCloned {
		t.Fatal("expected GitHub search choice to open search with cloned repositories hidden by default")
	}
	updated, _ = m.Update(githubSearchResultsMsg{repositories: m.searchResults})
	m = updated.(model)
	if m.searchLoading {
		t.Fatal("expected search results to finish loading")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("521")})
	m = updated.(model)
	results := m.filteredSearchResults()
	if got, want := len(results), 1; got != want || results[0].FullName != "mds-2026-27/DSCI_521_lab0" {
		t.Fatalf("filtered search results = %#v, want only the 521 repository", results)
	}
	m.githubSearch = ""
	m.clonedRepositories = map[string]bool{"mds-2026-27/DSCI_511_lab1": true}
	results = m.filteredSearchResults()
	if got, want := len(results), 2; got != want {
		t.Fatalf("hidden-clone results = %#v, want %d repositories", results, want)
	}
	for _, repository := range results {
		if repository.FullName == "mds-2026-27/DSCI_511_lab1" {
			t.Fatal("expected cloned repository to be hidden")
		}
	}
	if view := m.searchView(); !strings.Contains(view, "h show cloned") {
		t.Fatalf("expected search view to describe the active hide state, got %q", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(model)
	if m.hideCloned {
		t.Fatal("expected h to reveal cloned repositories")
	}
	if got, want := len(m.filteredSearchResults()), 3; got != want {
		t.Fatalf("shown-clone results = %d, want %d", got, want)
	}
	if got, want := lipgloss.Height(m.searchView()), m.viewportHeight(); got != want {
		t.Fatalf("search view height = %d, want terminal height %d", got, want)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(model).mode != repositoriesMode {
		t.Fatal("expected Esc to return from search to repositories")
	}
}

func TestSearchCloneRequiresConfirmation(t *testing.T) {
	m := model{mode: searchMode, width: 80, height: 20, searchResults: []githubSearchRepository{{FullName: "mds-2026-27/DSCI_511_lab1"}}, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.cloneConfirmation, "mds-2026-27/DSCI_511_lab1"; got != want {
		t.Fatalf("clone confirmation = %q, want %q", got, want)
	}
	if view := m.searchView(); !strings.Contains(view, "Clone mds-2026-27/DSCI_511_lab1") || !strings.Contains(view, "y yes") || !strings.Contains(view, "n no") || lipgloss.Height(view) != m.viewportHeight() {
		t.Fatalf("expected terminal-safe clone confirmation footer, got %q", view)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(model)
	if m.cloneConfirmation != "" {
		t.Fatal("expected n to cancel clone confirmation")
	}
}

func TestCloneGitHubRepositoryRefusesExistingDestination(t *testing.T) {
	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, "DSCI_511_lab1"), 0o755); err != nil {
		t.Fatal(err)
	}
	message := cloneGitHubRepository(workspace, "github.ubc.ca", "mds-2026-27/DSCI_511_lab1")()
	result, ok := message.(cloneRepositoryMsg)
	if !ok || result.err == nil || !strings.Contains(result.err.Error(), "already exists") {
		t.Fatalf("clone result = %#v, want existing destination error", message)
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
