// Package runner executes queued tasks and writes timestamped logs.
package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"belater/internal/queue"
)

// Result describes one task execution.
type Result struct {
	ExitCode   int
	Err        error
	StartedAt  time.Time
	FinishedAt time.Time
}

// Run executes task t via /bin/sh -c, streaming output to logPath.
// Every log line gets a [HH:MM:SS] prefix. On timeout the process group is
// killed so child processes do not survive.
func Run(t *queue.Task, logPath string) Result {
	res := Result{ExitCode: -1, StartedAt: time.Now()}
	defer func() { res.FinishedAt = time.Now() }()

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
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
	cmd.Env = append(os.Environ(), t.Env...)
	// Kill the whole process group on timeout.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	pr, pw, err := os.Pipe()
	if err != nil {
		res.Err = fmt.Errorf("pipe: %w", err)
		return res
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	// Prefix writer: adds [HH:MM:SS] to each line.
	done := make(chan struct{})
	go func() {
		defer close(done)
		w := bufio.NewWriter(logFile)
		br := bufio.NewReader(pr)
		for {
			line, err := br.ReadString('\n')
			if len(line) > 0 {
				fmt.Fprintf(w, "[%s] %s", time.Now().Format("15:04:05"), line)
			}
			if err != nil {
				if line := stringsTrimSuffix(line); line != "" && err == io.EOF {
					fmt.Fprintf(w, "[%s] %s", time.Now().Format("15:04:05"), line)
				}
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
	pw.Close()
	<-done
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

// stringsTrimSuffix removes a trailing newline (local helper to keep io loop simple).
func stringsTrimSuffix(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// killProcessGroup sends SIGKILL to the process group of cmd.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid := cmd.Process.Pid
	syscall.Kill(-pgid, syscall.SIGKILL)
}
