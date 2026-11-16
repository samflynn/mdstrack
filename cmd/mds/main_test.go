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
	m := model{mode: repositoriesMode, path: "/tmp/mds", repositories: []string{"511/alpha", "511/beta", "523/lab-one"}, favorites: map[string]bool{"511/alpha": true, "511/beta": true}, collapsed: map[string]bool{}}
	rows := m.repositoryListRows()
	if !rows[0].header || rows[0].key != "favorites" {
		t.Fatalf("first row = %#v, want the favorites header", rows[0])
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if !m.collapsed["favorites"] {
		t.Fatal("expected Enter on favorites to collapse it")
	}
	if view := m.repositoriesView(); strings.Contains(view, "alpha") || strings.Contains(view, "beta") {
		t.Fatalf("expected collapsed favorites to hide their repositories, got:\n%s", view)
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

// scrollingModel builds a list of nine repositories in three groups with two
// tracked repositories that live in different groups, so the tracked block
// contains a group header for each of them. Width 64 height 14 leaves ten body
// rows for sixteen list rows, which is enough for the block to pin and for the
// rest to scroll.
func scrollingModel() model {
	repositories := []string{}
	for _, lab := range []string{"lab-1", "lab-2", "lab-3"} {
		for _, name := range []string{"a", "b", "c"} {
			repositories = append(repositories, "DSCI_511_"+lab+"/"+name)
		}
	}
	repositories = append(repositories, "DSCI_522_lab-1/a", "DSCI_522_lab-2/b", "DSCI_522_lab-3/c")
	m := model{
		mode: repositoriesMode, width: 64, height: 14, now: time.Now(),
		repositories: repositories, favorites: map[string]bool{}, collapsed: map[string]bool{},
		groupTitles:    map[string]string{},
		favoritesTitle: "Priority",
		commitCounts:   map[string]int{},
		commitErrors:   map[string]error{},
	}
	m.favorites[repositories[0]] = true
	m.favorites[repositories[4]] = true
	return m
}

// bodyLines returns the rendered body of the repositories screen, which is
// everything between the accent rule and the footer.
func bodyLines(t *testing.T, m model) []string {
	t.Helper()
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	if len(lines) < 4 {
		t.Fatalf("view has %d lines, want at least 4", len(lines))
	}
	return lines[2 : len(lines)-1]
}

// cursorBodyLine returns the body row the selection marker is drawn on. Rows
// are rendered inside the gutter, so leading spaces are trimmed before looking
// for the marker.
func cursorBodyLine(t *testing.T, m model) int {
	t.Helper()
	for index, line := range bodyLines(t, m) {
		if strings.HasPrefix(strings.TrimLeft(line, " "), "▸") {
			return index
		}
	}
	return -1
}

// TestRepositoryScrollingDoesNotPinTheSelectionToTheTop is the regression guard
// for the reported behaviour: pressing down used to keep the selected row at
// the top of the screen and scroll everything above it out of view instead of
// moving the selection down the list.
func TestRepositoryScrollingDoesNotPinTheSelectionToTheTop(t *testing.T) {
	m := scrollingModel()
	if m.scrollTop != 0 {
		t.Fatalf("initial scrollTop = %d, want 0", m.scrollTop)
	}
	seen := map[int]bool{}
	for step := 0; step < 12; step++ {
		if line := cursorBodyLine(t, m); line >= 0 {
			seen[line] = true
		}
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = updated.(model)
	}
	if len(seen) < 3 {
		t.Fatalf("selection appeared on body lines %v; it must move down the window rather than staying pinned", seen)
	}
}

// TestTrackedSectionStaysVisibleWhileScrolling checks that the whole tracking
// block, including a group header per tracked repository, is present at every
// scroll offset.
func TestTrackedSectionStaysVisibleWhileScrolling(t *testing.T) {
	m := scrollingModel()
	rows := m.repositoryListRows()
	pinned := m.pinnedRowCount(rows)
	if pinned < 4 {
		t.Fatalf("pinnedRowCount = %d, want the header, group headers and repositories for both tracked repos", pinned)
	}
	for step := 0; step < 16; step++ {
		body := strings.Join(bodyLines(t, m), "\n")
		if !strings.Contains(body, "PRIORITY (TRACKING)") {
			t.Fatalf("step %d: tracking title missing from the body:\n%s", step, body)
		}
		for _, repository := range m.repositories {
			if !m.favorites[repository] {
				continue
			}
			_, name := repositoryGroup(repository)
			if !strings.Contains(body, name) {
				t.Fatalf("step %d: tracked repository %s missing from the body:\n%s", step, name, body)
			}
		}
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = updated.(model)
	}
}

// TestSelectionStaysInsideTheWindowAtEveryHeight guards the small-terminal
// cases: the selection must always be rendered, whether the tracked block is
// pinned or the list has fallen back to plain scrolling.
func TestSelectionStaysInsideTheWindowAtEveryHeight(t *testing.T) {
	for _, height := range []int{6, 8, 10, 12, 14, 20, 40} {
		m := scrollingModel()
		m.height = height
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 64, Height: height})
		m = updated.(model)
		rows := m.repositoryListRows()
		for step := 0; step < len(rows)+2; step++ {
			view := m.View()
			if got := lipgloss.Height(view); got != height {
				t.Fatalf("height %d step %d: rendered %d rows", height, step, got)
			}
			if cursorBodyLine(t, m) < 0 {
				t.Fatalf("height %d step %d: cursor row is not visible:\n%s", height, step, ansi.Strip(view))
			}
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
			m = updated.(model)
		}
	}
}

