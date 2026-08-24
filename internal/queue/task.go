package queue

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Status is a task lifecycle state.
type Status string

const (
	// Queued waits for the scheduler.
	Queued Status = "queued"
	// Running is executing right now.
	Running Status = "running"
	// Done finished with exit code 0.
	Done Status = "done"
	// Failed finished with a non-zero exit code (or timeout).
	Failed Status = "failed"
	// Paused is skipped by the scheduler until resumed.
	Paused Status = "paused"
	// Cancelled was removed by the user before execution.
	Cancelled Status = "cancelled"
)

// ValidStatuses is the set of legal Status values.
var ValidStatuses = map[Status]bool{
	Queued: true, Running: true, Done: true,
	Failed: true, Paused: true, Cancelled: true,
}

// Task is one queued command.
type Task struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Command     string     `json:"command"`
	Cwd         string     `json:"cwd"`
	Env         []string   `json:"env"`
	RunAt       time.Time  `json:"run_at"`
	Every       string     `json:"every"`             // "HH:MM" daily repeat, empty = once
	MaxRuns     int        `json:"max_runs"`          // 0 = unlimited (for Every tasks)
	Runs        int        `json:"runs"`              // completed executions
	Retries     int        `json:"retries"`           // extra attempts after a failure
	TimeoutSec  int        `json:"timeout_sec"`       // 0 = no timeout
	Parallel    bool       `json:"parallel"`          // does not occupy a serial slot
	OnDone      string     `json:"on_done,omitempty"` // command run after completion (notifications)
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	LastExit    int        `json:"last_exit"`              // -1 = never run
	DurationSec int        `json:"duration_sec,omitempty"` // last execution duration
	LastPid     int        `json:"last_pid,omitempty"`     // pid of the last execution (crash recovery)
}

// NewID returns an 8-hex-char random task ID.
func NewID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	// crypto/rand failure is fatal in practice; fall back to time-based.
	return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
}
