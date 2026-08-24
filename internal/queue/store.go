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

// lock acquires an exclusive flock for the duration of f.
func (s *Store) lock(f func() error) error {
	lf, err := os.OpenFile(filepath.Join(s.Dir, lockFile), os.O_CREATE|os.O_RDWR, 0o644)
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

// Load reads all tasks. A missing queue file yields an empty slice.
func (s *Store) Load() ([]*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var tasks []*Task
	err := s.lock(func() error {
		data, err := os.ReadFile(s.path())
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if len(data) == 0 {
			return nil
		}
		return json.Unmarshal(data, &tasks)
	})
	if err != nil {
		return nil, fmt.Errorf("load queue: %w", err)
	}
	return tasks, nil
}

// Save writes tasks atomically (tmp + rename) under flock.
func (s *Store) Save(tasks []*Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lock(func() error {
		data, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			return err
		}
		tmp := s.path() + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, s.path())
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
	tasks, err := s.Load()
	if err != nil {
		return err
	}
	tasks = append(tasks, t)
	return s.Save(tasks)
}

// Update replaces the task with the same ID, or errors.
func (s *Store) Update(t *Task) error {
	if !ValidStatuses[t.Status] {
		return fmt.Errorf("invalid status %q", t.Status)
	}
	tasks, err := s.Load()
	if err != nil {
		return err
	}
	for i, cur := range tasks {
		if cur.ID == t.ID {
			tasks[i] = t
			return s.Save(tasks)
		}
	}
	return fmt.Errorf("task %s not found", t.ID)
}

// Remove deletes a task by ID.
func (s *Store) Remove(id string) error {
	tasks, err := s.Load()
	if err != nil {
		return err
	}
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
		return fmt.Errorf("task %s not found", id)
	}
	return s.Save(kept)
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
