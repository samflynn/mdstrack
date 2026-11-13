package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const applicationName = "mds"
const defaultGitHubHost = "github.ubc.ca"
const commitGoal = 3
const commitProgressWidth = 12
const defaultRefreshIntervalMinutes = 5

// Layout metrics. The interface is drawn borderless: a three column gutter, a
// one line header, one accent rule, the body, and a one line footer. Every size
// is derived from these plus the measured frame size of the styles involved.
const (
	layoutGutter   = 3
	layoutAccent   = 1
	layoutFooter   = 1
	layoutMinWidth = 24
	layoutMinBody  = 1
	blankLine      = " "
)

// chromeStyle frames the whole interface. It carries the gutter and nothing
// else, so layoutMetrics can measure padding rather than guessing at offsets.
var chromeStyle = lipgloss.NewStyle().Padding(0, 0, 0, layoutGutter)

const statusLifetime = 4 * time.Second

// minLegendWidth is the narrowest footer that still shows a useful legend
// alongside the refresh clock. Below it the clock is dropped instead of
// squeezing the bindings down to an ellipsis.
const minLegendWidth = 20

// Palette. Every colour is adaptive so the interface keeps a usable contrast
// ratio on both light and dark terminals, and so NO_COLOR=1 degrades to plain
// text rather than losing information (state is also carried by glyphs).
var (
	colorAccent  = lipgloss.AdaptiveColor{Light: "#5B5BD6", Dark: "#A78BFA"}
	colorBrand   = lipgloss.AdaptiveColor{Light: "#3D3DC7", Dark: "#C4B5FD"}
	colorMuted   = lipgloss.AdaptiveColor{Light: "#5C5C66", Dark: "#9CA0AA"}
	colorSubtle  = lipgloss.AdaptiveColor{Light: "#8A8A94", Dark: "#6B7280"}
	colorDanger  = lipgloss.AdaptiveColor{Light: "#B3271E", Dark: "#F87171"}
	colorSuccess = lipgloss.AdaptiveColor{Light: "#0F766E", Dark: "#5EEAD4"}
	colorPending = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colorTracked = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"}
)

type config struct {
	MDSFolder              string            `json:"mds_folder"`
	GitHubUsername         string            `json:"github_username"`
	GitHubProfileName      string            `json:"github_profile_name"`
	GitHubHost             string            `json:"github_host"`
	DeadlineOverride       string            `json:"deadline_override,omitempty"`
	RefreshIntervalMinutes int               `json:"refresh_interval_minutes"`
	Favorites              map[string]bool   `json:"favorites"`
	Collapsed              map[string]bool   `json:"collapsed"`
	FavoritesTitle         string            `json:"favorites_title"`
	GroupTitles            map[string]string `json:"group_titles"`
}

type mode int

const (
	setupMode mode = iota
	repositoriesMode
	settingsMode
	searchMode
)

type commitCountsMsg struct {
	counts       map[string]int
	errors       map[string]error
	repositories []string
}

type refreshTickMsg struct {
	at time.Time
}

// statusExpiryMsg fires once per transient status message so stale notifications
// clear themselves instead of lingering for the life of the session.
type statusExpiryMsg struct {
	at    time.Time
	token int
}

type workspaceRefreshMsg struct {
	repositories []string
	err          error
	refreshedAt  time.Time
}

type repositoryActionMsg struct {
	err error
}

type githubSearchResultsMsg struct {
	repositories []githubSearchRepository
	err          error
}

type clonedRepositoriesMsg struct {
	repositories map[string]bool
}

type cloneRepositoryMsg struct {
	repository string
	path       string
	err        error
}

type githubSearchRepository struct {
	FullName string `json:"full_name"`
}

type repositoryListRow struct {
	group      string
	rawGroup   string
	repository string
	index      int
	key        string
	header     bool
}

// keyMap describes the repository list bindings once, so the persistent footer
// and the ? overlay cannot drift apart.
type keyMap struct {
	Move     key.Binding
	Track    key.Binding
	Search   key.Binding
	Open     key.Binding
	GitHub   key.Binding
	Rename   key.Binding
	Settings key.Binding
	Help     key.Binding
	Quit     key.Binding
}

// repoKeys is the single source of truth for the list bindings, so the
// persistent footer and the ? overlay cannot drift apart. It is a package-level
// value because it holds no per-model state.
var repoKeys = keyMap{
	Move:     key.NewBinding(key.WithKeys("up", "down", "j", "k"), key.WithHelp("↑/↓", "move")),
	Track:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "track")),
	Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Open:     key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open")),
	GitHub:   key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "GitHub")),
	Rename:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "rename")),
	Settings: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "settings")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// ShortHelp drives the persistent footer. It is deliberately the essentials:
// the footer has one row shared with the refresh clock, so the always-visible
// legend stays short and ends with ? help for everything else.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Move, k.Track, k.Search, k.Open, k.GitHub, k.Help}
}

// FullHelp drives the ? overlay, grouped by intent.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Move, k.Track},
		{k.Search, k.Rename},
		{k.Open, k.GitHub},
		{k.Settings, k.Help, k.Quit},
	}
}

type model struct {
	mode                   mode
	path                   string
	githubUsername         string
	githubProfileName      string
	githubHost             string
	deadlineOverride       string
	refreshIntervalMinutes int
	lastRefresh            time.Time
	nextRefresh            time.Time
	setupStep              int
	settingsCursor         int
	editingSetting         string
	settingInput           string
	repositories           []string
	favorites              map[string]bool
	commitCounts           map[string]int
	commitErrors           map[string]error
	loadingCommits         bool
	collapsed              map[string]bool
	favoritesTitle         string
	groupTitles            map[string]string
	editingTitle           bool
	editingGroup           string
	titleInput             string
	githubActions          bool
	editorActions          bool
	searchChooser          bool
	localSearchActive      bool
	localSearch            string
	helpVisible            bool
	actionStatus           string
	githubSearch           string
	searchResults          []githubSearchRepository
	searchCursor           int
	searchLoading          bool
	searchError            error
	hideCloned             bool
	clonedRepositories     map[string]bool
	cloneConfirmation      string
	cloneFolderInput       string
	enteringCloneFolder    bool
	cloneStatus            string
	cursor                 int
	err                    error
	width                  int
	height                 int
	// now is the single clock reading the view layer is allowed to use, so
	// View() stays deterministic and the renderer never sees a diffing artefact.
	now time.Time
	// statusToken identifies the most recent transient status message. Expiry
	// messages carrying a stale token are ignored.
	statusToken int
	spinner     spinner.Model
	help        help.Model

	// Search, clone-folder, and title text entry share one implementation
	// (textValue) so editing, unicode handling, and horizontal scrolling all
	// behave identically and stay testable.
	localSearchInput  textValue
	githubSearchInput textValue
	folderNameInput   textValue
	titleNameInput    textValue
	settingValueInput textValue
	setupPathInput    textValue
	setupUserInput    textValue
	setupNameInput    textValue
	setupHostInput    textValue
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorBrand)
	accentStyle   = lipgloss.NewStyle().Foreground(colorAccent)
	mutedStyle    = lipgloss.NewStyle().Foreground(colorMuted)
	subtleStyle   = lipgloss.NewStyle().Foreground(colorSubtle)
	errorStyle    = lipgloss.NewStyle().Foreground(colorDanger)
	promptStyle   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(colorAccent)
	groupStyle    = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	favoriteStyle = lipgloss.NewStyle().Foreground(colorTracked).Italic(true).Bold(true)
	cursorStyle   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
)

// commitBarStyle renders the fixed-width commit bar. Bubbles' progress component
// owns the filled/empty glyphs so the track cannot drift out of the layout. Fill
// and empty colours are left at the component defaults and the surrounding style
// supplies the state colour, which keeps the bar legible under NO_COLOR.
var commitBarStyle = progress.New(
	progress.WithWidth(commitProgressWidth),
	progress.WithFillCharacters('█', '░'),
	progress.WithoutPercentage(),
)

func newModelSpinner() spinner.Model {
	s := spinner.New(spinner.WithSpinner(spinner.Line), spinner.WithStyle(accentStyle))
	s.Spinner.FPS = 100 * time.Millisecond
	return s
}

func newModelHelp() help.Model {
	h := help.New()
	h.Styles.ShortKey = accentStyle
	h.Styles.ShortDesc = mutedStyle
	h.Styles.ShortSeparator = subtleStyle
	h.Styles.FullKey = accentStyle
	h.Styles.FullDesc = mutedStyle
	h.Styles.FullSeparator = subtleStyle
	h.Styles.Ellipsis = subtleStyle
	return h
}

