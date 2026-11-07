package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"mdstrack/internal/gh"
	"mdstrack/internal/model"
	"mdstrack/internal/scan"
	"mdstrack/internal/store"
	"mdstrack/internal/ui"
)

type Program struct {
	out, err io.Writer
	state    model.State
	store    store.Store
}

func New(out, err io.Writer) *Program { return &Program{out: out, err: err} }

func (p *Program) Run(ctx context.Context, args []string) error {
	path, err := store.DefaultPath()
	if err != nil {
		return err
	}
	p.store = store.Store{Path: path}
	p.state, err = store.Load(path)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return p.dashboard(ctx)
	}
	switch args[0] {
	case "setup":
		return p.setup(args[1:])
	case "status":
		return p.status(ctx, contains(args, "--json"))
	case "scan":
		return p.scan(ctx, contains(args, "--json"))
	case "add", "remove", "done":
		return p.mutate(args)
	case "week":
		return p.week(args[1:])
	case "workspace":
		return p.workspace(args[1:])
	case "auth":
		return p.auth(args[1:])
	case "--version", "version":
		fmt.Fprintln(p.out, "mds 0.1.0")
		return nil
	case "help", "--help", "-h":
		p.help()
		return nil
	default:
		return fmt.Errorf("unknown command %q (run mds help)", args[0])
	}
}

func (p *Program) setup(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: mds setup <name> <workspace-root> <email> [git-name]")
	}
	root, err := filepath.Abs(args[1])
	if err != nil {
		return err
	}
	if err := validateRoot(root); err != nil {
		return err
	}
	id := store.Slug(args[0])
	if id == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}
	name := args[0]
	gitName := ""
	if len(args) > 3 {
		gitName = args[3]
	}
	host := "github.com"
	if len(args) > 4 && strings.TrimSpace(args[4]) != "" {
		host = strings.TrimSpace(args[4])
	}
	gh := model.GitHub{Host: host, State: gh.State(context.Background(), host)}
	p.state.Workspaces[id] = model.Workspace{ID: id, Name: name, Root: root, User: model.Identity{Name: gitName, Email: args[2]}, GitHub: gh, CreatedAt: time.Now()}
	p.state.ActiveWorkspace = id
	store.EnsureWeek(&p.state, id, time.Now())
	if err := p.store.Save(p.state); err != nil {
		return err
	}
	fmt.Fprintf(p.out, "Workspace %q ready at %s. Run mds to open the dashboard.\n", name, root)
	return nil
}

func validateRoot(root string) error {
	if root == string(filepath.Separator) {
		return fmt.Errorf("workspace root cannot be /")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create workspace root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace root is not a directory")
	}
	if _, err := exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output(); err == nil {
		return fmt.Errorf("workspace root must not be inside an existing git repository")
	}
	return nil
}

func (p *Program) active() (model.Workspace, model.Week, error) {
	if p.state.ActiveWorkspace == "" {
		return model.Workspace{}, model.Week{}, fmt.Errorf("no workspace: run mds setup <name> <root> <email>")
	}
	ws, ok := p.state.Workspaces[p.state.ActiveWorkspace]
	if !ok {
		return model.Workspace{}, model.Week{}, fmt.Errorf("active workspace is missing")
	}
	week := store.EnsureWeek(&p.state, ws.ID, time.Now())
	return ws, week, nil
}

func (p *Program) repositories(ctx context.Context) (model.Workspace, model.Week, []model.Repo, error) {
	ws, week, err := p.active()
	if err != nil {
		return ws, week, nil, err
	}
	repos, err := scan.Repos(ctx, ws.Root, ws.User.Email, week.Start, week.Start.AddDate(0, 0, 7))
	return ws, week, repos, err
}

func (p *Program) scan(ctx context.Context, asJSON bool) error {
	_, _, repos, err := p.repositories(ctx)
	if err != nil {
		return err
	}
	if asJSON {
		return json.NewEncoder(p.out).Encode(repos)
	}
	for _, repo := range repos {
		fmt.Fprintf(p.out, "%s\t%s\n", repo.Name, repo.Path)
	}
	return nil
}

