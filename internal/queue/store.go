package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

const queueFile = "queue.json"
const lockFile = "queue.lock"

// Store persists tasks as one JSON file with atomic writes and flock.
type Store struct {
	Dir string
	mu  sync.Mutex
}

// Open prepares the state directory and returns a Store.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir %s: %w", dir, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}
	return &Store{Dir: dir}, nil
}

// path returns the queue file path.
func (s *Store) path() string { return filepath.Join(s.Dir, queueFile) }

// lock acquires an exclusive flock and runs f. The caller must already hold s.mu.
func (s *Store) lock(f func() error) error {
	lf, err := os.OpenFile(filepath.Join(s.Dir, lockFile), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("flock: %w", err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
	return f()
}

// mutate runs read-modify-write under the in-process mutex and flock.
// The read-modify-write cycle is atomic across processes.
func (s *Store) mutate(f func(tasks []*Task) ([]*Task, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lock(func() error {
		tasks, err := s.readLocked()
		if err != nil {
			return err
		}
		next, err := f(tasks)
		if err != nil {
			return err
		}
		return s.writeLocked(next)
	})
}

// readLocked loads tasks. Caller must hold s.mu and the flock.
func (s *Store) readLocked() ([]*Task, error) {
	data, err := os.ReadFile(s.path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// writeLocked persists tasks atomically. Caller must hold s.mu and the flock.
func (s *Store) writeLocked(tasks []*Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path() + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	// fsync before rename so a power loss cannot lose acknowledged state.
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path()); err != nil {
		return err
	}
	// fsync the directory so the rename itself is durable.
	dir, err := os.Open(s.Dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// Load reads all tasks. A missing queue file yields an empty slice.
func (s *Store) Load() ([]*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var tasks []*Task
	err := s.lock(func() error {
		var err error
		tasks, err = s.readLocked()
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("load queue: %w", err)
	}
	return tasks, nil
}

// Save persists the full task list. Callers must provide a complete,
// freshly loaded list to avoid clobbering concurrent mutations.
func (s *Store) Save(tasks []*Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lock(func() error {
		return s.writeLocked(tasks)
	})
}

// Add appends a task and persists.
func (s *Store) Add(t *Task) error {
	if t.ID == "" {
		t.ID = NewID()
	}
	if !ValidStatuses[t.Status] {
		return fmt.Errorf("invalid status %q", t.Status)
	}
	return s.mutate(func(tasks []*Task) ([]*Task, error) {
		return append(tasks, t), nil
	})
}

// Update replaces the task with the same ID, or errors.
func (s *Store) Update(t *Task) error {
	if !ValidStatuses[t.Status] {
		return fmt.Errorf("invalid status %q", t.Status)
	}
	return s.mutate(func(tasks []*Task) ([]*Task, error) {
		for i, cur := range tasks {
			if cur.ID == t.ID {
				tasks[i] = t
				return tasks, nil
			}
		}
		return nil, fmt.Errorf("task %s not found", t.ID)
	})
}

// Remove deletes a task by ID.
func (s *Store) Remove(id string) error {
	return s.mutate(func(tasks []*Task) ([]*Task, error) {
		kept := tasks[:0]
		found := false
		for _, t := range tasks {
			if t.ID == id {
				found = true
				continue
			}
			kept = append(kept, t)
		}
		if !found {
			return nil, fmt.Errorf("task %s not found", id)
		}
		return kept, nil
	})
}

// ClaimDue atomically transitions a task from Queued to Running.
// It is the compare-and-swap that prevents double dispatch when multiple
// daemon instances race on the same task. Returns an error when the task
// is missing or no longer Queued (another instance claimed it).
func (s *Store) ClaimDue(id string) (*Task, error) {
	var claimed *Task
	err := s.mutate(func(tasks []*Task) ([]*Task, error) {
		for i, t := range tasks {
			if t.ID == id {
				if t.Status != Queued {
					return nil, fmt.Errorf("task %s already claimed (status %s)", id, t.Status)
				}
				t.Status = Running
				claimed = tasks[i]
				return tasks, nil
			}
		}
		return nil, fmt.Errorf("task %s not found", id)
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

// CompareAndStatus atomically transitions a task from one status to another.
// Returns the updated task, or an error when the task is missing or not in
// the expected state (another process changed it first).
func (s *Store) CompareAndStatus(id string, from Status, to Status) (*Task, error) {
	var out *Task
	err := s.mutate(func(tasks []*Task) ([]*Task, error) {
		for i, t := range tasks {
			if t.ID == id {
				if t.Status != from {
					return nil, fmt.Errorf("task %s status is %s, want %s", id, t.Status, from)
				}
				t.Status = to
				out = tasks[i]
				return tasks, nil
			}
		}
		return nil, fmt.Errorf("task %s not found", id)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClaimForRun atomically claims a task for foreground execution: any status
// except Running is accepted (done/failed tasks can be re-run). Prevents a
// race where the scheduler dispatches the task while `belater run` starts it.
func (s *Store) ClaimForRun(id string) (*Task, error) {
	var out *Task
	err := s.mutate(func(tasks []*Task) ([]*Task, error) {
		for i, t := range tasks {
			if t.ID == id {
				if t.Status == Running {
					return nil, fmt.Errorf("task %s 正在运行", id)
				}
				t.Status = Running
				out = tasks[i]
				return tasks, nil
			}
		}
		return nil, fmt.Errorf("task %s not found", id)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Find returns the task with the given ID, or an error.
func (s *Store) Find(id string) (*Task, error) {
	tasks, err := s.Load()
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("task %s not found", id)
}

// LogPath returns the log file path for a task ID.
func (s *Store) LogPath(id string) string {
	return filepath.Join(s.Dir, "logs", id+".log")
}