// TestWithoutTrackedRepositoriesNothingIsPinned keeps the plain list free of a
// pinned block: with no favourites there is no tracking section to hold still.
func TestWithoutTrackedRepositoriesNothingIsPinned(t *testing.T) {
	m := scrollingModel()
	m.favorites = map[string]bool{}
	if got := m.pinnedRowCount(m.repositoryListRows()); got != 0 {
		t.Fatalf("pinnedRowCount with no favourites = %d, want 0", got)
	}
	for step := 0; step < 6; step++ {
		if cursorBodyLine(t, m) < 0 {
			t.Fatalf("step %d: cursor row is not visible", step)
		}
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = updated.(model)
	}
}

// TestTrackedBlockTooLargeFallsBackToScrolling covers a small terminal whose
// body cannot hold the tracked block and a scrolled row at once. Pinning the
// block there would leave nowhere to draw the selection, so the list has to
// scroll normally instead.
func TestTrackedBlockTooLargeFallsBackToScrolling(t *testing.T) {
	m := scrollingModel()
	// Track most of the list so the block far exceeds any small body.
	for _, repository := range m.repositories {
		m.favorites[repository] = true
	}
	m.height = 8
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 64, Height: 8})
	m = updated.(model)
	if got := m.pinnedRowCount(m.repositoryListRows()); got != 0 {
		t.Fatalf("pinnedRowCount = %d, want 0 so the list can scroll", got)
	}
	for step := 0; step < 14; step++ {
		if cursorBodyLine(t, m) < 0 {
			t.Fatalf("step %d: cursor row is not visible:\n%s", step, ansi.Strip(m.View()))
		}
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = updated.(model)
	}
}

// TestScrollIsRecomputedWhenTheRowSetChanges ensures a stale window cannot
// survive a change that removes rows, such as collapsing a group or filtering.
func TestScrollIsRecomputedWhenTheRowSetChanges(t *testing.T) {
	m := scrollingModel()
	for step := 0; step < 16; step++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = updated.(model)
	}
	if m.scrollTop == 0 {
		t.Fatal("expected the window to have advanced before filtering")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = updated.(model)
	for _, r := range "zzz-no-match" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	if !m.localSearchActive {
		t.Fatal("expected the local search to be active")
	}
	if got := len(m.repositoryListRows()); got != 0 {
		t.Fatalf("a non-matching filter should empty the list, got %d rows", got)
	}
	if m.scrollTop != 0 {
		t.Fatalf("scrollTop after filtering to no rows = %d, want 0", m.scrollTop)
	}
}

// --- Request 7: rename prompt visibility, caret blink, count fallback, legend ---

