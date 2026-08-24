package daemon

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"belater/internal/queue"
)

func newTestStore(t *testing.T) *queue.Store {
	t.Helper()
	s, err := queue.Open(filepath.Join(t.TempDir(), "belater"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNextEvery(t *testing.T) {
	loc := time.FixedZone("T", 0)
	base := time.Date(2026, 8, 24, 10, 0, 0, 0, loc) // Monday 10:00
	cases := []struct {
		name  string
		every string
		at    time.Time
		want  string
	}{
		{"later today", "14:30", base, "2026-08-24 14:30"},
		{"earlier today goes tomorrow", "08:00", base, "2026-08-25 08:00"},
		{"same minute goes tomorrow", "10:00", base, "2026-08-25 10:00"},
		{"midnight", "00:00", base, "2026-08-25 00:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := nextEvery(c.at, c.every)
			want, _ := time.ParseInLocation("2006-01-02 15:04", c.want, loc)
			if !got.Equal(want) {
				t.Errorf("nextEvery(%s, %s) = %v, want %v", c.at, c.every, got, want)
			}
		})
	}
	if got := nextEvery(base, "25:00"); !got.IsZero() {
		t.Errorf("invalid hour should be zero, got %v", got)
	}
	if got := nextEvery(base, "abc"); !got.IsZero() {
		t.Errorf("invalid format should be zero, got %v", got)
	}
}

func TestRecoverInterrupted(t *testing.T) {
	s := newTestStore(t)
	tk := &queue.Task{ID: "aa000001", Name: "crashed", Command: "true", Status: queue.Running, LastExit: -1}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	ok := &queue.Task{ID: "bb000002", Name: "fine", Command: "true", Status: queue.Queued, LastExit: -1}
	if err := s.Add(ok); err != nil {
		t.Fatal(err)
	}
	logger := log.New(io.Discard, "", 0)
	if err := recoverInterrupted(s, logger); err != nil {
		t.Fatal(err)
	}
	got, err := s.Find("aa000001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != queue.Queued {
		t.Errorf("interrupted task status = %s, want queued", got.Status)
	}
	got2, _ := s.Find("bb000002")
	if got2.Status != queue.Queued {
		t.Errorf("healthy task touched: %s", got2.Status)
	}
}

func TestDispatchRunsDueTask(t *testing.T) {
	s := newTestStore(t)
	tk := &queue.Task{
		ID: "cc000003", Name: "due", Command: "echo dispatched-ok",
		RunAt: time.Now().Add(-time.Minute), Status: queue.Queued, LastExit: -1,
	}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	future := &queue.Task{
		ID: "dd000004", Name: "future", Command: "echo nope",
		RunAt: time.Now().Add(time.Hour), Status: queue.Queued, LastExit: -1,
	}
	if err := s.Add(future); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	logger := log.New(io.Discard, "", 0)
	dispatchOnce(s, 1, &wg, logger)
	// Wait up to 5s for completion.
	waitRunning(&wg, 5*time.Second)

	got, err := s.Find("cc000003")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != queue.Done || got.LastExit != 0 {
		t.Errorf("due task outcome: status=%s exit=%d", got.Status, got.LastExit)
	}
	got2, _ := s.Find("dd000004")
	if got2.Status != queue.Queued {
		t.Errorf("future task should stay queued, got %s", got2.Status)
	}
	// Log file must contain the output.
	data, err := os.ReadFile(s.LogPath("cc000003"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "dispatched-ok") {
		t.Errorf("log missing output: %q", data)
	}
}

func TestDispatchRetry(t *testing.T) {
	s := newTestStore(t)
	tk := &queue.Task{
		ID: "ee000005", Name: "flaky", Command: "exit 1",
		RunAt: time.Now().Add(-time.Minute), Status: queue.Queued, Retries: 2, LastExit: -1,
	}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	logger := log.New(io.Discard, "", 0)
	dispatchOnce(s, 1, &wg, logger)
	waitRunning(&wg, 5*time.Second)

	got, err := s.Find("ee000005")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != queue.Queued {
		t.Errorf("failed task should retry, got %s", got.Status)
	}
	if got.Runs != 1 {
		t.Errorf("runs = %d, want 1", got.Runs)
	}
	if !got.RunAt.After(time.Now()) {
		t.Errorf("retry should be scheduled in the future, got %v", got.RunAt)
	}
}

func TestGCRemovesOldFinished(t *testing.T) {
	s := newTestStore(t)
	old := &queue.Task{
		ID: "ff000006", Name: "old", Command: "true", Status: queue.Done,
		CreatedAt: time.Now().Add(-8 * 24 * time.Hour), LastExit: 0,
	}
	if err := s.Add(old); err != nil {
		t.Fatal(err)
	}
	fresh := &queue.Task{
		ID: "11000007", Name: "fresh", Command: "true", Status: queue.Done,
		CreatedAt: time.Now(), LastExit: 0,
	}
	if err := s.Add(fresh); err != nil {
		t.Fatal(err)
	}
	queued := &queue.Task{
		ID: "22000008", Name: "queued", Command: "true", Status: queue.Queued,
		CreatedAt: time.Now().Add(-8 * 24 * time.Hour), LastExit: -1,
	}
	if err := s.Add(queued); err != nil {
		t.Fatal(err)
	}
	logger := log.New(io.Discard, "", 0)
	tasks, _ := s.Load()
	if err := gc(s, tasks, logger); err != nil {
		t.Fatal(err)
	}
	tasks, _ = s.Load()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks after gc, got %d", len(tasks))
	}
	for _, tk := range tasks {
		if tk.ID == "ff000006" {
			t.Error("old finished task survived gc")
		}
	}
}

func TestFinishTaskEveryReschedules(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	tk := &queue.Task{
		ID: "99000009", Name: "daily", Command: "true",
		Every: "02:00", MaxRuns: 3, Status: queue.Running,
		CreatedAt: now, LastExit: -1,
	}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	logger := log.New(io.Discard, "", 0)
	finishTask(s, tk, logger)

	got, err := s.Find("99000009")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != queue.Queued {
		t.Errorf("status = %s, want queued (rescheduled)", got.Status)
	}
	if got.Runs != 1 {
		t.Errorf("runs = %d, want 1", got.Runs)
	}
	// Next 02:00 must be strictly after the finish time.
	if !got.RunAt.After(now) {
		t.Errorf("RunAt = %v, want after %v", got.RunAt, now)
	}
	if got.RunAt.Hour() != 2 || got.RunAt.Minute() != 0 {
		t.Errorf("RunAt = %v, want 02:00", got.RunAt)
	}
}

func TestFinishTaskMaxRunsStops(t *testing.T) {
	s := newTestStore(t)
	tk := &queue.Task{
		ID: "aa00000a", Name: "daily", Command: "true",
		Every: "02:00", MaxRuns: 1, Status: queue.Running,
		CreatedAt: time.Now(), LastExit: -1,
	}
	if err := s.Add(tk); err != nil {
		t.Fatal(err)
	}
	logger := log.New(io.Discard, "", 0)
	finishTask(s, tk, logger)

	got, _ := s.Find("aa00000a")
	if got.Status != queue.Done {
		t.Errorf("status = %s, want done after max runs", got.Status)
	}
}

func readAll(path string) (string, error) {
	data, err := os.ReadFile(path)
	return string(data), err
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
