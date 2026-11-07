package git

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func Run(ctx context.Context, dir string, args ...string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(callCtx, "git", args...)
	cmd.Dir = dir
	b, err := cmd.Output()
	return strings.TrimSpace(string(b)), err
}

func Count(ctx context.Context, dir, email string, start, end time.Time) (int, error) {
	if email == "" {
		return 0, nil
	}
	result, err := Run(ctx, dir, "rev-list", "--count", "--no-merges", "--since="+start.Format(time.RFC3339), "--until="+end.Format(time.RFC3339), "--committer="+email, "HEAD")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(result)
}
