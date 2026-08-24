package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"belater/internal/queue"
)

func runTask(t *testing.T, task *queue.Task) (Result, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "task.log")
	res := Run(task, logPath)
	data, _ := os.ReadFile(logPath)
	return res, string(data)
}

func TestRunSuccess(t *testing.T) {
	res, log := runTask(t, &queue.Task{Command: "echo hello"})
	if res.ExitCode != 0 || res.Err != nil {
		t.Fatalf("expected success, got %+v", res)
	}
	if !strings.Contains(log, "hello") {
		t.Errorf("log missing output: %q", log)
	}
	if !strings.Contains(log, "[") || !strings.Contains(log, "]") {
		t.Errorf("log missing timestamp prefix: %q", log)
	}
}

func TestRunExitCode(t *testing.T) {
	res, _ := runTask(t, &queue.Task{Command: "exit 3"})
	if res.ExitCode != 3 {
		t.Errorf("expected exit 3, got %d (%v)", res.ExitCode, res.Err)
	}
}

func TestRunCwd(t *testing.T) {
	dir := t.TempDir()
	res, log := runTask(t, &queue.Task{Command: "pwd", Cwd: dir})
	if res.ExitCode != 0 {
		t.Fatalf("failed: %+v", res)
	}
	if !strings.Contains(log, dir) {
		t.Errorf("log missing cwd %s: %q", dir, log)
	}
}

func TestRunEnv(t *testing.T) {
	res, log := runTask(t, &queue.Task{Command: "echo $BELATER_TEST_VAR", Env: []string{"BELATER_TEST_VAR=bar"}})
	if res.ExitCode != 0 {
		t.Fatalf("failed: %+v", res)
	}
	if !strings.Contains(log, "bar") {
		t.Errorf("log missing env value: %q", log)
	}
}

func TestRunTimeout(t *testing.T) {
	start := time.Now()
	res, _ := runTask(t, &queue.Task{Command: "sleep 10", TimeoutSec: 1})
	elapsed := time.Since(start)
	if res.ExitCode == 0 {
		t.Error("expected non-zero exit on timeout")
	}
	if elapsed > 4*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
	if !strings.Contains(res.Err.Error(), "timeout") {
		t.Errorf("expected timeout error, got %v", res.Err)
	}
}

func TestRunChildProcessKilled(t *testing.T) {
	// The child spawns a grandchild that writes a marker file after 3s.
	// Timeout at 1s must kill the whole group, so the file never appears.
	marker := filepath.Join(t.TempDir(), "marker")
	cmd := "(sleep 3; touch " + marker + ") & wait"
	res, _ := runTask(t, &queue.Task{Command: cmd, TimeoutSec: 1})
	if res.ExitCode == 0 {
		t.Error("expected non-zero exit on timeout")
	}
	time.Sleep(3500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("grandchild survived: marker file created after timeout")
	}
}

func TestRunNoDuplicateLastLine(t *testing.T) {
	// A final line without trailing newline must appear exactly once.
	_, log := runTask(t, &queue.Task{Command: "printf no-newline"})
	n := strings.Count(log, "no-newline")
	if n != 1 {
		t.Errorf("last line duplicated: found %d occurrences in %q", n, log)
	}
}

func TestRunEscapedChildBoundedDrain(t *testing.T) {
	// A setsid child escapes the process group; the pipe drain must still
	// terminate within a bounded time (10s cap) instead of hanging forever.
	start := time.Now()
	res, _ := runTask(t, &queue.Task{Command: "setsid sh -c 'sleep 5' & exit 0", TimeoutSec: 1})
	elapsed := time.Since(start)
	if res.ExitCode != 0 {
		t.Errorf("main command exited 0, got %+v", res)
	}
	if elapsed > 15*time.Second {
		t.Errorf("drain took too long: %v", elapsed)
	}
}
