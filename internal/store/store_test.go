package store

import (
	"testing"
	"time"
)

func TestWeekStartIsMonday(t *testing.T) {
	date := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	if got := WeekStart(date).Weekday(); got != time.Monday {
		t.Fatalf("got %s, want Monday", got)
	}
}

func TestDeadlineLabel(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.Local)
	if got := deadlineForTest(now.Add(72*time.Hour), now); got != "4d left" {
		t.Fatalf("got %q", got)
	}
	if got := deadlineForTest(now.Add(time.Hour), now); got != "DUE TODAY" {
		t.Fatalf("got %q", got)
	}
}

func deadlineForTest(deadline, now time.Time) string {
	d := deadline.Sub(now)
	if d < 0 {
		return "over"
	}
	if d < 24*time.Hour {
		return "DUE TODAY"
	}
	return fmtDays(int(d.Hours()/24) + 1)
}
func fmtDays(n int) string { return string(rune('0'+n)) + "d left" }