// helpModel returns the help renderer sized to the current content width.
// Models built directly by tests have a zero-value help field, which would
// render every binding jammed together, so fall back to the house styling.
// Deriving the width here keeps it correct on resize without a separate
// WindowSizeMsg bookkeeping step.
func (m model) helpModel(width int) help.Model {
	h := m.help
	if h.ShortSeparator == "" {
		h = newModelHelp()
	}
	h.Width = max(1, width)
	return h
}

// textValue is a single-line text field. The owning prompt holds the text and
// passes it in on every interaction, so a field written directly (by a test, a
// config load, or a reset) is adopted automatically instead of drifting out of
// step with its textinput.
//
// The cursor is drawn by render rather than textinput.View so the composed line
// carries no trailing padding from textinput, which would otherwise break the
// exact-width layout contract.
//
// Arrow keys are never forwarded here: textinput binds left/right to its own
// cursor and up/down to suggestions, but in this interface those keys must keep
// driving list navigation. Callers only pass editing keys.
type textValue struct {
	prompt string
	input  textinput.Model
}

func newTextValue(prompt string) textValue {
	return textValue{prompt: prompt}.ready()
}

// ready focuses the field and clears any styling the zero value carries.
// A model built as a literal (rather than through newModel) has an unfocused
// textinput, and an unfocused textinput ignores every key, so update and bind
// route through here to stay usable either way.
func (t textValue) ready() textValue {
	if !t.input.Focused() {
		// Rebuilt rather than focused in place: the zero value has a nil blink
		// context, and focusing it would dereference that. The owning field is
		// the source of truth, so bind restores the text immediately after.
		t.input = textinput.New()
		t.input.Prompt = ""
		t.input.TextStyle = lipgloss.NewStyle()
		t.input.Cursor.Style = promptStyle
		t.input.Focus()
	}
	return t
}

// bind syncs the textinput with the owning field's current text and leaves the
// caret at the end, where these prompts expect it.
func (t *textValue) bind(value string) {
	if t.input.Value() != value {
		t.input.SetValue(value)
	}
	t.input.CursorEnd()
}

// update applies one editing key and returns the resulting text.
func (t *textValue) update(msg tea.KeyMsg, value string) (string, tea.Cmd) {
	*t = t.ready()
	t.bind(value)
	updated, cmd := t.input.Update(msg)
	t.input = updated
	return t.input.Value(), cmd
}

// render draws prompt plus text plus a cursor block, trimmed to width cells and
// showing the tail of the text when it does not fit.
func (t textValue) render(width int, value string) string {
	t = t.ready()
	t.bind(value)
	inner := max(1, width-lipgloss.Width(t.prompt))
	// The cursor block occupies one cell, so the text gets the rest. Inputs show
	// their tail because the caret is at the end.
	text := max(1, inner-1)
	return promptStyle.Render(t.prompt + truncateLeft(t.input.Value(), text) + "█")
}

func main() {
	initial, err := newModel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if _, err := tea.NewProgram(initial, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newModel() (model, error) {
	stored, err := loadConfig()
	if errors.Is(err, os.ErrNotExist) {
		return model{mode: setupMode}, nil
	}
	if err != nil {
		return model{}, fmt.Errorf("load mds configuration: %w", err)
	}

	repositories, err := findRepositories(stored.MDSFolder)
	now := time.Now()
	refreshInterval := configuredRefreshInterval(stored.RefreshIntervalMinutes)
	m := model{
		mode: repositoriesMode, path: stored.MDSFolder, githubUsername: stored.GitHubUsername, githubProfileName: stored.GitHubProfileName, githubHost: configuredGitHubHost(stored.GitHubHost), deadlineOverride: stored.DeadlineOverride, refreshIntervalMinutes: refreshInterval, lastRefresh: now, nextRefresh: now.Add(time.Duration(refreshInterval) * time.Minute), now: now, repositories: repositories, favorites: stored.Favorites, commitCounts: make(map[string]int), commitErrors: make(map[string]error), loadingCommits: hasTrackedRepositories(stored.Favorites), collapsed: stored.Collapsed, favoritesTitle: stored.FavoritesTitle, groupTitles: stored.GroupTitles, err: err,
	}
	m.spinner, m.help = newModelSpinner(), newModelHelp()
	m.localSearchInput = newTextValue("/ ")
	m.githubSearchInput = newTextValue("/ ")
	m.folderNameInput = newTextValue("  ")
	m.titleNameInput = newTextValue("> ")
	m.settingValueInput = newTextValue("> ")
	m.setupPathInput = newTextValue("> ")
	m.setupUserInput = newTextValue("> ")
	m.setupNameInput = newTextValue("> ")
	m.setupHostInput = newTextValue("> ")
	return m, nil
}

func (m model) Init() tea.Cmd {
	if m.mode != repositoriesMode {
		return nil
	}
	return tea.Batch(loadTrackedCommitCounts(m.path, m.repositories, m.favorites, m.githubUsername, m.githubProfileName, m.githubHost), m.spinner.Tick, nextRefreshTick())
}

func (m *model) startCommitCountLoad(repositories []string) tea.Cmd {
	m.loadingCommits = len(repositories) > 0
	if !m.loadingCommits {
		return nil
	}
	return tea.Batch(loadTrackedCommitCounts(m.path, repositories, m.favorites, m.githubUsername, m.githubProfileName, m.githubHost), m.spinner.Tick)
}

func hasTrackedRepositories(favorites map[string]bool) bool {
	for _, tracked := range favorites {
		if tracked {
			return true
		}
	}
	return false
}

func nextRefreshTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return refreshTickMsg{at: time.Now()}
	})
}

// nextStatusExpiry schedules the clear of a transient status message. The token
// ties the timer to one specific message so a newer status is never wiped early.
func nextStatusExpiry(token int) tea.Cmd {
	return tea.Tick(statusLifetime, func(time.Time) tea.Msg {
		return statusExpiryMsg{at: time.Now(), token: token}
	})
}

func (m model) refreshWorkspace() tea.Cmd {
	return func() tea.Msg {
		repositories, err := findRepositories(m.path)
		return workspaceRefreshMsg{repositories: repositories, err: err, refreshedAt: time.Now()}
	}
}

