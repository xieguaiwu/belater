package cli

import (
	"strings"
	"testing"
	"time"

	"belater/internal/daemon"
	"belater/internal/queue"
	"belater/internal/schedule"
)

// billingLoc is the zone every display test pins times to, so the tests do not
// depend on the machine timezone.
func billingLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("Asia/Shanghai unavailable: %v", err)
	}
	return loc
}

func bt(loc *time.Location, y int, mo time.Month, d, h, mi int) time.Time {
	return time.Date(y, mo, d, h, mi, 0, 0, loc)
}

// healthyScheduler is a running scheduler on the current build, ticking 15s.
func healthyScheduler(now time.Time) daemon.SchedulerState {
	return daemon.SchedulerState{
		Running: true,
		HB:      &daemon.Heartbeat{Version: Version, TickSec: 15, MaxParallel: 1, LastTickAt: now.Add(-3 * time.Second)},
	}
}

func TestFmtSwitchTime(t *testing.T) {
	loc := billingLoc(t)
	sat := bt(loc, 2026, 8, 29, 15, 49) // Saturday afternoon
	cases := []struct {
		name string
		now  time.Time
		next time.Time
		want string
	}{
		{"same day keeps the clock", sat, bt(loc, 2026, 8, 29, 20, 0), "20:00"},
		{"tomorrow is labelled", sat, bt(loc, 2026, 8, 30, 9, 0), "明天 09:00"},
		// The bug this guards: on Saturday 15:49 the next switch is Monday
		// 09:00, and a bare "09:00" reads as tomorrow morning.
		{"next week names the weekday", sat, bt(loc, 2026, 8, 31, 9, 0), "周一 09:00"},
		{"past switch keeps the clock", bt(loc, 2026, 8, 31, 9, 0), sat, "15:49"},
		{"midnight to midnight is one day", bt(loc, 2026, 8, 29, 0, 0), bt(loc, 2026, 8, 30, 0, 0), "明天 00:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := fmtSwitchTime(c.now, c.next, loc); got != c.want {
				t.Errorf("fmtSwitchTime = %q, want %q", got, c.want)
			}
		})
	}
}

// TestCalendarDayDiffAcrossDST: two midnights across a US DST change are 23 or
// 25 hours apart. Truncating would call them 0 days.
func TestCalendarDayDiffAcrossDST(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	spring := calendarDayDiff(bt(ny, 2026, 3, 7, 0, 0), bt(ny, 2026, 3, 8, 0, 0)) // 25h
	fall := calendarDayDiff(bt(ny, 2026, 11, 1, 0, 0), bt(ny, 2026, 11, 2, 0, 0)) // 23h
	if spring != 1 || fall != 1 {
		t.Errorf("calendarDayDiff across DST = %d/%d, want 1/1", spring, fall)
	}
}

func TestFmtRemaining(t *testing.T) {
	if got := fmtRemaining(time.Now().Add(2 * time.Hour)); !strings.HasSuffix(got, "h") && !strings.Contains(got, "m") {
		t.Errorf("future run should read as time left, got %q", got)
	}
	if strings.HasPrefix(fmtRemaining(time.Now().Add(2*time.Hour)), "已到期") {
		t.Error("future run must not read as overdue")
	}
	got := fmtRemaining(time.Now().Add(-5 * time.Minute))
	if !strings.HasPrefix(got, "已到期") || !strings.Contains(got, "5m") {
		t.Errorf("overdue run = %q, want 已到期 5m", got)
	}
}

func TestOverdueQueued(t *testing.T) {
	now := time.Now()
	tasks := []*queue.Task{
		{ID: "a1a1a1a1", Status: queue.Queued, RunAt: now.Add(-5 * time.Minute)},
		{ID: "b2b2b2b2", Status: queue.Queued, RunAt: now.Add(-2 * time.Hour)},
		{ID: "c3c3c3c3", Status: queue.Queued, RunAt: now.Add(-30 * time.Second)}, // inside grace
		{ID: "d4d4d4d4", Status: queue.Paused, RunAt: now.Add(-3 * time.Hour)},    // not queued
		{ID: "e5e5e5e5", Status: queue.Running, RunAt: now.Add(-3 * time.Hour)},   // already dispatched
		{ID: "f6f6f6f6", Status: queue.Queued, RunAt: now.Add(time.Hour)},         // future
	}
	n, worst := overdueQueued(tasks, now, overdueGrace)
	if n != 2 {
		t.Errorf("overdue count = %d, want 2 (grace and non-queued tasks excluded)", n)
	}
	if worst < 119*time.Minute || worst > 121*time.Minute {
		t.Errorf("worst lag = %v, want ~2h", worst)
	}
	if n, _ := overdueQueued(nil, now, overdueGrace); n != 0 {
		t.Errorf("empty queue reported %d overdue", n)
	}
}

