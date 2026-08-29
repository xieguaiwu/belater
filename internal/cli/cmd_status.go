package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"belater/internal/daemon"
	"belater/internal/queue"
	"belater/internal/schedule"
)

// overdueGrace is how late a queued task may be before status calls it stuck.
// One missed tick must not look like a stalled scheduler.
const overdueGrace = time.Minute

// countTasks tallies the queue by status.
func countTasks(tasks []*queue.Task) (queued, running, done int) {
	for _, t := range tasks {
		switch t.Status {
		case queue.Queued:
			queued++
		case queue.Running:
			running++
		case queue.Done:
			done++
		}
	}
	return
}

// overdueQueued counts queued tasks whose run time has passed by more than
// grace, and returns the longest lateness. A scheduler that is alive but not
// dispatching is otherwise invisible in the output.
func overdueQueued(tasks []*queue.Task, now time.Time, grace time.Duration) (int, time.Duration) {
	var n int
	var worst time.Duration
	for _, t := range tasks {
		if t.Status != queue.Queued {
			continue
		}
		lag := now.Sub(t.RunAt)
		if lag <= grace {
			continue
		}
		n++
		if lag > worst {
			worst = lag
		}
	}
	return n, worst
}

// isWeekendIn reports a Sat/Sun in the given zone.
func isWeekendIn(loc *time.Location, t time.Time) bool {
	wd := t.In(loc).Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

// age renders how long ago a timestamp was, clamped at zero for clock skew.
func age(now, then time.Time) time.Duration {
	if d := now.Sub(then); d > 0 {
		return d
	}
	return 0
}

// statusLines builds the human-readable report. It is pure — now, the queue and
// the scheduler probe are injected — so every branch is testable.
func statusLines(sch *schedule.Schedule, now time.Time, tasks []*queue.Task, sched daemon.SchedulerState) []string {
	loc := sch.Location
	kind := sch.KindAt(now)
	next := sch.NextSwitch(now)
	tick := sched.Tick()
	var out []string

	line := fmt.Sprintf("当前时段: %s (%.1fx 价格)", kind, sch.Multiplier(kind))
	if kind == schedule.KindOffPeak && isWeekendIn(loc, now) {
		line += " · 周末全天空闲"
	}
	out = append(out, line)

	out = append(out, fmt.Sprintf("下次切换: %s %s (%s 后)",
		fmtSwitchTime(now, next, loc), sch.KindAt(next), fmtDuration(age(next, now))))

	if kind == schedule.KindOffPeak {
		// The scheduler polls once per tick, so "立即" was never true. Without a
		// heartbeat the real interval is unknown — say so instead of guessing.
		if sched.HB == nil {
			out = append(out, "现在就是空闲时段 — 新任务将在下次调度检查时执行")
		} else {
			out = append(out, fmt.Sprintf("现在就是空闲时段 — 新任务将在下次调度检查（约 %s 内）执行", fmtDuration(tick)))
		}
	} else {
		nextFree := sch.NextOffPeakStart(now)
		out = append(out, fmt.Sprintf("下次空闲: %s (%s 后)",
			fmtSwitchTime(now, nextFree, loc), fmtDuration(age(nextFree, now))))
	}

	switch {
	case !sched.Running:
		out = append(out, "调度器: 未运行 — 到期任务不会执行 (运行 belater daemon 或 belater daemon --install)")
	default:
		desc := "运行中"
		if sched.HB == nil {
			desc += "（无心跳信息）"
		} else {
			details := []string{}
			if sched.HB.Version != "" {
				details = append(details, "v"+sched.HB.Version)
			}
			details = append(details, "tick "+fmtDuration(tick), "心跳 "+fmtAge(now, sched.HB.LastTickAt))
			desc += " (" + strings.Join(details, " · ") + ")"
		}
		out = append(out, "调度器: "+desc+" — 到期任务会自动执行")
	}

	if sched.Running && sched.HB == nil {
		out = append(out, "警告: 调度器未写心跳（旧版本或写入失败）— 无法确认它仍在推进。重启: systemctl --user restart belater")
	}

	if sched.Stalled(now) {
		out = append(out, fmt.Sprintf("警告: 调度器心跳停在 %s 前（tick %s）— 循环可能卡住，到期任务不会执行。重启: systemctl --user restart belater",
			fmtDuration(age(now, sched.HB.LastTickAt)), fmtDuration(tick)))
	}
	if v := sched.VersionBehind(Version); v != "" {
		out = append(out, fmt.Sprintf("警告: 运行中的调度器是 v%s，本命令是 v%s — 旧版仍在派发。重启: systemctl --user restart belater", v, Version))
	}
	queued, running, done := countTasks(tasks)
	if n, worst := overdueQueued(tasks, now, overdueGrace); n > 0 {
		cause := "调度器未推进"
		if running > 0 {
			cause = "串行槽位已占用或调度器未推进"
		}
		out = append(out, fmt.Sprintf("警告: %d 个任务已到期未执行（最久 %s）— %s", n, fmtDuration(worst), cause))
	}

	out = append(out, fmt.Sprintf("队列: %d 待执行 · %d 运行中 · %d 已完成", queued, running, done))
	return out
}

// cmdStatus shows the current billing window and queue summary.
func cmdStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "JSON 输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		return usageError(fs, "不接受位置参数")
	}

	sch := defaultSchedule()
	now := time.Now()
	sched := daemon.ProbeScheduler(stateDir(), now)

	var tasks []*queue.Task
	var loadErr error
	st, err := openStore()
	if err != nil {
		loadErr = err
	} else if tasks, err = st.Load(); err != nil {
		loadErr = err
	}
	if loadErr != nil && !*jsonOut {
		fmt.Fprintf(os.Stderr, "错误: %v\n", loadErr)
		return 1
	}

	if *jsonOut {
		kind := sch.KindAt(now)
		next := sch.NextSwitch(now)
		queued, running, doneN := countTasks(tasks)
		overdue, overdueSec := 0, int64(0)
		if n, worst := overdueQueued(tasks, now, overdueGrace); n > 0 {
			overdue, overdueSec = n, int64(worst.Seconds())
		}
		out := map[string]any{
			"now":                       now.Format(time.RFC3339),
			"window":                    kind.String(),
			"multiplier":                sch.Multiplier(kind),
			"next_switch_at":            next.Format(time.RFC3339),
			"next_switch_label":         fmtSwitchTime(now, next, sch.Location),
			"next_switch_kind":          sch.KindAt(next).String(),
			"next_switch_in_s":          int64(time.Until(next).Seconds()),
			"timezone":                  sch.Location.String(),
			"scheduler_running":         sched.Running,
			"scheduler_stalled":         sched.Stalled(now),
			"scheduler_has_heartbeat":   sched.HB != nil,
			"scheduler_tick_s":          int(sched.Tick().Seconds()),
			"scheduler_version":         "",
			"scheduler_version_behind":  "",
			"scheduler_heartbeat_age_s": int64(0),
			"overdue_queued":            overdue,
			"overdue_max_s":             overdueSec,
			"queued":                    queued,
			"running":                   running,
			"done":                      doneN,
		}
		// These keys stay present either way: a consumer must not have to guess
		// whether an absent field meant "unknown" or "unsupported version".
		if sched.HB != nil {
			out["scheduler_version"] = sched.HB.Version
			out["scheduler_heartbeat_age_s"] = int64(age(now, sched.HB.LastTickAt).Seconds())
			out["scheduler_version_behind"] = sched.VersionBehind(Version)
		}
		if loadErr != nil {
			out["queue_error"] = loadErr.Error()
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	for _, l := range statusLines(sch, now, tasks, sched) {
		fmt.Println(l)
	}
	return 0
}