// Update wraps update() to own the lifecycle of transient status messages in a
// single place. Whenever a status appears or is replaced it arms one expiry
// timer; the token makes a stale timer a no-op, so a newer status is never
// cleared early.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.update(msg)
	next, ok := updated.(model)
	if !ok {
		return updated, cmd
	}
	if _, resized := msg.(tea.WindowSizeMsg); resized {
		return next, cmd
	}
	newStatus := (next.actionStatus != "" && next.actionStatus != m.actionStatus) ||
		(next.cloneStatus != "" && next.cloneStatus != m.cloneStatus)
	if !newStatus {
		return next, cmd
	}
	next.statusToken++
	return next, tea.Batch(cmd, nextStatusExpiry(next.statusToken))
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.loadingCommits {
			return m, cmd
		}
		return m, nil
	case statusExpiryMsg:
		// Only the newest status owns the timer; older ones are discarded.
		if msg.token != m.statusToken {
			return m, nil
		}
		m.actionStatus, m.cloneStatus = "", ""
		return m, nil
	case commitCountsMsg:
		for _, repository := range msg.repositories {
			delete(m.commitCounts, repository)
			delete(m.commitErrors, repository)
		}
		for repository, count := range msg.counts {
			m.commitCounts[repository] = count
		}
		for repository, err := range msg.errors {
			m.commitErrors[repository] = err
		}
		m.loadingCommits = false
	case refreshTickMsg:
		m.now = msg.at
		if m.mode == repositoriesMode && !m.nextRefresh.After(msg.at) {
			return m, m.refreshWorkspace()
		}
		return m, nextRefreshTick()
	case workspaceRefreshMsg:
		m.lastRefresh = msg.refreshedAt
		m.nextRefresh = msg.refreshedAt.Add(time.Duration(m.refreshIntervalMinutes) * time.Minute)
		if msg.err != nil {
			m.err = fmt.Errorf("refresh repositories: %w", msg.err)
			return m, nextRefreshTick()
		}
		m.repositories, m.err = msg.repositories, nil
		return m, tea.Batch(loadTrackedCommitCounts(m.path, m.repositories, m.favorites, m.githubUsername, m.githubProfileName, m.githubHost), findClonedRepositories(m.path, m.repositories, m.githubHost), nextRefreshTick())
	case repositoryActionMsg:
		if msg.err != nil {
			m.actionStatus = "Action failed: " + msg.err.Error()
		}
	case githubSearchResultsMsg:
		m.searchResults, m.searchError, m.searchLoading = msg.repositories, msg.err, false
	case clonedRepositoriesMsg:
		m.clonedRepositories = msg.repositories
	case cloneRepositoryMsg:
		if msg.err != nil {
			m.cloneStatus = "Clone failed: " + msg.err.Error()
			break
		}
		m.cloneStatus = "Cloned " + msg.repository
		if m.clonedRepositories == nil {
			m.clonedRepositories = make(map[string]bool)
		}
		m.clonedRepositories[msg.repository] = true
		m.repositories, m.err = findRepositories(m.path)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || (msg.String() == "q" && m.mode != setupMode && !m.editingTitle && m.editingSetting == "") {
			return m, tea.Quit
		}
		if m.helpVisible {
			if msg.String() == "esc" || msg.String() == "?" {
				m.helpVisible = false
			}
			return m, nil
		}
		if m.mode == settingsMode {
			return m.updateSettings(msg)
		}
		if m.mode == searchMode {
			return m.updateSearch(msg)
		}
		if m.mode == repositoriesMode {
			if m.searchChooser {
				return m.updateSearchChooser(msg)
			}
			if m.editorActions {
				return m.updateEditorActions(msg)
			}
			if m.githubActions {
				return m.updateGitHubActions(msg)
			}
			if m.localSearchActive {
				switch msg.String() {
				case "esc":
					m.localSearch, m.localSearchActive, m.cursor = "", false, 0
					return m, nil
				case "backspace", "ctrl+h":
					next, cmd := m.localSearchInput.update(msg, m.localSearch)
					m.localSearch, m.cursor = next, 0
					return m, cmd
				}
				// Arrow keys fall through so they keep moving the list cursor.
				if isTextEditingKey(msg) {
					next, cmd := m.localSearchInput.update(msg, m.localSearch)
					m.localSearch, m.cursor = next, 0
					return m, cmd
				}
			}
			if m.editingTitle {
				switch msg.String() {
				case "enter":
					if m.editingGroup == "favorites" {
						m.favoritesTitle = strings.TrimSpace(m.titleInput)
					} else {
						if m.groupTitles == nil {
							m.groupTitles = make(map[string]string)
						}
						title := strings.TrimSpace(m.titleInput)
						if title == "" {
							delete(m.groupTitles, m.editingGroup)
						} else {
							m.groupTitles[m.editingGroup] = title
						}
					}
					m.editingGroup = ""
					m.editingTitle = false
					m.savePreferences()
				case "esc":
					m.editingGroup = ""
					m.editingTitle = false
				case "backspace", "ctrl+h":
					next, cmd := m.titleNameInput.update(msg, m.titleInput)
					m.titleInput = next
					return m, cmd
				default:
					if isTextEditingKey(msg) {
						next, cmd := m.titleNameInput.update(msg, m.titleInput)
						m.titleInput = next
						return m, cmd
					}
				}
				return m, nil
			}

			rows := m.repositoryListRows()
			switch msg.String() {
			case "?":
				m.helpVisible = true
			case "/":
				m.searchChooser, m.actionStatus = true, ""
			case "s":
				m.mode, m.err = settingsMode, nil
			case "up", "k":
				m.cursor = m.moveCursor(rows, -1)
			case "down", "j":
				m.cursor = m.moveCursor(rows, 1)
			case "e":
				if len(rows) > 0 && rows[m.cursor].header {
					row := rows[m.cursor]
					if row.key == "favorites" {
						m.editingGroup, m.editingTitle, m.titleInput = "favorites", true, m.favoritesTitle
					} else {
						m.editingGroup, m.editingTitle, m.titleInput = row.rawGroup, true, m.groupDisplayTitle(row.rawGroup)
					}
				}
			case "o":
				if _, ok := m.selectedRepository(rows); ok {
					m.editorActions, m.actionStatus = true, ""
				}
			case "g":
				if _, ok := m.selectedRepository(rows); ok {
					m.githubActions, m.actionStatus = true, ""
				}
			case "enter":
				if len(rows) > 0 {
					repository, tracked := m.activateRow(rows[m.cursor])
					if tracked {
						return m, m.startCommitCountLoad([]string{repository})
					}
					return m, nil
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "enter":
			return m.advanceSetup()
		case "backspace", "ctrl+h":
			next, cmd := m.setupFieldMsg(msg)
			return next, cmd
		default:
			if isTextEditingKey(msg) {
				next, cmd := m.setupFieldMsg(msg)
				return next, cmd
			}
		}
	}
	return m, nil
}

// setupFieldMsg routes an editing key into whichever setup step is active.
// Steps are numbered from 0 and each owns one text field, so the selection
// lives here rather than being repeated in the key handler and the view.
func (m model) setupFieldMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.setupInput(msg, m.setupValue())
	return m, nil
}

func (m model) setupValue() string {
	switch m.setupStep {
	case 0:
		return m.path
	case 1:
		return m.githubUsername
	case 2:
		return m.githubProfileName
	default:
		return m.githubHost
	}
}

func (m *model) setupInput(msg tea.KeyMsg, value string) {
	var next string
	switch m.setupStep {
	case 0:
		next, _ = m.setupPathInput.update(msg, value)
		m.path = next
	case 1:
		next, _ = m.setupUserInput.update(msg, value)
		m.githubUsername = next
	case 2:
		next, _ = m.setupNameInput.update(msg, value)
		m.githubProfileName = next
	default:
		next, _ = m.setupHostInput.update(msg, value)
		m.githubHost = next
	}
}

func (m model) advanceSetup() (tea.Model, tea.Cmd) {
	if m.setupStep == 0 {
		folder := strings.TrimSpace(m.path)
		if folder == "" {
			m.err = errors.New("enter the path to your UBC MDS folder")
			return m, nil
		}
		absoluteFolder, err := filepath.Abs(filepath.Clean(folder))
		if err != nil {
			m.err = fmt.Errorf("resolve folder: %w", err)
			return m, nil
		}
		info, err := os.Stat(absoluteFolder)
		if err != nil {
			m.err = fmt.Errorf("open folder: %w", err)
			return m, nil
		}
		if !info.IsDir() {
			m.err = errors.New("the MDS path must be a folder")
			return m, nil
		}
		m.path, m.setupStep, m.err = absoluteFolder, 1, nil
		return m, nil
	}

	if m.setupStep == 1 && strings.TrimSpace(m.githubUsername) == "" {
		m.err = errors.New("enter your GitHub username")
		return m, nil
	}
	if m.setupStep == 2 && strings.TrimSpace(m.githubProfileName) == "" {
		m.err = errors.New("enter your GitHub profile display name")
		return m, nil
	}
	if m.setupStep < 3 {
		m.setupStep++
		m.err = nil
		if m.setupStep == 3 && m.githubHost == "" {
			m.githubHost = defaultGitHubHost
		}
		return m, nil
	}
	folder := strings.TrimSpace(m.path)
	username := strings.TrimSpace(m.githubUsername)
	profileName := strings.TrimSpace(m.githubProfileName)
	host := normalizeGitHubHost(m.githubHost)
	if host == "" {
		m.err = errors.New("enter your GitHub host")
		return m, nil
	}
	if err := saveConfig(config{MDSFolder: folder, GitHubUsername: username, GitHubProfileName: profileName, GitHubHost: host, RefreshIntervalMinutes: defaultRefreshIntervalMinutes, Favorites: make(map[string]bool), Collapsed: make(map[string]bool)}); err != nil {
		m.err = fmt.Errorf("save configuration: %w", err)
		return m, nil
	}
	repositories, err := findRepositories(folder)
	now := time.Now()
	m.mode, m.path, m.githubUsername, m.githubProfileName, m.githubHost, m.refreshIntervalMinutes, m.lastRefresh, m.nextRefresh, m.repositories, m.favorites, m.collapsed, m.err = repositoriesMode, folder, username, profileName, host, defaultRefreshIntervalMinutes, now, now.Add(defaultRefreshIntervalMinutes*time.Minute), repositories, make(map[string]bool), make(map[string]bool), err
	return m, nil
}

func (m model) View() string {
	switch m.mode {
	case setupMode:
		return m.setupView()
	case settingsMode:
		return m.settingsView()
	case searchMode:
		return m.searchView()
	default:
		if m.helpVisible {
			return m.helpView()
		}
		return m.repositoriesView()
	}
}

