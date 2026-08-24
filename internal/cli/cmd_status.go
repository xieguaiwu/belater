package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"belater/internal/daemon"
	"belater/internal/queue"
	"belater/internal/schedule"
)

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

	if *jsonOut {
		kind := sch.KindAt(now)
		next := sch.NextSwitch(now)
		out := map[string]any{
			"now":              now.Format(time.RFC3339),
			"window":           kind.String(),
			"multiplier":       sch.Multiplier(kind),
			"next_switch_at":   next.Format(time.RFC3339),
			"next_switch_in_s": int64(time.Until(next).Seconds()),
			"timezone":         sch.Location.String(),
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	st, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	tasks, err := st.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	var queued, running, done int
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

	kind := sch.KindAt(now)
	next := sch.NextSwitch(now)
	nextIn := time.Until(next)
	if nextIn < 0 {
		nextIn = 0
	}

	nextKind := sch.KindAt(next)
	fmt.Printf("当前时段: %s (%.1fx 价格)\n", kind, sch.Multiplier(kind))
	fmt.Printf("下次切换: %s %s (%s 后)\n", next.Format("15:04"), nextKind, fmtDuration(nextIn))
	if sch.KindAt(now) == schedule.KindOffPeak {
		fmt.Printf("现在就是空闲时段 — 新任务将立即排队执行\n")
	} else {
		fmt.Printf("今日下次空闲: %s (%s 后)\n", next.Format("15:04"), fmtDuration(nextIn))
	}

	// Scheduler liveness: does anything hold the instance lock?
	dir := stateDir()
	if daemon.InstanceLockHeld(dir) {
		fmt.Println("调度器: 运行中 — 到期任务会自动执行")
	} else {
		fmt.Println("调度器: 未运行 — 到期任务不会执行 (运行 belater daemon 或 belater daemon --install)")
	}
	fmt.Printf("队列: %d 待执行 · %d 运行中 · %d 已完成\n", queued, running, done)
	return 0
}
