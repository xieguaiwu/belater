package schedule

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, layout, value string) time.Time {
	t.Helper()
	tm, err := time.ParseInLocation(layout, value, DefaultSchedule().Location)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return tm
}

func TestKindAt(t *testing.T) {
	s := DefaultSchedule()
	cases := []struct {
		name string
		time string
		want Kind
	}{
		{"fri morning peak", "2026-08-24 10:00", KindPeak},     // Monday
		{"fri before peak", "2026-08-24 08:59", KindOffPeak},   // Monday
		{"noon off-peak", "2026-08-24 12:00", KindOffPeak},     // Monday
		{"afternoon before", "2026-08-24 13:59", KindOffPeak},  // Monday
		{"afternoon peak", "2026-08-24 14:00", KindPeak},       // Monday
		{"evening off-peak", "2026-08-24 18:00", KindOffPeak},  // Monday
		{"night off-peak", "2026-08-24 23:59", KindOffPeak},    // Monday
		{"midnight off-peak", "2026-08-25 00:00", KindOffPeak}, // Tuesday
		{"sat day off-peak", "2026-08-29 10:00", KindOffPeak},  // Saturday
		{"sun afternoon off-peak", "2026-08-30 15:00", KindOffPeak}, // Sunday
		{"mon 09:00 peak closed", "2026-08-24 09:00", KindPeak},     // left-closed
		{"fri 18:00 open", "2026-08-28 18:00", KindOffPeak},         // right-open (Friday)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := s.KindAt(mustTime(t, "2006-01-02 15:04", c.time))
			if got != c.want {
				t.Errorf("KindAt(%s) = %v, want %v", c.time, got, c.want)
			}
		})
	}
}

func TestNextSwitch(t *testing.T) {
	s := DefaultSchedule()
	cases := []struct {
		name string
		at   string
		want string
	}{
		{"mid morning peak", "2026-08-24 10:00", "2026-08-24 12:00"}, // Monday
		{"noon boundary", "2026-08-24 12:00", "2026-08-24 14:00"},    // Monday
		{"afternoon peak", "2026-08-24 16:00", "2026-08-24 18:00"},   // Monday
		{"weekend jump to monday", "2026-08-28 18:00", "2026-08-31 09:00"}, // Friday -> Monday
		{"saturday no boundaries", "2026-08-29 10:00", "2026-08-31 09:00"}, // Saturday -> Monday
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := s.NextSwitch(mustTime(t, "2006-01-02 15:04", c.at))
			want := mustTime(t, "2006-01-02 15:04", c.want)
			if !got.Equal(want) {
				t.Errorf("NextSwitch(%s) = %v, want %v", c.at, got, want)
			}
		})
	}
}

func TestNextOffPeakStart(t *testing.T) {
	s := DefaultSchedule()
	cases := []struct {
		name string
		at   string
		want string
	}{
		{"morning peak to noon", "2026-08-24 10:30", "2026-08-24 12:00"}, // Monday
		{"afternoon peak to 18:00", "2026-08-24 16:00", "2026-08-24 18:00"}, // Monday
		{"already off-peak", "2026-08-24 19:00", "2026-08-24 19:00"}, // Monday
		{"before 09:00 same time", "2026-08-24 07:00", "2026-08-24 07:00"}, // Monday
		{"weekend stays", "2026-08-29 11:00", "2026-08-29 11:00"}, // Saturday
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := s.NextOffPeakStart(mustTime(t, "2006-01-02 15:04", c.at))
			want := mustTime(t, "2006-01-02 15:04", c.want)
			if !got.Equal(want) {
				t.Errorf("NextOffPeakStart(%s) = %v, want %v", c.at, got, want)
			}
		})
	}
}

func TestMultiplier(t *testing.T) {
	s := DefaultSchedule()
	if got := s.Multiplier(KindPeak); got != 1.0 {
		t.Errorf("peak multiplier = %v, want 1.0", got)
	}
	if got := s.Multiplier(KindOffPeak); got != 0.5 {
		t.Errorf("off-peak multiplier = %v, want 0.5", got)
	}
}

func TestDescribe(t *testing.T) {
	s := DefaultSchedule()
	got := s.Describe(mustTime(t, "2006-01-02 15:04", "2026-08-24 10:00")) // Monday peak
	if got != "高峰时段 (1.0x)" {
		t.Errorf("Describe(peak) = %q", got)
	}
	got = s.Describe(mustTime(t, "2006-01-02 15:04", "2026-08-24 20:00")) // Monday off-peak
	if got != "空闲时段 (0.5x)" {
		t.Errorf("Describe(off-peak) = %q", got)
	}
}
