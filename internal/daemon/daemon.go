// Package daemon runs the scheduler loop that dispatches due tasks.
package daemon

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
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

	if err := recoverInterrupted(cfg.Store, logger); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for {
		dispatchOnce(cfg.Store, cfg.MaxParallel, &wg, logger)

		if cfg.Once {
			break
		}

		select {
		case <-ctx.Done():
			logger.Printf("shutdown: waiting for %d running tasks (max 5m)", runningCount(cfg.Store))
			waitRunning(&wg, 5*time.Minute)
			return nil
		case <-time.After(cfg.Tick):
		}
	}

	// Once mode: wait for running tasks, bounded.
	waitRunning(&wg, 10*time.Minute)
	return nil
}

// recoverInterrupted resets tasks left in Running state by a crashed daemon.
func recoverInterrupted(s *queue.Store, logger *log.Logger) error {
	tasks, err := s.Load()
	if err != nil {
		return fmt.Errorf("load queue: %w", err)
	}
	changed := false
	for _, t := range tasks {
		if t.Status == queue.Running {
			t.Status = queue.Queued
			changed = true
			logger.Printf("recovered interrupted task %s (%s)", t.ID, t.Name)
		}
	}
	if changed {
		return s.Save(tasks)
	}
	return nil
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
func waitRunning(wg *sync.WaitGroup, timeout time.Duration) {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// dispatchOnce launches all due tasks that fit within MaxParallel.
// Completion handlers run in goroutines and persist via the store.
func dispatchOnce(s *queue.Store, maxParallel int, wg *sync.WaitGroup, logger *log.Logger) {
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
	var runningWg sync.WaitGroup
	for _, t := range tasks {
		if t.Status == queue.Running && !t.Parallel {
			serialRunning++
		}
	}

	// 3. Dispatch due tasks.
	for _, t := range tasks {
		if t.Status != queue.Queued || t.RunAt.After(now) {
			continue
		}
		if !t.Parallel && serialRunning >= maxParallel {
			continue // serial slot budget exhausted
		}
		serialRunning++
		t.Status = queue.Running
		if err := s.Update(t); err != nil {
			logger.Printf("mark running %s: %v", t.ID, err)
			continue
		}
		wg.Add(1)
		runningWg.Add(1)
		go func(t *queue.Task) {
			defer wg.Done()
			defer runningWg.Done()
			finishTask(s, t, logger)
		}(t)
	}
}

// finishTask runs the task and persists the outcome with retry/Every logic.
func finishTask(s *queue.Store, t *queue.Task, logger *log.Logger) {
	res := runner.Run(t, s.LogPath(t.ID))
	logger.Printf("task %s (%s) finished: exit=%d err=%v", t.ID, t.Name, res.ExitCode, res.Err)

	t.Runs++
	t.LastRun = &res.FinishedAt
	t.LastExit = res.ExitCode

	switch {
	case res.ExitCode == 0:
		if t.Every != "" && (t.MaxRuns == 0 || t.Runs < t.MaxRuns) {
			next := nextEvery(res.FinishedAt, t.Every)
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
		delay := 30 * time.Second * time.Duration(1<<(t.Runs-1))
		t.RunAt = time.Now().Add(delay)
		t.Status = queue.Queued
		logger.Printf("task %s will retry in %v (attempt %d/%d)", t.ID, delay, t.Runs, t.Retries)
	default:
		t.Status = queue.Failed
	}

	if err := s.Update(t); err != nil {
		logger.Printf("persist outcome %s: %v", t.ID, err)
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

// nextEvery computes the next occurrence of "HH:MM" strictly after t.
// Returns zero time when the format is invalid.
func nextEvery(t time.Time, every string) time.Time {
	loc := t.Location()
	var hh, mm int
	if _, err := fmt.Sscanf(every, "%d:%d", &hh, &mm); err != nil {
		return time.Time{}
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return time.Time{}
	}
	candidate := time.Date(t.Year(), t.Month(), t.Day(), hh, mm, 0, 0, loc)
	if !candidate.After(t) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}