func (m model) setupView() string {
	header := []string{titleStyle.Render("mds") + "  " + mutedStyle.Render("UBC MDS repository tracker")}
	body := []string{
		mutedStyle.Render("First-time setup"),
		blankLine,
		labelForSetupStep(m.setupStep),
		m.setupInputView(),
	}
	if m.err != nil {
		body = append(body, blankLine, errorStyle.Render("Error: "+m.err.Error()))
	}
	footer := []string{mutedStyle.Render("Enter continue · q quit")}
	return m.fullScreen(header, body, footer)
}

func (m model) setupInputView() string {
	return m.setupField().render(m.contentWidth(), m.setupValue())
}

// setupField is the text field owned by the active setup step.
func (m model) setupField() textValue {
	switch m.setupStep {
	case 1:
		return m.setupUserInput
	case 2:
		return m.setupNameInput
	case 3:
		return m.setupHostInput
	default:
		return m.setupPathInput
	}
}

func labelForSetupStep(step int) string {
	switch step {
	case 1:
		return "GitHub username:"
	case 2:
		return "GitHub profile display name:"
	case 3:
		return "GitHub host:"
	default:
		return "Location of UBC MDS folder:"
	}
}

func (m model) effectiveDeadline(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if m.deadlineOverride != "" {
		override, err := time.ParseInLocation("2006-01-02", m.deadlineOverride, now.Location())
		if err == nil && !override.Before(today) {
			return override
		}
	}
	return nextSaturday(today)
}

func nextSaturday(day time.Time) time.Time {
	return day.AddDate(0, 0, (int(time.Saturday)-int(day.Weekday())+7)%7)
}

func (m model) settingsView() string {
	width := m.contentWidth()
	header := []string{titleStyle.Render("mds") + "  " + mutedStyle.Render("Settings")}
	body := []string{}
	for index, setting := range settingsValues(m) {
		prefix := "  "
		style := mutedStyle
		if index == m.settingsCursor {
			prefix, style = "▸ ", cursorStyle
		}
		body = append(body, style.Render(truncate(prefix+setting.label+": "+setting.value, width)))
	}
	if m.editingSetting != "" {
		body = append(body, blankLine, m.settingValueInput.render(width, m.settingInput), mutedStyle.Render("Enter save · Esc cancel"))
	}
	if m.err != nil {
		body = append(body, blankLine, errorStyle.Render(truncate("Error: "+m.err.Error(), width)))
	}
	footer := []string{mutedStyle.Render("↑/↓ select · Enter edit · Esc back · q quit")}
	return m.fullScreen(header, body, footer)
}

func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.enteringCloneFolder {
		switch msg.String() {
		case "enter":
			folder, err := cloneFolderName(m.cloneFolderInput)
			if err != nil {
				m.cloneStatus = err.Error()
				return m, nil
			}
			repository := m.cloneConfirmation
			m.cloneConfirmation, m.cloneFolderInput, m.enteringCloneFolder, m.cloneStatus = "", "", false, ""
			return m, cloneGitHubRepository(m.path, m.githubHost, repository, folder)
		case "esc":
			m.cloneFolderInput, m.enteringCloneFolder = "", false
		default:
			if isTextEditingKey(msg) {
				next, cmd := m.folderNameInput.update(msg, m.cloneFolderInput)
				m.cloneFolderInput = next
				return m, cmd
			}
		}
		return m, nil
	}
	if m.cloneConfirmation != "" {
		switch msg.String() {
		case "y":
			repository := m.cloneConfirmation
			m.cloneConfirmation = ""
			return m, cloneGitHubRepository(m.path, m.githubHost, repository, "")
		case "f":
			m.enteringCloneFolder = true
		case "n", "esc":
			m.cloneConfirmation = ""
		}
		return m, nil
	}
	if m.searchLoading {
		if msg.String() == "esc" {
			m.mode = repositoriesMode
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.mode = repositoriesMode
	case "up", "k":
		m.searchCursor = max(0, m.searchCursor-1)
	case "down", "j":
		m.searchCursor = min(max(0, len(m.filteredSearchResults())-1), m.searchCursor+1)
	case "h":
		m.hideCloned = !m.hideCloned
		m.searchCursor = 0
	case "enter":
		results := m.filteredSearchResults()
		if m.searchCursor < len(results) {
			m.cloneConfirmation, m.cloneStatus = results[m.searchCursor].FullName, ""
		}
	case "backspace", "ctrl+h":
		next, cmd := m.githubSearchInput.update(msg, m.githubSearch)
		m.githubSearch, m.searchCursor = next, 0
		return m, cmd
	default:
		if isTextEditingKey(msg) {
			next, cmd := m.githubSearchInput.update(msg, m.githubSearch)
			m.githubSearch, m.searchCursor = next, 0
			return m, cmd
		}
	}
	return m, nil
}

func (m model) searchView() string {
	width := m.contentWidth()
	header := m.searchHeader()
	body := []string{m.githubSearchInput.render(width, m.githubSearch)}
	if m.searchLoading {
		body = append(body, blankLine, mutedStyle.Render("Loading accessible repositories..."))
	} else if m.searchError != nil {
		body = append(body, blankLine, errorStyle.Render(truncate("Unable to load repositories: "+m.searchError.Error(), width)))
	} else {
		results := m.filteredSearchResults()
		// The rows share the body with the filter line, so drop one for it.
		visible := max(1, m.visibleRows()-1)
		start := max(0, min(m.searchCursor, max(0, len(results)-visible)))
		end := min(len(results), start+visible)
		for index, repository := range results[start:end] {
			prefix := "  "
			style := mutedStyle
			if start+index == m.searchCursor {
				prefix, style = "▸ ", cursorStyle
			}
			body = append(body, style.Render(prefix+truncate(repository.FullName, width-2)))
		}
		if len(results) == 0 {
			body = append(body, blankLine, mutedStyle.Render("No accessible repositories match your search."))
		}
	}
	return m.fullScreen(header, body, []string{mutedStyle.Render(m.searchFooter())})
}

func (m model) searchFooter() string {
	hideClonedLabel := "h hide cloned"
	if m.hideCloned {
		hideClonedLabel = "h show cloned"
	}
	footer := "Type to filter · ↑/↓ move · Enter clone · " + hideClonedLabel + " · Esc back · q quit"
	if m.cloneConfirmation != "" {
		if m.enteringCloneFolder {
			footer = "New folder: " + m.cloneFolderInput + "█ · Enter create & clone · Esc back"
			if m.cloneStatus != "" {
				footer = m.cloneStatus + " · " + footer
			}
		} else {
			footer = "Clone " + m.cloneConfirmation + "? y workspace · f new folder · n no"
		}
	} else if m.cloneStatus != "" {
		footer = m.cloneStatus
	}
	return footer
}

func (m model) filteredSearchResults() []githubSearchRepository {
	query := strings.ToLower(strings.TrimSpace(m.githubSearch))
	results := make([]githubSearchRepository, 0, len(m.searchResults))
	for _, repository := range m.searchResults {
		if m.hideCloned && m.clonedRepositories[repository.FullName] {
			continue
		}
		if strings.Contains(strings.ToLower(repository.FullName), query) {
			results = append(results, repository)
		}
	}
	return results
}

type settingValue struct {
	key   string
	label string
	value string
}

func settingsValues(m model) []settingValue {
	return []settingValue{
		{key: "workspace", label: "Workspace", value: m.path},
		{key: "github", label: "GitHub username", value: m.githubUsername},
		{key: "profile-name", label: "GitHub profile display name", value: m.githubProfileName},
		{key: "host", label: "GitHub host", value: m.githubHost},
		{key: "deadline", label: "Deadline", value: m.deadlineDisplayValue(time.Now())},
		{key: "refresh-interval", label: "Refresh interval", value: fmt.Sprintf("%d minutes", m.refreshIntervalMinutes)},
	}
}

func (m model) deadlineDisplayValue(now time.Time) string {
	value := m.effectiveDeadline(now).Format("2006-01-02")
	if m.deadlineOverride != "" {
		override, err := time.ParseInLocation("2006-01-02", m.deadlineOverride, now.Location())
		if err == nil && !override.Before(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())) {
			return value + " (override)"
		}
	}
	return value + " (Saturday default)"
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editingSetting != "" {
		switch msg.String() {
		case "enter":
			return m.saveSetting()
		case "esc":
			m.editingSetting, m.settingInput = "", ""
		case "backspace", "ctrl+h":
			next, cmd := m.settingValueInput.update(msg, m.settingInput)
			m.settingInput = next
			return m, cmd
		default:
			if isTextEditingKey(msg) {
				next, cmd := m.settingValueInput.update(msg, m.settingInput)
				m.settingInput = next
				return m, cmd
			}
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.mode, m.err = repositoriesMode, nil
	case "up", "k":
		m.settingsCursor = max(0, m.settingsCursor-1)
	case "down", "j":
		m.settingsCursor = min(len(settingsValues(m))-1, m.settingsCursor+1)
	case "enter":
		setting := settingsValues(m)[m.settingsCursor]
		m.editingSetting, m.settingInput = setting.key, setting.value
		if setting.key == "deadline" {
			m.settingInput = m.deadlineOverride
		} else if setting.key == "refresh-interval" {
			m.settingInput = fmt.Sprintf("%d", m.refreshIntervalMinutes)
		}
	}
	return m, nil
}