func (p *Program) status(ctx context.Context, asJSON bool) error {
	ws, week, repos, err := p.repositories(ctx)
	if err != nil {
		return err
	}
	assigned := p.assigned(week, repos)
	if asJSON {
		entries := assignmentsByID(week)
		statusRepos := make([]model.StatusRepo, 0, len(assigned))
		for _, repo := range assigned {
			statusRepos = append(statusRepos, model.StatusRepo{Name: repo.Name, Path: repo.Path, YourCommits: repo.YourCommits, Target: model.CommitTarget, Done: entries[repo.ID].Done})
		}
		return json.NewEncoder(p.out).Encode(map[string]any{"workspace": ws.Name, "week": week.Name, "deadline": week.Deadline, "repos": statusRepos})
	}
	p.printDashboard(ws, week, assigned)
	return nil
}

func (p *Program) dashboard(ctx context.Context) error {
	if !isTerminal() {
		return p.status(ctx, false)
	}
	snapshot, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	return ui.Run(snapshot, func() (ui.Snapshot, error) { return p.snapshot(ctx) }, p.applyUIAction)
}

func (p *Program) snapshot(ctx context.Context) (ui.Snapshot, error) {
	ws, week, repos, err := p.repositories(ctx)
	if err != nil {
		return ui.Snapshot{}, err
	}
	entries := assignmentsByID(week)
	rows := make([]ui.Repo, 0, len(repos))
	for _, repo := range repos {
		_, assigned := entries[repo.ID]
		rows = append(rows, ui.Repo{ID: repo.ID, Name: repo.Name, Path: repo.Path, Remote: repo.Remote, Branch: repo.Branch, YourCommits: repo.YourCommits, Dirty: repo.Dirty, Done: entries[repo.ID].Done, Assigned: assigned})
	}
	remote, remoteErr := gh.MyRepos(ctx, ws.GitHub.Host)
	if remoteErr == nil {
		for _, repo := range remote {
			_, assigned := entries["remote:"+repo.FullName]
			rows = append(rows, ui.Repo{ID: "remote:" + repo.FullName, Name: repo.Name, Remote: repo.FullName, URL: repo.URL, RemoteOnly: true, Assigned: assigned, Done: entries["remote:"+repo.FullName].Done})
		}
	}
	for _, entry := range week.Entries {
		found := false
		for _, row := range rows {
			if row.ID == entry.RepoID {
				found = true
				break
			}
		}
		if !found {
			rows = append(rows, ui.Repo{ID: entry.RepoID, Name: entry.Name, Path: entry.Path, Remote: entry.Remote, Assigned: true, Done: entry.Done, RemoteOnly: entry.Path == ""})
		}
	}
	return ui.Snapshot{Workspace: ws.Name, Week: week.Name, Deadline: week.Deadline, Repos: rows, GitHubHost: ws.GitHub.Host, GitHubState: ws.GitHub.State}, nil
}

func (p *Program) applyUIAction(action ui.Action, repo ui.Repo) error {
	ws, week, err := p.active()
	if err != nil {
		return err
	}
	if action == ui.ActionClone {
		if repo.Remote == "" {
			return fmt.Errorf("this repository has no remote URL")
		}
		destination := filepath.Join(ws.Root, repo.Name)
		if _, err := os.Stat(destination); err == nil {
			return fmt.Errorf("%s already exists", destination)
		}
		return gh.Clone(context.Background(), ws.GitHub.Host, repo.Remote, destination)
	}
	index := -1
	for i, entry := range week.Entries {
		if entry.RepoID == repo.ID {
			index = i
			break
		}
	}
	switch action {
	case ui.ActionAdd:
		if index >= 0 {
			return fmt.Errorf("%s is already assigned", repo.Name)
		}
		week.Entries = append(week.Entries, model.Assignment{RepoID: repo.ID, Name: repo.Name, Path: repo.Path, Remote: repo.Remote, AddedAt: time.Now()})
	case ui.ActionRemove:
		if index < 0 {
			return fmt.Errorf("%s is not assigned", repo.Name)
		}
		week.Entries = append(week.Entries[:index], week.Entries[index+1:]...)
	case ui.ActionDone:
		if index < 0 {
			return fmt.Errorf("%s is not assigned", repo.Name)
		}
		now := time.Now()
		week.Entries[index].Done = !week.Entries[index].Done
		if week.Entries[index].Done {
			week.Entries[index].DoneAt = &now
		} else {
			week.Entries[index].DoneAt = nil
		}
	case ui.ActionOpen:
		if repo.Path == "" {
			return fmt.Errorf("clone the repository before opening it")
		}
		return openPath(repo.Path)
	}
	store.PutWeek(&p.state, ws.ID, week)
	return p.store.Save(p.state)
}

