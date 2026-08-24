package cli

import (
	"testing"
	"time"
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
