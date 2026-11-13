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
	"github.com/charmbracelet/x/ansi"
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

// TestRepositoryFooterKeepsCountdownIntact guards a Bubbles quirk: ShortHelpView
// can return a legend wider than the width it was given, which previously pushed
// the refresh countdown off the row. The countdown must either appear in full or
// be dropped deliberately, never shown half-eaten.
func TestRepositoryFooterKeepsCountdownIntact(t *testing.T) {
	const status = "Last 23:10 · Next 4:59"
	build := func(width int) model {
		return model{
			mode: repositoriesMode, width: width, height: 24, path: "/tmp/mds",
			repositories: []string{"511/lab-one"}, favorites: map[string]bool{"511/lab-one": true},
			commitCounts: map[string]int{"511/lab-one": 3}, collapsed: map[string]bool{},
			lastRefresh: time.Date(2026, 11, 12, 23, 10, 0, 0, time.Local),
			nextRefresh: time.Date(2026, 11, 12, 23, 15, 0, 0, time.Local),
			now:         time.Date(2026, 11, 12, 23, 10, 1, 0, time.Local),
		}
	}
	for width := 24; width <= 130; width++ {
		m := build(width)
		updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		lines := strings.Split(updated.(model).View(), "\n")
		last := lines[len(lines)-1]
		if got := lipgloss.Width(last); got != width {
			t.Errorf("width %d: footer is %d cells, want %d", width, got, width)
		}
		footer := ansi.Strip(last)
		if strings.Contains(footer, "Next") && !strings.Contains(footer, status) {
			t.Errorf("width %d: countdown truncated to %q", width, strings.TrimRight(footer, " "))
		}
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

// TestEveryScreenFitsTheTerminal pins the layout contract: whatever the state
// or the size, a screen renders exactly the terminal's row count and never
// exceeds its column count. This is what stops a long name, an extra status
// line, or a taller header from silently pushing the footer off screen.
func TestEveryScreenFitsTheTerminal(t *testing.T) {
	sizes := []struct{ width, height int }{{24, 6}, {30, 8}, {40, 10}, {60, 16}, {80, 20}, {120, 40}, {200, 60}}
	base := func() model {
		return model{
			mode: repositoriesMode, path: "/tmp/mds", githubUsername: "u", githubProfileName: "n", githubHost: "github.com",
			repositories: []string{"511/lab-one", "511/a-very-long-repository-name-that-must-truncate-safely", "521/lab-three", "521/lab-four"},
			favorites:    map[string]bool{"511/lab-one": true, "521/lab-three": true, "521/lab-four": true},
			commitCounts: map[string]int{"511/lab-one": 3, "521/lab-three": 1},
			commitErrors: map[string]error{"521/lab-four": errors.New("boom")},
			collapsed:    map[string]bool{}, groupTitles: map[string]string{"511": "DSCI 511"},
		}
	}
	states := map[string]func() model{
		"repositories":   base,
		"local search":   func() model { m := base(); m.localSearchActive, m.localSearch = true, "lab"; return m },
		"search chooser": func() model { m := base(); m.searchChooser = true; return m },
		"editor actions": func() model { m := base(); m.editorActions = true; return m },
		"github actions": func() model { m := base(); m.githubActions = true; return m },
		"status":         func() model { m := base(); m.actionStatus = "Copied GitHub URL"; return m },
		"renaming":       func() model { m := base(); m.editingTitle, m.titleInput = true, "My Group"; return m },
		"settings": func() model {
			m := base()
			m.mode, m.editingSetting, m.settingInput = settingsMode, "profile-name", "Shrey Mudgal"
			return m
		},
		"setup":          func() model { m := base(); m.mode, m.setupStep = setupMode, 2; return m },
		"search":         func() model { m := base(); m.mode, m.githubSearch = searchMode, "abc"; return m },
		"search loading": func() model { m := base(); m.mode, m.searchLoading = searchMode, true; return m },
		"search error":   func() model { m := base(); m.mode, m.searchError = searchMode, errors.New("boom"); return m },
		"clone folder": func() model {
			m := base()
			m.mode, m.cloneConfirmation, m.enteringCloneFolder, m.cloneFolderInput = searchMode, "a/b", true, "week-one"
			return m
		},
		"clone confirm":   func() model { m := base(); m.mode, m.cloneConfirmation = searchMode, "a/b"; return m },
		"help":            func() model { m := base(); m.helpVisible = true; return m },
		"scan error":      func() model { m := base(); m.err = errors.New("boom"); return m },
		"no repositories": func() model { m := base(); m.repositories = nil; return m },
		"loading commits": func() model { m := base(); m.loadingCommits = true; return m },
	}
	for _, size := range sizes {
		for name, build := range states {
			m := build()
			updated, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
			view := updated.(model).View()
			if got := lipgloss.Height(view); got != size.height {
				t.Errorf("%s at %dx%d: height = %d, want %d", name, size.width, size.height, got, size.height)
			}
			for index, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > size.width {
					t.Errorf("%s at %dx%d: line %d width = %d, exceeds %d", name, size.width, size.height, index, got, size.width)
				}
			}
		}
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
	if view := m.searchView(); !strings.Contains(view, "Clone mds-2026-27/DSCI_511_lab1") || !strings.Contains(view, "y workspace") || !strings.Contains(view, "f new folder") || !strings.Contains(view, "n no") || lipgloss.Height(view) != m.viewportHeight() {
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
	message := cloneGitHubRepository(workspace, "github.ubc.ca", "mds-2026-27/DSCI_511_lab1", "")()
	result, ok := message.(cloneRepositoryMsg)
	if !ok || result.err == nil || !strings.Contains(result.err.Error(), "already exists") {
		t.Fatalf("clone result = %#v, want existing destination error", message)
	}
}

func TestSearchCloneCanCreateNamedFolder(t *testing.T) {
	m := model{mode: searchMode, width: 80, height: 20, searchResults: []githubSearchRepository{{FullName: "mds-2026-27/DSCI_511_lab1"}}, favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(model)
	if !m.enteringCloneFolder {
		t.Fatal("expected f to request a new clone folder")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("week-one")})
	m = updated.(model)
	if view := m.searchView(); !strings.Contains(view, "New folder: week-one") || !strings.Contains(view, "Enter create & clone") {
		t.Fatalf("expected folder prompt, got %q", view)
	}
	if folder, err := cloneFolderName(m.cloneFolderInput); err != nil || folder != "week-one" {
		t.Fatalf("clone folder = (%q, %v), want week-one", folder, err)
	}
}

func TestCloneFolderNameRejectsPaths(t *testing.T) {
	for _, value := range []string{"", ".", "..", "course/week-one", "../outside"} {
		if _, err := cloneFolderName(value); err == nil {
			t.Errorf("cloneFolderName(%q) succeeded, want error", value)
		}
	}
}

func TestSearchCloneFolderShowsValidationError(t *testing.T) {
	m := model{mode: searchMode, width: 80, height: 20, cloneConfirmation: "mds-2026-27/DSCI_511_lab1", enteringCloneFolder: true, cloneFolderInput: "week/one", favorites: map[string]bool{}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.cloneStatus, "folder name must not contain a path"; got != want {
		t.Fatalf("clone folder error = %q, want %q", got, want)
	}
	if !strings.Contains(m.searchView(), m.cloneStatus) {
		t.Fatalf("expected folder validation error in search view")
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

func TestTextFieldsShowTailOfLongValues(t *testing.T) {
	// Inputs keep the caret at the end, so a value that does not fit must show
	// its tail: the part being typed is the part worth seeing.
	m := model{mode: setupMode, width: 30, height: 10, path: "/Users/penrose/Documents/MDS"}
	view := m.setupView()
	if !strings.Contains(view, "…ers/penrose/Documents/MDS") {
		t.Fatalf("expected the setup path to show its tail, got %q", view)
	}
	if strings.Contains(view, "/Users/penrose/Do") {
		t.Fatalf("expected the setup path to be trimmed from the left, got %q", view)
	}
}

func TestCommitProgressBarIsPlainUnderNoColor(t *testing.T) {
	// The bar is the densest colour carrier in the list, so it must survive a
	// colourless terminal: glyphs alone have to communicate the magnitude.
	for _, count := range []int{0, 1, 2, 3, 5} {
		if got := lipgloss.Width(commitProgressBar(count)); got != commitProgressWidth {
			t.Errorf("commitProgressBar(%d) width = %d, want %d", count, got, commitProgressWidth)
		}
	}
	if got, want := ansi.Strip(commitProgressBar(3)), "████████████"; got != want {
		t.Errorf("stripped commitProgressBar(3) = %q, want %q", got, want)
	}
}

func TestStatusMessagesExpireOnceAndAreNotClearedEarly(t *testing.T) {
	// A transient status must not outlive its timer, and a newer status must not
	// be wiped by the older status' still-pending timer.
	m := model{mode: repositoriesMode, width: 80, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}}

	updated, cmd := m.Update(repositoryActionMsg{err: errors.New("first")})
	m = updated.(model)
	if m.actionStatus != "Action failed: first" {
		t.Fatalf("actionStatus = %q, want %q", m.actionStatus, "Action failed: first")
	}
	if cmd == nil {
		t.Fatal("expected a status to arm an expiry timer")
	}
	firstToken := m.statusToken

	updated, _ = m.Update(repositoryActionMsg{err: errors.New("second")})
	m = updated.(model)
	if m.statusToken == firstToken {
		t.Fatal("expected a newer status to take ownership of the timer")
	}

	// The stale timer from "first" must be ignored.
	updated, _ = m.Update(statusExpiryMsg{token: firstToken})
	m = updated.(model)
	if m.actionStatus != "Action failed: second" {
		t.Fatalf("stale timer cleared the newer status: actionStatus = %q", m.actionStatus)
	}

	// The current timer clears it.
	updated, _ = m.Update(statusExpiryMsg{token: m.statusToken})
	m = updated.(model)
	if m.actionStatus != "" {
		t.Fatalf("current timer left actionStatus = %q, want empty", m.actionStatus)
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
	// Wide (CJK) and emoji runes occupy two cells, so a cell budget must not be
	// treated as a rune budget or the line overflows the terminal.
	for _, tc := range []struct {
		name  string
		value string
		width int
	}{
		{"cjk", "课程一/lab-one", 8},
		{"emoji", "lab-🎉-one", 7},
		{"mixed", "資料/DSCI_511_😀", 6},
	} {
		for _, got := range []string{truncate(tc.value, tc.width), truncateLeft(tc.value, tc.width)} {
			if w := lipgloss.Width(got); w > tc.width {
				t.Errorf("%s: %q renders %d cells, exceeds %d", tc.name, got, w, tc.width)
			}
		}
	}
	// Styled input must be measured by its visible text, not its escape codes.
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).Render("repository-name")
	if got := lipgloss.Width(truncate(styled, 8)); got > 8 {
		t.Errorf("styled truncate() renders %d cells, exceeds 8", got)
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

func TestGroupTitleCanBeEditedAndPersistsAcrossSections(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{mode: repositoriesMode, path: "/tmp/mds", repositories: []string{"511/lab-one", "511/lab-two", "523/lab-one"}, favorites: map[string]bool{"511/lab-one": true}, collapsed: map[string]bool{}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(model)
	if !m.editingTitle || m.editingGroup != "511" || m.titleInput != "511" {
		t.Fatalf("group edit state = (%t, %q, %q), want editing 511", m.editingTitle, m.editingGroup, m.titleInput)
	}
	m.titleInput = "Data Science 511"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.groupDisplayTitle("511"), "Data Science 511"; got != want {
		t.Fatalf("group title = %q, want %q", got, want)
	}
	for _, row := range m.repositoryListRows() {
		if row.header && row.rawGroup == "511" && row.group != "Data Science 511" {
			t.Fatalf("renamed group row = %#v", row)
		}
	}
	stored, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := stored.GroupTitles["511"], "Data Science 511"; got != want {
		t.Fatalf("stored group title = %q, want %q", got, want)
	}
}

func TestClearingGroupTitleRestoresFolderName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := model{mode: repositoriesMode, path: "/tmp/mds", repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}, groupTitles: map[string]string{"511": "Data Science 511"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(model)
	m.titleInput = ""
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if got, want := m.groupDisplayTitle("511"), "511"; got != want {
		t.Fatalf("group title after clear = %q, want %q", got, want)
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
