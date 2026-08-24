package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"belater/internal/daemon"
	"belater/internal/queue"
	"belater/internal/schedule"
)

// envList collects repeated --env KEY=VALUE flags.
type envList []string

func (e *envList) String() string { return strings.Join(*e, ",") }
func (e *envList) Set(v string) error {
	if !strings.Contains(v, "=") {
		return fmt.Errorf("--env 需要 KEY=VALUE 格式，收到 %q", v)
	}
	*e = append(*e, v)
	return nil
}

// shQuote single-quotes s for /bin/sh.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// cmdAdd enqueues a task.
func cmdAdd(args []string) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	piPrompt := fs.String("pi", "", "pi-agent 模式：把提示词交给 pi -p 执行")
	piArgs := fs.String("pi-args", "", "附加到 pi 命令的参数（如 --no-session）")
	at := fs.String("at", "", "指定执行时间 HH:MM（今天或明天）")
	delay := fs.String("delay", "", "从现在起延迟执行（如 2h、90m）")
	nowFlag := fs.Bool("now", false, "立即执行（等下一个调度 tick）")
	every := fs.String("every", "", "每日重复执行 HH:MM")
	maxRuns := fs.Int("max-runs", 0, "--every 的最大执行次数（0 = 不限）")
	retries := fs.Int("retries", 0, "失败后的额外重试次数")
	name := fs.String("name", "", "任务显示名")
	cwd := fs.String("cwd", "", "工作目录（默认当前目录）")
	envs := envList{}
	fs.Var(&envs, "env", "环境变量 KEY=VALUE（可重复）")
	timeout := fs.String("timeout", "", "超时时间（如 30m、2h；0 = 不限）")
	parallel := fs.Bool("parallel", false, "不占用串行槽位，可与其它任务并行")
	onDone := fs.String("on-done", "", "任务完成后执行的命令（env: BELATER_TASK_ID/NAME/STATUS/EXIT）")
	notify := fs.Bool("notify", false, "完成后桌面通知（等价于 --on-done notify-send）")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Build the command.
	var command string
	isPi := *piPrompt != ""
	if isPi && fs.NArg() > 0 {
		return usageError(fs, "--pi 与位置参数命令互斥，只选一种")
	}
	if !isPi && fs.NArg() == 0 {
		return usageError(fs, "缺少命令：提供位置参数命令，或用 --pi 指定 pi-agent 提示词")
	}
	if *piArgs != "" && !isPi {
		return usageError(fs, "--pi-args 只能与 --pi 一起使用")
	}
	if isPi {
		command = "pi -p " + shQuote(*piPrompt)
		if *piArgs != "" {
			command += " " + *piArgs
		}
	} else {
		command = strings.Join(fs.Args(), " ")
	}

	sch := defaultSchedule()
	now := time.Now()

	// Compute RunAt.
	var runAt time.Time
	scheduleFlags := 0
	for _, f := range []bool{*at != "", *delay != "", *nowFlag} {
		if f {
			scheduleFlags++
		}
	}
	if scheduleFlags > 1 {
		return usageError(fs, "--at、--delay、--now 互斥，只选一种")
	}
	// Times are interpreted in the billing timezone (Asia/Shanghai), not the
	// machine local zone, so peak/off-peak labels stay correct everywhere.
	schedLoc := sch.Location
	switch {
	case *at != "":
		runAt = parseHHMM(*at, now, schedLoc)
		if runAt.IsZero() {
			return usageError(fs, "--at 格式应为 HH:MM")
		}
	case *delay != "":
		d, err := time.ParseDuration(*delay)
		if err != nil {
			return usageError(fs, "--delay 格式无效: %v", err)
		}
		runAt = now.Add(d)
	case *nowFlag:
		runAt = now
	default:
		runAt = sch.NextOffPeakStart(now)
	}

	var timeoutSec int
	if *timeout != "" {
		d, err := time.ParseDuration(*timeout)
		if err != nil {
			return usageError(fs, "--timeout 格式无效: %v", err)
		}
		timeoutSec = int(d.Seconds())
	}

	everyWarn := false
	if *every != "" {
		everyTime := parseHHMM(*every, now, schedLoc)
		if everyTime.IsZero() {
			return usageError(fs, "--every 格式应为 HH:MM")
		}
		if sch.KindAt(everyTime) == schedule.KindPeak {
			everyWarn = true
		}
	}

	taskName := *name
	if taskName == "" {
		if isPi {
			r := []rune(*piPrompt)
			if len(r) > 40 {
				taskName = "pi: " + string(r[:39]) + "…"
			} else {
				taskName = "pi: " + *piPrompt
			}
		} else {
			taskName = command
		}
	}

	if *cwd == "" {
		if wd, err := os.Getwd(); err == nil {
			*cwd = wd
		}
	}

	if *onDone != "" && *notify {
		return usageError(fs, "--on-done 与 --notify 互斥，只选一种")
	}
	if *notify {
		*onDone = "notify-send belater \"任务完成: $BELATER_TASK_NAME (exit $BELATER_TASK_EXIT)\""
	}

	task := &queue.Task{
		Name:       taskName,
		Command:    command,
		Cwd:        *cwd,
		Env:        envs,
		RunAt:      runAt,
		Every:      *every,
		MaxRuns:    *maxRuns,
		Retries:    *retries,
		TimeoutSec: timeoutSec,
		Parallel:   *parallel,
		OnDone:     *onDone,
		Status:     queue.Queued,
		CreatedAt:  now,
		LastExit:   -1,
	}

	st, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	if err := st.Add(task); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}

	// Feedback.
	wait := time.Until(runAt)
	if wait < 0 {
		wait = 0
	}
	when := "空闲时段"
	if sch.KindAt(runAt) == schedule.KindPeak {
		when = "高峰时段"
	}
	if *at != "" {
		when += "（用户指定）"
	}
	fmt.Printf("已入队 %s · %s · %s 执行 (%s 后)\n", task.ID, task.Name, when, fmtDuration(wait))
	if isPi {
		fmt.Printf("  pi-agent 任务: %s\n", command)
	}
	if runAt.Equal(now) && !*nowFlag && sch.KindAt(now) == schedule.KindOffPeak {
		fmt.Println("  当前已是空闲时段，任务将在调度器下次 tick 时执行")
	}
	if everyWarn {
		fmt.Printf("  警告: --every %s 落在高峰时段，将按 1.0x 价格计费\n", *every)
	}
	if !*nowFlag && *at == "" && *delay == "" {
		if daemon.InstanceLockHeld(stateDir()) {
			fmt.Println("  调度器已在运行 — 任务将自动执行")
		} else {
			fmt.Println("  提示: 运行 belater daemon 或 belater daemon --install 来执行队列")
		}
	}
	return 0
}

// parseHHMM parses "HH:MM" in loc as today or tomorrow (if already past).
func parseHHMM(s string, now time.Time, loc *time.Location) time.Time {
	var hh, mm int
	if _, err := fmt.Sscanf(s, "%d:%d", &hh, &mm); err != nil {
		return time.Time{}
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return time.Time{}
	}
	t := time.Date(now.Year(), now.Month(), now.Day(), hh, mm, 0, 0, loc)
	if !t.After(now) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}