func (m model) saveSetting() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.settingInput)
	if value == "" && m.editingSetting != "deadline" {
		m.err = errors.New("a setting cannot be empty")
		return m, nil
	}
	if m.editingSetting == "workspace" {
		folder, err := filepath.Abs(filepath.Clean(value))
		if err != nil {
			m.err = fmt.Errorf("resolve folder: %w", err)
			return m, nil
		}
		info, err := os.Stat(folder)
		if err != nil {
			m.err = fmt.Errorf("open folder: %w", err)
			return m, nil
		}
		if !info.IsDir() {
			m.err = errors.New("the workspace path must be a folder")
			return m, nil
		}
		repositories, err := findRepositories(folder)
		if err != nil {
			m.err = fmt.Errorf("scan repositories: %w", err)
			return m, nil
		}
		m.path, m.repositories = folder, repositories
	} else if m.editingSetting == "github" {
		m.githubUsername = value
	} else if m.editingSetting == "profile-name" {
		m.githubProfileName = value
	} else if m.editingSetting == "deadline" {
		if value != "" {
			deadline, err := time.ParseInLocation("2006-01-02", value, time.Local)
			if err != nil {
				m.err = errors.New("deadline must use YYYY-MM-DD")
				return m, nil
			}
			m.deadlineOverride = deadline.Format("2006-01-02")
		} else {
			m.deadlineOverride = ""
		}
	} else if m.editingSetting == "refresh-interval" {
		interval, err := parseRefreshInterval(value)
		if err != nil {
			m.err = err
			return m, nil
		}
		m.refreshIntervalMinutes = interval
		m.nextRefresh = time.Now().Add(time.Duration(interval) * time.Minute)
	} else {
		host := normalizeGitHubHost(value)
		if host == "" {
			m.err = errors.New("GitHub host cannot be empty")
			return m, nil
		}
		m.githubHost = host
	}
	m.editingSetting, m.settingInput, m.err = "", "", nil
	m.savePreferences()
	return m, nil
}

func (m model) updateSearchChooser(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.searchChooser = false
	case "l":
		m.searchChooser, m.localSearchActive, m.localSearch, m.cursor = false, true, "", 0
	case "g":
		m.searchChooser = false
		m.mode, m.githubSearch, m.searchCursor, m.searchError = searchMode, "", 0, nil
		m.hideCloned, m.searchLoading = true, true
		return m, tea.Batch(loadGitHubRepositories(m.githubHost), findClonedRepositories(m.path, m.repositories, m.githubHost))
	}
	return m, nil
}

func (m model) updateEditorActions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.editorActions = false
		return m, nil
	}
	repository, ok := m.selectedRepository(m.repositoryListRows())
	if !ok {
		m.editorActions = false
		return m, nil
	}
	path := filepath.Join(m.path, repository)
	if msg.String() == "f" {
		m.editorActions = false
		return m, runRepositoryAction("open", path)
	}
	applications := map[string]string{"v": "Visual Studio Code", "p": "Positron"}
	application, ok := applications[msg.String()]
	if !ok {
		return m, nil
	}
	m.editorActions = false
	return m, openInApplication(application, path)
}

func (m model) updateGitHubActions(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.githubActions = false
		return m, nil
	}
	repository, ok := m.selectedRepository(m.repositoryListRows())
	if !ok {
		m.githubActions = false
		return m, nil
	}
	url, err := m.githubURL(repository)
	if err != nil {
		m.actionStatus = "GitHub action unavailable: " + err.Error()
		return m, nil
	}
	switch msg.String() {
	case "o":
		m.githubActions = false
		return m, openURL(url)
	case "c":
		m.githubActions = false
		m.actionStatus = "Copied GitHub URL"
		return m, copyToClipboard(url)
	}
	return m, nil
}

func (m model) selectedRepository(rows []repositoryListRow) (string, bool) {
	if m.cursor < 0 || m.cursor >= len(rows) || rows[m.cursor].header || rows[m.cursor].key == "divider" {
		return "", false
	}
	return m.repositories[rows[m.cursor].index], true
}

func (m model) githubURL(repository string) (string, error) {
	remote, err := gitOriginURL(filepath.Join(m.path, repository))
	if err != nil {
		return "", err
	}
	host, owner, name, ok := githubRepository(remote)
	if !ok {
		return "", errors.New("origin is not hosted on GitHub")
	}
	return fmt.Sprintf("https://%s/%s/%s", host, owner, name), nil
}

func openInApplication(application, path string) tea.Cmd {
	return func() tea.Msg {
		return repositoryActionMsg{err: exec.Command("open", "-a", application, path).Run()}
	}
}

func openURL(url string) tea.Cmd {
	return runRepositoryAction("open", url)
}

func copyToClipboard(value string) tea.Cmd {
	return func() tea.Msg {
		command := exec.Command("pbcopy")
		command.Stdin = strings.NewReader(value)
		return repositoryActionMsg{err: command.Run()}
	}
}

func runRepositoryAction(command string, argument string) tea.Cmd {
	return func() tea.Msg {
		return repositoryActionMsg{err: exec.Command(command, argument).Run()}
	}
}

func (m model) helpView() string {
	header := []string{titleStyle.Render("mds") + "  " + mutedStyle.Render("Keyboard shortcuts")}
	body := []string{m.helpModel(m.contentWidth()).FullHelpView(repoKeys.FullHelp())}
	footer := []string{mutedStyle.Render("? or Esc close · q quit")}
	return m.fullScreen(header, body, footer)
}

// repositoriesHeader is the two line list header. headerHeight() derives its
// height from this, so the body can be sized before the header is rendered.
func (m model) repositoriesHeader() []string {
	return m.repositoriesHeaderLines(m.contentWidth())
}

// repositoriesHeaderLines takes its width so headerHeight can measure the row
// count without recursing back through layoutMetrics.
func (m model) repositoriesHeaderLines(width int) []string {
	return []string{
		titleStyle.Render("mds") + "  " + mutedStyle.Render("UBC MDS repositories"),
		mutedStyle.Render(truncate(m.path, width)),
	}
}

// searchHeader is the two line search header.
func (m model) searchHeader() []string {
	return []string{
		titleStyle.Render("mds") + "  " + mutedStyle.Render("GitHub repository search"),
		mutedStyle.Render("Accessible repositories on " + m.githubHost),
	}
}

// titleField labels the rename prompt according to what is being renamed.
func (m model) titleField() textValue {
	if m.editingGroup == "favorites" {
		return m.titleNameInput
	}
	field := m.titleNameInput
	field.prompt = "Rename group: "
	return field
}

func (m model) repositoriesView() string {
	width := m.contentWidth()
	header := m.repositoriesHeader()
	body := []string{}
	if m.err != nil {
		body = append(body, errorStyle.Render(truncate("Unable to scan repositories: "+m.err.Error(), width)))
	} else if len(m.repositories) == 0 {
		body = append(body, mutedStyle.Render("No Git repositories found in this folder."))
	} else {
		body = append(body, m.repositoryRowsView(width)...)
	}
	if m.editingTitle {
		body = append(body, blankLine, m.titleField().render(width, m.titleInput), mutedStyle.Render("Enter save · Esc cancel · empty restores the folder name"))
	}
	if repository, err := m.firstCommitError(); err != nil {
		body = append(body, blankLine, errorStyle.Render(truncate(fmt.Sprintf("gh error (%s): %v", repository, err), width)))
	}
	return m.fullScreen(header, body, []string{mutedStyle.Render(truncate(m.repositoryFooter(width), width))})
}

