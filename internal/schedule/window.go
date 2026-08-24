// Package schedule computes DeepSeek peak/off-peak billing windows.
//
// Billing rules (effective 2026-08-17, weekend rule updated 2026-08-23):
//   - Weekdays (Mon-Fri): peak 09:00-12:00 and 14:00-18:00 Beijing time.
//   - Weekends (Sat-Sun): all day off-peak.
//   - Off-peak price is 50% of peak price.
package schedule

import (
	"fmt"
	"time"
)

// Kind is a billing window kind.
type Kind int

const (
	// KindPeak is the full-price window.
	KindPeak Kind = iota
	// KindOffPeak is the half-price window.
	KindOffPeak
)

func (k Kind) String() string {
	if k == KindPeak {
		return "高峰"
	}
	return "空闲"
}

// Schedule defines peak/off-peak windows in a fixed location.
type Schedule struct {
	// Location is the billing timezone (Asia/Shanghai for DeepSeek).
	Location *time.Location
	// WeekdayPeaks is a list of [start, end) minute-of-day ranges
	// that are peak on weekdays. E.g. {{540,720},{840,1080}}.
	WeekdayPeaks [][2]int
	// WeekendAllOffPeak makes Saturday and Sunday off-peak all day.
	WeekendAllOffPeak bool
}

// DefaultSchedule returns the DeepSeek billing schedule.
func DefaultSchedule() *Schedule {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// Asia/Shanghai is always present in the Go tzdata; fall back to
		// a fixed UTC+8 offset if the system tz database is missing.
		loc = time.FixedZone("CST", 8*60*60)
	}
	return &Schedule{
		Location:          loc,
		WeekdayPeaks:      [][2]int{{540, 720}, {840, 1080}}, // 09:00-12:00, 14:00-18:00
		WeekendAllOffPeak: true,
	}
}

// Multiplier returns the price multiplier for a window kind.
func (s *Schedule) Multiplier(kind Kind) float64 {
	if kind == KindOffPeak {
		return 0.5
	}
	return 1.0
}

// minuteOfDay returns the local minute-of-day for t in s.Location.
func (s *Schedule) minuteOfDay(t time.Time) int {
	lt := t.In(s.Location)
	return lt.Hour()*60 + lt.Minute()
}

// isWeekend reports whether the local weekday of t is Sat or Sun.
func (s *Schedule) isWeekend(t time.Time) bool {
	wd := t.In(s.Location).Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

// KindAt returns the billing window kind for time t.
func (s *Schedule) KindAt(t time.Time) Kind {
	if s.WeekendAllOffPeak && s.isWeekend(t) {
		return KindOffPeak
	}
	m := s.minuteOfDay(t)
	for _, w := range s.WeekdayPeaks {
		if m >= w[0] && m < w[1] {
			return KindPeak
		}
	}
	return KindOffPeak
}

// describeKind returns a human-readable window description.
func (s *Schedule) describeKind(t time.Time) string {
	kind := s.KindAt(t)
	return fmt.Sprintf("%s时段 (%.1fx)", kind, s.Multiplier(kind))
}

// boundarySet returns all window boundaries from day d 00:00 to day d+1 00:00
// as local times. Weekends produce no boundaries when WeekendAllOffPeak is set.
func (s *Schedule) boundarySet(d time.Time) []time.Time {
	loc := s.Location
	start := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	var out []time.Time
	if s.WeekendAllOffPeak {
		wd := start.Weekday()
		if wd == time.Saturday || wd == time.Sunday {
			return out
		}
	}
	for _, w := range s.WeekdayPeaks {
		out = append(out, start.Add(time.Duration(w[0])*time.Minute))
		out = append(out, start.Add(time.Duration(w[1])*time.Minute))
	}
	return out
}

// NextSwitch returns the next window boundary strictly after t.
func (s *Schedule) NextSwitch(t time.Time) time.Time {
	loc := s.Location
	lt := t.In(loc)
	day := time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, loc)
	// Check today, then the next up to 8 days (weekend rule can jump to Monday).
	for i := 0; i < 8; i++ {
		for _, b := range s.boundarySet(day.AddDate(0, 0, i)) {
			if b.After(t) {
				return b
			}
		}
	}
	// Unreachable: weekday peaks guarantee boundaries every weekday.
	return day.AddDate(0, 0, 7)
}

// NextOffPeakStart returns t when t is already off-peak; otherwise the start
// of the next off-peak window.
func (s *Schedule) NextOffPeakStart(t time.Time) time.Time {
	if s.KindAt(t) == KindOffPeak {
		return t
	}
	next := s.NextSwitch(t)
	for s.KindAt(next) != KindOffPeak {
		next = s.NextSwitch(next)
	}
	return next
}

// Describe returns a short status line for time t.
func (s *Schedule) Describe(t time.Time) string {
	return s.describeKind(t)
}

// DayEndOffPeak returns true when t is outside today's peak windows
// (used by status output for "remaining off-peak today").
func (s *Schedule) DayEndOffPeak(t time.Time) bool {
	return s.KindAt(t) == KindOffPeak
}
