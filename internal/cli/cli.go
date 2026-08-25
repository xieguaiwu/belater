// Package cli implements the belater command-line interface.
package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"belater/internal/queue"
	"belater/internal/schedule"
)

// Version is the tool version.
const Version = "0.2.1"

// Main dispatches subcommands and returns the process exit code.
func Main(args []string) int {
	if len(args) == 0 {
		printHelp(os.Stdout)
		return 0
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "status":
		return cmdStatus(rest)
	case "add":
		return cmdAdd(rest)
	case "list":
		return cmdList(rest)
	case "results":
		return cmdResults(rest)
	case "rm":
		return cmdRemove(rest)
	case "pause":
		return cmdPause(rest)
	case "resume":
		return cmdResume(rest)
	case "run":
		return cmdRun(rest)
	case "log":
		return cmdLog(rest)
	case "daemon":
		return cmdDaemon(rest)
	case "config":
		return cmdConfig(rest)
	case "help", "-h", "--help":
		printHelp(os.Stdout)
		return 0
	case "version", "--version", "-v":
		fmt.Printf("belater %s\n", Version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "未知命令 %q\n\n", cmd)
		printHelp(os.Stderr)
		return 2
	}
}

func printHelp(w *os.File) {
	fmt.Fprintf(w, `belater — DeepSeek 峰谷计费延迟执行器 v%s

把大型任务（如 pi-agent 多 agent 工作）排到空闲时段自动执行。
空闲时段价格 = 高峰的 50%%。高峰：周一至周五 09:00-12:00、14:00-18:00（北京时间）；周末全天空闲。

用法:
  belater status              查看当前计费窗口与队列概况
  belater add <命令...>       入队任务（默认下次空闲时段执行）
  belater list                查看队列
  belater results              查看已完成任务的结果汇总
  belater rm <id>             删除任务
  belater pause <id>          暂停任务
  belater resume <id>         恢复任务
  belater run <id>            立即前台执行
  belater log <id>            查看任务日志
  belater daemon              运行调度器（前台）
  belater daemon --install    安装为 systemd 用户服务
  belater config              显示调度配置
  belater version             显示版本

常用示例:
  belater add --pi "重构 auth 模块并补充测试"
  belater add --delay 2h ./long_task.sh
  belater add --every 02:00 --max-runs 7 --pi "生成今日代码审查报告"
  belater add --notify --pi "跑完整测试矩阵"   # 完成后桌面通知
  belater daemon --once       # 配合 cron 每 5 分钟检查一次

环境变量:
  BELATER_STATE_DIR           状态目录（默认 ~/.config/belater）

详细帮助: belater <命令> --help
`, Version)
}

// stateDir returns the state directory.
func stateDir() string {
	if d := os.Getenv("BELATER_STATE_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".belater"
	}
	return filepath.Join(home, ".config", "belater")
}

// openStore opens the queue store in the state dir.
func openStore() (*queue.Store, error) {
	return queue.Open(stateDir())
}

// defaultSchedule returns the DeepSeek billing schedule.
func defaultSchedule() *schedule.Schedule {
	return schedule.DefaultSchedule()
}

// usageError prints a message and returns exit code 2.
func usageError(fs *flag.FlagSet, format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "错误: "+format+"\n", a...)
	fmt.Fprintf(os.Stderr, "用法: belater %s\n", fs.Name())
	fs.PrintDefaults()
	return 2
}

// fmtDuration renders a duration as "Xd Yh Zm" (or "Xs" when under a minute).
func fmtDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	d = d.Round(time.Minute)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 || days > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if mins > 0 {
		parts = append(parts, fmt.Sprintf("%dm", mins))
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, " ")
}

// fmtRunAt renders a scheduled time in local time.
func fmtRunAt(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04")
}

// fmtName shortens a task name for tables.
func fmtName(name string) string {
	r := []rune(name)
	if len(r) > 28 {
		return string(r[:27]) + "…"
	}
	return name
}
