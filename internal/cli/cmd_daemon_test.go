package cli

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"belater/internal/queue"
)

// holdInstanceLock acquires the daemon.lock flock exactly like a running
// scheduler does, so InstanceLockHeld reports true.
func holdInstanceLock(t *testing.T, dir string) func() {
	t.Helper()
	lf, err := os.OpenFile(filepath.Join(dir, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lf.Close()
		t.Fatal(err)
	}
	return func() {
		syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)
		lf.Close()
	}
}

// TestLockActiveHintIsNotAnError guards the wording: a running scheduler is a
// normal state, so the hint must not look like a failure.
func TestLockActiveHintIsNotAnError(t *testing.T) {
	hint := lockActiveHint()
	if strings.Contains(hint, "错误") {
		t.Errorf("lockActiveHint() must not contain 错误, got: %s", hint)
	}
	for _, want := range []string{"已在运行", "belater status"} {
		if !strings.Contains(hint, want) {
			t.Errorf("lockActiveHint() should mention %q: %s", want, hint)
		}
	}
}

// TestDaemonLockConflictExitsZero: when another scheduler already holds the
// instance lock, both --once and foreground daemon must exit 0 (the queue is
// being serviced) instead of failing.
func TestDaemonLockConflictExitsZero(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BELATER_STATE_DIR", dir)
	st, err := queue.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	release := holdInstanceLock(t, dir)
	defer release()

	if code := runOnce(st, time.Second, 1); code != 0 {
		t.Errorf("runOnce with held lock: exit %d, want 0 (existing scheduler services the queue)", code)
	}
	if code := runForever(st, time.Second, 1); code != 0 {
		t.Errorf("runForever with held lock: exit %d, want 0 (existing scheduler services the queue)", code)
	}
}