// editorModel returns a repositories screen with the rename editor open and the
// given value already typed.
func editorModel(height int, value string) model {
	m := model{
		mode: repositoriesMode, width: 72, height: height, path: "/tmp/mds",
		repositories:   []string{"511/lab-one", "511/lab-two", "522/lab-three"},
		favorites:      map[string]bool{},
		collapsed:      map[string]bool{},
		groupTitles:    map[string]string{},
		commitCounts:   map[string]int{},
		commitErrors:   map[string]error{},
		editingTitle:   true,
		titleNameInput: newTextValue("Group name: "),
		titleInput:     value,
	}
	return m
}

// TestRenamePromptStaysVisibleOnShortTerminals is the regression guard for the
// reported "I cannot see what I type" behaviour. The body is a fixed height and
// the row list used to consume all of it, so the prompt was the first line the
// block dropped.
func TestRenamePromptStaysVisibleOnShortTerminals(t *testing.T) {
	for _, height := range []int{4, 6, 8, 10, 14, 24} {
		m := editorModel(height, "Renamed Group")
		view := ansi.Strip(m.View())
		if !strings.Contains(view, "Renamed Group") {
			t.Errorf("height %d: typed value missing from view %q", height, view)
		}
		if !strings.Contains(view, "Rename group:") {
			t.Errorf("height %d: rename prompt missing from view %q", height, view)
		}
	}
}

// TestTypedRenameTextIsVisibleWhileEditing types through the real Update path so
// the rendered value is the one the input actually holds.
func TestTypedRenameTextIsVisibleWhileEditing(t *testing.T) {
	m := editorModel(8, "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(model)
	for _, r := range "Renamed" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Renamed") {
		t.Fatalf("typed text missing from view %q", view)
	}
}