// repositoryRowsView renders the scrolling list window. The visible count comes
// from the body height so the cursor always has a row to sit on, and the
// overflow indicator is only reserved when the list is actually clipped.
func (m model) repositoryRowsView(width int) []string {
	rows := m.repositoryListRows()
	start, end := m.repositoryWindow(rows)
	overflow := start > 0 || end < len(rows)
	visible := m.repositoryVisibleRows(overflow)
	start = max(0, min(m.cursor, max(0, len(rows)-visible)))
	end = min(len(rows), start+visible)
	if overflow {
		// Re-check against the window we actually settled on.
		overflow = start > 0 || end < len(rows)
	}

	lines := make([]string, 0, visible+1)
	for index, row := range rows[start:end] {
		selected := start+index == m.cursor
		if row.key == "divider" {
			lines = append(lines, mutedStyle.Render(strings.Repeat("─", width)))
			continue
		}
		indent := ""
		if strings.HasPrefix(row.key, "favorite:") {
			indent = "  "
		}
		if row.header {
			prefix := "⌄ "
			if m.collapsed[row.key] {
				prefix = "› "
			}
			label := indent + prefix + truncate(strings.ToUpper(row.group), width-len(indent)-3)
			switch {
			case selected:
				lines = append(lines, cursorStyle.Render("▸ "+label))
			case row.key == "favorites":
				lines = append(lines, favoriteStyle.Render("★ "+label))
			default:
				lines = append(lines, groupStyle.Render(label))
			}
			continue
		}
		lines = append(lines, m.repositoryRowView(row, width, selected, indent))
	}
	if overflow {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("%d–%d of %d", start+1, end, len(rows))))
	}
	return lines
}

// repositoryRowView renders one repository line: the tracking marker and name
// on the left, the deadline and commit progress right-aligned.
func (m model) repositoryRowView(row repositoryListRow, width int, selected bool, indent string) string {
	repository := m.repositories[row.index]
	marker := "[ ]"
	if m.favorites[repository] {
		marker = "[X]"
	}

	countText, deadlineText := "", ""
	nameStyle := lipgloss.NewStyle()
	if m.favorites[repository] {
		deadlineText = deadlineStatus(m.effectiveDeadline(m.clock()), m.clock())
		switch {
		case m.commitErrors[repository] != nil:
			countText, nameStyle = "GH ERROR", errorStyle
		case m.loadingCommits && m.commitCounts[repository] == 0:
			countText = m.spinner.View() + " Loading"
		default:
			if count, ok := m.commitCounts[repository]; ok {
				countText, nameStyle = commitProgress(count), commitProgressStyle(count)
			}
		}
	}

	prefix := indent + marker + " "
	if selected {
		prefix = "▸ " + prefix
	}
	rightText := deadlineText
	if countText != "" {
		if rightText != "" {
			rightText += "  "
		}
		rightText += countText
	}
	available := width - lipgloss.Width(prefix) - lipgloss.Width(rightText)
	name := truncate(row.repository, max(1, available-1))
	prefixText := selectedStyle.Render(prefix)
	if selected {
		prefixText = cursorStyle.Render(prefix)
	}
	rowText := prefixText + nameStyle.Render(name)
	if rightText != "" {
		rowText += strings.Repeat(" ", max(1, width-lipgloss.Width(rowText)-lipgloss.Width(rightText))) + rightText
	}
	return rowText
}

// repositoryFooter prefers the most specific active mode, and only falls back to
// the key legend plus refresh clock when nothing else is competing for the row.
func (m model) repositoryFooter(width int) string {
	switch {
	case m.localSearchActive:
		return "/ " + m.localSearch + "█ · ↑/↓ move · Enter track · Esc clear search"
	case m.searchChooser:
		return "Search · l local workspace · g GitHub repositories · Esc cancel"
	case m.editorActions:
		return "Open · v VS Code · p Positron · f Finder · Esc cancel"
	case m.githubActions:
		return "GitHub actions · o open URL · c copy URL · Esc cancel"
	case m.actionStatus != "":
		return m.actionStatus
	case m.cloneStatus != "":
		return m.cloneStatus
	}

	status := m.refreshStatus(m.clock())
	statusWidth := lipgloss.Width(status)
	legendWidth := width - statusWidth - 2
	if legendWidth < minLegendWidth {
		// Too narrow for both. The bindings are actionable and the countdown is
		// decorative, so keep the legend and drop the clock.
		return m.helpModel(width).ShortHelpView(repoKeys.ShortHelp())
	}
	// The legend is built to the width it actually gets, so Bubbles elides it
	// itself rather than rendering past the row and being cut mid-word.
	legend := m.helpModel(legendWidth).ShortHelpView(repoKeys.ShortHelp())
	return legend + strings.Repeat(" ", max(1, width-lipgloss.Width(legend)-statusWidth)) + status
}

func (m model) firstCommitError() (string, error) {
	for _, repository := range m.repositories {
		if err := m.commitErrors[repository]; err != nil {
			return repository, err
		}
	}
	return "", nil
}

func (m model) favoritesDisplayTitle() string {
	if m.favoritesTitle == "" {
		return "TRACKING"
	}
	return strings.ToUpper(m.favoritesTitle) + " (TRACKING)"
}

func (m model) groupDisplayTitle(group string) string {
	if title := strings.TrimSpace(m.groupTitles[group]); title != "" {
		return title
	}
	return group
}

func (m *model) activateRow(row repositoryListRow) (string, bool) {
	if row.key == "divider" {
		return "", false
	}
	if row.header {
		if m.collapsed == nil {
			m.collapsed = make(map[string]bool)
		}
		m.collapsed[row.key] = !m.collapsed[row.key]
		m.savePreferences()
		return "", false
	}
	if m.favorites == nil {
		m.favorites = make(map[string]bool)
	}
	nextRepository := ""
	for _, next := range m.repositoryListRows()[m.cursor+1:] {
		if !next.header {
			nextRepository = m.repositories[next.index]
			break
		}
	}
	repository := m.repositories[row.index]
	m.favorites[repository] = !m.favorites[repository]
	tracked := m.favorites[repository]
	if !tracked {
		delete(m.commitCounts, repository)
		delete(m.commitErrors, repository)
	}
	m.savePreferences()
	if nextRepository != "" {
		m.cursor = m.rowCursorForRepository(nextRepository)
	} else {
		m.cursor = min(m.cursor, max(0, len(m.repositoryListRows())-1))
	}
	return repository, tracked
}

func (m *model) savePreferences() {
	if err := saveConfig(config{MDSFolder: m.path, GitHubUsername: m.githubUsername, GitHubProfileName: m.githubProfileName, GitHubHost: m.githubHost, DeadlineOverride: m.deadlineOverride, RefreshIntervalMinutes: m.refreshIntervalMinutes, Favorites: m.favorites, Collapsed: m.collapsed, FavoritesTitle: m.favoritesTitle, GroupTitles: m.groupTitles}); err != nil {
		m.err = fmt.Errorf("save preferences: %w", err)
	}
}

func (m model) rowCursorForRepository(repository string) int {
	for cursor, row := range m.repositoryListRows() {
		if !row.header && m.repositories[row.index] == repository {
			return cursor
		}
	}
	return 0
}

func (m model) repositoryListRows() []repositoryListRow {
	favorites, regular := []int{}, []int{}
	for index, repository := range m.repositories {
		if m.localSearch != "" && !strings.Contains(strings.ToLower(repository), strings.ToLower(m.localSearch)) {
			continue
		}
		if m.favorites[repository] {
			favorites = append(favorites, index)
		} else {
			regular = append(regular, index)
		}
	}
	rows := []repositoryListRow{}
	if len(favorites) > 0 {
		rows = append(rows, repositoryListRow{group: m.favoritesDisplayTitle(), key: "favorites", header: true})
		if !m.collapsed["favorites"] {
			rows = appendGroupedRepositoryRows(rows, m.repositories, favorites, "favorite:", m.collapsed, m.groupTitles)
		}
		if len(regular) > 0 {
			rows = append(rows, repositoryListRow{key: "divider"})
		}
	}
	return appendGroupedRepositoryRows(rows, m.repositories, regular, "group:", m.collapsed, m.groupTitles)
}

func appendGroupedRepositoryRows(rows []repositoryListRow, repositories []string, indexes []int, prefix string, collapsed map[string]bool, groupTitles map[string]string) []repositoryListRow {
	previousGroup := ""
	for _, index := range indexes {
		group, name := repositoryGroup(repositories[index])
		key := prefix + group
		if group != previousGroup {
			displayGroup := group
			if title := strings.TrimSpace(groupTitles[group]); title != "" {
				displayGroup = title
			}
			rows = append(rows, repositoryListRow{group: displayGroup, rawGroup: group, key: key, header: true})
			previousGroup = group
		}
		if !collapsed[key] {
			rows = append(rows, repositoryListRow{repository: name, index: index, key: prefix + "repository"})
		}
	}
	return rows
}

