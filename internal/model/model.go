package model

import "time"

const CommitTarget = 3

type Identity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type GitHub struct {
	Host  string `json:"host"`
	User  string `json:"user,omitempty"`
	State string `json:"state"`
}

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Root      string    `json:"root"`
	User      Identity  `json:"user"`
	GitHub    GitHub    `json:"github"`
	CreatedAt time.Time `json:"createdAt"`
}

type Assignment struct {
	RepoID  string     `json:"repoId"`
	Name    string     `json:"name,omitempty"`
	Path    string     `json:"path,omitempty"`
	Remote  string     `json:"remote,omitempty"`
	Note    string     `json:"note,omitempty"`
	AddedAt time.Time  `json:"addedAt"`
	Done    bool       `json:"done"`
	DoneAt  *time.Time `json:"doneAt,omitempty"`
}

type Week struct {
	Key      string       `json:"key"`
	Name     string       `json:"name"`
	Start    time.Time    `json:"start"`
	Deadline *time.Time   `json:"deadline,omitempty"`
	Entries  []Assignment `json:"entries"`
}

type State struct {
	Version         int                        `json:"version"`
	ActiveWorkspace string                     `json:"activeWorkspace"`
	Workspaces      map[string]Workspace       `json:"workspaces"`
	Weeks           map[string]map[string]Week `json:"weeks"`
}

type Repo struct {
	ID            string
	Name          string
	Path          string
	Remote        string
	Branch        string
	YourCommits   int
	TotalCommits  int
	LastCommitAt  time.Time
	Dirty         bool
	Ahead, Behind int
}

type StatusRepo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	YourCommits int    `json:"yourCommits"`
	Target      int    `json:"target"`
	Done        bool   `json:"done"`
}