func (p *Program) assigned(week model.Week, repos []model.Repo) []model.Repo {
	byID := map[string]model.Repo{}
	for _, repo := range repos {
		byID[repo.ID] = repo
	}
	var result []model.Repo
	for _, entry := range week.Entries {
		if repo, ok := byID[entry.RepoID]; ok {
			result = append(result, repo)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
func (p *Program) printDashboard(ws model.Workspace, week model.Week, repos []model.Repo) {
	deadline := "no deadline"
	if week.Deadline != nil {
		deadline = deadlineLabel(*week.Deadline, time.Now())
	}
	fmt.Fprintf(p.out, "\n%s · %s    %s\n%s\n", ws.Name, week.Name, deadline, strings.Repeat("─", 54))
	entries := assignmentsByID(week)
	done := 0
	for _, repo := range repos {
		if entries[repo.ID].Done {
			done++
			continue
		}
		fmt.Fprintf(p.out, "%-24s %s %d/%d\n  %s%s\n", repo.Name, bar(repo.YourCommits), repo.YourCommits, model.CommitTarget, repo.Branch, dirty(repo.Dirty))
	}
	if done > 0 {
		fmt.Fprintf(p.out, "✓ %d done · use mds done <repo> to reopen\n", done)
	}
	fmt.Fprintf(p.out, "%s\n%d/%d complete · %d commits\n", strings.Repeat("─", 54), done, len(repos), sum(repos))
}

func assignmentsByID(week model.Week) map[string]model.Assignment {
	entries := make(map[string]model.Assignment, len(week.Entries))
	for _, entry := range week.Entries {
		entries[entry.RepoID] = entry
	}
	return entries
}
func bar(n int) string {
	if n > model.CommitTarget {
		n = model.CommitTarget
	}
	return strings.Repeat("█", n) + strings.Repeat("░", model.CommitTarget-n)
}
func dirty(value bool) string {
	if value {
		return " · dirty"
	}
	return ""
}
func sum(repos []model.Repo) int {
	n := 0
	for _, r := range repos {
		n += r.YourCommits
	}
	return n
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

func (p *Program) mutate(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: mds %s <repo>", args[0])
	}
	ws, week, err := p.active()
	if err != nil {
		return err
	}
	_, _, repos, err := p.repositories(context.Background())
	if err != nil {
		return err
	}
	var selected *model.Repo
	for i := range repos {
		if repos[i].Name == args[1] || repos[i].Path == args[1] {
			selected = &repos[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("repo %q not found in workspace", args[1])
	}
	index := -1
	for i, e := range week.Entries {
		if e.RepoID == selected.ID {
			index = i
			break
		}
	}
	switch args[0] {
	case "add":
		if index >= 0 {
			return fmt.Errorf("%s is already assigned", selected.Name)
		}
		week.Entries = append(week.Entries, model.Assignment{RepoID: selected.ID, AddedAt: time.Now()})
	case "remove":
		if index < 0 {
			return fmt.Errorf("%s is not assigned", selected.Name)
		}
		week.Entries = append(week.Entries[:index], week.Entries[index+1:]...)
	case "done":
		if index < 0 {
			return fmt.Errorf("%s is not assigned", selected.Name)
		}
		now := time.Now()
		week.Entries[index].Done = !week.Entries[index].Done
		if week.Entries[index].Done {
			week.Entries[index].DoneAt = &now
		} else {
			week.Entries[index].DoneAt = nil
		}
	default:
		return fmt.Errorf("unknown mutation")
	}
	store.PutWeek(&p.state, ws.ID, week)
	if err := p.store.Save(p.state); err != nil {
		return err
	}
	fmt.Fprintf(p.out, "Updated %s.\n", selected.Name)
	return nil
}

func (p *Program) week(args []string) error {
	ws, week, err := p.active()
	if err != nil {
		return err
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: mds week name <name> | mds week due <YYYY-MM-DD HH:MM|none>")
	}
	switch args[0] {
	case "name":
		week.Name = strings.TrimSpace(strings.Join(args[1:], " "))
		if week.Name == "" {
			_, n := week.Start.ISOWeek()
			week.Name = fmt.Sprintf("W%d", n)
		}
	case "due":
		if args[1] == "none" {
			week.Deadline = nil
		} else {
			value := strings.Join(args[1:], " ")
			parsed, parseErr := time.ParseInLocation("2006-01-02 15:04", value, time.Local)
			if parseErr != nil {
				parsed, parseErr = time.ParseInLocation("2006-01-02", value, time.Local)
				if parseErr == nil {
					parsed = parsed.Add(23*time.Hour + 59*time.Minute)
				}
				if parseErr != nil {
					return fmt.Errorf("deadline must be YYYY-MM-DD or YYYY-MM-DD HH:MM")
				}
			}
			week.Deadline = &parsed
		}
	default:
		return fmt.Errorf("unknown week action")
	}
	store.PutWeek(&p.state, ws.ID, week)
	return p.store.Save(p.state)
}
func (p *Program) workspace(args []string) error {
	if len(args) == 0 || args[0] == "list" {
		for id, ws := range p.state.Workspaces {
			marker := " "
			if id == p.state.ActiveWorkspace {
				marker = "*"
			}
			fmt.Fprintf(p.out, "%s %s\t%s\n", marker, ws.Name, ws.Root)
		}
		return nil
	}
	if args[0] == "use" && len(args) == 2 {
		for id, ws := range p.state.Workspaces {
			if id == args[1] || ws.Name == args[1] {
				p.state.ActiveWorkspace = id
				return p.store.Save(p.state)
			}
		}
		return fmt.Errorf("workspace %q not found", args[1])
	}
	return fmt.Errorf("usage: mds workspace list|use <name>")
}
func (p *Program) auth(args []string) error {
	host := "github.com"
	if p.state.ActiveWorkspace != "" {
		if workspace, ok := p.state.Workspaces[p.state.ActiveWorkspace]; ok {
			host = workspace.GitHub.Host
		}
	}
	state := gh.State(context.Background(), host)
	if contains(args, "--json") {
		return json.NewEncoder(p.out).Encode(map[string]string{"state": state, "host": host})
	}
	fmt.Fprintf(p.out, "%s: %s\n", host, state)
	if state != "ok" {
		fmt.Fprintf(p.out, "Run: gh auth login --hostname %s --git-protocol https --web\n", host)
	}
	return nil
}
func (p *Program) help() {
	fmt.Fprint(p.out, "mds — MDS repository tracker\n\nCommands:\n  mds setup <name> <root> <email> [git-name] [github-host]\n  mds [dashboard]\n  mds status [--json]\n  mds scan [--json]\n  mds add|remove|done <repo>\n  mds week name <name>\n  mds week due <YYYY-MM-DD [HH:MM]|none>\n  mds workspace list|use <name>\n  mds auth [--json]\n")
}
func contains(values []string, wanted string) bool {
	for _, v := range values {
		if v == wanted {
			return true
		}
	}
	return false
}
func isTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

func openPath(path string) error {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	if runtime.GOOS == "windows" {
		command = "explorer"
	}
	return exec.Command(command, path).Start()
}

var _ = strconv.Itoa
