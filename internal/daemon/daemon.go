// Package daemon runs the scheduler loop that dispatches due tasks.
package daemon

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"belater/internal/queue"
	"belater/internal/runner"
	"belater/internal/schedule"
)

// Config configures the scheduler.
type Config struct {
	Store       *queue.Store
	Schedule    *schedule.Schedule
	Tick        time.Duration // dispatch interval
	MaxParallel int           // concurrent non-parallel tasks
	Once        bool          // one dispatch pass, then wait and exit
	LogWriter   io.Writer     // daemon log destination
	Version     string        // build version, recorded in the heartbeat
}

// Run starts the scheduler loop until ctx is cancelled.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Tick <= 0 {
		cfg.Tick = 15 * time.Second
	}
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 1
	}
	lw := cfg.LogWriter
	if lw == nil {
		lw = io.Discard
	}
	logger := log.New(lw, "", log.LstdFlags)

	// Single-instance lock: a second daemon (systemd + cron --once, or a
	// manual foreground run) must not dispatch the same tasks twice.
	unlock, err := acquireInstanceLock(cfg.Store.Dir, logger)
	if err != nil {
		return err
	}
	defer unlock()

	if err := recoverInterrupted(cfg.Store, logger); err != nil {
		return err
	}

	sch := cfg.Schedule
	if sch == nil {
		sch = schedule.DefaultSchedule()
	}

	// Liveness pulse for `belater status`: once at startup, then after every
	// dispatch pass. A failure here must never stop the scheduler, so it is
	// logged once and swallowed.
	hb := Heartbeat{
		PID:         os.Getpid(),
		Version:     cfg.Version,
		TickSec:     ceilSeconds(cfg.Tick),
		MaxParallel: cfg.MaxParallel,
		Once:        cfg.Once,
		StartedAt:   time.Now(),
	}
	var hbFailed bool
	var hbMu sync.Mutex
	pulse := func() {
		hbMu.Lock()
		hb.LastTickAt = time.Now()
		snapshot := hb
		hbMu.Unlock()
		if err := WriteHeartbeat(cfg.Store.Dir, snapshot); err != nil && !hbFailed {
			hbFailed = true
			logger.Printf("heartbeat 写入失败（不影响执行，后续不再重复记录）: %v", err)
		}
	}
	pulse()

	var wg sync.WaitGroup
	for {
		dispatchOnce(cfg.Store, sch, cfg.MaxParallel, &wg, logger)
		pulse()

		if cfg.Once {
			break
		}

		select {
		case <-ctx.Done():
			logger.Printf("shutdown: waiting for %d running tasks (max 5m)", runningCount(cfg.Store))
			waitRunning(&wg, 5*time.Minute, cfg.Tick, pulse)
			return nil
		case <-time.After(cfg.Tick):
		}
	}

	// Once mode: wait for running tasks, bounded.
	// The wait must keep pulsing: a task longer than the grace window is busy
	// work, not a wedged loop, and `belater status` must not say otherwise.
	waitRunning(&wg, 10*time.Minute, cfg.Tick, pulse)
	return nil
}

// acquireInstanceLock takes an exclusive non-blocking flock on daemon.lock.
// The returned release func drops the lock. Fails when another instance runs.
func acquireInstanceLock(dir string, logger *log.Logger) (func(), error) {
	path := filepath.Join(dir, "daemon.lock")
	lf, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open instance lock: %w", err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lf.Close()
		return nil, fmt.Errorf("另一个 belater daemon 实例正在运行（%s 被占用）\n  任务会由现有调度器自动执行，无需再次启动。\n  查看: belater status 或 systemctl --user status belater\n  停止服务: systemctl --user stop belater 或 belater daemon --uninstall", path)
	}
	logger.Printf("instance lock acquired: %s", path)
	return func() {
		syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
		lf.Close()
	}, nil
}

// InstanceLockHeld reports whether another daemon currently holds the
// instance lock. Used by `belater status` to show scheduler liveness.
func InstanceLockHeld(dir string) bool {
	path := filepath.Join(dir, "daemon.lock")
	lf, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false
	}
	defer lf.Close()
	err = syscall.Flock(int(lf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		// Lock acquired: nobody holds it. Release immediately.
		syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
		return false
	}
	return true
}

