package queue

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "belater-test"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func sampleTask() *Task {
	now := time.Date(2026, 8, 24, 22, 0, 0, 0, time.UTC)
	return &Task{
		ID: "abc12345", Name: "测试", Command: "echo hi",
		RunAt: now, Status: Queued, CreatedAt: now, LastExit: -1,
	}
}

func TestLoadEmpty(t *testing.T) {
	s := newTestStore(t)
	tasks, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected empty, got %d", len(tasks))
	}
}

func TestAddLoadRoundtrip(t *testing.T) {
	s := newTestStore(t)
	if err := s.Add(sampleTask()); err != nil {
		t.Fatalf("Add: %v", err)
	}
	t2 := sampleTask()
	t2.ID = "deadbeef"
	if err := s.Add(t2); err != nil {
		t.Fatalf("Add 2: %v", err)
	}
	tasks, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	got := tasks[0]
	if got.ID != "abc12345" || got.Name != "测试" || got.Command != "echo hi" {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
	if !got.RunAt.Equal(sampleTask().RunAt) {
		t.Errorf("RunAt mismatch: %v", got.RunAt)
	}
	if got.LastExit != -1 || got.Status != Queued {
		t.Errorf("field mismatch: %+v", got)
	}
}

func TestUpdateAndRemove(t *testing.T) {
	s := newTestStore(t)
	if err := s.Add(sampleTask()); err != nil {
		t.Fatalf("Add: %v", err)
	}
	upd := sampleTask()
	upd.Status = Done
	upd.LastExit = 0
	if err := s.Update(upd); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := s.Find("abc12345")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Status != Done || got.LastExit != 0 {
		t.Errorf("update not persisted: %+v", got)
	}
	if err := s.Remove("abc12345"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	tasks, _ := s.Load()
	if len(tasks) != 0 {
		t.Errorf("expected empty after remove, got %d", len(tasks))
	}
}

func TestNoTmpLeftover(t *testing.T) {
	s := newTestStore(t)
	if err := s.Add(sampleTask()); err != nil {
		t.Fatalf("Add: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(s.Dir, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("leftover tmp files: %v", matches)
	}
}

func TestConcurrentAdds(t *testing.T) {
	s := newTestStore(t)
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				tk := sampleTask()
				tk.ID = ""
				tk.Name = "t"
				if err := s.Add(tk); err != nil {
					t.Errorf("Add: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	tasks, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(tasks) != 200 {
		t.Errorf("expected 200 tasks, got %d", len(tasks))
	}
}

func TestInvalidStatusRejected(t *testing.T) {
	s := newTestStore(t)
	bad := sampleTask()
	bad.Status = Status("bogus")
	if err := s.Add(bad); err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestEnvRoundtrip(t *testing.T) {
	s := newTestStore(t)
	tk := sampleTask()
	tk.Env = []string{"FOO=bar", "BAZ=qux"}
	if err := s.Add(tk); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got, err := s.Find("abc12345")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got.Env) != 2 || got.Env[0] != "FOO=bar" {
		t.Errorf("env mismatch: %v", got.Env)
	}
}

func TestLogPath(t *testing.T) {
	s := newTestStore(t)
	p := s.LogPath("abc12345")
	if filepath.Base(p) != "abc12345.log" {
		t.Errorf("LogPath = %s", p)
	}
	if _, err := os.Stat(filepath.Dir(p)); err != nil {
		t.Errorf("logs dir missing: %v", err)
	}
}
