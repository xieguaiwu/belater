package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"belater/internal/daemon"
	"belater/internal/queue"
	"belater/internal/runner"
)

// systemdUnit is the user service unit installed by daemon --install.
// The explicit PATH covers per-user bin dirs that systemd user environments
// omit (npm-global, ~/.local/bin); %h expands to the home directory.
const systemdUnit = `[Unit]
Description=belater off-peak task scheduler
After=network-online.target

[Service]
Type=simple
Environment=PATH=%%h/.local/bin:%%h/.npm-global/bin:%%h/go/bin:/usr/local/bin:/usr/bin:/bin
ExecStart=%s daemon
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

// cmdDaemon runs the scheduler, or manages the systemd user service.
func cmdDaemon(args []string) int {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	once := fs.Bool("once", false, "检查一次并等待任务完成，然后退出（配合 cron）")
	tick := fs.String("tick", "15s", "调度检查间隔")
	maxParallel := fs.Int("max-parallel", 1, "同时执行的串行任务数")
	install := fs.Bool("install", false, "安装 systemd 用户服务")
	uninstall := fs.Bool("uninstall", false, "卸载 systemd 用户服务")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		return usageError(fs, "不接受位置参数")
	}
	if *install && *uninstall {
		return usageError(fs, "--install 与 --uninstall 互斥")
	}

	if *install {
		return installSystemd()
	}
	if *uninstall {
		return uninstallSystemd()
	}

	tickDur, err := time.ParseDuration(*tick)
	if err != nil {
		return usageError(fs, "--tick 格式无效: %v", err)
	}

	st, err := openStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}

	if *once {
		return runOnce(st, tickDur, *maxParallel)
	}
	return runForever(st, tickDur, *maxParallel)
}

func runOnce(st *queue.Store, tick time.Duration, maxParallel int) int {
	// Pre-check the instance lock: when another scheduler (systemd service,
	// another --once, or a foreground daemon) already runs, this pass is a
	// no-op — due tasks are handled by that instance. Exit 0 so cron never
	// sees a spurious failure. Run re-checks atomically as a safety net.
	if daemon.InstanceLockHeld(stateDir()) {
		fmt.Fprintln(os.Stdout, lockActiveHint())
		return 0
	}
	fmt.Printf("belater daemon --once: 检查队列 (tick %s, max-parallel %d)\n", tick, maxParallel)
	err := daemon.Run(context.Background(), daemon.Config{
		Store:       st,
		Schedule:    defaultSchedule(),
		Tick:        tick,
		MaxParallel: maxParallel,
		Once:        true,
		LogWriter:   os.Stdout,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	return 0
}

func runForever(st *queue.Store, tick time.Duration, maxParallel int) int {
	// Pre-check the instance lock: a manual `belater daemon` while the
	// systemd service (or another instance) runs is a normal state — the
	// queue is already being serviced. Inform and exit 0 instead of
	// reporting an error.
	if daemon.InstanceLockHeld(stateDir()) {
		fmt.Fprintln(os.Stdout, lockActiveHint())
		return 0
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	fmt.Printf("belater daemon: 调度器运行中 (tick %s, max-parallel %d) — Ctrl+C 退出\n", tick, maxParallel)
	err := daemon.Run(ctx, daemon.Config{
		Store:       st,
		Schedule:    defaultSchedule(),
		Tick:        tick,
		MaxParallel: maxParallel,
		LogWriter:   os.Stdout,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	fmt.Println("调度器已退出")
	return 0
}

// lockActiveHint is the informational message shown when another scheduler
// instance already holds daemon.lock. This is a normal state — the queue is
// being serviced — so the message is neutral and the exit code is 0.
func lockActiveHint() string {
	return "调度器已在运行（daemon.lock 被占用）— 任务会由现有调度器自动执行，无需再次启动。\n" +
		"  查看: belater status 或 systemctl --user status belater\n" +
		"  停止服务: systemctl --user stop belater 或 belater daemon --uninstall"
}

// installSystemd writes the user unit, reloads, and enables the service.
func installSystemd() int {
	bin, err := os.Executable()
	if err != nil {
		bin = filepath.Join(mustHome(), ".local", "bin", "belater")
	}
	binAbs, err := filepath.Abs(bin)
	if err != nil {
		binAbs = bin
	}

	unitDir := filepath.Join(mustHome(), ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	unitPath := filepath.Join(unitDir, "belater.service")
	unit := fmt.Sprintf(systemdUnit, binAbs)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}

	if err := systemctl("daemon-reload"); err != nil {
		fmt.Fprintf(os.Stderr, "警告: daemon-reload 失败: %v\n", err)
	}
	if err := systemctl("enable", "--now", "belater.service"); err != nil {
		fmt.Fprintf(os.Stderr, "警告: enable --now 失败: %v\n", err)
		return 1
	}
	fmt.Printf("已安装并启动: %s\n", unitPath)
	return 0
}

// uninstallSystemd stops and removes the user unit.
func uninstallSystemd() int {
	if err := systemctl("disable", "--now", "belater.service"); err != nil {
		fmt.Fprintf(os.Stderr, "警告: disable --now 失败: %v\n", err)
	}
	unitPath := filepath.Join(mustHome(), ".config", "systemd", "user", "belater.service")
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "警告: 删除 unit 失败: %v\n", err)
	}
	if err := systemctl("daemon-reload"); err != nil {
		fmt.Fprintf(os.Stderr, "警告: daemon-reload 失败: %v\n", err)
	}
	fmt.Println("已卸载 belater 服务")
	return 0
}

func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

func mustHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

// runTaskForeground runs a task with live output to stdout.
func runTaskForeground(t *queue.Task, logPath string) runner.Result {
	return runner.RunWithWriter(t, logPath, os.Stdout)
}
