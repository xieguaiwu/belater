package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HeartbeatName is the file the scheduler refreshes on every dispatch tick.
const HeartbeatName = "daemon.heartbeat"

// fallbackTickSec matches the default Config.Tick, used when a heartbeat
// records no usable tick.
const fallbackTickSec = 15

// ceilSeconds converts a tick interval to whole seconds, rounding up so the
// stale-grace window is never shorter than the tick itself.
func ceilSeconds(d time.Duration) int {
	n := int(d / time.Second)
	if d%time.Second != 0 {
		n++
	}
	return n
}

// Heartbeat records what the scheduler last did, for `belater status`.
//
// The instance lock only proves that some process holds the flock. A wedged
// loop keeps the lock forever while doing nothing, and a daemon that survived
// a binary upgrade keeps running stale code. Both make the queue look healthy
// while tasks rot. The heartbeat answers the two questions the lock cannot:
// is the dispatch loop still turning, and which build is it?
type Heartbeat struct {
	// PID is the scheduler process id.
	PID int `json:"pid"`
	// Version is the belater build that wrote this file.
	Version string `json:"version"`
	// TickSec is the dispatch interval in seconds.
	TickSec int `json:"tick_sec"`
	// MaxParallel is the serial slot budget of the running scheduler.
	MaxParallel int `json:"max_parallel"`
	// Once marks a cron-style single dispatch pass.
	Once bool `json:"once"`
	// StartedAt is when the scheduler loop began.
	StartedAt time.Time `json:"started_at"`
	// LastTickAt is when the dispatch pass last ran.
	LastTickAt time.Time `json:"last_tick_at"`
}

// HeartbeatPath returns the heartbeat file path inside dir.
func HeartbeatPath(dir string) string { return filepath.Join(dir, HeartbeatName) }

// WriteHeartbeat publishes hb atomically (temp file + rename), so a reader
// never observes a half-written file.
func WriteHeartbeat(dir string, hb Heartbeat) error {
	data, err := json.Marshal(hb)
	if err != nil {
		return fmt.Errorf("encode heartbeat: %w", err)
	}
	tmp := HeartbeatPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write heartbeat: %w", err)
	}
	if err := os.Rename(tmp, HeartbeatPath(dir)); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("replace heartbeat: %w", err)
	}
	return nil
}

// ReadHeartbeat loads the heartbeat file. A missing or unusable file is an
// error: callers treat it as "no liveness data", never as a scheduler failure.
func ReadHeartbeat(dir string) (Heartbeat, error) {
	var hb Heartbeat
	data, err := os.ReadFile(HeartbeatPath(dir))
	if err != nil {
		return hb, err
	}
	if err := json.Unmarshal(data, &hb); err != nil {
		return hb, fmt.Errorf("parse heartbeat: %w", err)
	}
	if hb.LastTickAt.IsZero() {
		return hb, fmt.Errorf("heartbeat has no tick time")
	}
	return hb, nil
}

// Grace returns how late a heartbeat may run before it counts as stalled:
// three ticks plus a 30 s margin for a long dispatch pass.
func (hb Heartbeat) Grace() time.Duration {
	tick := hb.TickSec
	if tick <= 0 {
		tick = fallbackTickSec
	}
	return 3*time.Duration(tick)*time.Second + 30*time.Second
}

// Stale reports whether the dispatch loop stopped checking in.
func (hb Heartbeat) Stale(now time.Time) bool {
	return now.Sub(hb.LastTickAt) > hb.Grace()
}

// SchedulerState is what a command learns about the live scheduler.
type SchedulerState struct {
	// Running is true while some process holds the instance lock.
	Running bool
	// HB is the last heartbeat, nil when none is readable.
	HB *Heartbeat
}

// ProbeScheduler reports scheduler liveness from the instance lock plus the
// heartbeat file.
func ProbeScheduler(dir string, now time.Time) SchedulerState {
	st := SchedulerState{Running: InstanceLockHeld(dir)}
	if hb, err := ReadHeartbeat(dir); err == nil {
		st.HB = &hb
	} else {
		st.HB = nil
	}
	return st
}

// Stalled reports a held lock with a late heartbeat: the scheduler is alive but
// its loop is not turning. Without heartbeat data there is nothing to judge,
// so it reports false rather than crying wolf.
func (st SchedulerState) Stalled(now time.Time) bool {
	return st.Running && st.HB != nil && st.HB.Stale(now)
}

// VersionBehind returns the running scheduler's version when it differs from
// cliVersion — the scheduler was started before the binary was upgraded. It
// returns "" when there is nothing to report.
func (st SchedulerState) VersionBehind(cliVersion string) string {
	if !st.Running || st.HB == nil || cliVersion == "" {
		return ""
	}
	if st.HB.Version == "" || st.HB.Version == cliVersion {
		return ""
	}
	return st.HB.Version
}

// Tick returns the reported dispatch interval, or the default when unknown.
func (st SchedulerState) Tick() time.Duration {
	if st.HB != nil && st.HB.TickSec > 0 {
		return time.Duration(st.HB.TickSec) * time.Second
	}
	return fallbackTickSec * time.Second
}
