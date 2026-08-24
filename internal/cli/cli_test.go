package cli

import (
	"testing"
	"time"

	"belater/internal/queue"
)

func TestShQuote(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello", "'hello'"},
		{"it's", `'it'\''s'`},
		{"a'b'c", `'a'\''b'\''c'`},
	}
	for _, c := range cases {
		if got := shQuote(c.in); got != c.want {
			t.Errorf("shQuote(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestParseHHMM(t *testing.T) {
	loc := time.FixedZone("T", 8*3600)
	now := time.Date(2026, 8, 24, 14, 0, 0, 0, loc) // Monday 14:00
	cases := []struct {
		in   string
		want string
	}{
		{"16:30", "2026-08-24 16:30"},
		{"09:00", "2026-08-25 09:00"}, // already passed today
		{"14:00", "2026-08-25 14:00"}, // equal to now -> tomorrow
		{"00:00", "2026-08-25 00:00"},
	}
	for _, c := range cases {
		got := parseHHMM(c.in, now, loc)
		want, _ := time.ParseInLocation("2006-01-02 15:04", c.want, loc)
		if !got.Equal(want) {
			t.Errorf("parseHHMM(%s) = %v, want %v", c.in, got, want)
		}
	}
	if !parseHHMM("25:00", now, loc).IsZero() {
		t.Error("invalid hour must be zero")
	}
	if !parseHHMM("ab", now, loc).IsZero() {
		t.Error("invalid format must be zero")
	}
}

func TestFmtDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{2 * time.Minute, "2m"},
		{90 * time.Minute, "1h 30m"},
		{26 * time.Hour, "1d 2h"},
		{3*24*time.Hour + 4*time.Hour + 5*time.Minute, "3d 4h 5m"},
	}
	for _, c := range cases {
		if got := fmtDuration(c.d); got != c.want {
			t.Errorf("fmtDuration(%v) = %s, want %s", c.d, got, c.want)
		}
	}
}

func TestFmtName(t *testing.T) {
	long := "这是一段非常长的任务名称用来测试截断逻辑是否正常工作超过二十八字符"
	if got := fmtName(long); len([]rune(got)) != 28 {
		t.Errorf("fmtName length = %d, want 28 (got %q)", len([]rune(got)), got)
	}
	if got := fmtName("short"); got != "short" {
		t.Errorf("fmtName(short) = %s", got)
	}
}

func TestCollectResults(t *testing.T) {
	now := time.Now()
	old := now.Add(-3 * 24 * time.Hour)
	tasks := []*queue.Task{
		{ID: "11111111", Name: "new-fail", Status: queue.Failed, LastExit: 1, DurationSec: 30, LastRun: &now},
		{ID: "22222222", Name: "old-done", Status: queue.Done, LastExit: 0, DurationSec: 5, LastRun: &old},
		{ID: "33333333", Name: "queued", Status: queue.Queued, LastExit: -1, LastRun: nil},
		{ID: "44444444", Name: "recent-done", Status: queue.Done, LastExit: 0, DurationSec: 90, LastRun: &now},
	}

	// Default 7-day window: excludes only non-terminal tasks.
	rows := collectResults(tasks, 7)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	// Newest first: the two now-tasks before the 3-day-old one.
	if rows[0].ID != "11111111" && rows[0].ID != "44444444" {
		t.Errorf("expected recent tasks first, got %s", rows[0].ID)
	}
	if rows[1].ID == rows[0].ID {
		t.Error("duplicate row")
	}
	if rows[2].ID != "22222222" {
		t.Errorf("expected oldest task last, got %s", rows[2].ID)
	}

	// 1-day window excludes the 3-day-old task.
	rows = collectResults(tasks, 1)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows for 1 day, got %d", len(rows))
	}
	for _, r := range rows {
		if r.ID == "22222222" {
			t.Error("old task must be filtered out")
		}
	}
}

func TestCollectResultsEmpty(t *testing.T) {
	rows := collectResults(nil, 7)
	if len(rows) != 0 {
		t.Errorf("expected empty, got %d", len(rows))
	}
}