// recoverInterrupted resets tasks left in Running state by a crashed daemon.
// Tasks whose last pid is still alive are marked Failed (the old process may
// still be running) instead of re-run, to avoid double execution.
func recoverInterrupted(s *queue.Store, logger *log.Logger) error {
	tasks, err := s.Load()
	if err != nil {
		// A corrupt queue must not wedge the daemon into a restart loop:
		// quarantine the file and continue with an empty queue.
		bad := filepath.Join(s.Dir, "queue.json.bad-"+time.Now().Format("20060102-150405"))
		if rerr := os.Rename(filepath.Join(s.Dir, "queue.json"), bad); rerr == nil {
			logger.Printf("queue.json 损坏，已隔离到 %s（队列重置为空）", bad)
			return nil
		}
		return fmt.Errorf("load queue: %w", err)
	}
	changed := false
	for _, t := range tasks {
		if t.Status != queue.Running {
			continue
		}
		if t.LastPid > 0 && pidAlive(t.LastPid) {
			t.Status = queue.Failed
			t.LastExit = -1
			logger.Printf("task %s (%s) 孤儿进程 pid=%d 仍存活，标记 Failed 而非重跑", t.ID, t.Name, t.LastPid)
		} else {
			t.Status = queue.Queued
			logger.Printf("recovered interrupted task %s (%s)", t.ID, t.Name)
		}
		changed = true
	}
	if changed {
		return s.Save(tasks)
	}
	return nil
}

// pidAlive reports whether a process with the given pid exists.
func pidAlive(pid int) bool {
	_, err := os.Stat(filepath.Join("/proc", fmt.Sprintf("%d", pid)))
	return err == nil
}

// runningCount counts tasks in Running state.
func runningCount(s *queue.Store) int {
	tasks, err := s.Load()
	if err != nil {
		return 0
	}
	n := 0
	for _, t := range tasks {
		if t.Status == queue.Running {
			n++
		}
	}
	return n
}

// waitRunning waits for all Running tasks to finish, bounded by timeout.
// onBeat, when given, runs every interval for the duration of the wait, so a
// scheduler occupied by a long task keeps reporting itself as alive.
func waitRunning(wg *sync.WaitGroup, timeout, interval time.Duration, onBeat func()) {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	if onBeat == nil || interval <= 0 {
		select {
		case <-done:
		case <-time.After(timeout):
		}
		return
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	onBeat()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			onBeat()
		case <-timer.C:
			return
		}
	}
}

// dispatchOnce launches all due tasks that fit within MaxParallel.
// Claiming is a compare-and-swap (Store.ClaimDue), so a racing instance
// cannot double-dispatch: the loser gets an error and skips the task.
func dispatchOnce(s *queue.Store, sch *schedule.Schedule, maxParallel int, wg *sync.WaitGroup, logger *log.Logger) {
	tasks, err := s.Load()
	if err != nil {
		logger.Printf("load queue: %v", err)
		return
	}
	now := time.Now()

	// 1. Garbage collect old finished tasks (older than 7 days).
	if err := gc(s, tasks, logger); err != nil {
		logger.Printf("gc: %v", err)
	}

	// 2. Count running non-parallel tasks to enforce the serial slot budget.
	serialRunning := 0
	for _, t := range tasks {
		if t.Status == queue.Running && !t.Parallel {
			serialRunning++
		}
	}

	// 3. Dispatch due tasks via CAS claim.
	for _, t := range tasks {
		if t.Status != queue.Queued || t.RunAt.After(now) {
			continue
		}
		if !t.Parallel && serialRunning >= maxParallel {
			continue // serial slot budget exhausted
		}
		serialRunning++
		claimed, err := s.ClaimDue(t.ID)
		if err != nil {
			// Another instance claimed it in the meantime; do not run twice.
			logger.Printf("claim %s: %v (skip)", t.ID, err)
			continue
		}
		wg.Add(1)
		go func(t *queue.Task) {
			defer wg.Done()
			finishTask(s, sch, t, logger)
		}(claimed)
	}
}