func TestStatusLinesWeekendOffPeak(t *testing.T) {
	loc := billingLoc(t)
	now := bt(loc, 2026, 8, 29, 15, 49) // Saturday, matches the reported bug
	lines := statusLines(schedule.DefaultSchedule(), now, nil, healthyScheduler(now))

	want := []string{
		"当前时段: 空闲 (0.5x 价格) · 周末全天空闲",
		"下次切换: 周一 09:00 高峰",
		"现在就是空闲时段 — 新任务将在下次调度检查（约 15s 内）执行",
		"调度器: 运行中 (v" + Version + " · tick 15s · 心跳 3s 前)",
		"队列: 0 待执行 · 0 运行中 · 0 已完成",
	}
	if len(lines) != len(want) {
		t.Fatalf("lines =\n%s\nwant %d lines", strings.Join(lines, "\n"), len(want))
	}
	for i, w := range want {
		if !strings.Contains(lines[i], w) {
			t.Errorf("line %d = %q, want it to contain %q", i, lines[i], w)
		}
	}
	// "立即" was never true: the scheduler polls once per tick.
	if strings.Contains(lines[2], "立即") {
		t.Errorf("off-peak line still promises instant execution: %q", lines[2])
	}
}

func TestStatusLinesPeakWeekday(t *testing.T) {
	loc := billingLoc(t)
	now := bt(loc, 2026, 8, 31, 10, 0) // Monday 10:00, morning peak
	lines := statusLines(schedule.DefaultSchedule(), now, nil, healthyScheduler(now))
	body := strings.Join(lines, "\n")

	if !strings.Contains(lines[0], "当前时段: 高峰 (1.0x 价格)") {
		t.Errorf("peak line wrong: %q", lines[0])
	}
	if strings.Contains(lines[0], "周末") {
		t.Errorf("weekday must not carry the weekend hint: %q", lines[0])
	}
	if !strings.Contains(lines[1], "下次切换: 12:00 空闲 (2h 后)") {
		t.Errorf("next switch wrong: %q", lines[1])
	}
	if !strings.Contains(lines[2], "下次空闲: 12:00 (2h 后)") {
		t.Errorf("next off-peak wrong: %q", lines[2])
	}
	if strings.Contains(body, "警告") {
		t.Errorf("a healthy scheduler must produce no warnings:\n%s", body)
	}
}

func TestStatusLinesLateNightNextSwitchIsTomorrow(t *testing.T) {
	loc := billingLoc(t)
	now := bt(loc, 2026, 8, 31, 23, 30) // Monday night; Tuesday 09:00 is next
	lines := statusLines(schedule.DefaultSchedule(), now, nil, healthyScheduler(now))
	if !strings.Contains(lines[1], "下次切换: 明天 09:00 高峰") {
		t.Errorf("late-night switch = %q, want 明天 09:00", lines[1])
	}
}

