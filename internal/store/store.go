package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mdstrack/internal/model"
)

type Store struct{ Path string }

func DefaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "mdstrack", "state.json"), nil
}

func Load(path string) (model.State, error) {
	state := model.State{Version: 1, Workspaces: map[string]model.Workspace{}, Weeks: map[string]map[string]model.Week{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(b, &state); err != nil {
		backup := path + ".bak." + time.Now().Format("20060102T150405")
		if renameErr := os.Rename(path, backup); renameErr != nil {
			return state, fmt.Errorf("invalid state and backup failed: %w", err)
		}
		return state, fmt.Errorf("invalid state was saved as %s", backup)
	}
	if state.Workspaces == nil {
		state.Workspaces = map[string]model.Workspace{}
	}
	if state.Weeks == nil {
		state.Weeks = map[string]map[string]model.Week{}
	}
	return state, nil
}

func (s Store) Save(state model.State) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".state-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.Path)
}

func WeekStart(now time.Time) time.Time {
	local := now.In(time.Local)
	days := (int(local.Weekday()) + 6) % 7
	return time.Date(local.Year(), local.Month(), local.Day()-days, 0, 0, 0, 0, time.Local)
}

func WeekKey(start time.Time) string { y, w := start.ISOWeek(); return fmt.Sprintf("%04d-W%02d", y, w) }
func DefaultWeek(start time.Time) model.Week {
	deadline := start.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute)
	_, number := start.ISOWeek()
	return model.Week{Key: WeekKey(start), Name: fmt.Sprintf("W%d", number), Start: start, Deadline: &deadline, Entries: []model.Assignment{}}
}

func EnsureWeek(state *model.State, workspaceID string, now time.Time) model.Week {
	if state.Weeks[workspaceID] == nil {
		state.Weeks[workspaceID] = map[string]model.Week{}
	}
	start := WeekStart(now)
	key := WeekKey(start)
	week, ok := state.Weeks[workspaceID][key]
	if !ok {
		week = DefaultWeek(start)
		state.Weeks[workspaceID][key] = week
	}
	return week
}

func PutWeek(state *model.State, workspaceID string, week model.Week) {
	state.Weeks[workspaceID][week.Key] = week
}

func Slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	dash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
