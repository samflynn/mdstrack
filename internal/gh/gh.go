package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Repo struct {
	Name      string    `json:"name"`
	FullName  string    `json:"nameWithOwner"`
	URL       string    `json:"url"`
	SSHURL    string    `json:"sshUrl"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func command(ctx context.Context, host, user string, args ...string) ([]byte, error) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, "gh", args...)
	cmd.Env = append(os.Environ(), "GH_HOST="+host)
	if user != "" {
		token, err := authToken(callCtx, host, user)
		if err != nil {
			return nil, fmt.Errorf("GitHub account %q on %s is unavailable: %w", user, host, err)
		}
		cmd.Env = append(cmd.Env, "GH_TOKEN="+token)
	}
	return cmd.Output()
}

func authToken(ctx context.Context, host, user string) (string, error) {
	output, err := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", host, "--user", user).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func State(ctx context.Context, host, user string) string {
	if _, err := exec.LookPath("gh"); err != nil {
		return "missing"
	}
	output, err := command(ctx, host, "", "auth", "status", "--hostname", host, "--json", "hosts")
	if err != nil {
		return "unauthenticated"
	}
	var response struct {
		Hosts map[string][]struct {
			State string `json:"state"`
			Login string `json:"login"`
		} `json:"hosts"`
	}
	if json.Unmarshal(output, &response) != nil || len(response.Hosts[host]) == 0 {
		return "unauthenticated"
	}
	for _, account := range response.Hosts[host] {
		if account.Login == user && account.State == "success" {
			return "ok"
		}
	}
	return "unauthenticated"
}

func MyRepos(ctx context.Context, host, user string) ([]Repo, error) {
	if strings.TrimSpace(user) == "" {
		return nil, fmt.Errorf("no GitHub username is configured")
	}
	output, err := command(ctx, host, user, "repo", "list", "--limit", "200", "--json", "name,nameWithOwner,url,sshUrl,updatedAt")
	if err != nil {
		return nil, err
	}
	var repos []Repo
	return repos, json.Unmarshal(output, &repos)
}

func Search(ctx context.Context, host, user, query string) ([]Repo, error) {
	if strings.TrimSpace(user) == "" {
		return nil, fmt.Errorf("no GitHub username is configured")
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	output, err := command(ctx, host, user, "search", "repos", query, "--limit", "30", "--json", "name,fullName,url,updatedAt")
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Name      string    `json:"name"`
		FullName  string    `json:"fullName"`
		URL       string    `json:"url"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}
	repos := make([]Repo, 0, len(raw))
	for _, repo := range raw {
		repos = append(repos, Repo{Name: repo.Name, FullName: repo.FullName, URL: repo.URL, UpdatedAt: repo.UpdatedAt})
	}
	return repos, nil
}

func Clone(ctx context.Context, host, user, source, destination string) error {
	if strings.TrimSpace(user) == "" {
		return fmt.Errorf("no GitHub username is configured")
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if !strings.Contains(source, "://") && !strings.Contains(source, "@") {
		output, err := command(callCtx, host, user, "repo", "clone", source, destination)
		if err != nil {
			return fmt.Errorf("gh repo clone: %s", strings.TrimSpace(string(output)))
		}
		return nil
	}
	output, err := exec.CommandContext(callCtx, "git", "clone", source, destination).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %s", strings.TrimSpace(string(output)))
	}
	return nil
}
