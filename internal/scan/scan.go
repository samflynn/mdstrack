package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitcmd "mdstrack/internal/git"
	"mdstrack/internal/model"
)

func Repos(ctx context.Context, root, email string, start, end time.Time) ([]model.Repo, error) {
	var repos []model.Repo
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == ".git" {
			repo, ok := inspect(ctx, filepath.Dir(path), email, start, end)
			if ok {
				repos = append(repos, repo)
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == ".venv") {
			if d.Name() == ".git" {
				repo, ok := inspect(ctx, filepath.Dir(path), email, start, end)
				if ok {
					repos = append(repos, repo)
				}
				return filepath.SkipDir
			}
			return filepath.SkipDir
		}
		return nil
	})
	return repos, err
}

func inspect(ctx context.Context, path, email string, start, end time.Time) (model.Repo, bool) {
	if _, err := gitcmd.Run(ctx, path, "rev-parse", "--git-dir"); err != nil {
		return model.Repo{}, false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return model.Repo{}, false
	}
	hash := sha256.Sum256([]byte(abs))
	id := hex.EncodeToString(hash[:])[:16]
	repo := model.Repo{ID: id, Name: filepath.Base(abs), Path: abs}
	repo.Branch, _ = gitcmd.Run(ctx, abs, "branch", "--show-current")
	remote, _ := gitcmd.Run(ctx, abs, "remote", "get-url", "origin")
	repo.Remote = normalizeRemote(remote)
	repo.YourCommits, _ = gitcmd.Count(ctx, abs, email, start, end)
	total, err := gitcmd.Run(ctx, abs, "rev-list", "--count", "HEAD")
	if err == nil {
		fmtS := strings.TrimSpace(total)
		for _, c := range fmtS {
			if c < '0' || c > '9' {
				return repo, true
			}
			repo.TotalCommits = repo.TotalCommits*10 + int(c-'0')
		}
	}
	dirty, _ := gitcmd.Run(ctx, abs, "status", "--porcelain", "-uno")
	repo.Dirty = dirty != ""
	if stamp, err := gitcmd.Run(ctx, abs, "log", "-1", "--format=%cI"); err == nil {
		repo.LastCommitAt, _ = time.Parse(time.RFC3339, stamp)
	}
	return repo, true
}

func normalizeRemote(remote string) string {
	remote = strings.TrimSuffix(strings.TrimSpace(remote), ".git")
	remote = strings.TrimPrefix(remote, "https://github.com/")
	remote = strings.TrimPrefix(remote, "git@github.com:")
	remote = strings.TrimPrefix(remote, "git@github.ubc.ca:")
	remote = strings.TrimPrefix(remote, "https://github.ubc.ca/")
	return remote
}

var _ = os.ErrNotExist