// finishTask runs the task and persists the outcome with retry/Every logic.
// When the task has an OnDone hook, it runs after the outcome is persisted
// with BELATER_TASK_* env vars (ID, NAME, STATUS, EXIT).
func finishTask(s *queue.Store, sch *schedule.Schedule, t *queue.Task, logger *log.Logger) {
	res := runner.Run(t, s.LogPath(t.ID))
	logger.Printf("task %s (%s) finished: exit=%d err=%v", t.ID, t.Name, res.ExitCode, res.Err)

	t.Runs++
	t.LastRun = &res.FinishedAt
	t.LastExit = res.ExitCode
	t.DurationSec = int(res.FinishedAt.Sub(res.StartedAt).Seconds())
	t.LastPid = res.Pid

	switch {
	case res.ExitCode == 0:
		if t.Every != "" && (t.MaxRuns == 0 || t.Runs < t.MaxRuns) {
			next := nextEvery(res.FinishedAt, t.Every, sch)
			if next.IsZero() {
				logger.Printf("task %s: invalid every %q, marking done", t.ID, t.Every)
				t.Status = queue.Done
			} else {
				t.RunAt = next
				t.Status = queue.Queued
				logger.Printf("task %s rescheduled to %v", t.ID, next)
			}
		} else {
			t.Status = queue.Done
		}
	case t.Runs <= t.Retries:
		// Retry with exponential backoff: 30s, 60s, 120s, ...
		// Clamp the retry time into an off-peak window so a failure at the
		// tail of an off-peak window never retries at full peak price.
		delay := 30 * time.Second * time.Duration(1<<(t.Runs-1))
		retryAt := sch.NextOffPeakStart(time.Now().Add(delay))
		t.RunAt = retryAt
		t.Status = queue.Queued
		logger.Printf("task %s will retry at %v (attempt %d/%d)", t.ID, retryAt, t.Runs, t.Retries)
	default:
		t.Status = queue.Failed
	}

	if err := s.Update(t); err != nil {
		logger.Printf("persist outcome %s: %v", t.ID, err)
	}

	// Notify hook runs after state is persisted; its own failure is logged
	// but never changes the task outcome.
	if t.OnDone != "" {
		RunOnDoneHook(t, s.LogPath(t.ID), logger)
	}
}

// RunOnDoneHook executes the task's OnDone hook with BELATER_TASK_* env vars.
// Exported so the foreground `belater run` path reuses the same hook logic.
func RunOnDoneHook(t *queue.Task, logPath string, logger *log.Logger) {
	hook := exec.Command("/bin/sh", "-c", t.OnDone)
	env := append(os.Environ(),
		"BELATER_TASK_ID="+t.ID,
		"BELATER_TASK_NAME="+t.Name,
		"BELATER_TASK_STATUS="+string(t.Status),
		fmt.Sprintf("BELATER_TASK_EXIT=%d", t.LastExit),
	)
	hook.Env = env
	if t.Cwd != "" {
		hook.Dir = t.Cwd
	}
	out, err := hook.CombinedOutput()
	if err != nil {
		logger.Printf("on-done hook %s failed: %v: %s", t.ID, err, strings.TrimSpace(string(out)))
		return
	}
	if len(out) > 0 {
		logger.Printf("on-done hook %s: %s", t.ID, strings.TrimSpace(string(out)))
	}
}

// gc removes finished tasks (done/failed/cancelled) older than 7 days.
func gc(s *queue.Store, tasks []*queue.Task, logger *log.Logger) error {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	kept := tasks[:0]
	removed := 0
	for _, t := range tasks {
		terminal := t.Status == queue.Done || t.Status == queue.Failed || t.Status == queue.Cancelled
		if terminal && t.CreatedAt.Before(cutoff) {
			removed++
			continue
		}
		kept = append(kept, t)
	}
	if removed > 0 {
		logger.Printf("gc: removed %d finished tasks", removed)
		return s.Save(kept)
	}
	return nil
}

// nextEvery computes the next occurrence of "HH:MM" strictly after t,
// interpreted in the billing timezone. Returns zero time when invalid.
func nextEvery(t time.Time, every string, sch *schedule.Schedule) time.Time {
	loc := sch.Location
	var hh, mm int
	if _, err := fmt.Sscanf(every, "%d:%d", &hh, &mm); err != nil {
		return time.Time{}
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return time.Time{}
	}
	// Convert t into the billing timezone before picking the date.
	lt := t.In(loc)
	candidate := time.Date(lt.Year(), lt.Month(), lt.Day(), hh, mm, 0, 0, loc)
	if !candidate.After(t) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}