func TestStatusLinesSchedulerWarnings(t *testing.T) {
	loc := billingLoc(t)
	now := bt(loc, 2026, 8, 31, 10, 0)

	// Not running at all.
	lines := statusLines(schedule.DefaultSchedule(), now, nil, daemon.SchedulerState{Running: false})
	if !strings.Contains(strings.Join(lines, "\n"), "调度器: 未运行") {
		t.Errorf("idle state missing:\n%s", strings.Join(lines, "\n"))
	}

	// Wedged: lock held, heartbeat old.
	stalled := daemon.SchedulerState{Running: true, HB: &daemon.Heartbeat{Version: Version, TickSec: 15, LastTickAt: now.Add(-10 * time.Minute)}}
	body := strings.Join(statusLines(schedule.DefaultSchedule(), now, nil, stalled), "\n")
	if !strings.Contains(body, "警告: 调度器心跳停在 10m 前") {
		t.Errorf("stall warning missing:\n%s", body)
	}
	if !strings.Contains(body, "systemctl --user restart belater") {
		t.Errorf("stall warning must say how to recover:\n%s", body)
	}
	// The liveness line still shows the real heartbeat age, not "运行中" alone.
	if !strings.Contains(body, "心跳 10m 前") {
		t.Errorf("heartbeat age missing from the liveness line:\n%s", body)
	}

	// Stale build: the service was started before the binary was upgraded.
	old := daemon.SchedulerState{Running: true, HB: &daemon.Heartbeat{Version: "0.2.3", TickSec: 15, LastTickAt: now.Add(-time.Second)}}
	body = strings.Join(statusLines(schedule.DefaultSchedule(), now, nil, old), "\n")
	if !strings.Contains(body, "警告: 运行中的调度器是 v0.2.3，本命令是 v"+Version) {
		t.Errorf("version warning missing:\n%s", body)
	}

	// Overdue tasks with a live scheduler.
	overdue := []*queue.Task{{ID: "aaaa2222", Name: "job", Status: queue.Queued, RunAt: now.Add(-4 * time.Minute)}}
	body = strings.Join(statusLines(schedule.DefaultSchedule(), now, overdue, healthyScheduler(now)), "\n")
	if !strings.Contains(body, "警告: 1 个任务已到期未执行（最久 4m）") {
		t.Errorf("overdue warning missing:\n%s", body)
	}

	// The queue line stays last even with warnings above it.
	lines = statusLines(schedule.DefaultSchedule(), now, overdue, healthyScheduler(now))
	if !strings.HasPrefix(lines[len(lines)-1], "队列: 1 待执行") {
		t.Errorf("queue summary must stay last, got %q", lines[len(lines)-1])
	}
}

func TestStatusLinesNoHeartbeatData(t *testing.T) {
	loc := billingLoc(t)
	now := bt(loc, 2026, 8, 31, 10, 0)
	// An older scheduler build holds the lock and writes no heartbeat.
	st := daemon.SchedulerState{Running: true}
	body := strings.Join(statusLines(schedule.DefaultSchedule(), now, nil, st), "\n")
	if !strings.Contains(body, "调度器: 运行中（无心跳信息）") {
		t.Errorf("running without heartbeat data should say so:\n%s", body)
	}
	if !strings.Contains(body, "警告: 调度器未写心跳") {
		t.Errorf("a heartbeat-less scheduler must be flagged:\n%s", body)
	}
	if strings.Contains(body, "未心跳（tick") {
		t.Errorf("absent heartbeat data must not be reported as a stalled loop:\n%s", body)
	}
	// Without a heartbeat the real tick is unknown: reporting "约 15s 内" would be
	// a guess, and an old `--tick 60s` daemon would make it wrong.
	for _, l := range statusLines(schedule.DefaultSchedule(), now, nil, st) {
		if strings.Contains(l, "新任务将在") && strings.Contains(l, "约") {
			t.Errorf("off-peak line must not invent a tick without heartbeat data: %q", l)
		}
	}
}

// TestStatusLinesHeartbeatWithoutVersion covers a heartbeat written by a build
// that did not report a version: no "v" badge, no version warning.
func TestStatusLinesHeartbeatWithoutVersion(t *testing.T) {
	loc := billingLoc(t)
	now := bt(loc, 2026, 8, 31, 10, 0)
	st := daemon.SchedulerState{Running: true, HB: &daemon.Heartbeat{TickSec: 15, LastTickAt: now.Add(-time.Second)}}
	body := strings.Join(statusLines(schedule.DefaultSchedule(), now, nil, st), "\n")
	if strings.Contains(body, "v0.") || strings.Contains(body, "· v ") {
		t.Errorf("no version to show:\n%s", body)
	}
	if strings.Contains(body, "版本") {
		t.Errorf("unknown version must not trigger the version warning:\n%s", body)
	}
	if !strings.Contains(body, "调度器: 运行中 (tick 15s · 心跳 1s 前)") {
		t.Errorf("liveness line wrong:\n%s", body)
	}
}

func TestOverdueQueuedGraceBoundary(t *testing.T) {
	now := time.Now()
	against := func(lag time.Duration) []*queue.Task {
		return []*queue.Task{{ID: "1111aaaa", Status: queue.Queued, RunAt: now.Add(-lag)}}
	}
	if n, _ := overdueQueued(against(overdueGrace), now, overdueGrace); n != 0 {
		t.Errorf("a task exactly one grace window late must not be reported, got %d", n)
	}
	n, worst := overdueQueued(against(overdueGrace+time.Second), now, overdueGrace)
	if n != 1 || worst <= overdueGrace {
		t.Errorf("task past the grace window must be reported, got n=%d worst=%v", n, worst)
	}
}
