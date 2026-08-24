// Package runner executes queued tasks and writes timestamped logs.
package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"belater/internal/queue"
)

// Result describes one task execution.
type Result struct {
	ExitCode   int
	Err        error
	Pid        int
	StartedAt  time.Time
	FinishedAt time.Time
}

// Run executes task t via /bin/sh -c, streaming output to logPath.
func Run(t *queue.Task, logPath string) Result {
	return RunWithWriter(t, logPath, nil)
}

// userBinDirs are common per-user binary directories that interactive shells
// add to PATH but systemd/cron environments often lack (e.g. npm-global,
// ~/.local/bin). A daemon-spawned task would otherwise fail with
// "command not found" for tools like pi.
var userBinDirs = []string{
	".local/bin",
	".npm-global/bin",
	"go/bin",
}

// buildEnv returns the task environment with user bin dirs appended to PATH
// when missing. An explicit PATH in the task env wins (no injection).
func buildEnv(t *queue.Task) []string {
	env := os.Environ()
	// Honor an explicit PATH override in the task env.
	for _, kv := range t.Env {
		if strings.HasPrefix(kv, "PATH=") {
			return append(env, t.Env...)
		}
	}
	path := os.Getenv("PATH")
	home, err := os.UserHomeDir()
	if err == nil {
		for _, sub := range userBinDirs {
			dir := filepath.Join(home, sub)
			if info, err := os.Stat(dir); err == nil && info.IsDir() && !inPath(path, dir) {
				path = dir + ":" + path
			}
		}
	}
	env = append(env, "PATH="+path)
	return append(env, t.Env...)
}

// inPath reports whether dir appears as a PATH component.
func inPath(path, dir string) bool {
	for _, p := range strings.Split(path, ":") {
		if p == dir {
			return true
		}
	}
	return false
}

// RunWithWriter is Run with an extra live-output writer (e.g. os.Stdout).
// Every log line gets a [HH:MM:SS] prefix. On timeout the process group is
// killed so child processes do not survive.
func RunWithWriter(t *queue.Task, logPath string, live io.Writer) (res Result) {
	res = Result{ExitCode: -1, StartedAt: time.Now()}
	defer func() { res.FinishedAt = time.Now() }()

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		res.Err = fmt.Errorf("open log: %w", err)
		return res
	}
	defer logFile.Close()

	ctx := context.Background()
	var cancel context.CancelFunc
	if t.TimeoutSec > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(t.TimeoutSec)*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", t.Command)
	if t.Cwd != "" {
		cmd.Dir = t.Cwd
	}
	cmd.Env = buildEnv(t)
	// Kill the whole process group on timeout.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	pr, pw, err := os.Pipe()
	if err != nil {
		res.Err = fmt.Errorf("pipe: %w", err)
		return res
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	// Prefix writer: adds [HH:MM:SS] to each line. A final line without
	// trailing newline is written exactly once (the EOF branch never re-writes).
	done := make(chan struct{})
	go func() {
		defer close(done)
		w := bufio.NewWriter(logFile)
		br := bufio.NewReader(pr)
		for {
			line, err := br.ReadString('\n')
			if len(line) > 0 {
				prefixed := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), line)
				w.WriteString(prefixed)
				if live != nil {
					io.WriteString(live, prefixed)
				}
			}
			if err != nil {
				break
			}
		}
		w.Flush()
	}()

	runErr := cmd.Run()
	// Kill the process group before draining the pipe: lingering children
	// still hold the pipe write end, so draining would block until they die.
	if runErr != nil && ctx.Err() == context.DeadlineExceeded {
		killProcessGroup(cmd)
	}
	res.Pid = cmd.Process.Pid
	pw.Close()
	// Bounded drain: an escaped child (setsid/daemonize) that holds the pipe
	// write end must not block the task forever. Close the read end after 10s.
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		pr.Close()
		<-done
	}
	pr.Close()

	if runErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			res.Err = fmt.Errorf("timeout after %ds", t.TimeoutSec)
		} else if ee, ok := runErr.(*exec.ExitError); ok {
			res.ExitCode = ee.ExitCode()
			res.Err = fmt.Errorf("exit code %d", ee.ExitCode())
		} else {
			res.Err = fmt.Errorf("run: %w", runErr)
		}
		return res
	}
	res.ExitCode = 0
	return res
}

// killProcessGroup sends SIGKILL to the process group of cmd.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid := cmd.Process.Pid
	syscall.Kill(-pgid, syscall.SIGKILL)
}