// TestCaretBlinksWhileRenaming drives the timer the app owns. The caret glyph
// must alternate between the visible block and a blank cell, and a tick left
// over from a closed editor must not touch anything.
func TestCaretBlinksWhileRenaming(t *testing.T) {
	m := model{
		mode: repositoriesMode, width: 72, height: 20, path: "/tmp/mds",
		repositories: []string{"511/lab-one"}, favorites: map[string]bool{},
		collapsed: map[string]bool{}, groupTitles: map[string]string{},
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(model)
	if !m.editingTitle {
		t.Fatal("expected e to open the rename editor")
	}
	if !m.blinkActive {
		t.Fatal("expected opening the rename editor to arm the caret blink")
	}
	if cmd == nil {
		t.Fatal("expected opening the rename editor to return the blink timer")
	}

	updated, cmd = m.Update(blinkMsg{token: m.blinkToken})
	m = updated.(model)
	if m.titleNameInput.input.Cursor.Blink != true {
		t.Fatal("expected the first tick to flip the cursor into its blank frame")
	}
	if cmd == nil {
		t.Fatal("expected a blink tick to re-arm the timer")
	}

	updated, _ = m.Update(blinkMsg{token: m.blinkToken})
	if blink := updated.(model).titleNameInput.input.Cursor.Blink; blink != false {
		t.Fatal("expected the second tick to flip the cursor back to the block frame")
	}

	before := m.titleNameInput.input.Cursor.Blink
	updated, cmd = m.Update(blinkMsg{token: m.blinkToken + 99})
	if blink := updated.(model).titleNameInput.input.Cursor.Blink; blink != before {
		t.Fatal("expected a stale blink tick to be ignored")
	}
	if cmd != nil {
		t.Fatal("expected a stale blink tick not to re-arm the timer")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m = updated.(model); m.editingTitle || m.blinkActive {
		t.Fatal("expected Esc to close the rename editor and stop the blink")
	}
}

// TestCaretGlyphsAlternate pins the two frames the caret renders.
func TestCaretGlyphsAlternate(t *testing.T) {
	if got := textCaret(true); got != " " {
		t.Fatalf("blank frame = %q, want a space", got)
	}
	if got := textCaret(false); got != "█" {
		t.Fatalf("block frame = %q, want █", got)
	}
}

// TestSettingsValueEditorStaysVisibleOnShortTerminals covers the other editor
// that shares the reserved rows.
func TestSettingsValueEditorStaysVisibleOnShortTerminals(t *testing.T) {
	for _, height := range []int{4, 6, 8, 12, 24} {
		m := model{
			mode: settingsMode, width: 72, height: height, path: "/tmp/mds",
			favorites: map[string]bool{}, collapsed: map[string]bool{},
			groupTitles:    map[string]string{},
			editingSetting: "profile-name", settingInput: "Shrey Mudgal",
		}
		if view := ansi.Strip(m.View()); !strings.Contains(view, "Shrey Mudgal") {
			t.Errorf("height %d: settings value missing from view %q", height, view)
		}
	}
}

// TestShortHelpListsRenameShortcut is the regression guard for the missing e
// shortcut in the footer legend.
func TestShortHelpListsRenameShortcut(t *testing.T) {
	found := false
	for _, binding := range repoKeys.ShortHelp() {
		if key, _ := binding.Help().Key, binding.Help().Desc; key == repoKeys.Rename.Help().Key {
			found = true
		}
	}
	if !found {
		t.Fatal("expected the short help legend to list the e rename shortcut")
	}
	if got := repoKeys.Rename.Help().Desc; got != "rename" {
		t.Fatalf("rename help description = %q, want rename", got)
	}
	now := time.Now()
	m := model{mode: repositoriesMode, width: 110, height: 20, repositories: []string{"511/lab-one"}, favorites: map[string]bool{}, collapsed: map[string]bool{}, lastRefresh: now, nextRefresh: now.Add(5 * time.Minute)}
	if view := ansi.Strip(m.repositoriesView()); !strings.Contains(view, "e rename") {
		t.Fatalf("expected the repository footer to show the rename shortcut, got %q", view)
	}
}

// TestTrackedCommitCountsAcceptProfileNameAlone is the regression guard for the
// vanished commit counter: the loader used to bail out whenever the GitHub
// username was blank, which threw away the profile display name fallback.
func TestTrackedCommitCountsAcceptProfileNameAlone(t *testing.T) {
	workspace := t.TempDir()
	repository := "DSCI_511_lab1/mds"
	if err := os.MkdirAll(filepath.Join(workspace, repository), 0o755); err != nil {
		t.Fatalf("create workspace repository: %v", err)
	}
	tracked := map[string]bool{repository: true}

	// A non-repository directory cannot yield a count, but reaching that
	// failure at all proves the loader looked instead of bailing out on the
	// blank username. Bailing out leaves errors empty.
	message := loadTrackedCommitCounts(workspace, []string{repository}, tracked, "", "Shrey Mudgal", "github.ubc.ca")()
	counts, ok := message.(commitCountsMsg)
	if !ok {
		t.Fatalf("message = %T, want commitCountsMsg", message)
	}
	if len(counts.errors) != 1 {
		t.Fatalf("errors = %v, want the tracked repository to be inspected despite a blank username", counts.errors)
	}

	// Without a profile name there is genuinely nothing to match on.
	message = loadTrackedCommitCounts(workspace, []string{repository}, tracked, "", "", "github.ubc.ca")()
	counts, _ = message.(commitCountsMsg)
	if len(counts.errors) != 0 {
		t.Fatalf("errors = %v, want no lookup when both identifier fields are blank", counts.errors)
	}
}

// repositoriesModel is the small fixture the presentation tests share: two
// groups, one tracked repository, and a fixed clock so deadline wording and
// celebration windows never depend on when the suite runs.
func repositoriesModel(now time.Time) model {
	m := model{
		mode: repositoriesMode, path: "/tmp/mds", favoritesTitle: "Priority",
		repositories: []string{"511/alpha", "511/beta", "511/delta", "521/gamma"},
		favorites:    map[string]bool{"511/alpha": true},
		commitCounts: map[string]int{"511/alpha": 1},
		commitErrors: map[string]error{},
		collapsed:    map[string]bool{}, groupTitles: map[string]string{},
		celebrating: map[string]bool{}, celebratedUntil: map[string]time.Time{},
		now: now,
	}
	m.spinner = newModelSpinner()
	m.help = newModelHelp()
	m.width, m.height = 80, 20
	return m
}

func TestGroupHeaderBadgeCountsMembersAndSkipsSingleton(t *testing.T) {
	m := repositoriesModel(time.Now())
	var badges []string
	for _, row := range m.repositoryListRows() {
		if row.header && strings.HasPrefix(row.key, "group:") {
			badges = append(badges, m.repositoryRowLine(row, 40, false))
		}
	}
	// 511 has two members and 521 only one, so only 511 earns a badge: a count
	// of one is already visible from the single row beneath it.
	joined := ansi.Strip(strings.Join(badges, "\n"))
	if !strings.Contains(joined, "511 (2)") {
		t.Fatalf("expected the two member group to show a count badge, got:\n%s", joined)
	}
	if strings.Contains(joined, "521 (1)") {
		t.Fatalf("expected a single member group to omit its badge, got:\n%s", joined)
	}
}

func TestTreeGuidesNestUnderTheHeadingAndCloseEveryGroup(t *testing.T) {
	m := repositoriesModel(time.Now())
	lines := map[string]string{}
	headings := map[string]string{}
	for _, row := range m.repositoryListRows() {
		line := ansi.Strip(m.repositoryRowLine(row, 40, false))
		if row.header {
			headings[row.key] = line
			continue
		}
		if row.key == "divider" {
			continue
		}
		lines[row.repository] = line
	}
	// beta is the first of two in the untracked 511 group, so it leaves a tee
	// behind for delta to hang from, and delta is last so its branch closes.
	if !strings.HasPrefix(lines["beta"], treeChildTee) {
		t.Fatalf("expected a non-final row to use a tee, got %q", lines["beta"])
	}
	if !strings.HasPrefix(lines["delta"], treeChildElbow) {
		t.Fatalf("expected the final row of a group to close with an elbow, got %q", lines["delta"])
	}
	// gamma sits alone in 521, so it is simultaneously the first and last row of
	// its group and must still close rather than dangle a tee.
	if !strings.HasPrefix(lines["gamma"], treeChildElbow) {
		t.Fatalf("expected a single member group to close with an elbow, got %q", lines["gamma"])
	}
	// The regression this guards: a heading must never be indented deeper than
	// the repositories beneath it, or the group reads as a child of its members.
	if !strings.HasPrefix(headings["group:511"], rowGroupBar) {
		t.Fatalf("expected the heading to sit at the root of the list, got %q", headings["group:511"])
	}
	// Every repository row is pushed right by its branch guide, so the heading
	// stays flush at the root and the members read as nested underneath it.
	for name, line := range lines {
		if !strings.HasPrefix(line, treeChildTee) && !strings.HasPrefix(line, treeChildElbow) {
			t.Fatalf("expected %s to be indented under its heading, got %q", name, line)
		}
	}
}

func TestStateMarkerEncodesEveryRepositoryState(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name   string
		adjust func(*model)
		repo   string
		want   string
	}{
		{"untracked", func(m *model) {}, "511/beta", untrackedMarker},
		{"tracked", func(m *model) {}, "511/alpha", trackedMarker},
		{"goal", func(m *model) { m.commitCounts["511/alpha"] = commitGoal }, "511/alpha", doneMarker},
		{"celebrating", func(m *model) {
			m.commitCounts["511/alpha"] = commitGoal
			m.celebrating["511/alpha"] = true
			m.celebratedUntil["511/alpha"] = now.Add(celebrateFor)
		}, "511/alpha", celebratingMarker},
	} {
		// The marker is the rescue cue when colour is unavailable, so each state
		// has to differ by glyph and not only by style.
		m := repositoriesModel(now)
		tc.adjust(&m)
		for _, row := range m.repositoryListRows() {
			if row.header || m.repositories[row.index] != tc.repo {
				continue
			}
			if line := ansi.Strip(m.repositoryRowLine(row, 50, false)); !strings.Contains(line, tc.want) {
				t.Fatalf("%s: expected marker %q in %q", tc.name, tc.want, line)
			}
			break
		}
	}
}

func TestCelebrationEndsOnItsOwnClock(t *testing.T) {
	now := time.Now()
	m := repositoriesModel(now)
	m.celebrating["511/alpha"] = true
	m.celebratedUntil["511/alpha"] = now.Add(celebrateFor)
	if !m.isCelebrating("511/alpha") {
		t.Fatal("expected a fresh celebration to be active")
	}
	// The cue has to expire without waiting for a message, because nothing else
	// clears it: the footer status that announced it expires earlier.
	m.now = now.Add(celebrateFor)
	if m.isCelebrating("511/alpha") {
		t.Fatal("expected the celebration to lapse once its window closed")
	}
}

func TestCommitCountsCelebrateTheGoalExactlyOnce(t *testing.T) {
	m := repositoriesModel(time.Now())
	updated, _ := m.Update(commitCountsMsg{counts: map[string]int{"511/alpha": commitGoal}, errors: map[string]error{}, repositories: []string{"511/alpha"}})
	m = updated.(model)
	if !m.isCelebrating("511/alpha") {
		t.Fatal("expected the first crossing of the goal to celebrate")
	}
	firstWindow := m.celebratedUntil["511/alpha"]

	updated, _ = m.Update(commitCountsMsg{counts: map[string]int{"511/alpha": commitGoal}, errors: map[string]error{}, repositories: []string{"511/alpha"}})
	m = updated.(model)
	if !m.celebrating["511/alpha"] || !m.celebratedUntil["511/alpha"].Equal(firstWindow) {
		t.Fatal("expected a repeated refresh at the goal not to restart the celebration")
	}
}

func TestDeadlineLineCarriesPhraseAndWeekTrack(t *testing.T) {
	deadline := time.Date(2026, time.November, 14, 9, 0, 0, 0, time.UTC)
	now := deadline.AddDate(0, 0, -4)
	m := repositoriesModel(now)
	line := ansi.Strip(m.deadlineLine(deadline, now))
	if !strings.Contains(line, "4 days left") {
		t.Fatalf("expected the calendar phrase in %q", line)
	}
	if got, want := strings.Count(line, deadlineTrackFill)+strings.Count(line, deadlineTrackEmpty), deadlineTrackCells; got != want {
		t.Fatalf("track = %d cells, want %d: %q", got, want, line)
	}
	// The track is drawn with distinct fill and empty glyphs, so the countdown
	// still reads without colour.
	if !strings.Contains(line, deadlineTrackFill) || !strings.Contains(line, deadlineTrackEmpty) {
		t.Fatalf("expected both track glyphs in %q", line)
	}

	overdue := repositoriesModel(deadline.AddDate(0, 0, 2))
	if line := ansi.Strip(overdue.deadlineLine(deadline, overdue.now)); strings.Contains(line, deadlineTrackFill) {
		t.Fatalf("expected a past deadline to show an empty track, got %q", line)
	}
}

func TestDeadlinePulseOnlyRunsInsideTheLastDay(t *testing.T) {
	deadline := time.Date(2026, time.November, 14, 9, 0, 0, 0, time.UTC)
	far := repositoriesModel(deadline.AddDate(0, 0, -4))
	far.deadlineOverride = "2026-11-14"
	if far.deadlinePulseActive() {
		t.Fatal("expected no pulse four days out")
	}
	last := repositoriesModel(deadline)
	last.deadlineOverride = "2026-11-14"
	if !last.deadlinePulseActive() {
		t.Fatal("expected a pulse on the deadline day")
	}
	// The cue is about a deadline that exists, so an empty tracking block has
	// nothing to breathe for.
	untracked := repositoriesModel(deadline)
	untracked.favorites = map[string]bool{}
	untracked.deadlineOverride = "2026-11-14"
	if untracked.deadlinePulseActive() {
		t.Fatal("expected no pulse without tracked repositories")
	}
}

func TestStalePulseTickIsIgnored(t *testing.T) {
	m := repositoriesModel(time.Now())
	m.pulseActive, m.pulseToken = true, 3
	updated, _ := m.Update(pulseTickMsg{token: 2})
	if got := updated.(model); got.pulsePhase {
		t.Fatal("expected a beat from a retired cue to be dropped")
	}
	updated, cmd := m.Update(pulseTickMsg{token: 3})
	m = updated.(model)
	if !m.pulsePhase || cmd == nil {
		t.Fatal("expected the current beat to toggle and re-arm")
	}
}

func TestSelectedRowBandSpansTheFullWidth(t *testing.T) {
	m := repositoriesModel(time.Now())
	var selected repositoryListRow
	for _, row := range m.repositoryListRows() {
		if !row.header {
			selected = row
			break
		}
	}
	plain := m.repositoryRowLine(selected, 40, false)
	banded := m.repositoryRowLine(selected, 40, true)
	// The band continues behind the padding, so the highlight is a solid strip
	// rather than a smear that stops after the text.
	if lipgloss.Width(plain) != 40 || lipgloss.Width(banded) != 40 {
		t.Fatalf("row widths = %d and %d, want 40", lipgloss.Width(plain), lipgloss.Width(banded))
	}
	if len(banded) <= len(plain) {
		t.Fatalf("expected the selected row to carry extra styling, got %q vs %q", banded, plain)
	}
	if strings.Contains(plain, "\x1b[48;") {
		t.Fatalf("expected an unselected row to have no background, got %q", plain)
	}
}

func TestWideLayoutSplitsOnlyWhenThereIsRoom(t *testing.T) {
	narrow := repositoriesModel(time.Now())
	narrow.width, narrow.height = 64, 16
	if narrow.wideLayout() {
		t.Fatal("expected 64 columns to stay single pane")
	}
	if strings.Contains(ansi.Strip(narrow.repositoriesView()), "Path") {
		t.Fatal("expected no details pane in the narrow layout")
	}

	wide := repositoriesModel(time.Now())
	wide.width, wide.height = 140, 30
	if !wide.wideLayout() {
		t.Fatal("expected a wide terminal to use the two pane layout")
	}
	// The details pane follows the cursor, so it has to describe the selected
	// repository once the cursor leaves the section headers.
	wide.cursor = 2
	view := ansi.Strip(wide.repositoriesView())
	// The details pane describes the selection without a network or subprocess
	// call, so it must be readable from model state alone.
	if !strings.Contains(view, "│") || !strings.Contains(view, "Path") || !strings.Contains(view, "511/alpha") {
		t.Fatalf("expected the details pane to describe the selection, got:\n%s", view)
	}
}

func TestHeaderBreadcrumbNamesTheWeek(t *testing.T) {
	now := time.Date(2026, time.November, 11, 12, 0, 0, 0, time.UTC)
	m := repositoriesModel(now)
	header := ansi.Strip(strings.Join(m.repositoriesHeaderLines(80), "\n"))
	if !strings.Contains(header, "week of Nov 9") {
		t.Fatalf("expected the breadcrumb to name the week, got:\n%s", header)
	}
	if !strings.Contains(header, "Priority") {
		t.Fatalf("expected the breadcrumb to name the tracking block, got:\n%s", header)
	}
}

func TestShimmerFillsTheProgressTrackWhileLoading(t *testing.T) {
	m := repositoriesModel(time.Now())
	m.loadingCommits = true
	m.commitCounts = map[string]int{}
	if got, want := lipgloss.Width(ansi.Strip(m.shimmer())), commitProgressWidth; got != want {
		t.Fatalf("shimmer width = %d, want %d", got, want)
	}
	// The known-count bar must be untouched by the loading cue, or the two
	// states would be indistinguishable at the same count.
	if got, want := lipgloss.Width(ansi.Strip(m.shimmer())), lipgloss.Width(ansi.Strip(commitProgressBar(0))); got != want {
		t.Fatalf("shimmer width %d differs from the real track %d", got, want)
	}
	for _, row := range m.repositoryListRows() {
		if row.repository != "alpha" {
			continue
		}
		if line := ansi.Strip(m.repositoryRowLine(row, 60, false)); !strings.Contains(line, "Loading") {
			t.Fatalf("expected the loading cue on a tracked row, got %q", line)
		}
	}
}
