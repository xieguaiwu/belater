package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"belater/internal/queue"
)

// resultRow is one task in the results view.
type resultRow struct {
	ID         string
	Name       string
	Status     queue.Status
	ExitCode   int
	Duration   time.Duration
	FinishedAt time.Time
}

// collectResults gathers terminal-state tasks finished within days days,
// newest first. days <= 0 means no time filter.
func collectResults(tasks []*queue.Task, days int) []resultRow {
	var rows []resultRow
	cutoff := time.Time{}
	if days > 0 {
		cutoff = time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	}
	for _, t := range tasks {
		terminal := t.Status == queue.Done || t.Status == queue.Failed || t.Status == queue.Cancelled
		if !terminal || t.LastRun == nil {
			continue
		}
		if !cutoff.IsZero() && t.LastRun.Before(cutoff) {
			continue
		}
		rows = append(rows, resultRow{
			ID:         t.ID,
			Name:       t.Name,
			Status:     t.Status,
			ExitCode:   t.LastExit,
			Duration:   time.Duration(t.DurationSec) * time.Second,
			FinishedAt: *t.LastRun,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].FinishedAt.After(rows[j].FinishedAt) })
	return rows
}

// cmdResults shows a summary of finished tasks.
func cmdResults(args []string) int {
	fs := flag.NewFlagSet("results", flag.ContinueOnError)
	days := fs.Int("days", 7, "只看最近 N 天（0 = 全部）")
	jsonOut := fs.Bool("json", false, "JSON 输出")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		return usageError(fs, "不接受位置参数")
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
	rows := collectResults(tasks, *days)

	if *jsonOut {
		out := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			out = append(out, map[string]any{
				"id": r.ID, "name": r.Name, "status": r.Status,
				"exit_code": r.ExitCode, "duration_s": int(r.Duration.Seconds()),
				"finished_at": r.FinishedAt.Format(time.RFC3339),
			})
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	if len(rows) == 0 {
		fmt.Printf("最近 %d 天没有已完成的任务（belater list 查看队列）\n", *days)
		return 0
	}

	fmt.Printf("%-10s %-7s %-7s %-9s %-17s %s\n", "ID", "状态", "退出码", "耗时", "完成时间", "名称")
	for _, r := range rows {
		fmt.Printf("%-10s %-7s %-7d %-9s %-17s %s\n",
			r.ID, r.Status, r.ExitCode, fmtDuration(r.Duration),
			r.FinishedAt.Local().Format("2006-01-02 15:04"), fmtName(r.Name))
	}
	fmt.Println()
	fmt.Println("查看详情: belater log <id>")
	return 0
}