func (m model) moveCursor(rows []repositoryListRow, direction int) int {
	cursor := m.cursor
	for candidate := cursor + direction; candidate >= 0 && candidate < len(rows); candidate += direction {
		if rows[candidate].key != "divider" {
			return candidate
		}
	}
	return cursor
}

// repositoryVisibleRows is the number of repository rows that fit in the body.
// The overflow indicator steals a row, so it has to be accounted for while
// scrolling rather than being appended after the fact (which would push the
// footer off the screen).
func (m model) repositoryVisibleRows(overflow bool) int {
	rows := m.visibleRows()
	if overflow {
		rows--
	}
	return max(1, rows)
}

func (m model) repositoryWindow(rows []repositoryListRow) (int, int) {
	visible := max(1, m.visibleRows())
	cursorRow := min(m.cursor, max(0, len(rows)-1))
	start := max(0, cursorRow-visible+1)
	end := min(len(rows), start+visible)
	if end-start < visible {
		start = max(0, end-visible)
	}
	return start, end
}

func repositoryGroup(repository string) (string, string) {
	parts := strings.Split(filepath.ToSlash(repository), "/")
	if len(parts) == 1 {
		return "ROOT", parts[0]
	}
	return parts[0], strings.Join(parts[1:], "/")
}

// layoutMetrics derives every size from the measured frame of the styles that
// actually draw, rather than from hand-counted offsets. width/height are the
// full terminal; contentWidth is what a line may occupy after the gutter.
func (m model) layoutMetrics() (width, height, contentWidth, bodyHeight int) {
	width, height = m.viewportWidth(), m.viewportHeight()
	contentWidth = max(1, width-chromeStyle.GetHorizontalFrameSize())
	bodyHeight = max(layoutMinBody, height-m.chromeHeight())
	return width, height, contentWidth, bodyHeight
}

// chromeHeight counts the rows the interface draws outside the body: the
// header, the accent rule under it, the footer, and any vertical padding the
// chrome style adds. Keeping this in one place is what guarantees View() emits
// exactly viewportHeight rows.
func (m model) chromeHeight() int {
	return chromeStyle.GetVerticalFrameSize() + m.headerHeight() + layoutAccent + layoutFooter
}

// headerHeight is the row count headerLines() would produce, so the body can be
// sized before the header is assembled.
func (m model) headerHeight() int {
	switch m.mode {
	case repositoriesMode:
		return len(m.repositoriesHeaderLines(1))
	case searchMode:
		return len(m.searchHeader())
	default:
		return 1
	}
}

// clock is the single time source for the view layer. In production the tick
// loop keeps m.now current so View() stays deterministic; models built by hand
// (tests) have a zero now and fall back to the wall clock.
func (m model) clock() time.Time {
	if m.now.IsZero() {
		return time.Now()
	}
	return m.now
}

// visibleRows is the number of list rows the body can show on the current
// screen. Every scroll window is derived from this so the cursor cannot walk
// off the bottom of a short terminal.
func (m model) visibleRows() int {
	return max(1, m.contentHeight())
}

// fullScreen composes header, body and footer into exactly viewportHeight lines
// of at most viewportWidth cells. Each block is padded to its exact height, so
// the footer always sits on the last row and the body absorbs the slack.
func (m model) fullScreen(header, body, footer []string) string {
	_, height, contentWidth, bodyHeight := m.layoutMetrics()
	// Reserve the declared header height so every caller produces exactly
	// height rows; a header shorter or taller than headerHeight() would
	// otherwise silently push the footer off screen.
	headerHeight := max(len(header), m.headerHeight())

	composed := lipgloss.JoinVertical(lipgloss.Left,
		block(header, contentWidth, headerHeight),
		accentStyle.Render(strings.Repeat("─", contentWidth)),
		block(body, contentWidth, bodyHeight),
		block(footer, contentWidth, layoutFooter),
	)

	composed = lipgloss.NewStyle().MaxWidth(contentWidth).MaxHeight(height).Render(composed)
	return chromeStyle.MaxHeight(height).Render(composed)
}

// block renders lines as a fixed-width, fixed-height region. Lines past height
// are dropped from the bottom and short blocks are padded with blanks, so the
// frame geometry never depends on how much content a screen happens to have.
func block(lines []string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	blank := strings.Repeat(" ", width)
	padded := make([]string, 0, height)
	for _, line := range lines {
		padded = append(padded, lipgloss.NewStyle().Width(width).MaxWidth(width).Render(truncate(line, width)))
	}
	for len(padded) < height {
		padded = append(padded, blank)
	}
	return strings.Join(padded, "\n")
}

func (m model) viewportWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

func (m model) viewportHeight() int {
	if m.height > 0 {
		return m.height
	}
	return 24
}

// contentWidth is the usable width of a body line, excluding the gutter.
func (m model) contentWidth() int {
	_, _, contentWidth, _ := m.layoutMetrics()
	return contentWidth
}

// contentHeight is the usable height of the body region.
func (m model) contentHeight() int {
	_, _, _, bodyHeight := m.layoutMetrics()
	return bodyHeight
}

// truncate fits value into width cells, accounting for wide characters, emoji
// sequences and pre-existing ANSI escapes.
func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(value, width, "…")
}

// truncateLeft keeps the tail of value within width cells, for paths and inputs
// where the end is the interesting part.
func truncateLeft(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= width {
		return value
	}
	return ansi.TruncateLeft(value, ansi.StringWidth(value)-width+1, "…")
}

func deadlineStatus(deadline, now time.Time) string {
	deadlineDay := time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 0, 0, 0, 0, deadline.Location())
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := int(deadlineDay.Sub(nowDay).Hours() / 24)
	switch {
	case days == 0:
		return promptStyle.Render("due today")
	case days == 1:
		return promptStyle.Render("1 day left")
	case days > 1:
		return promptStyle.Render(fmt.Sprintf("%d days left", days))
	case days == -1:
		return errorStyle.Render("1 day overdue")
	default:
		return errorStyle.Render(fmt.Sprintf("%d days overdue", -days))
	}
}

// commitProgressRatio maps a commit count onto 0..1 for the bar. The goal is a
// floor, not a ceiling, so anything at or above it fills the track completely.
func commitProgressRatio(count int) float64 {
	return float64(min(max(count, 0), commitGoal)) / float64(commitGoal)
}

// commitProgress renders the tracking progress. The count text always follows
// the bar, and the bar's fill is what actually encodes progress, so the state
// survives NO_COLOR and colour-vision differences.
func commitProgress(count int) string {
	style := commitProgressStyle(count)
	return style.Render(commitProgressBar(count)) + " " + style.Render(fmt.Sprintf("%d/%d", count, commitGoal))
}

// commitProgressBar renders the fixed-width track through Bubbles' progress
// component. ViewAs is deliberately used instead of View: it renders a
// percentage without mutating the component, which keeps View() a pure function
// of the model.
func commitProgressBar(count int) string {
	return commitBarStyle.ViewAs(commitProgressRatio(count))
}

// commitProgressStyle colours the bar by how far along the week is. It uses one
// hue per state rather than a red/amber/green traffic light: the fill glyphs
// already carry the magnitude, so colour only has to reinforce it, and the
// display stays readable for colourblind users and under NO_COLOR.
func commitProgressStyle(count int) lipgloss.Style {
	switch {
	case count >= commitGoal:
		return lipgloss.NewStyle().Foreground(colorSuccess)
	case count >= commitGoal/2:
		return lipgloss.NewStyle().Foreground(colorPending)
	default:
		return lipgloss.NewStyle().Foreground(colorTracked)
	}
}

func findRepositories(root string) ([]string, error) {
	var repositories []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() || entry.Name() != ".git" {
			return nil
		}
		repository := filepath.Dir(path)
		relative, err := filepath.Rel(root, repository)
		if err != nil {
			return err
		}
		repositories = append(repositories, relative)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(repositories)
	return repositories, nil
}

