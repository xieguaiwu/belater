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

// cmdList shows the task queue.
func cmdList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "JSON 输出")
	all := fs.Bool("all", false, "包含已完成/失败任务")
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
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].RunAt.Before(tasks[j].RunAt) })

	visible := tasks
	if !*all {
		visible = nil
		for _, t := range tasks {
			if t.Status == queue.Queued || t.Status == queue.Running || t.Status == queue.Paused {
				visible = append(visible, t)
			}
		}
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(visible, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	if len(visible) == 0 {
		fmt.Println("队列为空")
		return 0
	}

	fmt.Printf("%-10s %-8s %-17s %-12s %s\n", "ID", "状态", "计划执行", "剩余", "名称")
	for _, t := range visible {
		remaining := "-"
		switch t.Status {
		case queue.Queued:
			remaining = fmtDuration(time.Until(t.RunAt))
		case queue.Running:
			remaining = "运行中"
		case queue.Paused:
			remaining = "已暂停"
		}
		fmt.Printf("%-10s %-8s %-17s %-12s %s\n",
			t.ID, t.Status, fmtRunAt(t.RunAt), remaining, fmtName(t.Name))
	}
	return 0
}

// validID reports whether s looks like a task ID (8 hex chars).
func validID(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// cmdRemove deletes a task.
func cmdRemove(args []string) int {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		return usageError(fs, "需要一个任务 ID")
	}
	id := fs.Arg(0)
	if !validID(id) {
		return usageError(fs, "无效的任务 ID %q", id)
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	if err := st.Remove(id); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	fmt.Printf("已删除 %s\n", id)
	return 0
}

// cmdPause pauses a queued task.
func cmdPause(args []string) int {
	return setStatus(args, "pause", queue.Paused)
}

// cmdResume resumes a paused task.
func cmdResume(args []string) int {
	return setStatus(args, "resume", queue.Queued)
}

func setStatus(args []string, name string, status queue.Status) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		return usageError(fs, "需要一个任务 ID")
	}
	id := fs.Arg(0)
	if !validID(id) {
		return usageError(fs, "无效的任务 ID %q", id)
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	t, err := st.Find(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	if status == queue.Paused && t.Status != queue.Queued {
		fmt.Fprintf(os.Stderr, "错误: 只能暂停 queued 任务（当前 %s）\n", t.Status)
		return 1
	}
	if status == queue.Queued && t.Status != queue.Paused {
		fmt.Fprintf(os.Stderr, "错误: 只能恢复 paused 任务（当前 %s）\n", t.Status)
		return 1
	}
	t.Status = status
	if err := st.Update(t); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	fmt.Printf("已%s %s\n", name, t.ID)
	return 0
}

// cmdRun executes a task immediately in the foreground.
func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		return usageError(fs, "需要一个任务 ID")
	}
	id := fs.Arg(0)
	if !validID(id) {
		return usageError(fs, "无效的任务 ID %q", id)
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	t, err := st.Find(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	if t.Status == queue.Running {
		fmt.Fprintf(os.Stderr, "错误: 任务 %s 正在运行\n", t.ID)
		return 1
	}

	t.Status = queue.Running
	if err := st.Update(t); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	fmt.Printf("执行 %s (%s)...\n", t.ID, t.Name)

	res := runTaskForeground(t, st.LogPath(t.ID))

	t.Runs++
	t.LastRun = &res.FinishedAt
	t.LastExit = res.ExitCode
	if res.ExitCode == 0 {
		t.Status = queue.Done
		fmt.Printf("完成: 退出码 0\n")
	} else {
		t.Status = queue.Failed
		fmt.Printf("失败: %v\n", res.Err)
	}
	if err := st.Update(t); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	if res.ExitCode != 0 {
		return 1
	}
	return 0
}

// cmdLog shows a task's log file.
func cmdLog(args []string) int {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	tail := fs.Int("tail", 0, "只显示最后 N 行（0 = 全部）")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		return usageError(fs, "需要一个任务 ID")
	}
	id := fs.Arg(0)
	if !validID(id) {
		return usageError(fs, "无效的任务 ID %q", id)
	}
	st, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	path := st.LogPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	if *tail > 0 {
		lines := splitLines(string(data))
		if len(lines) > *tail {
			lines = lines[len(lines)-*tail:]
		}
		for _, l := range lines {
			fmt.Println(l)
		}
	} else {
		fmt.Print(string(data))
	}
	return 0
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
