package queue

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
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

func TestCompareAndStatus(t *testing.T) {
	s := newTestStore(t)
	tk := sampleTask()
	tk.Status = Queued
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	got, err := s.CompareAndStatus("abc12345", Queued, Paused)
	if err != nil {
		t.Fatalf("CAS queued->paused: %v", err)
	}
	if got.Status != Paused {
		t.Errorf("status = %s, want paused", got.Status)
	}
	// Wrong source state must fail.
	if _, err := s.CompareAndStatus("abc12345", Queued, Running); err == nil {
		t.Error("CAS from wrong state must fail")
	}
	// Missing task must fail.
	if _, err := s.CompareAndStatus("ffffffff", Queued, Running); err == nil {
		t.Error("CAS on missing task must fail")
	}
}

func TestClaimForRun(t *testing.T) {
	s := newTestStore(t)
	for i, st := range []Status{Queued, Done, Failed, Paused} {
		tk := sampleTask()
		tk.ID = NewID()
		tk.Status = st
		if err := s.Add(tk); err != nil {
			t.Fatal(err)
		}
		got, err := s.ClaimForRun(tk.ID)
		if err != nil {
			t.Fatalf("claim %s: %v", st, err)
		}
		if got.Status != Running {
			t.Errorf("claim from %s: status = %s", st, got.Status)
		}
		_ = i
	}
	// Claiming a running task must fail.
	running := sampleTask()
	running.ID = NewID()
	running.Status = Running
	if err := s.Add(running); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimForRun(running.ID); err == nil {
		t.Error("claiming a running task must fail")
	}
}

// TestLoadDoesNotWaitForTheWriteLock keeps reads off the write lock. The flock
// is per open file description, so this second handle is a real contender —
// exactly what the scheduler holds for each mutation. Blocking here is what
// turned `belater status` into a multi-second wait.
func TestLoadDoesNotWaitForTheWriteLock(t *testing.T) {
	s := newTestStore(t)
	if err := s.Add(sampleTask()); err != nil {
		t.Fatal(err)
	}
	lf, err := os.OpenFile(filepath.Join(s.Dir, lockFile), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) })

	type result struct {
		tasks []*Task
		err   error
	}
	done := make(chan result, 1)
	go func() {
		tasks, err := s.Load()
		done <- result{tasks, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("Load with the write lock held: %v", r.err)
		}
		if len(r.tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(r.tasks))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Load waited for the write lock — status/list stall behind the scheduler")
	}
}

// TestLoadSeesCommittedWrites guards the claim that lock-free reads are safe:
// writers publish by rename, so a completed write is always visible.
func TestLoadSeesCommittedWrites(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 20; i++ {
		tk := sampleTask()
		tk.ID = NewID()
		if err := s.Add(tk); err != nil {
			t.Fatal(err)
		}
		tasks, err := s.Load()
		if err != nil {
			t.Fatalf("Load after Add %d: %v", i, err)
		}
		if len(tasks) != i+1 {
			t.Fatalf("after %d adds Load returned %d tasks", i+1, len(tasks))
		}
	}
}