func loadTrackedCommitCounts(workspace string, repositories []string, favorites map[string]bool, username, profileName, configuredHost string) tea.Cmd {
	return func() tea.Msg {
		counts := make(map[string]int)
		failures := make(map[string]error)
		if username == "" {
			return commitCountsMsg{counts: counts, errors: failures, repositories: repositories}
		}
		for _, repository := range repositories {
			if !favorites[repository] {
				continue
			}
			fullPath := filepath.Join(workspace, repository)
			remote, err := gitOriginURL(fullPath)
			if err != nil {
				failures[repository] = err
				continue
			}
			host, owner, name, ok := githubRepository(remote)
			if !ok {
				failures[repository] = errors.New("origin is not hosted on GitHub")
				continue
			}
			if !strings.EqualFold(host, configuredHost) {
				failures[repository] = fmt.Errorf("origin host %q does not match configured GitHub host %q", host, configuredHost)
				continue
			}
			count, err := githubCommitCount(configuredHost, owner, name, username, profileName)
			if err != nil {
				failures[repository] = err
				continue
			}
			counts[repository] = count
		}
		return commitCountsMsg{counts: counts, errors: failures, repositories: repositories}
	}
}

func loadGitHubRepositories(host string) tea.Cmd {
	return func() tea.Msg {
		output, err := exec.Command("gh", "api", "--hostname", host, "--paginate", "--slurp", "user/repos?affiliation=owner,collaborator,organization_member&per_page=100&sort=full_name").CombinedOutput()
		if err != nil {
			return githubSearchResultsMsg{err: fmt.Errorf("query GitHub repositories: %s", strings.TrimSpace(string(output)))}
		}
		var pages [][]githubSearchRepository
		if err := json.Unmarshal(output, &pages); err != nil {
			return githubSearchResultsMsg{err: fmt.Errorf("decode GitHub repositories: %w", err)}
		}
		var repositories []githubSearchRepository
		for _, page := range pages {
			repositories = append(repositories, page...)
		}
		sort.Slice(repositories, func(left, right int) bool {
			return strings.ToLower(repositories[left].FullName) < strings.ToLower(repositories[right].FullName)
		})
		return githubSearchResultsMsg{repositories: repositories}
	}
}

func findClonedRepositories(workspace string, repositories []string, configuredHost string) tea.Cmd {
	return func() tea.Msg {
		cloned := make(map[string]bool)
		for _, repository := range repositories {
			remote, err := gitOriginURL(filepath.Join(workspace, repository))
			if err != nil {
				continue
			}
			host, owner, name, ok := githubRepository(remote)
			if ok && strings.EqualFold(host, configuredHost) {
				cloned[owner+"/"+name] = true
			}
		}
		return clonedRepositoriesMsg{repositories: cloned}
	}
}

func cloneFolderName(value string) (string, error) {
	folder := strings.TrimSpace(value)
	if folder == "" {
		return "", errors.New("enter a folder name")
	}
	if folder == "." || folder == ".." || filepath.Base(folder) != folder {
		return "", errors.New("folder name must not contain a path")
	}
	return folder, nil
}

func cloneGitHubRepository(workspace, host, repository, folder string) tea.Cmd {
	return func() tea.Msg {
		name := filepath.Base(repository)
		destinationRoot := workspace
		if folder != "" {
			destinationRoot = filepath.Join(workspace, folder)
			if err := os.Mkdir(destinationRoot, 0o755); err != nil {
				if errors.Is(err, os.ErrExist) {
					return cloneRepositoryMsg{repository: repository, path: destinationRoot, err: fmt.Errorf("folder %q already exists", folder)}
				}
				return cloneRepositoryMsg{repository: repository, path: destinationRoot, err: fmt.Errorf("create folder %q: %w", folder, err)}
			}
		}
		destination := filepath.Join(destinationRoot, name)
		if _, err := os.Stat(destination); err == nil {
			return cloneRepositoryMsg{repository: repository, path: destination, err: fmt.Errorf("destination %q already exists", name)}
		} else if !errors.Is(err, os.ErrNotExist) {
			return cloneRepositoryMsg{repository: repository, path: destination, err: fmt.Errorf("inspect clone destination: %w", err)}
		}
		url := fmt.Sprintf("https://%s/%s.git", host, repository)
		output, err := exec.Command("git", "clone", url, destination).CombinedOutput()
		if err != nil {
			if folder != "" {
				if removeErr := os.Remove(destinationRoot); removeErr != nil {
					return cloneRepositoryMsg{repository: repository, path: destination, err: fmt.Errorf("git clone: %s; remove empty folder %q: %w", strings.TrimSpace(string(output)), folder, removeErr)}
				}
			}
			return cloneRepositoryMsg{repository: repository, path: destination, err: fmt.Errorf("git clone: %s", strings.TrimSpace(string(output)))}
		}
		return cloneRepositoryMsg{repository: repository, path: destination}
	}
}

func gitOriginURL(repository string) (string, error) {
	output, err := exec.Command("git", "-C", repository, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return "", fmt.Errorf("read origin URL: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func githubRepository(remote string) (string, string, string, bool) {
	remote = strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	var host, path string
	switch {
	case strings.HasPrefix(remote, "git@"):
		parts := strings.SplitN(strings.TrimPrefix(remote, "git@"), ":", 2)
		if len(parts) != 2 {
			return "", "", "", false
		}
		host, path = parts[0], parts[1]
	case strings.HasPrefix(remote, "https://"):
		parts := strings.SplitN(strings.TrimPrefix(remote, "https://"), "/", 2)
		if len(parts) != 2 {
			return "", "", "", false
		}
		host, path = parts[0], parts[1]
	case strings.HasPrefix(remote, "ssh://git@"):
		parts := strings.SplitN(strings.TrimPrefix(remote, "ssh://git@"), "/", 2)
		if len(parts) != 2 {
			return "", "", "", false
		}
		host, path = parts[0], parts[1]
	default:
		return "", "", "", false
	}
	if !strings.Contains(host, "github") {
		return "", "", "", false
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	return host, parts[0], parts[1], true
}

type githubCommit struct {
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	Commit struct {
		Author struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"commit"`
}

func githubCommitCount(host, owner, repository, username, displayName string) (int, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/commits?per_page=100", owner, repository)
	output, err := exec.Command("gh", "api", "--hostname", host, "--paginate", "--slurp", endpoint).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("query GitHub commits: %s", strings.TrimSpace(string(output)))
	}
	var pages [][]githubCommit
	if err := json.Unmarshal(output, &pages); err != nil {
		return 0, fmt.Errorf("decode GitHub commits: %w", err)
	}
	count := 0
	for _, commits := range pages {
		for _, commit := range commits {
			if commitMatchesUser(commit, username, displayName) {
				count++
			}
		}
	}
	return count, nil
}

func commitMatchesUser(commit githubCommit, username, displayName string) bool {
	if commit.Author != nil && strings.EqualFold(commit.Author.Login, username) {
		return true
	}
	return displayName != "" && strings.EqualFold(commit.Commit.Author.Name, displayName)
}

func configPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, applicationName, "config.json"), nil
}

func loadConfig() (config, error) {
	path, err := configPath()
	if err != nil {
		return config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}
	var stored config
	if err := json.Unmarshal(data, &stored); err != nil {
		return config{}, err
	}
	if stored.MDSFolder == "" {
		return config{}, errors.New("configuration has no MDS folder")
	}
	if stored.Favorites == nil {
		stored.Favorites = make(map[string]bool)
	}
	if stored.Collapsed == nil {
		stored.Collapsed = make(map[string]bool)
	}
	stored.GitHubHost = configuredGitHubHost(stored.GitHubHost)
	return stored, nil
}

func configuredRefreshInterval(interval int) int {
	if interval > 0 {
		return interval
	}
	return defaultRefreshIntervalMinutes
}

func parseRefreshInterval(value string) (int, error) {
	interval, err := strconv.Atoi(value)
	if err != nil || interval <= 0 {
		return 0, errors.New("refresh interval must be a positive number of minutes")
	}
	return interval, nil
}

func (m model) refreshStatus(now time.Time) string {
	if m.lastRefresh.IsZero() || m.nextRefresh.IsZero() {
		return "Refresh pending"
	}
	remaining := max(0, int(m.nextRefresh.Sub(now).Seconds()))
	return fmt.Sprintf("Last %s · Next %d:%02d", m.lastRefresh.Format("15:04"), remaining/60, remaining%60)
}

func configuredGitHubHost(host string) string {
	if normalized := normalizeGitHubHost(host); normalized != "" {
		return normalized
	}
	return defaultGitHubHost
}

func normalizeGitHubHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	return strings.TrimSuffix(host, "/")
}

// isTextEditingKey reports whether a key inserts or deletes text. Arrow keys
// are deliberately excluded: textinput binds left/right to its own cursor, but
// in this interface they must keep driving list navigation.
func isTextEditingKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace, tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlH, tea.KeyCtrlU, tea.KeyCtrlW, tea.KeyCtrlK:
		return true
	}
	return false
}

func saveConfig(stored config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
