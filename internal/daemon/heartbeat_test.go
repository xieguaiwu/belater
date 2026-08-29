package daemon

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"belater/internal/queue"
)

func TestHeartbeatRoundtrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Truncate(time.Second)
	in := Heartbeat{PID: 4242, Version: "9.9.9", TickSec: 20, MaxParallel: 2, StartedAt: now, LastTickAt: now}
	if err := WriteHeartbeat(dir, in); err != nil {
		t.Fatalf("WriteHeartbeat: %v", err)
	}
	if _, err := os.Stat(HeartbeatPath(dir) + ".tmp"); err == nil {
		t.Error("temp file left behind after rename")
	}
	out, err := ReadHeartbeat(dir)
	if err != nil {
		t.Fatalf("ReadHeartbeat: %v", err)
	}
	if out.PID != in.PID || out.Version != in.Version || out.TickSec != in.TickSec || out.MaxParallel != in.MaxParallel {
		t.Errorf("heartbeat fields lost: got %+v want %+v", out, in)
	}
	if !out.LastTickAt.Equal(in.LastTickAt) {
		t.Errorf("LastTickAt = %v, want %v", out.LastTickAt, in.LastTickAt)
	}
}

func TestReadHeartbeatRejectsUnusableFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadHeartbeat(dir); err == nil {
		t.Error("missing heartbeat must be reported as no data")
	}
	if err := os.WriteFile(HeartbeatPath(dir), []byte(`{"pid":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHeartbeat(dir); err == nil {
		t.Error("heartbeat without a tick time must be rejected")
	}
	if err := os.WriteFile(HeartbeatPath(dir), []byte(`{not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadHeartbeat(dir); err == nil {
		t.Error("corrupt heartbeat must be rejected")
	}
}

func TestHeartbeatStale(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		hb   Heartbeat
		want bool
	}{
		{"fresh", Heartbeat{TickSec: 15, LastTickAt: now.Add(-30 * time.Second)}, false},
		{"wedged loop", Heartbeat{TickSec: 15, LastTickAt: now.Add(-2 * time.Minute)}, true},
		{"tick 0 falls back", Heartbeat{LastTickAt: now.Add(-2 * time.Minute)}, true},
		{"long tick tolerates its own pace", Heartbeat{TickSec: 3600, LastTickAt: now.Add(-20 * time.Minute)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.hb.Stale(now); got != c.want {
				t.Errorf("Stale() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestProbeSchedulerStates(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	// Nothing running, nothing recorded.
	st := ProbeScheduler(dir, now)
	if st.Running {
		t.Error("no process holds the lock, so status must not claim a scheduler")
	}
	if st.Stalled(now) {
		t.Error("not running can never be stalled")
	}

	unlock, err := acquireInstanceLock(dir, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	st = ProbeScheduler(dir, now)
	if !st.Running {
		t.Fatal("lock held: status must report the scheduler as running")
	}
	if st.HB != nil {
		t.Error("no heartbeat file yet, so there must be no liveness data")
	}
	// A scheduler predating heartbeats must not be called stalled.
	if st.Stalled(now) {
		t.Error("without heartbeat data status must not claim a stall")
	}
	if v := st.VersionBehind("0.2.4"); v != "" {
		t.Errorf("without heartbeat data no version may be reported, got %q", v)
	}

	// Healthy: fresh heartbeat, same version.
	if err := WriteHeartbeat(dir, Heartbeat{Version: "0.2.4", TickSec: 15, LastTickAt: now.Add(-2 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	st = ProbeScheduler(dir, now)
	if st.Stalled(now) {
		t.Error("fresh heartbeat is not stalled")
	}
	if v := st.VersionBehind("0.2.4"); v != "" {
		t.Errorf("same version must not be reported as behind, got %q", v)
	}
	if v := st.VersionBehind("0.3.0"); v != "0.2.4" {
		t.Errorf("VersionBehind(0.3.0) = %q, want 0.2.4", v)
	}
	if got := st.Tick(); got != 15*time.Second {
		t.Errorf("Tick() = %v, want 15s", got)
	}

	// Wedged: the lock is still held but the loop stopped turning.
	if err := WriteHeartbeat(dir, Heartbeat{Version: "0.2.4", TickSec: 15, LastTickAt: now.Add(-10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	st = ProbeScheduler(dir, now)
	if !st.Stalled(now) {
		t.Error("held lock + 10-minute-old heartbeat must report stalled")
	}
}

func TestCeilSeconds(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want int
	}{
		{15 * time.Second, 15},
		{50 * time.Millisecond, 1},
		{1500 * time.Millisecond, 2},
		{90 * time.Second, 90},
	}
	for _, c := range cases {
		if got := ceilSeconds(c.d); got != c.want {
			t.Errorf("ceilSeconds(%v) = %d, want %d", c.d, got, c.want)
		}
	}
}

func TestRunWritesHeartbeat(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	st, err := queue.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), Config{
		Store: st, Tick: 50 * time.Millisecond, MaxParallel: 1,
		Once: true, Version: "test-build", LogWriter: io.Discard,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	hb, err := ReadHeartbeat(dir)
	if err != nil {
		t.Fatalf("a dispatch pass must leave a heartbeat: %v", err)
	}
	if hb.Version != "test-build" {
		t.Errorf("Version = %q, want test-build", hb.Version)
	}
	if hb.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", hb.PID, os.Getpid())
	}
	if hb.TickSec < 1 {
		t.Errorf("TickSec = %d, want >= 1", hb.TickSec)
	}
	if time.Since(hb.LastTickAt) > time.Minute {
		t.Errorf("LastTickAt = %v, want recent", hb.LastTickAt)
	}
}

// TestOnceModePulsesWhileWaitingForTasks guards the false stall: cron --once
// waits for the tasks it dispatched, and a task longer than the grace window
// used to leave the heartbeat frozen — so a concurrent `belater status` called
// a busy scheduler a wedged one.
func TestOnceModePulsesWhileWaitingForTasks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	st, err := queue.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	tk := &queue.Task{
		ID: "aaaa0001", Name: "slow", Command: "sleep 2",
		RunAt: time.Now().Add(-time.Minute), Status: queue.Queued, LastExit: -1,
	}
	if err := st.Add(tk); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), Config{
			Store: st, Tick: 30 * time.Millisecond, MaxParallel: 1,
			Once: true, Version: "t", LogWriter: io.Discard,
		})
	}()

	var first time.Time
	for i := 0; i < 200 && first.IsZero(); i++ {
		hb, err := ReadHeartbeat(dir)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		first = hb.LastTickAt
	}
	if first.IsZero() {
		t.Fatal("no heartbeat appeared")
	}

	time.Sleep(300 * time.Millisecond)
	hb, err := ReadHeartbeat(dir)
	if err != nil {
		t.Fatalf("re-read heartbeat: %v", err)
	}
	if !hb.LastTickAt.After(first) {
		t.Errorf("heartbeat frozen at %v while a task runs; status would report a false stall", first)
	}
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// TestHeartbeatGraceBoundaries pins the edge: exactly at the grace window is
// still healthy, one millisecond past it is a stalled loop.
func TestHeartbeatGraceBoundaries(t *testing.T) {
	now := time.Now()
	hb := Heartbeat{TickSec: 15}
	grace := hb.Grace()
	atBoundary := Heartbeat{TickSec: 15, LastTickAt: now.Add(-grace)}
	if atBoundary.Stale(now) {
		t.Error("exactly at the grace window is not yet stale")
	}
	justPast := Heartbeat{TickSec: 15, LastTickAt: now.Add(-grace - time.Millisecond)}
	if !justPast.Stale(now) {
		t.Error("one millisecond past the grace window must count as stale")
	}
}
